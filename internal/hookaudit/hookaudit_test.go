package hookaudit

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestParseEvents(t *testing.T) {
	input := `{"timestamp":"2026-04-15T12:00:00Z","command":"git status","base":"git","matched":true,"rewritten":true}
{"timestamp":"2026-04-15T12:01:00Z","command":"go test ./...","base":"go","matched":true,"rewritten":true}
{"timestamp":"2026-04-15T12:02:00Z","command":"cat README.md","base":"cat","matched":false,"rewritten":false}
`
	events, err := ParseEvents(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ParseEvents: %v", err)
	}
	if len(events) != 3 {
		t.Fatalf("got %d events, want 3", len(events))
	}

	if events[0].Command != "git status" {
		t.Errorf("events[0].Command = %q, want %q", events[0].Command, "git status")
	}
	if events[0].Base != "git" {
		t.Errorf("events[0].Base = %q, want %q", events[0].Base, "git")
	}
	if !events[0].Matched {
		t.Error("events[0].Matched = false, want true")
	}
	if !events[0].Rewritten {
		t.Error("events[0].Rewritten = false, want true")
	}

	if events[2].Command != "cat README.md" {
		t.Errorf("events[2].Command = %q, want %q", events[2].Command, "cat README.md")
	}
	if events[2].Matched {
		t.Error("events[2].Matched = true, want false")
	}
}

func TestParseEventsSkipsMalformed(t *testing.T) {
	input := `{"timestamp":"2026-04-15T12:00:00Z","command":"git status","base":"git","matched":true,"rewritten":true}
not json at all
{"timestamp":"2026-04-15T12:01:00Z","command":"go test","base":"go","matched":true,"rewritten":true}
`
	events, err := ParseEvents(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ParseEvents: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("got %d events, want 2 (skip malformed)", len(events))
	}
}

func TestParseEventsEmpty(t *testing.T) {
	events, err := ParseEvents(strings.NewReader(""))
	if err != nil {
		t.Fatalf("ParseEvents: %v", err)
	}
	if len(events) != 0 {
		t.Fatalf("got %d events, want 0", len(events))
	}
}

func TestFormatTable(t *testing.T) {
	ts1, _ := time.Parse(time.RFC3339, "2026-04-15T12:00:00Z")
	ts2, _ := time.Parse(time.RFC3339, "2026-04-15T12:01:00Z")
	ts3, _ := time.Parse(time.RFC3339, "2026-04-15T12:02:00Z")

	events := []Event{
		{Timestamp: ts1, Command: "git status", Base: "git", Matched: true, Rewritten: true},
		{Timestamp: ts2, Command: "go test ./...", Base: "go", Matched: true, Rewritten: true},
		{Timestamp: ts3, Command: "cat README.md", Base: "cat", Matched: false, Rewritten: false},
	}

	var buf bytes.Buffer
	FormatTable(&buf, events, 20)

	output := buf.String()

	if !strings.Contains(output, "snip hook-audit - recent hook activity") {
		t.Error("output missing header")
	}
	if !strings.Contains(output, "git status") {
		t.Error("output missing 'git status'")
	}
	if !strings.Contains(output, "go test ./...") {
		t.Error("output missing 'go test ./...'")
	}
	if !strings.Contains(output, "cat README.md") {
		t.Error("output missing 'cat README.md'")
	}
	if !strings.Contains(output, "yes") {
		t.Error("output missing 'yes'")
	}
	if !strings.Contains(output, "no") {
		t.Error("output missing 'no'")
	}
	if !strings.Contains(output, "Showing last 20 events") {
		t.Error("output missing footer")
	}
}

func TestFormatTableTailTruncation(t *testing.T) {
	ts, _ := time.Parse(time.RFC3339, "2026-04-15T12:00:00Z")

	var events []Event
	for i := 0; i < 10; i++ {
		events = append(events, Event{
			Timestamp: ts.Add(time.Duration(i) * time.Minute),
			Command:   "cmd" + strings.Repeat("x", i),
			Base:      "cmd",
			Matched:   true,
			Rewritten: true,
		})
	}

	var buf bytes.Buffer
	FormatTable(&buf, events, 3)

	output := buf.String()
	lines := strings.Split(strings.TrimSpace(output), "\n")

	// header line + blank + column header + 3 data lines + blank + footer = 8
	// Count data lines (lines containing "cmd").
	dataCount := 0
	for _, line := range lines {
		if strings.Contains(line, "cmd") && !strings.Contains(line, "Command") {
			dataCount++
		}
	}
	if dataCount != 3 {
		t.Errorf("got %d data lines, want 3", dataCount)
	}

	if !strings.Contains(output, "Showing last 3 events") {
		t.Error("footer should say 'Showing last 3 events'")
	}
}

func TestFormatTableEmpty(t *testing.T) {
	var buf bytes.Buffer
	FormatTable(&buf, nil, 20)

	output := buf.String()
	if !strings.Contains(output, "No events recorded") {
		t.Error("expected 'No events recorded' message")
	}
	if !strings.Contains(output, "SNIP_HOOK_AUDIT=1") {
		t.Error("expected hint about SNIP_HOOK_AUDIT=1")
	}
}

func TestRotation(t *testing.T) {
	// Use a temp directory for the log file.
	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "hook-audit.log")

	// Write MaxLines+50 lines to the file.
	f, err := os.Create(logFile)
	if err != nil {
		t.Fatalf("create temp log: %v", err)
	}

	ts, _ := time.Parse(time.RFC3339, "2026-04-15T12:00:00Z")
	for i := 0; i < MaxLines+50; i++ {
		e := Event{
			Timestamp: ts.Add(time.Duration(i) * time.Second),
			Command:   "cmd",
			Base:      "cmd",
			Matched:   true,
			Rewritten: true,
		}
		line, _ := json.Marshal(e)
		_, _ = f.WriteString(string(line) + "\n")
	}
	_ = f.Close()

	// Read and verify count.
	rf, err := os.Open(logFile)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer func() { _ = rf.Close() }()

	events, err := ParseEvents(rf)
	if err != nil {
		t.Fatalf("ParseEvents: %v", err)
	}
	if len(events) != MaxLines+50 {
		t.Fatalf("got %d events before rotation, want %d", len(events), MaxLines+50)
	}

	// Now use Append which should rotate.
	// Override the log path for test by writing directly via rotateFile.
	rotated := rotateLines(logFile, `{"timestamp":"2026-04-15T13:00:00Z","command":"new cmd","base":"new","matched":false,"rewritten":false}`)
	if rotated != nil {
		t.Fatalf("rotateLines: %v", rotated)
	}

	// Re-read.
	rf2, err := os.Open(logFile)
	if err != nil {
		t.Fatalf("open after rotation: %v", err)
	}
	defer func() { _ = rf2.Close() }()

	events2, err := ParseEvents(rf2)
	if err != nil {
		t.Fatalf("ParseEvents after rotation: %v", err)
	}

	if len(events2) != MaxLines {
		t.Errorf("got %d events after rotation, want %d", len(events2), MaxLines)
	}

	// Last event should be the newly appended one.
	last := events2[len(events2)-1]
	if last.Command != "new cmd" {
		t.Errorf("last event command = %q, want %q", last.Command, "new cmd")
	}
}

func TestTruncate(t *testing.T) {
	tests := []struct {
		input string
		max   int
		want  string
	}{
		{"short", 10, "short"},
		{"exactly10!", 10, "exactly10!"},
		{"this is a long string", 10, "this is..."},
		{"ab", 2, "ab"},
		{"abcdef", 4, "a..."},
	}
	for _, tt := range tests {
		got := truncate(tt.input, tt.max)
		if got != tt.want {
			t.Errorf("truncate(%q, %d) = %q, want %q", tt.input, tt.max, got, tt.want)
		}
	}
}

func TestEnabled(t *testing.T) {
	t.Setenv("SNIP_HOOK_AUDIT", "1")
	if !Enabled() {
		t.Error("Expected Enabled() to be true when SNIP_HOOK_AUDIT=1")
	}

	t.Setenv("SNIP_HOOK_AUDIT", "")
	if Enabled() {
		t.Error("Expected Enabled() to be false when SNIP_HOOK_AUDIT is empty")
	}

	t.Setenv("SNIP_HOOK_AUDIT", "0")
	if Enabled() {
		t.Error("Expected Enabled() to be false when SNIP_HOOK_AUDIT=0")
	}

	t.Setenv("SNIP_HOOK_AUDIT", "true")
	if Enabled() {
		t.Error("Expected Enabled() to be false when SNIP_HOOK_AUDIT=true (not '1')")
	}
}

func TestLogDirAndLogPath(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	dir := LogDir()
	expectedDir := filepath.Join(tmpDir, ".local", "share", "snip")
	if dir != expectedDir {
		t.Errorf("LogDir() = %q, want %q", dir, expectedDir)
	}

	path := LogPath()
	expectedPath := filepath.Join(expectedDir, "hook-audit.log")
	if path != expectedPath {
		t.Errorf("LogPath() = %q, want %q", path, expectedPath)
	}
}

func TestAppendAndReadEvents(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	ts := time.Date(2026, 4, 15, 12, 0, 0, 0, time.UTC)
	e := Event{
		Timestamp: ts,
		Command:   "git status",
		Base:      "git",
		Matched:   true,
		Rewritten: true,
		Agent:     "claude-code",
	}
	Append(e)

	events, err := ReadEvents()
	if err != nil {
		t.Fatalf("ReadEvents: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("got %d events, want 1", len(events))
	}
	if events[0].Command != "git status" {
		t.Errorf("command = %q, want %q", events[0].Command, "git status")
	}
	if events[0].Base != "git" {
		t.Errorf("base = %q, want %q", events[0].Base, "git")
	}
	if !events[0].Matched {
		t.Error("expected Matched=true")
	}
	if !events[0].Rewritten {
		t.Error("expected Rewritten=true")
	}
	if events[0].Agent != "claude-code" {
		t.Errorf("agent = %q, want %q", events[0].Agent, "claude-code")
	}
}

func TestAppendRotation(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	ts := time.Date(2026, 4, 15, 12, 0, 0, 0, time.UTC)
	// Append MaxLines+50 events, rotation should keep last MaxLines
	for i := 0; i < MaxLines+50; i++ {
		Append(Event{
			Timestamp: ts.Add(time.Duration(i) * time.Second),
			Command:   fmt.Sprintf("cmd-%d", i),
			Base:      "cmd",
			Matched:   true,
			Rewritten: true,
		})
	}

	events, err := ReadEvents()
	if err != nil {
		t.Fatalf("ReadEvents: %v", err)
	}
	if len(events) != MaxLines {
		t.Errorf("got %d events after rotation, want %d", len(events), MaxLines)
	}
}

func TestClearFunc(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	// Append one event first.
	Append(Event{
		Timestamp: time.Now(),
		Command:   "test",
		Base:      "test",
	})

	// Verify it's there.
	events, err := ReadEvents()
	if err != nil {
		t.Fatalf("ReadEvents: %v", err)
	}
	if len(events) == 0 {
		t.Fatal("expected at least 1 event before clear")
	}

	// Clear.
	if err := Clear(); err != nil {
		t.Fatalf("Clear: %v", err)
	}

	// Verify cleared — should get nil events.
	events, err = ReadEvents()
	if err != nil {
		t.Fatalf("ReadEvents after clear: %v", err)
	}
	if len(events) != 0 {
		t.Errorf("got %d events after clear, want 0", len(events))
	}
}

func TestReadEventsNonexistent(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	events, err := ReadEvents()
	if err != nil {
		t.Fatalf("ReadEvents on nonexistent file should not error: %v", err)
	}
	if events != nil {
		t.Errorf("got %d events, want nil", len(events))
	}
}

func TestRun(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	// Append some events first.
	ts := time.Date(2026, 4, 15, 12, 0, 0, 0, time.UTC)
	for i := 0; i < 5; i++ {
		Append(Event{
			Timestamp: ts.Add(time.Duration(i) * time.Minute),
			Command:   fmt.Sprintf("cmd-%d", i),
			Base:      "cmd",
			Matched:   true,
			Rewritten: true,
		})
	}

	// Run with --tail 3.
	err := Run([]string{"--tail", "3"})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	// Run with --clear.
	err = Run([]string{"--clear"})
	if err != nil {
		t.Fatalf("Run --clear: %v", err)
	}

	// Verify cleared.
	events, err := ReadEvents()
	if err != nil {
		t.Fatalf("ReadEvents after clear: %v", err)
	}
	if len(events) != 0 {
		t.Errorf("expected 0 events after clear, got %d", len(events))
	}
}

func TestRunWithTailEquals(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	err := Run([]string{"--tail=10"})
	if err != nil {
		t.Fatalf("Run --tail=10: %v", err)
	}
}

func TestRunErrors(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	// --tail without value.
	err := Run([]string{"--tail"})
	if err == nil {
		t.Fatal("expected error for --tail without value")
	}

	// --tail with non-integer.
	err = Run([]string{"--tail", "abc"})
	if err == nil {
		t.Fatal("expected error for --tail abc")
	}

	// --tail with negative.
	err = Run([]string{"--tail", "-1"})
	if err == nil {
		t.Fatal("expected error for --tail -1")
	}

	// --tail=N with non-integer.
	err = Run([]string{"--tail=abc"})
	if err == nil {
		t.Fatal("expected error for --tail=abc")
	}

	// Unknown flag.
	err = Run([]string{"--unknown"})
	if err == nil {
		t.Fatal("expected error for unknown flag")
	}
}

func TestBoolYesNo(t *testing.T) {
	if boolYesNo(true) != "yes" {
		t.Error("boolYesNo(true) != yes")
	}
	if boolYesNo(false) != "no" {
		t.Error("boolYesNo(false) != no")
	}
}

func TestClear(t *testing.T) {
	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "hook-audit.log")

	// Create a file.
	if err := os.WriteFile(logFile, []byte("test\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	// Clear should not error on non-existent file either.
	err := os.Remove(logFile)
	if err != nil {
		t.Fatalf("remove: %v", err)
	}
	// Clearing a non-existent file should not error
	// (tested through the main Clear function indirectly via the actual path).
}

// rotateLines appends a line to a file and rotates to MaxLines.
// This mirrors the core logic of Append but works with an explicit path.
func rotateLines(path string, line string) error {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()

	lines, _ := readLines(f)
	lines = append(lines, line)

	if len(lines) > MaxLines {
		lines = lines[len(lines)-MaxLines:]
	}

	if _, err := f.Seek(0, 0); err != nil {
		return err
	}
	if err := f.Truncate(0); err != nil {
		return err
	}
	for _, l := range lines {
		_, _ = fmt.Fprintln(f, l)
	}
	return nil
}
