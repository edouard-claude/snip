package hook

import (
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/edouard-claude/snip/internal/hookaudit"
)

// antigravityHookInput represents the JSON payload from Antigravity PreToolUse.
type antigravityHookInput struct {
	ToolCall antigravityToolInput `json:"toolCall"`
}

// antigravityToolInput holds the command field from toolCall.
type antigravityToolInput struct {
	Name string              `json:"name"`
	Args antigravityToolArgs `json:"args"`
}

type antigravityToolArgs struct {
	CommandLine string `json:"CommandLine"`
}

// RunAntigravity reads a Antigravity PreToolUse JSON payload from r, determines if the
// command should be rewritten through snip, and writes the rewrite JSON to w.
// If no rewrite is needed, nothing is written (pass-through).
//
// commands is the list of supported base command names from the filter registry.
// snipBin is the absolute path to the snip binary.
//
// Returns nil on success. Errors are returned but the caller should always
// exit 0 (graceful degradation).
func RunAntigravity(r io.Reader, w io.Writer, commands []string, prefixes []TransparentPrefix, snipBin string) error {
	audit := hookaudit.Enabled()

	data, err := io.ReadAll(r)
	if err != nil {
		return fmt.Errorf("read stdin: %w", err)
	}

	var input antigravityHookInput
	if err := json.Unmarshal(data, &input); err != nil {
		return nil // malformed JSON: pass through silently
	}

	if input.ToolCall.Name != "run_command" {
		return nil
	}

	if input.ToolCall.Args.CommandLine == "" {
		return nil
	}

	// Commands with a command substitution or carriage return cannot be safely
	// segmented or attested; pass through unchanged (#88).
	if HasUnverifiableConstruct(input.ToolCall.Args.CommandLine) {
		return nil
	}

	cmdSet := make(map[string]struct{}, len(commands))
	for _, c := range commands {
		cmdSet[c] = struct{}{}
	}

	// Rewrite every runnable segment whose base command snip supports.
	res := RewriteCommand(input.ToolCall.Args.CommandLine, cmdSet, prefixes, snipBin)
	if !res.Changed {
		// Audit: nothing matched (or already rewritten).
		if audit {
			base := firstBase(input.ToolCall.Args.CommandLine)
			_, matched := cmdSet[base]
			hookaudit.Append(hookaudit.Event{
				Timestamp: time.Now().UTC(),
				Command:   input.ToolCall.Args.CommandLine,
				Base:      base,
				Matched:   matched,
				Rewritten: false,
			})
		}
		return nil
	}

	// Please refer to the documentation included with Antigravity for the output format.
	// ~./gemini/antigravity/builtin/skills/agy-customizations/docs/hooks.md
	overwriteInput := map[string]any{
		"CommandLine": res.Command,
	}

	hookOutput := map[string]any{
		"overwrite": overwriteInput,
	}
	// Prompt the user for permission (respects "Always Allow" cache).
	if res.AllKnown {
		hookOutput["decision"] = "ask"
		hookOutput["reason"] = "snip auto-rewrite"
	}

	// Audit: command matched and rewritten.
	if audit {
		hookaudit.Append(hookaudit.Event{
			Timestamp: time.Now().UTC(),
			Command:   input.ToolCall.Args.CommandLine,
			Base:      firstBase(input.ToolCall.Args.CommandLine),
			Matched:   true,
			Rewritten: true,
		})
	}

	enc := json.NewEncoder(w)
	return enc.Encode(hookOutput)
}
