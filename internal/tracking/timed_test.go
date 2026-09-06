//go:build !lite

package tracking

import (
	"testing"
)

func TestTimedExecution(t *testing.T) {
	tracker := newTestTracker(t)

	timed := Start(tracker)
	err := timed.Track("git log", "snip git log", 500, 100)
	if err != nil {
		t.Fatalf("timed track: %v", err)
	}

	summary, err := tracker.GetSummary()
	if err != nil {
		t.Fatalf("summary: %v", err)
	}
	if summary.TotalCommands != 1 {
		t.Errorf("total commands = %d", summary.TotalCommands)
	}
}

func TestTimedExecutionNilTracker(t *testing.T) {
	timed := Start(nil)
	err := timed.Track("cmd", "snip cmd", 100, 50)
	if err != nil {
		t.Fatalf("expected nil tracker to be no-op: %v", err)
	}
	err = timed.TrackPassthrough("cmd", 100)
	if err != nil {
		t.Fatalf("expected nil tracker passthrough to be no-op: %v", err)
	}
}

func TestTimedExecutionTrackPassthrough(t *testing.T) {
	tracker := newTestTracker(t)
	timed := Start(tracker)
	err := timed.TrackPassthrough("npm install", 500)
	if err != nil {
		t.Fatalf("timed track passthrough: %v", err)
	}

	summary, err := tracker.GetSummary()
	if err != nil {
		t.Fatalf("summary: %v", err)
	}
	if summary.TotalSaved != 0 {
		t.Errorf("expected 0 saved for passthrough, got %d", summary.TotalSaved)
	}
	if summary.TotalCommands != 1 {
		t.Errorf("expected 1 command, got %d", summary.TotalCommands)
	}
}
