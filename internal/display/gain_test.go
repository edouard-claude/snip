package display

import (
	"path/filepath"
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
