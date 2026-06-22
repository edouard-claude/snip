package hook

import (
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/edouard-claude/snip/internal/hookaudit"
)

// piAgent is the value written to hookaudit.Event.Agent for Pi events.
const piAgent = "pi"

// piToolName is the bash tool name emitted by the Pi coding agent (pi.dev).
// Pi uses lowercase "bash" whereas Claude Code uses "Bash".
const piToolName = "bash"

// RunPi reads a Pi PreToolUse JSON payload from r, determines if the command
// should be rewritten through snip, and writes the rewrite JSON to w. If no
// rewrite is needed, nothing is written (pass-through).
//
// Pi natively exposes hooks via a TypeScript event system; runtime hook
// commands defined in settings.json require the community extension
// @hsingjui/pi-hooks, which mirrors Claude Code's hookSpecificOutput format
// (including updatedInput). RunPi therefore emits the same response shape as
// Run, only changing the expected tool_name from "Bash" to "bash".
//
// Returns nil on success. Errors are returned but the caller should always
// exit 0 (graceful degradation).
func RunPi(r io.Reader, w io.Writer, commands []string, snipBin string) error {
	audit := hookaudit.Enabled()

	data, err := io.ReadAll(r)
	if err != nil {
		return fmt.Errorf("read stdin: %w", err)
	}

	var input hookInput
	if err := json.Unmarshal(data, &input); err != nil {
		return nil
	}

	if input.ToolName != piToolName {
		return nil
	}

	var ti toolInput
	if err := json.Unmarshal(input.ToolInput, &ti); err != nil {
		return nil
	}
	if ti.Command == "" {
		return nil
	}

	// Commands with a command substitution or carriage return cannot be safely
	// segmented or attested; pass through unchanged (#88).
	if HasUnverifiableConstruct(ti.Command) {
		return nil
	}

	cmdSet := make(map[string]struct{}, len(commands))
	for _, c := range commands {
		cmdSet[c] = struct{}{}
	}

	res := RewriteCommand(ti.Command, cmdSet, snipBin)
	if !res.Changed {
		if audit {
			base := firstBase(ti.Command)
			_, matched := cmdSet[base]
			hookaudit.Append(hookaudit.Event{
				Timestamp: time.Now().UTC(),
				Command:   ti.Command,
				Base:      base,
				Matched:   matched,
				Rewritten: false,
				Agent:     piAgent,
			})
		}
		return nil
	}

	var originalInput map[string]any
	if err := json.Unmarshal(input.ToolInput, &originalInput); err != nil {
		return nil
	}
	originalInput["command"] = res.Command

	hookOutput := map[string]any{
		"hookEventName": "PreToolUse",
		"updatedInput":  originalInput,
	}
	// Only auto-allow when every runnable segment is a snip-supported command;
	// otherwise rewrite for savings but defer the decision so the user is
	// prompted for the uninspected segment (#88).
	if res.AllKnown {
		hookOutput["permissionDecision"] = "allow"
		hookOutput["permissionDecisionReason"] = "snip auto-rewrite"
	}

	output := map[string]any{"hookSpecificOutput": hookOutput}

	if audit {
		hookaudit.Append(hookaudit.Event{
			Timestamp: time.Now().UTC(),
			Command:   ti.Command,
			Base:      firstBase(ti.Command),
			Matched:   true,
			Rewritten: true,
			Agent:     piAgent,
		})
	}

	enc := json.NewEncoder(w)
	return enc.Encode(output)
}
