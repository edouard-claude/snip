//go:build !lite

package display

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

func TestRunGainNoData(t *testing.T) {
	tracker := newTestTracker(t)
	err := RunGain(tracker, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunGainWithData(t *testing.T) {
	tracker := newTestTracker(t)
	seedTracker(t, tracker)

	err := RunGain(tracker, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunGainDaily(t *testing.T) {
	tracker := newTestTracker(t)
	seedTracker(t, tracker)

	err := RunGain(tracker, []string{"--daily"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunGainWeekly(t *testing.T) {
	tracker := newTestTracker(t)
	seedTracker(t, tracker)

	err := RunGain(tracker, []string{"--weekly"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunGainMonthly(t *testing.T) {
	tracker := newTestTracker(t)
	seedTracker(t, tracker)

	err := RunGain(tracker, []string{"--monthly"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunGainTop(t *testing.T) {
	tracker := newTestTracker(t)
	seedTracker(t, tracker)

	err := RunGain(tracker, []string{"--top", "5"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunGainTopDefault(t *testing.T) {
	tracker := newTestTracker(t)
	seedTracker(t, tracker)

	err := RunGain(tracker, []string{"--top"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunGainHistory(t *testing.T) {
	tracker := newTestTracker(t)
	seedTracker(t, tracker)

	err := RunGain(tracker, []string{"--history", "5"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunGainHistoryDefault(t *testing.T) {
	tracker := newTestTracker(t)
	seedTracker(t, tracker)

	err := RunGain(tracker, []string{"--history"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunGainJSON(t *testing.T) {
	tracker := newTestTracker(t)
	seedTracker(t, tracker)

	err := RunGain(tracker, []string{"--json"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunGainCSV(t *testing.T) {
	tracker := newTestTracker(t)
	seedTracker(t, tracker)

	err := RunGain(tracker, []string{"--csv"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunGainNilTracker(t *testing.T) {
	err := RunGain(nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestCmdColWidth verifies that cmdColWidth returns the correct column width
// for various terminal widths, clamping to a minimum of 20.
func TestCmdColWidth(t *testing.T) {
	tests := []struct {
		termWidth  int
		fixedWidth int
		want       int
	}{
		{120, 36, 84}, // nominal: plenty of room
		{30, 36, 20},  // narrow: clamped to minimum
		{56, 36, 20},  // boundary: w==20, clamped
	}
	for _, tc := range tests {
		got := cmdColWidth(tc.termWidth, tc.fixedWidth)
		if got != tc.want {
			t.Errorf("cmdColWidth(%d, %d) = %d, want %d", tc.termWidth, tc.fixedWidth, got, tc.want)
		}
	}
}

// TestRunGainNoTruncate verifies that --no-truncate causes long command names
// to appear untruncated in the output.
func TestRunGainNoTruncate(t *testing.T) {
	tracker := newTestTracker(t)

	longCmd := strings.Repeat("x", 200)
	_ = tracker.Track(longCmd, "snip "+longCmd, 5000, 100, 200)

	// Capture stdout
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w

	runErr := RunGain(tracker, []string{"--top", "1", "--no-truncate"})

	w.Close()
	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	os.Stdout = old

	if runErr != nil {
		t.Fatalf("unexpected error: %v", runErr)
	}
	if !strings.Contains(buf.String(), longCmd) {
		t.Errorf("expected long command (%d chars) to appear untruncated in output", len(longCmd))
	}
}
