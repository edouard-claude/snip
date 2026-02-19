package utils

import (
	"testing"
)

func TestTruncate(t *testing.T) {
	tests := []struct {
		name string
		s    string
		max  int
		want string
	}{
		{"short string", "hello", 10, "hello"},
		{"exact length", "hello", 5, "hello"},
		{"truncated", "hello world", 8, "hello..."},
		{"empty", "", 5, ""},
		{"zero max", "hello", 0, ""},
		{"max 3", "hello", 3, "hel"},
		{"max 2", "hello", 2, "he"},
		{"unicode", "héllo wörld", 8, "héllo..."},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Truncate(tt.s, tt.max)
			if got != tt.want {
				t.Errorf("Truncate(%q, %d) = %q, want %q", tt.s, tt.max, got, tt.want)
			}
		})
	}
}

func TestStripANSI(t *testing.T) {
	tests := []struct {
		name string
		s    string
		want string
	}{
		{"no ansi", "hello", "hello"},
		{"color", "\x1b[31mred\x1b[0m", "red"},
		{"bold", "\x1b[1mbold\x1b[0m", "bold"},
		{"mixed", "normal \x1b[32mgreen\x1b[0m normal", "normal green normal"},
		{"empty", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := StripANSI(tt.s)
			if got != tt.want {
				t.Errorf("StripANSI(%q) = %q, want %q", tt.s, got, tt.want)
			}
		})
	}
}

func TestEstimateTokens(t *testing.T) {
	tests := []struct {
		name string
		s    string
		want int
	}{
		{"empty", "", 0},
		{"4 chars", "abcd", 1},
		{"5 chars", "abcde", 2},
		{"8 chars", "abcdefgh", 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := EstimateTokens(tt.s)
			if got != tt.want {
				t.Errorf("EstimateTokens(%q) = %d, want %d", tt.s, got, tt.want)
			}
		})
	}
}

func TestFormatTokens(t *testing.T) {
	tests := []struct {
		n    int
		want string
	}{
		{0, "0"},
		{694, "694"},
		{1000, "1.0K"},
		{59200, "59.2K"},
		{1200000, "1.2M"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := FormatTokens(tt.n)
			if got != tt.want {
				t.Errorf("FormatTokens(%d) = %q, want %q", tt.n, got, tt.want)
			}
		})
	}
}

func TestCountLines(t *testing.T) {
	tests := []struct {
		name string
		s    string
		want int
	}{
		{"empty", "", 0},
		{"one line", "hello", 1},
		{"one line with newline", "hello\n", 1},
		{"two lines", "hello\nworld", 2},
		{"three lines", "a\nb\nc\n", 3},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CountLines(tt.s)
			if got != tt.want {
				t.Errorf("CountLines(%q) = %d, want %d", tt.s, got, tt.want)
			}
		})
	}
}

func TestCompactPath(t *testing.T) {
	tests := []struct {
		path string
		want string
	}{
		{"src/main.go", "main.go"},
		{"lib/utils.js", "utils.js"},
		{"internal/foo/bar.go", "foo/bar.go"},
		{"main.go", "main.go"},
		{"vendor/pkg/mod.go", "pkg/mod.go"},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			got := CompactPath(tt.path)
			if got != tt.want {
				t.Errorf("CompactPath(%q) = %q, want %q", tt.path, got, tt.want)
			}
		})
	}
}

func TestOkConfirmation(t *testing.T) {
	if got := OkConfirmation("push", "to origin/main"); got != "ok push to origin/main" {
		t.Errorf("got %q", got)
	}
	if got := OkConfirmation("done", ""); got != "ok done" {
		t.Errorf("got %q", got)
	}
}
