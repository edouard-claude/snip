package hook

import (
	"fmt"
	"strings"
)

// snipSuggestion is the outcome of analyzing a command for a deny-and-steer
// re-run suggestion, used by Grok Build because its PreToolUse hook cannot
// rewrite the command in place.
type snipSuggestion struct {
	// command is the full suggested re-run command; empty when no snip filter
	// matched.
	command string
	// base is the base command of the first segment, for audit events.
	base string
	// alreadySnip is true when the command is already wrapped through snip.
	alreadySnip bool
}

// suggestSnipRerun builds the "re-run through snip" suggestion for command.
// Only the first runnable segment is inspected; any remainder (further
// segments, extra lines) is carried into the suggestion verbatim.
func suggestSnipRerun(command string, cmdSet map[string]struct{}, prefixes []TransparentPrefix, snipBin string) snipSuggestion {
	firstLine := command
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
		return snipSuggestion{base: base, alreadySnip: true}
	}

	restOfCmd := command[len(firstSegment):]

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
			return snipSuggestion{base: base}
		}
		suggested = prefix + envVars + quotedBin + " run -- " + bareCmd + restOfCmd
	}

	return snipSuggestion{command: suggested, base: base}
}
