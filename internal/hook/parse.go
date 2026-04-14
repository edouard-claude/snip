package hook

import "strings"

// ExtractFirstSegment returns the first command segment from cmd, stopping at
// the first unquoted ';', '|', '&', or newline. Quoted regions (single and
// double quotes) and backslash escapes inside double quotes are respected.
func ExtractFirstSegment(cmd string) string {
	var quote byte
	for i := 0; i < len(cmd); i++ {
		ch := cmd[i]

		// Stop at newline (ignore heredoc bodies).
		if ch == '\n' {
			return cmd[:i]
		}

		if quote != 0 {
			// Inside double quotes, backslash escapes the next char.
			if ch == '\\' && quote == '"' && i+1 < len(cmd) {
				i++ // skip escaped char
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
		case ';', '|', '&':
			return cmd[:i]
		}
	}
	return cmd
}

// ParseSegment decomposes a command segment into its parts:
//   - prefix: leading whitespace
//   - envVars: env var assignments (e.g. "CGO_ENABLED=0 ")
//   - bareCmd: the command without env vars (e.g. "go test ./...")
//
// The caller can reconstruct the original as prefix + envVars + bareCmd + suffix.
func ParseSegment(segment string) (prefix, envVars, bareCmd string) {
	// Leading whitespace
	trimmed := strings.TrimLeft(segment, " \t")
	prefix = segment[:len(segment)-len(trimmed)]

	// Trailing whitespace
	core := strings.TrimRight(trimmed, " \t")
	suffix := trimmed[len(core):]

	// Strip env var assignments: KEY=VALUE sequences at the start
	rest := core
	var envParts []string
	for {
		// An env var assignment is WORD=NON_SPACE followed by a space.
		eqIdx := strings.IndexByte(rest, '=')
		if eqIdx <= 0 {
			break
		}
		// Verify the part before '=' is a valid env var name.
		name := rest[:eqIdx]
		if !isEnvVarName(name) {
			break
		}
		// Find the end of the value (next unquoted space).
		valStart := eqIdx + 1
		valEnd := findValueEnd(rest[valStart:])
		assignment := rest[:valStart+valEnd]

		// There must be a space after the assignment for it to be an env prefix.
		afterAssignment := rest[valStart+valEnd:]
		if len(afterAssignment) == 0 || (afterAssignment[0] != ' ' && afterAssignment[0] != '\t') {
			break
		}
		envParts = append(envParts, assignment)
		// Skip the trailing space(s) after this assignment.
		rest = strings.TrimLeft(afterAssignment, " \t")
	}

	if len(envParts) > 0 {
		envVars = strings.Join(envParts, " ") + " "
	}
	bareCmd = rest + suffix
	return prefix, envVars, bareCmd
}

// BaseCommand extracts the first word (executable name) from a command string.
func BaseCommand(cmd string) string {
	trimmed := strings.TrimLeft(cmd, " \t")
	// Find first space or tab
	for i := 0; i < len(trimmed); i++ {
		if trimmed[i] == ' ' || trimmed[i] == '\t' {
			return trimmed[:i]
		}
	}
	return trimmed
}

// isEnvVarName checks if s is a valid environment variable name: [A-Za-z_][A-Za-z0-9_]*
func isEnvVarName(s string) bool {
	if len(s) == 0 {
		return false
	}
	for i, c := range s {
		if i == 0 && !isEnvVarStart(c) {
			return false
		}
		if i > 0 && !isEnvVarChar(c) {
			return false
		}
	}
	return true
}

func isEnvVarStart(c rune) bool {
	return (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || c == '_'
}

func isEnvVarChar(c rune) bool {
	return isEnvVarStart(c) || (c >= '0' && c <= '9')
}

// findValueEnd returns the length of an env var value, handling quoted values.
func findValueEnd(s string) int {
	if len(s) == 0 {
		return 0
	}
	// Quoted value
	if s[0] == '"' || s[0] == '\'' {
		quote := s[0]
		for i := 1; i < len(s); i++ {
			if s[i] == '\\' && quote == '"' && i+1 < len(s) {
				i++ // skip escaped char
				continue
			}
			if s[i] == quote {
				return i + 1
			}
		}
		return len(s)
	}
	// Unquoted: stop at space/tab
	for i := 0; i < len(s); i++ {
		if s[i] == ' ' || s[i] == '\t' {
			return i
		}
	}
	return len(s)
}
