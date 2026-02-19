package display

import (
	"path/filepath"
	"testing"

	"snip/internal/tracking"
)

func newTestTracker(t *testing.T) *tracking.Tracker {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	tracker, err := tracking.NewTracker(dbPath)
	if err != nil {
		t.Fatalf("new tracker: %v", err)
	}
	t.Cleanup(func() { tracker.Close() })
	return tracker
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

	tracker.Track("git log", "snip git log", 1000, 200, 50)
	tracker.Track("go test", "snip go test", 2000, 300, 100)

	err := RunGain(tracker, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunGainDaily(t *testing.T) {
	tracker := newTestTracker(t)
	tracker.Track("cmd", "snip cmd", 500, 100, 30)

	err := RunGain(tracker, []string{"--daily"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunGainHistory(t *testing.T) {
	tracker := newTestTracker(t)
	tracker.Track("cmd1", "snip cmd1", 500, 100, 30)
	tracker.Track("cmd2", "snip cmd2", 800, 200, 40)

	err := RunGain(tracker, []string{"--history", "5"})
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
