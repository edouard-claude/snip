//go:build !lite

package tracking

import (
	"path/filepath"
	"testing"
)

func newTestTracker(t *testing.T) *Tracker {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	tracker, err := NewTracker(dbPath)
	if err != nil {
		t.Fatalf("new tracker: %v", err)
	}
	t.Cleanup(func() { _ = tracker.Close() })
	return tracker
}

func TestNewTracker(t *testing.T) {
	tracker := newTestTracker(t)
	if tracker == nil {
		t.Fatal("tracker is nil")
	}
}

func TestTrack(t *testing.T) {
	tracker := newTestTracker(t)

	err := tracker.Track("git log", "snip git log", 1000, 200, 50)
	if err != nil {
		t.Fatalf("track: %v", err)
	}

	summary, err := tracker.GetSummary()
	if err != nil {
		t.Fatalf("summary: %v", err)
	}
	if summary.TotalCommands != 1 {
		t.Errorf("total commands = %d", summary.TotalCommands)
	}
	if summary.TotalSaved != 800 {
		t.Errorf("total saved = %d", summary.TotalSaved)
	}
	if summary.AvgSavings < 79 || summary.AvgSavings > 81 {
		t.Errorf("avg savings = %.1f%%", summary.AvgSavings)
	}
}

func TestTrackPassthrough(t *testing.T) {
	tracker := newTestTracker(t)

	err := tracker.TrackPassthrough("npm install", 500, 100)
	if err != nil {
		t.Fatalf("track passthrough: %v", err)
	}

	summary, err := tracker.GetSummary()
	if err != nil {
		t.Fatalf("summary: %v", err)
	}
	if summary.TotalSaved != 0 {
		t.Errorf("expected 0 saved for passthrough, got %d", summary.TotalSaved)
	}
}

func TestGetRecent(t *testing.T) {
	tracker := newTestTracker(t)

	_ = tracker.Track("cmd1", "snip cmd1", 100, 30, 10)
	_ = tracker.Track("cmd2", "snip cmd2", 200, 50, 20)
	_ = tracker.Track("cmd3", "snip cmd3", 300, 80, 30)

	recent, err := tracker.GetRecent(2)
	if err != nil {
		t.Fatalf("recent: %v", err)
	}
	if len(recent) != 2 {
		t.Fatalf("got %d records, want 2", len(recent))
	}
	// Most recent first
	if recent[0].OriginalCmd != "cmd3" {
		t.Errorf("first = %q", recent[0].OriginalCmd)
	}
}

func TestGetDaily(t *testing.T) {
	tracker := newTestTracker(t)

	_ = tracker.Track("cmd1", "snip cmd1", 100, 30, 10)
	_ = tracker.Track("cmd2", "snip cmd2", 200, 50, 20)

	daily, err := tracker.GetDaily(7)
	if err != nil {
		t.Fatalf("daily: %v", err)
	}
	if len(daily) != 1 {
		t.Fatalf("got %d days, want 1", len(daily))
	}
	if daily[0].Commands != 2 {
		t.Errorf("commands = %d", daily[0].Commands)
	}
}

func TestGetByCommand(t *testing.T) {
	tracker := newTestTracker(t)

	_ = tracker.Track("git log", "snip git log", 1000, 200, 50)
	_ = tracker.Track("git log", "snip git log", 800, 100, 40)
	_ = tracker.Track("go test", "snip go test", 2000, 300, 100)
	_ = tracker.Track("ls -la", "snip ls -la", 50, 30, 5)

	stats, err := tracker.GetByCommand(10)
	if err != nil {
		t.Fatalf("by command: %v", err)
	}
	if len(stats) != 3 {
		t.Fatalf("got %d commands, want 3", len(stats))
	}
	// go test has most saved (1700), then git log (1500), then ls -la (20)
	if stats[0].Command != "go test" {
		t.Errorf("first command = %q, want go test", stats[0].Command)
	}
	if stats[0].SavedTokens != 1700 {
		t.Errorf("go test saved = %d, want 1700", stats[0].SavedTokens)
	}
	if stats[1].Command != "git log" {
		t.Errorf("second command = %q, want git log", stats[1].Command)
	}
	if stats[1].Count != 2 {
		t.Errorf("git log count = %d, want 2", stats[1].Count)
	}
}

func TestGetByCommandLimit(t *testing.T) {
	tracker := newTestTracker(t)

	_ = tracker.Track("cmd1", "snip cmd1", 100, 30, 10)
	_ = tracker.Track("cmd2", "snip cmd2", 200, 50, 20)
	_ = tracker.Track("cmd3", "snip cmd3", 300, 80, 30)

	stats, err := tracker.GetByCommand(2)
	if err != nil {
		t.Fatalf("by command: %v", err)
	}
	if len(stats) != 2 {
		t.Fatalf("got %d commands, want 2", len(stats))
	}
}

func TestGetWeekly(t *testing.T) {
	tracker := newTestTracker(t)

	_ = tracker.Track("cmd1", "snip cmd1", 100, 30, 10)
	_ = tracker.Track("cmd2", "snip cmd2", 200, 50, 20)

	weekly, err := tracker.GetWeekly(4)
	if err != nil {
		t.Fatalf("weekly: %v", err)
	}
	if len(weekly) != 1 {
		t.Fatalf("got %d weeks, want 1", len(weekly))
	}
	if weekly[0].Commands != 2 {
		t.Errorf("commands = %d, want 2", weekly[0].Commands)
	}
}

func TestGetMonthly(t *testing.T) {
	tracker := newTestTracker(t)

	_ = tracker.Track("cmd1", "snip cmd1", 500, 100, 30)
	_ = tracker.Track("cmd2", "snip cmd2", 800, 200, 40)

	monthly, err := tracker.GetMonthly(6)
	if err != nil {
		t.Fatalf("monthly: %v", err)
	}
	if len(monthly) != 1 {
		t.Fatalf("got %d months, want 1", len(monthly))
	}
	if monthly[0].Commands != 2 {
		t.Errorf("commands = %d, want 2", monthly[0].Commands)
	}
	if monthly[0].SavedTokens != 1000 {
		t.Errorf("saved = %d, want 1000", monthly[0].SavedTokens)
	}
}

func TestDBPath(t *testing.T) {
	t.Setenv("SNIP_DB_PATH", "/custom/path.db")
	if got := DBPath(""); got != "/custom/path.db" {
		t.Errorf("got %q", got)
	}

	t.Setenv("SNIP_DB_PATH", "")
	if got := DBPath("/config/path.db"); got != "/config/path.db" {
		t.Errorf("got %q", got)
	}
}
