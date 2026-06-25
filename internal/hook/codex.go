package hook

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/edouard-claude/snip/internal/hookaudit"
)

// codexAgent is the value written to hookaudit.Event.Agent for Codex events.
const codexAgent = "codex"

// RunCodex reads a Codex PreToolUse JSON payload from r, determines if the
// command matches a snip filter, and writes a deny-with-suggestion response
// telling Codex (and the user) to re-run the command through snip.
//
// Codex's PreToolUse hook cannot rewrite the command in place — only "deny"
// with a free-form reason is honored. See openai/codex#18491. When that
// limitation is lifted, this function can return updatedInput like Run does.
//
// Always returns nil; the caller must exit 0 (graceful degradation).
func RunCodex(r io.Reader, w io.Writer, commands []string, prefixes []TransparentPrefix, snipBin string) error {
	audit := hookaudit.Enabled()

	data, err := io.ReadAll(r)
	if err != nil {
		return fmt.Errorf("read stdin: %w", err)
	}

	var input hookInput
	if err := json.Unmarshal(data, &input); err != nil {
		return nil
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

	firstLine := ti.Command
	if idx := strings.IndexByte(firstLine, '\n'); idx >= 0 {
		firstLine = firstLine[:idx]
	}
	firstSegment := ExtractFirstSegment(firstLine)

	prefix, envVars, bareCmd := ParseSegment(firstSegment)
	base := BaseCommand(bareCmd)

	quotedBin := fmt.Sprintf("%q", snipBin)
	trimmed := strings.TrimLeft(bareCmd, " \t")
	if base == quotedBin || base == snipBin ||
		strings.HasPrefix(trimmed, quotedBin) || strings.HasPrefix(trimmed, snipBin) {
		return nil
	}

	cmdSet := make(map[string]struct{}, len(commands))
	for _, c := range commands {
		cmdSet[c] = struct{}{}
	}

	restOfCmd := ti.Command[len(firstSegment):]

	// Transparent runner prefix (e.g. "uv run pytest"): suggest wrapping the
	// inner command so its filter applies, leaving the prefix in place. Only when
	// a known inner command is located; otherwise fall back to the plain base
	// check below.
	var suggested string
	if tp, restAfter, ok := matchTransparentPrefix(bareCmd, prefixes); ok {
		if before, _, found := LocateInner(restAfter, cmdSet, tp.ValueFlags, tp.SkipFlags); found {
			suggested = prefix + envVars + tp.Prefix + " " + before + quotedBin + " run -- " + restAfter[len(before):] + restOfCmd
		}
	}

	if suggested == "" {
		if _, ok := cmdSet[base]; !ok {
			if audit {
				hookaudit.Append(hookaudit.Event{
					Timestamp: time.Now().UTC(),
					Command:   ti.Command,
					Base:      base,
					Matched:   false,
					Rewritten: false,
					Agent:     codexAgent,
				})
			}
			return nil
		}
		suggested = prefix + envVars + quotedBin + " run -- " + bareCmd + restOfCmd
	}

	reason := fmt.Sprintf("snip can filter this command. Re-run as: %s", suggested)

	output := map[string]any{
		"hookSpecificOutput": map[string]any{
			"hookEventName":            "PreToolUse",
			"permissionDecision":       "deny",
			"permissionDecisionReason": reason,
		},
	}

	if audit {
		hookaudit.Append(hookaudit.Event{
			Timestamp: time.Now().UTC(),
			Command:   ti.Command,
			Base:      base,
			Matched:   true,
			Rewritten: false,
			Agent:     codexAgent,
		})
	}

	enc := json.NewEncoder(w)
	return enc.Encode(output)
}
