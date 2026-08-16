package hook

import (
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
// regions and comments are respected: a comment runs to the end of its line and
// the shell reads no operator inside it). Within each resulting group the head
// command is wrapped only when its stdout is read directly by the LLM. A head
// whose output feeds a consumer — a pipe stage or a file redirection ('>') — is
// left raw, because snip's lossy compaction (path compaction, line truncation,
// match caps, dedup, reformatting) would silently corrupt the exact count and
// content that consumer depends on (issue #111).
//
// Two kinds of group are never runnable commands and are emitted verbatim
// (issue #133):
//
//   - Groups inside a compound command (loop, conditional, brace group, function
//     body, subshell), tracked by blockScope — see its doc comment for exactly
//     which openers and closers are recognised. The #111 guard is per-group, so
//     it could not see that `done | wc -l` consumes the loop body sitting in an
//     earlier group; the body was wrapped and the count silently wrong.
//   - Heredoc bodies, which are literal text the command writes, not commands to
//     run. Rewriting them corrupted the Makefile or script an agent was writing.
//     The group carrying the '<<' operator is left raw too, because `snip run`
//     never wires stdin to the child.
//
// Both also force AllKnown false: snip does not attest what it did not inspect
// (issue #88). The scope is per-group, so a block or a heredoc never disables
// filtering for unrelated top-level commands in the same message.
//
// The caller must reject commands containing unverifiable constructs
// (HasUnverifiableConstruct) before calling this, so cmd here is free of command
// substitution and carriage returns.
func RewriteCommand(cmd string, cmdSet map[string]struct{}, prefixes []TransparentPrefix, snipBin string) RewriteResult {
	quotedBin := quoteSnipBin(snipBin)

	var b strings.Builder
	b.Grow(len(cmd) + 32)

	changed := false
	allKnown := true

	var scope blockScope
	// Set while the group under construction carries a '<<' operator: its stdin
	// comes from a heredoc body, which snip run cannot forward.
	heredocGroup := false

	flush := func(group string) {
		if scope.inBlock() || heredocGroup {
			b.WriteString(group)
			if heredocGroup || strings.TrimSpace(group) != "" {
				allKnown = false
			}
		} else {
			out, headKnown, hasTail := rewriteGroup(group, cmdSet, prefixes, quotedBin, snipBin)
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
		scope.advance(group)
		heredocGroup = false
	}

	groupStart := 0
	var quote byte
	// Heredoc bodies opened on the line being scanned, drained at its newline.
	var pending []heredocDelim
	// Nesting of unquoted "((" arithmetic, where '<<' is a left shift and not a
	// heredoc operator. Arming a heredoc there would swallow the rest of the
	// command and silently disable filtering for it (issue #133): "((x = 1 << n))"
	// would open a heredoc on delimiter "n". Command substitution is rejected
	// upstream (HasUnverifiableConstruct), so "((" can only be an arithmetic
	// command here. Reset at every newline so an unbalanced "((" cannot disarm
	// heredoc detection for the rest of the command.
	arith := 0

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
		case '#':
			if isWordStart(cmd, i) {
				// A comment runs to the end of the line, so the shell reads no
				// operator inside it: a ';', '&&', '||' or '|' there is not a
				// group boundary and a '<<' there does not open a heredoc.
				// Splitting on them handed blockScope a group that starts in
				// command position but is really comment text, and a closing
				// reserved word in the comment ("# count matches; done below")
				// then closed a live block and the rest of its body was
				// rewritten. Skipping to the newline leaves the comment inside
				// its group; blockScope.advance skips it there in turn.
				if nl := strings.IndexByte(cmd[i:], '\n'); nl >= 0 {
					i += nl
				} else {
					i = len(cmd)
				}
				continue
			}
			i++
		case '<':
			// "<<" opens a heredoc; "<<<" is a here-string, consumed whole so its
			// operand is not read as a delimiter.
			if i+1 < len(cmd) && cmd[i+1] == '<' {
				if i+2 < len(cmd) && cmd[i+2] == '<' {
					i += 3
					continue
				}
				if arith == 0 {
					if d, next, ok := parseHeredocDelim(cmd, i+2); ok {
						pending = append(pending, d)
						heredocGroup = true
						i = next
						continue
					}
				}
			}
			i++
		case '(':
			if i+1 < len(cmd) && cmd[i+1] == '(' {
				arith++
				i += 2
				continue
			}
			i++
		case ')':
			if arith > 0 && i+1 < len(cmd) && cmd[i+1] == ')' {
				arith--
				i += 2
				continue
			}
			i++
		case '\n':
			// A trailing unquoted backslash escapes the newline: the shell reads
			// one command across the two lines, so this is not a group boundary.
			// Splitting here put a trailing consumer in a different group from
			// the head, the #111 guard never saw it, and the head was rewritten
			// while the consumer read snip's compacted output (issue #138).
			// Pending heredocs keep the old behaviour: their bodies start at the
			// next line and must still be drained here.
			if len(pending) == 0 && escapesNewline(cmd, i) {
				i++
				continue
			}
			flush(cmd[groupStart:i])
			b.WriteByte('\n')
			i++
			groupStart = i
			arith = 0
			if len(pending) > 0 {
				end := drainHeredocs(cmd, i, pending)
				b.WriteString(cmd[i:end])
				pending = pending[:0]
				i = end
				groupStart = i
			}
		case ';':
			flush(cmd[groupStart:i])
			b.WriteByte(ch)
			i++
			// ";;" ends a case arm, as do its ";&" and ";;&" variants. What
			// follows is the next arm's pattern rather than a command, and
			// blockScope cannot see it: ';' is a group boundary, so advance is
			// never handed one (issue #138).
			if i < len(cmd) && (cmd[i] == ';' || cmd[i] == '&') {
				scope.caseArmEnd()
			}
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
func rewriteGroup(group string, cmdSet map[string]struct{}, prefixes []TransparentPrefix, quotedBin, snipBin string) (out string, headKnown, hasTail bool) {
	head, tail := splitFirstPipe(group)
	hasTail = strings.TrimSpace(tail) != ""

	// A head whose stdout feeds a downstream consumer must not be wrapped: snip's
	// lossy compaction (compact_path, truncate_lines, head caps, dedup,
	// reformatting) is only safe when the LLM reads the output directly. A pipe
	// stage or a file redirection consumes the producer's exact count and
	// content, so wrapping would silently corrupt it (issue #111). splitFirstPipe
	// detects the pipe; hasTopLevelRedirect detects '>' (covering '>', '>>', '2>',
	// '&>'). For the pipe case hasTail also keeps the #88 no-auto-allow guard.
	feedsConsumer := hasTail || hasTopLevelRedirect(head)

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

	// Transparent runner prefix (e.g. "uv run pytest"): strip the prefix, locate
	// the inner command, and wrap the inner command so its filter applies. The
	// prefix is re-prepended unchanged so the wrapped snip still runs inside the
	// runner's environment. Attempted only when the inner command is a known snip
	// command; otherwise fall through to the normal base check below. Because the
	// inner base must be in cmdSet, a runner executing an unknown program
	// (e.g. "uv run bash -c ...") is never rewritten here and never auto-allowed.
	if tp, rest, ok := matchTransparentPrefix(bareCmd, prefixes); ok {
		if before, _, found := LocateInner(rest, cmdSet, tp.ValueFlags, tp.SkipFlags); found {
			// Known inner command, but leave the group raw when its output feeds a
			// consumer (issue #111). headKnown stays true so a redirected-but-known
			// producer does not needlessly block auto-allow of sibling groups.
			if feedsConsumer {
				return group, true, hasTail
			}
			wrappedHead := prefix + envVars + tp.Prefix + " " + before + runInvocation(quotedBin) + rest[len(before):]
			return wrappedHead + tail, true, hasTail
		}
	}

	if _, ok := cmdSet[base]; !ok {
		return group, false, hasTail
	}

	// Known producer feeding a downstream consumer: leave raw (issue #111).
	if feedsConsumer {
		return group, true, hasTail
	}

	wrappedHead := prefix + envVars + runInvocation(quotedBin) + bareCmd
	return wrappedHead + tail, true, hasTail
}

// escapesNewline reports whether the newline at i is escaped by the backslash
// before it, which joins the two lines into one command. Backslashes are counted
// because an even run is a literal backslash and really ends the line
// ("git log \\\\" is a trailing backslash argument, not a continuation). The
// caller only reaches here outside quotes, where a backslash can escape.
func escapesNewline(cmd string, i int) bool {
	n := 0
	for j := i - 1; j >= 0 && cmd[j] == '\\'; j-- {
		n++
	}
	return n%2 == 1
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

// hasTopLevelRedirect reports whether group contains an unquoted output
// redirection ('>'), i.e. the producer's stdout/stderr is written to a file
// rather than read by the LLM. Any top-level '>' counts, covering '>', '>>',
// '2>', '&>' and '>&'. snip fails safe here: an over-detection only forgoes
// compaction, whereas a miss would silently corrupt the redirected file
// (issue #111). '<' (stdin) is ignored — it never affects the filtered stdout.
func hasTopLevelRedirect(group string) bool {
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
		case '>':
			return true
		}
	}
	return false
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
