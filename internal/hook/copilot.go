package hook

import (
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/edouard-claude/snip/internal/hookaudit"
)

// copilotAgent is the value written to hookaudit.Event.Agent for Copilot events.
const copilotAgent = "copilot"

// copilotInput represents the PreToolUse payload from GitHub Copilot CLI or the
// VS Code agent hook surface. Two input shapes are accepted:
//
//   - VS Code agent hooks (Claude-style, snake_case) carry the command in a
//     tool_input object, only with a non-Bash tool name (e.g. run_in_terminal):
//     {"tool_name":"run_in_terminal","tool_input":{"command":"git status"}}
//   - GitHub Copilot CLI carries the command in toolArgs. The hooks reference
//     types toolArgs as an object ({"command":"git status"}); the legacy bash
//     bridge encoded it as a JSON string. Both are accepted.
//
// RunCopilot is tool-name agnostic: any payload carrying a command (in either
// shape) is eligible; payloads without one pass through untouched.
type copilotInput struct {
	ToolInput json.RawMessage `json:"tool_input"`
	ToolArgs  json.RawMessage `json:"toolArgs"`
}

// RunCopilot reads a GitHub Copilot CLI / VS Code PreToolUse payload from r,
// rewrites every snip-supported command segment, and writes the response in the
// envelope the detected host expects:
//
//   - VS Code object shape (tool_input) -> Claude Code's hookSpecificOutput
//     envelope, where the rewrite lives in hookSpecificOutput.updatedInput.
//   - GitHub Copilot CLI shape (toolArgs) -> a flat envelope where the rewrite
//     lives in modifiedArgs (the field the Copilot hooks reference defines):
//     {"permissionDecision":"allow","permissionDecisionReason":"snip auto-rewrite",
//     "modifiedArgs":{"command":"…/snip run -- git status"}}
//
// permissionDecision is emitted only when every runnable segment is attested
// (AllKnown). When an uninspected segment is present the command is still
// rewritten for token savings but the decision is omitted so the host still
// prompts the user (#88). If no rewrite is needed, nothing is written.
//
// Returns nil on success. Errors are returned but the caller should always
// exit 0 (graceful degradation).
func RunCopilot(r io.Reader, w io.Writer, commands []string, prefixes []TransparentPrefix, snipBin string) error {
	audit := hookaudit.Enabled()

	data, err := io.ReadAll(r)
	if err != nil {
		return fmt.Errorf("read stdin: %w", err)
	}

	var input copilotInput
	if err := json.Unmarshal(data, &input); err != nil {
		return nil // malformed JSON: pass through silently
	}

	// Resolve the command and the full input object it lives in, accepting both
	// the VS Code object shape (tool_input) and the Copilot CLI shape (toolArgs).
	// objectShape reports which envelope the host expects in return (see below).
	command, originalInput, objectShape, ok := copilotCommand(input)
	if !ok || command == "" {
		return nil
	}

	// Commands with a command substitution or carriage return cannot be safely
	// segmented or attested; pass through unchanged (#88).
	if HasUnverifiableConstruct(command) {
		return nil
	}

	cmdSet := make(map[string]struct{}, len(commands))
	for _, c := range commands {
		cmdSet[c] = struct{}{}
	}

	res := RewriteCommand(command, cmdSet, prefixes, snipBin)
	if !res.Changed {
		if audit {
			base := firstBase(command)
			_, matched := cmdSet[base]
			hookaudit.Append(hookaudit.Event{
				Timestamp: time.Now().UTC(),
				Command:   command,
				Base:      base,
				Matched:   matched,
				Rewritten: false,
				Agent:     copilotAgent,
			})
		}
		return nil
	}

	// Preserve any extra input fields (e.g. cwd), replacing command.
	originalInput["command"] = res.Command

	// Emit the envelope the detected host expects: the VS Code object shape
	// (tool_input) consumes Claude Code's hookSpecificOutput envelope, whereas
	// the GitHub Copilot CLI shape (toolArgs) consumes a flat object whose
	// rewrite lives in modifiedArgs. Only auto-allow when every runnable segment
	// is a snip-supported command; otherwise rewrite for savings but omit the
	// decision so the host still prompts the user for the uninspected segment (#88).
	var output map[string]any
	if objectShape {
		hookOutput := map[string]any{
			"hookEventName": "PreToolUse",
			"updatedInput":  originalInput,
		}
		if res.AllKnown {
			hookOutput["permissionDecision"] = "allow"
			hookOutput["permissionDecisionReason"] = "snip auto-rewrite"
		}
		output = map[string]any{"hookSpecificOutput": hookOutput}
	} else {
		output = map[string]any{"modifiedArgs": originalInput}
		if res.AllKnown {
			output["permissionDecision"] = "allow"
			output["permissionDecisionReason"] = "snip auto-rewrite"
		}
	}

	if audit {
		hookaudit.Append(hookaudit.Event{
			Timestamp: time.Now().UTC(),
			Command:   command,
			Base:      firstBase(command),
			Matched:   true,
			Rewritten: true,
			Agent:     copilotAgent,
		})
	}

	enc := json.NewEncoder(w)
	return enc.Encode(output)
}

// copilotCommand extracts the command string and the mutable input object it
// belongs to from either accepted shape. The returned map is the object whose
// "command" field the caller overwrites with the rewrite, so extra fields are
// preserved in the response. objectShape is true when the command came from the
// VS Code tool_input object (which expects the hookSpecificOutput envelope) and
// false when it came from the Copilot CLI toolArgs (flat envelope). ok is false
// when neither shape carries a command.
func copilotCommand(input copilotInput) (command string, originalInput map[string]any, objectShape, ok bool) {
	// VS Code object shape: tool_input.{command}.
	if m, decoded := decodeObject(input.ToolInput); decoded {
		if cmd, _ := m["command"].(string); cmd != "" {
			return cmd, m, true, true
		}
	}

	// Copilot CLI shape: toolArgs is an object (per the hooks reference) or a
	// JSON-encoded string wrapping that object (the legacy bridge encoding).
	if m, decoded := decodeToolArgs(input.ToolArgs); decoded {
		if cmd, _ := m["command"].(string); cmd != "" {
			return cmd, m, false, true
		}
	}

	return "", nil, false, false
}

// decodeObject unmarshals raw as a JSON object. Returns ok=false when raw is
// empty, null, or not a JSON object (e.g. a string or array).
func decodeObject(raw json.RawMessage) (map[string]any, bool) {
	if len(raw) == 0 {
		return nil, false
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil || m == nil {
		return nil, false
	}
	return m, true
}

// decodeToolArgs unmarshals toolArgs as a JSON object, or as a JSON-encoded
// string that itself wraps a JSON object (the legacy bash bridge shape).
func decodeToolArgs(raw json.RawMessage) (map[string]any, bool) {
	if m, ok := decodeObject(raw); ok {
		return m, true
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil && s != "" {
		return decodeObject(json.RawMessage(s))
	}
	return nil, false
}
