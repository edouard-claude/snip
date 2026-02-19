package utils

import (
	"fmt"
	"math"
	"strings"
	"unicode/utf8"
)

var ansiRe = NewLazyRegex(`\x1b\[[0-9;]*[a-zA-Z]`)

// Truncate truncates s to max runes, appending "..." if truncated.
func Truncate(s string, max int) string {
	if max <= 0 {
		return ""
	}
	if utf8.RuneCountInString(s) <= max {
		return s
	}
	runes := []rune(s)
	if max <= 3 {
		return string(runes[:max])
	}
	return string(runes[:max-3]) + "..."
}

// StripANSI removes ANSI escape codes from s.
func StripANSI(s string) string {
	return ansiRe.Re().ReplaceAllString(s, "")
}

// EstimateTokens estimates token count using ~4 chars/token heuristic.
func EstimateTokens(s string) int {
	n := len(s)
	if n == 0 {
		return 0
	}
	return int(math.Ceil(float64(n) / 4.0))
}

// FormatTokens formats a token count for display: "1.2M", "59.2K", "694".
func FormatTokens(n int) string {
	switch {
	case n >= 1_000_000:
		return fmt.Sprintf("%.1fM", float64(n)/1_000_000)
	case n >= 1_000:
		return fmt.Sprintf("%.1fK", float64(n)/1_000)
	default:
		return fmt.Sprintf("%d", n)
	}
}

// CountLines counts the number of lines in s.
func CountLines(s string) int {
	if s == "" {
		return 0
	}
	n := strings.Count(s, "\n")
	if !strings.HasSuffix(s, "\n") {
		n++
	}
	return n
}

// CompactPath strips common prefixes like src/, lib/, internal/ from a path.
func CompactPath(path string) string {
	prefixes := []string{"src/", "lib/", "internal/", "pkg/", "vendor/"}
	for _, p := range prefixes {
		if strings.HasPrefix(path, p) {
			return path[len(p):]
		}
	}
	return path
}

// OkConfirmation produces a compact confirmation message.
func OkConfirmation(action, detail string) string {
	if detail == "" {
		return "ok " + action
	}
	return fmt.Sprintf("ok %s %s", action, detail)
}
