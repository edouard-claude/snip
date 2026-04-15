//go:build !lite

package economics

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/edouard-claude/snip/internal/tracking"
)

func newTestTracker(t *testing.T) *tracking.Tracker {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	tracker, err := tracking.NewTracker(dbPath)
	if err != nil {
		t.Fatalf("new tracker: %v", err)
	}
	t.Cleanup(func() { _ = tracker.Close() })
	return tracker
}

func seedTracker(t *testing.T, tracker *tracking.Tracker) {
	t.Helper()
	_ = tracker.Track("git log", "snip git log", 1000, 200, 50)
	_ = tracker.Track("go test", "snip go test", 2000, 300, 100)
	_ = tracker.Track("git log", "snip git log", 800, 100, 40)
	_ = tracker.Track("ls -la", "snip ls -la", 50, 30, 5)
}

func TestCostForTokens(t *testing.T) {
	tests := []struct {
		name    string
		tokens  int
		priceM  float64
		want    float64
	}{
		{"1M tokens at sonnet price", 1_000_000, 3.00, 3.00},
		{"500K tokens at sonnet price", 500_000, 3.00, 1.50},
		{"1M tokens at haiku price", 1_000_000, 0.25, 0.25},
		{"1M tokens at opus price", 1_000_000, 15.00, 15.00},
		{"zero tokens", 0, 3.00, 0.00},
		{"2.3M tokens at sonnet price", 2_300_000, 3.00, 6.90},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CostForTokens(tt.tokens, tt.priceM)
			diff := got - tt.want
			if diff < 0 {
				diff = -diff
			}
			if diff > 0.001 {
				t.Errorf("CostForTokens(%d, %.2f) = %.4f, want %.4f", tt.tokens, tt.priceM, got, tt.want)
			}
		})
	}
}

func TestFormatCost(t *testing.T) {
	tests := []struct {
		amount float64
		want   string
	}{
		{0.25, "$0.25"},
		{1.50, "$1.50"},
		{6.90, "$6.90"},
		{15.00, "$15.0"},
		{150.00, "$150"},
		{0.00, "$0.00"},
		{0.58, "$0.58"},
		{34.50, "$34.5"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := FormatCost(tt.amount)
			if got != tt.want {
				t.Errorf("FormatCost(%.2f) = %q, want %q", tt.amount, got, tt.want)
			}
		})
	}
}

func TestTierByName(t *testing.T) {
	tests := []struct {
		name string
		want string
	}{
		{"haiku", "Haiku"},
		{"Haiku", "Haiku"},
		{"HAIKU", "Haiku"},
		{"sonnet", "Sonnet"},
		{"opus", "Opus"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tier := TierByName(tt.name)
			if tier == nil {
				t.Fatalf("TierByName(%q) returned nil", tt.name)
			}
			if tier.Name != tt.want {
				t.Errorf("TierByName(%q).Name = %q, want %q", tt.name, tier.Name, tt.want)
			}
		})
	}
}

func TestTierByNameUnknown(t *testing.T) {
	tier := TierByName("unknown")
	if tier != nil {
		t.Errorf("TierByName(\"unknown\") = %v, want nil", tier)
	}
}

func TestTierPricing(t *testing.T) {
	expected := map[string]float64{
		"Haiku":  0.25,
		"Sonnet": 3.00,
		"Opus":   15.00,
	}
	for _, tier := range Tiers {
		want, ok := expected[tier.Name]
		if !ok {
			t.Errorf("unexpected tier %q", tier.Name)
			continue
		}
		if tier.PriceM != want {
			t.Errorf("tier %q price = %.2f, want %.2f", tier.Name, tier.PriceM, want)
		}
	}
}

func TestRunNilTracker(t *testing.T) {
	err := Run(nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunWithData(t *testing.T) {
	tracker := newTestTracker(t)
	seedTracker(t, tracker)

	// Capture stdout
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w

	runErr := Run(tracker, nil)

	_ = w.Close()
	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	os.Stdout = old

	if runErr != nil {
		t.Fatalf("unexpected error: %v", runErr)
	}

	output := buf.String()
	if !strings.Contains(output, "cc-economics") {
		t.Error("expected output to contain 'cc-economics'")
	}
	if !strings.Contains(output, "Haiku") {
		t.Error("expected output to contain 'Haiku'")
	}
	if !strings.Contains(output, "Sonnet") {
		t.Error("expected output to contain 'Sonnet'")
	}
	if !strings.Contains(output, "Opus") {
		t.Error("expected output to contain 'Opus'")
	}
}

func TestRunWithTierFilter(t *testing.T) {
	tracker := newTestTracker(t)
	seedTracker(t, tracker)

	// Capture stdout
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w

	runErr := Run(tracker, []string{"--tier", "sonnet"})

	_ = w.Close()
	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	os.Stdout = old

	if runErr != nil {
		t.Fatalf("unexpected error: %v", runErr)
	}

	output := buf.String()
	if !strings.Contains(output, "Sonnet") {
		t.Error("expected output to contain 'Sonnet'")
	}
	if strings.Contains(output, "Haiku") {
		t.Error("expected output NOT to contain 'Haiku' when filtered to sonnet")
	}
	if strings.Contains(output, "Opus") {
		t.Error("expected output NOT to contain 'Opus' when filtered to sonnet")
	}
}

func TestRunUnknownTier(t *testing.T) {
	tracker := newTestTracker(t)
	seedTracker(t, tracker)

	err := Run(tracker, []string{"--tier", "gpt4"})
	if err == nil {
		t.Fatal("expected error for unknown tier")
	}
	if !strings.Contains(err.Error(), "unknown tier") {
		t.Errorf("expected 'unknown tier' error, got: %v", err)
	}
}

func TestEqFold(t *testing.T) {
	tests := []struct {
		a, b string
		want bool
	}{
		{"Haiku", "haiku", true},
		{"HAIKU", "haiku", true},
		{"haiku", "haiku", true},
		{"haiku", "sonnet", false},
		{"abc", "abcd", false},
	}
	for _, tt := range tests {
		got := eqFold(tt.a, tt.b)
		if got != tt.want {
			t.Errorf("eqFold(%q, %q) = %v, want %v", tt.a, tt.b, got, tt.want)
		}
	}
}
