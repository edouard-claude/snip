package hook

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
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
func Run(r io.Reader, w io.Writer, commands []string, snipBin string) error {
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

	// Extract first segment (up to unquoted ; | & or newline).
	firstLine := ti.Command
	if idx := strings.IndexByte(firstLine, '\n'); idx >= 0 {
		firstLine = firstLine[:idx]
	}
	firstSegment := ExtractFirstSegment(firstLine)

	prefix, envVars, bareCmd := ParseSegment(firstSegment)
	base := BaseCommand(bareCmd)

	// Skip if already rewritten (starts with snip path).
	quotedBin := fmt.Sprintf("%q", snipBin)
	if base == quotedBin || base == snipBin || strings.HasPrefix(strings.TrimLeft(bareCmd, " \t"), quotedBin) || strings.HasPrefix(strings.TrimLeft(bareCmd, " \t"), snipBin) {
		return nil
	}

	// Check if base command is supported.
	cmdSet := make(map[string]struct{}, len(commands))
	for _, c := range commands {
		cmdSet[c] = struct{}{}
	}
	if _, ok := cmdSet[base]; !ok {
		return nil
	}

	// Build rewritten command.
	rest := ti.Command[len(firstSegment):]
	rewritten := prefix + envVars + quotedBin + " -- " + bareCmd + rest

	// Preserve all original tool_input fields, replacing command.
	var originalInput map[string]any
	if err := json.Unmarshal(input.ToolInput, &originalInput); err != nil {
		return nil
	}
	originalInput["command"] = rewritten

	output := map[string]any{
		"hookSpecificOutput": map[string]any{
			"hookEventName":            "PreToolUse",
			"permissionDecision":       "allow",
			"permissionDecisionReason": "snip auto-rewrite",
			"updatedInput":             originalInput,
		},
	}

	enc := json.NewEncoder(w)
	return enc.Encode(output)
}
