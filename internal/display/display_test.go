package display

import (
	"strings"
	"testing"
)

func TestFormatSeparator(t *testing.T) {
	s := FormatSeparator(10)
	if len([]rune(s)) != 10 {
		t.Errorf("rune len = %d, want 10", len([]rune(s)))
	}
}

func TestFormatTable(t *testing.T) {
	headers := []string{"Name", "Count", "Pct"}
	rows := [][]string{
		{"git log", "42", "78.5%"},
		{"go test", "15", "85.2%"},
	}

	result := FormatTable(headers, rows)
	if !strings.Contains(result, "Name") {
		t.Error("missing header")
	}
	if !strings.Contains(result, "git log") {
		t.Error("missing row data")
	}
	lines := strings.Split(strings.TrimSpace(result), "\n")
	if len(lines) != 4 { // header + separator + 2 rows
		t.Errorf("got %d lines, want 4", len(lines))
	}
}

func TestFormatTableEmpty(t *testing.T) {
	result := FormatTable(nil, nil)
	if result != "" {
		t.Errorf("expected empty, got %q", result)
	}
}
