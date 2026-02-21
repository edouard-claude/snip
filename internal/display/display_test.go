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

func TestFormatBar(t *testing.T) {
	tests := []struct {
		name     string
		value    int
		maxVal   int
		width    int
		wantLen  int
		wantFull bool
	}{
		{"full bar", 100, 100, 10, 10, true},
		{"half bar", 50, 100, 10, 10, false},
		{"empty bar", 0, 100, 10, 10, false},
		{"zero max", 50, 0, 10, 10, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bar := FormatBar(tt.value, tt.maxVal, tt.width)
			runes := []rune(bar)
			if len(runes) != tt.wantLen {
				t.Errorf("bar rune len = %d, want %d", len(runes), tt.wantLen)
			}
			if tt.wantFull {
				for _, r := range runes {
					if r != '█' {
						t.Errorf("expected all filled, got %c", r)
						break
					}
				}
			}
		})
	}
}

func TestFormatSparkline(t *testing.T) {
	values := []float64{10, 50, 80, 30, 100}
	spark := FormatSparkline(values)
	runes := []rune(spark)
	if len(runes) != 5 {
		t.Errorf("sparkline len = %d, want 5", len(runes))
	}
	// Last value is max (100), should be highest block
	if runes[4] != '█' {
		t.Errorf("max value should be █, got %c", runes[4])
	}
}

func TestFormatSparklineEmpty(t *testing.T) {
	spark := FormatSparkline(nil)
	if spark != "" {
		t.Errorf("expected empty, got %q", spark)
	}
}

func TestTierLabel(t *testing.T) {
	tests := []struct {
		pct  float64
		want string
	}{
		{95, "Elite"},
		{75, "Great"},
		{55, "Good"},
		{35, "Fair"},
		{10, "Low"},
	}
	for _, tt := range tests {
		got := TierLabel(tt.pct)
		if got != tt.want {
			t.Errorf("TierLabel(%.0f) = %q, want %q", tt.pct, got, tt.want)
		}
	}
}

func TestColorSavingsNonTTY(t *testing.T) {
	// Non-TTY: should return plain text (no ANSI codes)
	result := ColorSavings(85.3)
	if !strings.Contains(result, "85.3%") {
		t.Errorf("expected 85.3%%, got %q", result)
	}
}
