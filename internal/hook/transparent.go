package hook

import "strings"

// TransparentPrefix is a wrapper command that does not change *what* program
// runs, only *how* it runs (e.g. "uv run", "poetry run", "docker exec ctr").
// snip strips the prefix, locates the inner command, wraps that inner command
// so its filter applies, and re-prepends the prefix unchanged. This mirrors
// rtk's transparent_prefixes concept.
type TransparentPrefix struct {
	// Prefix is the literal wrapper, matched on a word boundary: "uv run" matches
	// "uv run ..." but not "uv runner".
	Prefix string
	// ValueFlags are runner flags that consume the following token as their value
	// (e.g. uv's "--python 3.12"). They let LocateInner skip a flag's value while
	// scanning for the inner command. Empty for prefixes with no value-taking
	// flags before the inner command.
	ValueFlags map[string]bool
	// SkipFlags reports whether flags may appear between the prefix and the inner
	// command (true for runners like "uv run" whose options precede the command).
	// When false, the inner command must immediately follow the prefix; any flag
	// fails closed. This guards shell wrappers whose flags change semantics
	// (e.g. "command -v pytest" is a non-executing lookup, not an exec wrapper).
	SkipFlags bool
}

// BuiltinTransparentPrefixes are always active, in addition to any
// user-configured prefixes. Restricted to wrappers that transparently exec an
// inner program. Remote-code runners (npx, bunx, pnpm dlx) are intentionally
// excluded: they fetch and run arbitrary code and must not be treated as a
// transparent pass-through.
var BuiltinTransparentPrefixes = []TransparentPrefix{
	{Prefix: "uv run", SkipFlags: true, ValueFlags: flagSet(
		"--python", "-p",
		"--with", "--with-requirements", "--with-editable",
		"--index", "--default-index", "--extra",
		"--directory", "--project",
		"--resolution", "--prerelease", "--index-strategy",
		"--exclude-newer", "--config-file",
	)},
	{Prefix: "poetry run", SkipFlags: true},
	{Prefix: "pdm run", SkipFlags: true},
	{Prefix: "pipenv run", SkipFlags: true},
	{Prefix: "rye run", SkipFlags: true},
	{Prefix: "hatch run", SkipFlags: true},
	// Shell wrappers that exec their argument list unchanged. SkipFlags stays
	// false: their flags change semantics (e.g. "command -v" is a lookup that
	// does not execute its operand), so the inner command must follow directly.
	{Prefix: "noglob"},
	{Prefix: "nocorrect"},
	{Prefix: "command"},
	{Prefix: "exec"},
}

// MergeTransparentPrefixes returns the built-in prefixes plus any user-configured
// ones. Results are ordered longest-prefix-first so a more specific prefix
// ("docker exec ctr") is matched before a shorter one ("exec") it contains.
func MergeTransparentPrefixes(user []string) []TransparentPrefix {
	out := make([]TransparentPrefix, 0, len(BuiltinTransparentPrefixes)+len(user))
	out = append(out, BuiltinTransparentPrefixes...)
	for _, p := range user {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, TransparentPrefix{Prefix: p})
		}
	}
	// Stable insertion sort by descending prefix length (small slice, no import).
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && len(out[j].Prefix) > len(out[j-1].Prefix); j-- {
			out[j], out[j-1] = out[j-1], out[j]
		}
	}
	return out
}

// matchTransparentPrefix returns the first prefix whose literal wrapper begins
// bareCmd on a word boundary, along with the remainder after the prefix
// (leading whitespace trimmed). ok is false when no prefix matches or the
// matched prefix has no inner command (e.g. "uv run" alone).
func matchTransparentPrefix(bareCmd string, prefixes []TransparentPrefix) (tp TransparentPrefix, rest string, ok bool) {
	for _, p := range prefixes {
		if r, found := stripWordPrefix(bareCmd, p.Prefix); found && r != "" {
			return p, r, true
		}
	}
	return TransparentPrefix{}, "", false
}

// stripWordPrefix removes prefix from s when s is exactly prefix or prefix
// followed by whitespace, returning the remainder with leading whitespace
// trimmed. It guards against partial-word matches ("uv runner").
func stripWordPrefix(s, prefix string) (rest string, ok bool) {
	if !strings.HasPrefix(s, prefix) {
		return "", false
	}
	r := s[len(prefix):]
	if r == "" {
		return "", true // exact match, no inner command
	}
	if r[0] != ' ' && r[0] != '\t' {
		return "", false // partial word, not a real prefix
	}
	return strings.TrimLeft(r, " \t"), true
}

// LocateInner scans rest (the text after a transparent prefix) for the inner
// command. It skips runner flags: value-taking flags (per valueFlags) also
// consume their following token. The first non-flag token must be a known snip
// command; otherwise LocateInner fails closed (ok=false) so an unrecognized
// inner command — or an argument mistaken for one — is never rewritten.
//
// On success, before is the slice of rest preceding the inner command (so the
// caller can reconstruct "<prefix> <before><snip run --> <inner...>"), and
// innerBase is the inner command's base name for attestation checks.
func LocateInner(rest string, cmdSet map[string]struct{}, valueFlags map[string]bool, skipFlags bool) (before, innerBase string, ok bool) {
	toks := splitTokens(rest)
	for i := 0; i < len(toks); i++ {
		t := toks[i].text
		if strings.HasPrefix(t, "-") {
			if !skipFlags {
				// Flags are not permitted between this prefix and the command:
				// fail closed rather than risk a semantics-changing flag.
				return "", "", false
			}
			// "--flag=value" is self-contained; "--flag value" consumes the next
			// token only when --flag is a known value-taking flag.
			if valueFlags[t] && !strings.Contains(t, "=") {
				i++
			}
			continue
		}
		base := BaseCommand(t)
		if _, known := cmdSet[base]; known {
			return rest[:toks[i].start], base, true
		}
		// First non-flag token is not a known command: fail closed. This keeps
		// "uv run echo pytest" from being rewritten as if pytest were the target.
		return "", "", false
	}
	return "", "", false
}

// rawToken is a whitespace-delimited token with its start offset in the source.
type rawToken struct {
	text  string
	start int
}

// splitTokens splits s on unquoted whitespace, preserving single- and
// double-quoted regions (and backslash escapes inside double quotes) so a
// quoted flag value is not broken apart. Offsets index into s.
func splitTokens(s string) []rawToken {
	var toks []rawToken
	i := 0
	for i < len(s) {
		for i < len(s) && (s[i] == ' ' || s[i] == '\t') {
			i++
		}
		if i >= len(s) {
			break
		}
		start := i
		var quote byte
		for i < len(s) {
			c := s[i]
			if quote != 0 {
				if c == '\\' && quote == '"' && i+1 < len(s) {
					i += 2
					continue
				}
				if c == quote {
					quote = 0
				}
				i++
				continue
			}
			if c == ' ' || c == '\t' {
				break
			}
			if c == '\'' || c == '"' {
				quote = c
			}
			i++
		}
		toks = append(toks, rawToken{text: s[start:i], start: start})
	}
	return toks
}

// flagSet builds a set from flag names.
func flagSet(flags ...string) map[string]bool {
	m := make(map[string]bool, len(flags))
	for _, f := range flags {
		m[f] = true
	}
	return m
}
