package discover

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestParseBashCommand(t *testing.T) {
	cutoff := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)

	tests := []struct {
		name string
		line string
		want string
	}{
		{
			name: "simple bash command",
			line: `{"type":"assistant","message":{"role":"assistant","content":[{"type":"tool_use","name":"Bash","input":{"command":"go test ./..."}}]},"timestamp":"2026-02-19T07:42:14.131Z"}`,
			want: "go",
		},
		{
			name: "git status command",
			line: `{"type":"assistant","message":{"role":"assistant","content":[{"type":"tool_use","name":"Bash","input":{"command":"git status"}}]},"timestamp":"2026-02-19T07:42:14.131Z"}`,
			want: "git",
		},
		{
			name: "command with env vars",
			line: `{"type":"assistant","message":{"role":"assistant","content":[{"type":"tool_use","name":"Bash","input":{"command":"CGO_ENABLED=0 go build ./..."}}]},"timestamp":"2026-02-19T07:42:14.131Z"}`,
			want: "go",
		},
		{
			name: "command with pipe",
			line: `{"type":"assistant","message":{"role":"assistant","content":[{"type":"tool_use","name":"Bash","input":{"command":"git log --oneline | head -5"}}]},"timestamp":"2026-02-19T07:42:14.131Z"}`,
			want: "git",
		},
		{
			name: "command with semicolons",
			line: `{"type":"assistant","message":{"role":"assistant","content":[{"type":"tool_use","name":"Bash","input":{"command":"make build; make test"}}]},"timestamp":"2026-02-19T07:42:14.131Z"}`,
			want: "make",
		},
		{
			name: "non-bash tool use",
			line: `{"type":"assistant","message":{"role":"assistant","content":[{"type":"tool_use","name":"Read","input":{"file_path":"/tmp/foo"}}]},"timestamp":"2026-02-19T07:42:14.131Z"}`,
			want: "",
		},
		{
			name: "user message ignored",
			line: `{"type":"user","message":{"role":"user","content":"hello"},"timestamp":"2026-02-19T07:42:14.131Z"}`,
			want: "",
		},
		{
			name: "malformed json",
			line: `{invalid json`,
			want: "",
		},
		{
			name: "empty command",
			line: `{"type":"assistant","message":{"role":"assistant","content":[{"type":"tool_use","name":"Bash","input":{"command":""}}]},"timestamp":"2026-02-19T07:42:14.131Z"}`,
			want: "",
		},
		{
			name: "command with full path",
			line: `{"type":"assistant","message":{"role":"assistant","content":[{"type":"tool_use","name":"Bash","input":{"command":"/usr/bin/git status"}}]},"timestamp":"2026-02-19T07:42:14.131Z"}`,
			want: "git",
		},
		{
			name: "command with newline",
			line: `{"type":"assistant","message":{"role":"assistant","content":[{"type":"tool_use","name":"Bash","input":{"command":"git add . && git commit -m \"fix\"\ngit push"}}]},"timestamp":"2026-02-19T07:42:14.131Z"}`,
			want: "git",
		},
		{
			name: "system message ignored",
			line: `{"type":"system","content":"some system message","timestamp":"2026-02-19T07:42:14.131Z"}`,
			want: "",
		},
		{
			name: "multiple content items picks bash",
			line: `{"type":"assistant","message":{"role":"assistant","content":[{"type":"text","text":"hello"},{"type":"tool_use","name":"Bash","input":{"command":"npm install"}}]},"timestamp":"2026-02-19T07:42:14.131Z"}`,
			want: "npm",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseBashCommand([]byte(tt.line), cutoff)
			if got != tt.want {
				t.Errorf("parseBashCommand() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestParseBashCommandTimestamp(t *testing.T) {
	// Command before cutoff should be ignored
	cutoff := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	line := `{"type":"assistant","message":{"role":"assistant","content":[{"type":"tool_use","name":"Bash","input":{"command":"go test ./..."}}]},"timestamp":"2026-02-19T07:42:14.131Z"}`
	got := parseBashCommand([]byte(line), cutoff)
	if got != "" {
		t.Errorf("expected empty for old command, got %q", got)
	}

	// Command after cutoff should be included
	line = `{"type":"assistant","message":{"role":"assistant","content":[{"type":"tool_use","name":"Bash","input":{"command":"go test ./..."}}]},"timestamp":"2026-04-01T07:42:14.131Z"}`
	got = parseBashCommand([]byte(line), cutoff)
	if got != "go" {
		t.Errorf("expected %q for recent command, got %q", "go", got)
	}
}

func TestExtractBaseCommand(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"go test ./...", "go"},
		{"git status", "git"},
		{"CGO_ENABLED=0 go build", "go"},
		{"FOO=bar BAZ=1 make test", "make"},
		{"make build; make test", "make"},
		{"git log | head -5", "git"},
		{"/usr/bin/git status", "git"},
		{"npm install && npm test", "npm"},
		{"echo hello", "echo"},
		{"  ls -la", "ls"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := extractBaseCommand(tt.input)
			if got != tt.want {
				t.Errorf("extractBaseCommand(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestExtractCommands(t *testing.T) {
	// Create a temp JSONL file
	dir := t.TempDir()
	path := filepath.Join(dir, "session.jsonl")

	lines := []string{
		`{"type":"assistant","message":{"role":"assistant","content":[{"type":"tool_use","name":"Bash","input":{"command":"go test ./..."}}]},"timestamp":"2026-04-01T10:00:00.000Z"}`,
		`{"type":"assistant","message":{"role":"assistant","content":[{"type":"tool_use","name":"Bash","input":{"command":"git status"}}]},"timestamp":"2026-04-01T10:01:00.000Z"}`,
		`{"type":"user","message":{"role":"user","content":"check status"},"timestamp":"2026-04-01T10:02:00.000Z"}`,
		`{"type":"assistant","message":{"role":"assistant","content":[{"type":"tool_use","name":"Bash","input":{"command":"make build"}}]},"timestamp":"2026-04-01T10:03:00.000Z"}`,
		`{invalid json line}`,
		`{"type":"assistant","message":{"role":"assistant","content":[{"type":"tool_use","name":"Read","input":{"file_path":"/tmp/x"}}]},"timestamp":"2026-04-01T10:04:00.000Z"}`,
	}

	data := ""
	for _, l := range lines {
		data += l + "\n"
	}
	if err := os.WriteFile(path, []byte(data), 0644); err != nil {
		t.Fatal(err)
	}

	cutoff := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	commands := extractCommands(path, cutoff)

	if len(commands) != 3 {
		t.Fatalf("expected 3 commands, got %d: %v", len(commands), commands)
	}
	if commands[0] != "go" {
		t.Errorf("commands[0] = %q, want %q", commands[0], "go")
	}
	if commands[1] != "git" {
		t.Errorf("commands[1] = %q, want %q", commands[1], "git")
	}
	if commands[2] != "make" {
		t.Errorf("commands[2] = %q, want %q", commands[2], "make")
	}
}

func TestScan(t *testing.T) {
	// Create temp project dir with session files
	dir := t.TempDir()

	// Session 1: two supported, one unsupported
	session1 := filepath.Join(dir, "session1.jsonl")
	writeLines(t, session1, []string{
		`{"type":"assistant","message":{"role":"assistant","content":[{"type":"tool_use","name":"Bash","input":{"command":"git status"}}]},"timestamp":"2026-04-01T10:00:00.000Z"}`,
		`{"type":"assistant","message":{"role":"assistant","content":[{"type":"tool_use","name":"Bash","input":{"command":"go test ./..."}}]},"timestamp":"2026-04-01T10:01:00.000Z"}`,
		`{"type":"assistant","message":{"role":"assistant","content":[{"type":"tool_use","name":"Bash","input":{"command":"cat file.txt"}}]},"timestamp":"2026-04-01T10:02:00.000Z"}`,
	})

	// Session 2: one supported
	session2 := filepath.Join(dir, "session2.jsonl")
	writeLines(t, session2, []string{
		`{"type":"assistant","message":{"role":"assistant","content":[{"type":"tool_use","name":"Bash","input":{"command":"git log --oneline"}}]},"timestamp":"2026-04-01T10:00:00.000Z"}`,
	})

	// Subagent session inside a session directory
	sessionDir := filepath.Join(dir, "abc123")
	subDir := filepath.Join(sessionDir, "subagents")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatal(err)
	}
	subSession := filepath.Join(subDir, "agent-xyz.jsonl")
	writeLines(t, subSession, []string{
		`{"type":"assistant","message":{"role":"assistant","content":[{"type":"tool_use","name":"Bash","input":{"command":"make build"}}]},"timestamp":"2026-04-01T10:00:00.000Z"}`,
	})

	supportedCmds := map[string]struct{}{
		"git":  {},
		"go":   {},
		"make": {},
	}

	cutoff := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	result := scan([]string{dir}, supportedCmds, cutoff)

	if result.SessionsScanned != 3 {
		t.Errorf("sessions scanned = %d, want 3", result.SessionsScanned)
	}
	if result.TotalCommands != 5 {
		t.Errorf("total commands = %d, want 5", result.TotalCommands)
	}
	if result.SupportedCount != 4 {
		t.Errorf("supported count = %d, want 4", result.SupportedCount)
	}
	if result.UnsupportedCount != 1 {
		t.Errorf("unsupported count = %d, want 1", result.UnsupportedCount)
	}

	// Check that unsupported contains "cat"
	found := false
	for _, s := range result.Unsupported {
		if s.Name == "cat" && s.Count == 1 {
			found = true
		}
	}
	if !found {
		t.Error("expected 'cat' in unsupported stats")
	}
}

func TestMapToStats(t *testing.T) {
	m := map[string]int{
		"git":  10,
		"make": 5,
		"go":   10,
	}
	stats := mapToStats(m)

	if len(stats) != 3 {
		t.Fatalf("expected 3 stats, got %d", len(stats))
	}
	// Same count sorted alphabetically: go before git
	if stats[0].Name != "git" && stats[0].Name != "go" {
		t.Errorf("first stat should be git or go, got %s", stats[0].Name)
	}
	if stats[0].Count != 10 {
		t.Errorf("first stat count = %d, want 10", stats[0].Count)
	}
	// Last should be make with count 5
	if stats[2].Name != "make" || stats[2].Count != 5 {
		t.Errorf("last stat = %v, want {make 5}", stats[2])
	}
}

func TestCwdToProjectName(t *testing.T) {
	got := cwdToProjectName("/Users/edouard/Code/go/snip")
	want := "-Users-edouard-Code-go-snip"
	if got != want {
		t.Errorf("cwdToProjectName() = %q, want %q", got, want)
	}
}

func TestParseArgs(t *testing.T) {
	tests := []struct {
		name  string
		args  []string
		all   bool
		since int
	}{
		{"defaults", nil, false, 7},
		{"all flag", []string{"--all"}, true, 7},
		{"since flag", []string{"--since", "14"}, false, 14},
		{"both flags", []string{"--all", "--since", "30"}, true, 30},
		{"since without value", []string{"--since"}, false, 7},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := parseArgs(tt.args)
			if opts.All != tt.all {
				t.Errorf("All = %v, want %v", opts.All, tt.all)
			}
			if opts.Since != tt.since {
				t.Errorf("Since = %d, want %d", opts.Since, tt.since)
			}
		})
	}
}

func TestFindSessionFiles(t *testing.T) {
	dir := t.TempDir()

	// Create some session files
	writeLines(t, filepath.Join(dir, "session1.jsonl"), []string{"{}"})
	writeLines(t, filepath.Join(dir, "session2.jsonl"), []string{"{}"})

	// Create a non-jsonl file (should be ignored)
	writeLines(t, filepath.Join(dir, "notes.txt"), []string{"hello"})

	// Create a subagent dir
	sessionDir := filepath.Join(dir, "session-uuid")
	subDir := filepath.Join(sessionDir, "subagents")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatal(err)
	}
	writeLines(t, filepath.Join(subDir, "agent-abc.jsonl"), []string{"{}"})

	files := findSessionFiles([]string{dir})

	if len(files) != 3 {
		t.Errorf("expected 3 files, got %d: %v", len(files), files)
	}
}

func TestEmptyScan(t *testing.T) {
	dir := t.TempDir()
	cutoff := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	result := scan([]string{dir}, map[string]struct{}{}, cutoff)

	if result.SessionsScanned != 0 {
		t.Errorf("sessions scanned = %d, want 0", result.SessionsScanned)
	}
	if result.TotalCommands != 0 {
		t.Errorf("total commands = %d, want 0", result.TotalCommands)
	}
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w
	fn()
	_ = w.Close()
	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	os.Stdout = old
	return buf.String()
}

func TestPrintResultEmpty(t *testing.T) {
	output := captureStdout(t, func() {
		printResult(Result{SessionsScanned: 3, TotalCommands: 0})
	})
	if !strings.Contains(output, "No Bash commands found") {
		t.Errorf("expected 'No Bash commands found' in output, got: %q", output)
	}
}

func TestPrintResultWithData(t *testing.T) {
	result := Result{
		SessionsScanned:  10,
		TotalCommands:    100,
		SupportedCount:   83,
		UnsupportedCount: 17,
		Supported: []CommandStat{
			{Name: "git", Count: 50},
			{Name: "go", Count: 33},
		},
		Unsupported: []CommandStat{
			{Name: "cat", Count: 17},
		},
	}
	output := captureStdout(t, func() {
		printResult(result)
	})
	for _, want := range []string{"Command", "Count", "git", "83%"} {
		if !strings.Contains(output, want) {
			t.Errorf("expected %q in output, got: %q", want, output)
		}
	}
}

func TestPrintResultTruncatesLongNames(t *testing.T) {
	longName := strings.Repeat("a", 200)
	result := Result{
		SessionsScanned:  1,
		TotalCommands:    1,
		SupportedCount:   1,
		UnsupportedCount: 0,
		Supported: []CommandStat{
			{Name: longName, Count: 1},
		},
	}
	output := captureStdout(t, func() {
		printResult(result)
	})
	if strings.Contains(output, longName) {
		t.Errorf("expected long name (%d chars) to be truncated in output", len(longName))
	}
	if !strings.Contains(output, "...") {
		t.Errorf("expected truncated form ending in '...' in output")
	}
}

func writeLines(t *testing.T, path string, lines []string) {
	t.Helper()
	data := ""
	for _, l := range lines {
		data += l + "\n"
	}
	if err := os.WriteFile(path, []byte(data), 0644); err != nil {
		t.Fatal(err)
	}
}
