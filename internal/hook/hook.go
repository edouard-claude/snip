package hook

import (
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/edouard-claude/snip/internal/hookaudit"
)

// hookInput represents the JSON payload from Claude Code PreToolUse.
type hookInput struct {
	ToolName  string          `json:"tool_name"`
	ToolInput json.RawMessage `json:"tool_input"`
}

// toolInput holds the command field from tool_input.
type toolInput struct {
	Command string `json:"command"`
}

// Run reads a Claude Code PreToolUse JSON payload from r, determines if the
// command should be rewritten through snip, and writes the rewrite JSON to w.
// If no rewrite is needed, nothing is written (pass-through).
//
// commands is the list of supported base command names from the filter registry.
// snipBin is the absolute path to the snip binary.
//
// Returns nil on success. Errors are returned but the caller should always
// exit 0 (graceful degradation).
func Run(r io.Reader, w io.Writer, commands []string, prefixes []TransparentPrefix, snipBin string) error {
	audit := hookaudit.Enabled()

	data, err := io.ReadAll(r)
	if err != nil {
		return fmt.Errorf("read stdin: %w", err)
	}

	var input hookInput
	if err := json.Unmarshal(data, &input); err != nil {
		return nil // malformed JSON: pass through silently
	}

	if input.ToolName != "Bash" {
		return nil
	}

	var ti toolInput
	if err := json.Unmarshal(input.ToolInput, &ti); err != nil {
		return nil
	}
	if ti.Command == "" {
		return nil
	}

	// Commands containing a command substitution ($(...) or backticks) or a
	// carriage return cannot be safely segmented or attested: the substituted
	// content executes without ever being inspected. Pass through unchanged so
	// Claude Code's confirmation prompt still fires (#88).
	if HasUnverifiableConstruct(ti.Command) {
		return nil
	}

	cmdSet := make(map[string]struct{}, len(commands))
	for _, c := range commands {
		cmdSet[c] = struct{}{}
	}

	// Rewrite every runnable segment whose base command snip supports.
	res := RewriteCommand(ti.Command, cmdSet, prefixes, snipBin)
	if !res.Changed {
		// Audit: nothing matched (or already rewritten).
		if audit {
			base := firstBase(ti.Command)
			_, matched := cmdSet[base]
			hookaudit.Append(hookaudit.Event{
				Timestamp: time.Now().UTC(),
				Command:   ti.Command,
				Base:      base,
				Matched:   matched,
				Rewritten: false,
			})
		}
		return nil
	}

	// Preserve all original tool_input fields, replacing command.
	var originalInput map[string]any
	if err := json.Unmarshal(input.ToolInput, &originalInput); err != nil {
		return nil
	}
	originalInput["command"] = res.Command

	hookOutput := map[string]any{
		"hookEventName": "PreToolUse",
		"updatedInput":  originalInput,
	}
	// Only auto-allow when every runnable segment is a snip-supported command.
	// If any segment is unknown (e.g. a trailing `&& curl ... | sh`), the
	// command is still rewritten for token savings but the decision is left to
	// Claude Code so the user is prompted for the uninspected segment (#88).
	if res.AllKnown {
		hookOutput["permissionDecision"] = "allow"
		hookOutput["permissionDecisionReason"] = "snip auto-rewrite"
	}

	output := map[string]any{"hookSpecificOutput": hookOutput}

	// Audit: command matched and rewritten.
	if audit {
		hookaudit.Append(hookaudit.Event{
			Timestamp: time.Now().UTC(),
			Command:   ti.Command,
			Base:      firstBase(ti.Command),
			Matched:   true,
			Rewritten: true,
		})
	}

	enc := json.NewEncoder(w)
	return enc.Encode(output)
}
