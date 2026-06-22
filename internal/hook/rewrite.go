package hook

import (
	"fmt"
	"strings"
)

// RewriteResult is the outcome of segmenting and rewriting a compound command.
type RewriteResult struct {
	// Command is the rewritten command line.
	Command string
	// Changed reports whether at least one segment was wrapped in snip. When
	// false the caller should pass the original command through unchanged.
	Changed bool
	// AllKnown reports whether every runnable command segment is a known base
	// command (and thus attested by snip). Only when AllKnown is true may the
	// caller emit permissionDecision "allow": otherwise an uninspected segment
	// would have its confirmation prompt suppressed. See issue #88.
	AllKnown bool
}

// RewriteCommand rewrites each runnable segment of cmd whose base command is in
// cmdSet by wrapping it in `<snipBin> run -- ...`, mirroring rtk's per-segment
// rewrite so token savings are preserved across compound commands.
//
// cmd is split on top-level ';', '&&', '||', '&' and newline boundaries (quoted
// regions are respected). Within each resulting group only the first pipeline
// stage (the text before the first top-level '|') is eligible for rewriting,
// matching snip's line-by-line stream-filtering model: snip filters the
// producer, later pipe stages consume the filtered output untouched.
//
// The caller must reject commands containing unverifiable constructs
// (HasUnverifiableConstruct) before calling this, so cmd here is free of command
// substitution and carriage returns.
func RewriteCommand(cmd string, cmdSet map[string]struct{}, snipBin string) RewriteResult {
	quotedBin := fmt.Sprintf("%q", snipBin)

	var b strings.Builder
	b.Grow(len(cmd) + 32)

	changed := false
	allKnown := true

	flush := func(group string) {
		out, headKnown, hasTail := rewriteGroup(group, cmdSet, quotedBin, snipBin)
		b.WriteString(out)
		if out != group {
			changed = true
		}
		// Empty/whitespace-only groups (e.g. a trailing ';') do not carry a
		// command and never block auto-allow.
		if strings.TrimSpace(group) != "" && (!headKnown || hasTail) {
			allKnown = false
		}
	}

	groupStart := 0
	var quote byte
	for i := 0; i < len(cmd); {
		ch := cmd[i]
		if quote != 0 {
			if ch == '\\' && quote == '"' && i+1 < len(cmd) {
				i += 2
				continue
			}
			if ch == quote {
				quote = 0
			}
			i++
			continue
		}
		switch ch {
		case '\'':
			quote = '\''
			i++
		case '"':
			quote = '"'
			i++
		case '\n', ';':
			flush(cmd[groupStart:i])
			b.WriteByte(ch)
			i++
			groupStart = i
		case '&':
			flush(cmd[groupStart:i])
			if i+1 < len(cmd) && cmd[i+1] == '&' {
				b.WriteString("&&")
				i += 2
			} else {
				b.WriteByte('&')
				i++
			}
			groupStart = i
		case '|':
			if i+1 < len(cmd) && cmd[i+1] == '|' {
				// "||" is a group boundary; a single "|" stays inside the group.
				flush(cmd[groupStart:i])
				b.WriteString("||")
				i += 2
				groupStart = i
			} else {
				i++
			}
		default:
			i++
		}
	}
	flush(cmd[groupStart:])

	return RewriteResult{Command: b.String(), Changed: changed, AllKnown: allKnown}
}

// rewriteGroup rewrites the head command of a single execution group (the text
// between two sequential boundaries). It returns the rewritten group, whether
// the head is a known/attested base command, and whether the group has a
// non-empty pipeline tail (extra stages that were left uninspected).
func rewriteGroup(group string, cmdSet map[string]struct{}, quotedBin, snipBin string) (out string, headKnown, hasTail bool) {
	head, tail := splitFirstPipe(group)
	hasTail = strings.TrimSpace(tail) != ""

	prefix, envVars, bareCmd := ParseSegment(head)
	base := BaseCommand(bareCmd)
	if base == "" {
		return group, false, hasTail
	}

	// Already wrapped in snip: treat as attested, leave untouched.
	trimmed := strings.TrimLeft(bareCmd, " \t")
	if base == quotedBin || base == snipBin ||
		strings.HasPrefix(trimmed, quotedBin) || strings.HasPrefix(trimmed, snipBin) {
		return group, true, hasTail
	}

	if _, ok := cmdSet[base]; !ok {
		return group, false, hasTail
	}

	wrappedHead := prefix + envVars + quotedBin + " run -- " + bareCmd
	return wrappedHead + tail, true, hasTail
}

// splitFirstPipe splits group at its first top-level '|' (quoted regions are
// respected). head is the text before the pipe; tail includes the '|' and the
// remaining pipeline stages. When there is no pipe, tail is empty.
func splitFirstPipe(group string) (head, tail string) {
	var quote byte
	for i := 0; i < len(group); i++ {
		ch := group[i]
		if quote != 0 {
			if ch == '\\' && quote == '"' && i+1 < len(group) {
				i++
				continue
			}
			if ch == quote {
				quote = 0
			}
			continue
		}
		switch ch {
		case '\'':
			quote = '\''
		case '"':
			quote = '"'
		case '|':
			return group[:i], group[i:]
		}
	}
	return group, ""
}

// firstBase returns the base command of cmd's first segment, used for audit
// telemetry that predates per-segment rewriting.
func firstBase(cmd string) string {
	firstLine := cmd
	if idx := strings.IndexByte(firstLine, '\n'); idx >= 0 {
		firstLine = firstLine[:idx]
	}
	_, _, bareCmd := ParseSegment(ExtractFirstSegment(firstLine))
	return BaseCommand(bareCmd)
}
