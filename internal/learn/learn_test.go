package learn

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestExtractCommandEntries(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "session.jsonl")

	lines := []string{
		// Assistant sends a Bash tool_use
		`{"type":"assistant","message":{"role":"assistant","content":[{"type":"tool_use","id":"toolu_1","name":"Bash","input":{"command":"go test ./..."}}]},"timestamp":"2026-04-01T10:00:00.000Z"}`,
		// User returns tool_result with error
		`{"type":"user","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"toolu_1","content":"FAIL: TestFoo","is_error":true}]},"timestamp":"2026-04-01T10:00:05.000Z"}`,
		// Assistant retries with a fix
		`{"type":"assistant","message":{"role":"assistant","content":[{"type":"tool_use","id":"toolu_2","name":"Bash","input":{"command":"go test -run TestFoo ./..."}}]},"timestamp":"2026-04-01T10:00:10.000Z"}`,
		// User returns success
		`{"type":"user","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"toolu_2","content":"ok  \tgithub.com/example/pkg\t0.5s"}]},"timestamp":"2026-04-01T10:00:15.000Z"}`,
	}

	writeLines(t, path, lines)

	cutoff := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	entries := extractCommandEntries(path, cutoff)

	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}

	// First entry should be an error
	if entries[0].Command != "go test ./..." {
		t.Errorf("entries[0].Command = %q, want %q", entries[0].Command, "go test ./...")
	}
	if entries[0].BaseCmd != "go" {
		t.Errorf("entries[0].BaseCmd = %q, want %q", entries[0].BaseCmd, "go")
	}
	if !entries[0].IsError {
		t.Error("entries[0].IsError = false, want true")
	}
	if !entries[0].HasResult {
		t.Error("entries[0].HasResult = false, want true")
	}
	if entries[0].Output != "FAIL: TestFoo" {
		t.Errorf("entries[0].Output = %q, want %q", entries[0].Output, "FAIL: TestFoo")
	}

	// Second entry should be success
	if entries[1].Command != "go test -run TestFoo ./..." {
		t.Errorf("entries[1].Command = %q, want %q", entries[1].Command, "go test -run TestFoo ./...")
	}
	if entries[1].IsError {
		t.Error("entries[1].IsError = true, want false")
	}
}

func TestExtractCommandEntriesTimestamp(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "session.jsonl")

	lines := []string{
		// Old command (before cutoff)
		`{"type":"assistant","message":{"role":"assistant","content":[{"type":"tool_use","id":"toolu_old","name":"Bash","input":{"command":"go test ./..."}}]},"timestamp":"2026-01-01T10:00:00.000Z"}`,
		// Recent command (after cutoff)
		`{"type":"assistant","message":{"role":"assistant","content":[{"type":"tool_use","id":"toolu_new","name":"Bash","input":{"command":"git status"}}]},"timestamp":"2026-04-01T10:00:00.000Z"}`,
	}

	writeLines(t, path, lines)

	cutoff := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	entries := extractCommandEntries(path, cutoff)

	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].Command != "git status" {
		t.Errorf("expected git status, got %q", entries[0].Command)
	}
}

func TestDetectPatterns(t *testing.T) {
	entries := []commandEntry{
		{Command: "go test ./...", BaseCmd: "go", IsError: true, HasResult: true, Output: "FAIL: build errors"},
		{Command: "go vet ./...", BaseCmd: "go", IsError: false, HasResult: true, Output: ""},
		{Command: "go test ./...", BaseCmd: "go", IsError: false, HasResult: true, Output: "ok"},
	}

	patterns := detectPatterns(entries)

	if len(patterns) != 1 {
		t.Fatalf("expected 1 pattern, got %d", len(patterns))
	}

	p := patterns[0]
	if p.BaseCommand != "go" {
		t.Errorf("BaseCommand = %q, want %q", p.BaseCommand, "go")
	}
	if p.ErrorCommand != "go test ./..." {
		t.Errorf("ErrorCommand = %q, want %q", p.ErrorCommand, "go test ./...")
	}
	if p.FixCommand != "go vet ./..." {
		t.Errorf("FixCommand = %q, want %q", p.FixCommand, "go vet ./...")
	}
}

func TestDetectPatternsNoMatch(t *testing.T) {
	// All commands succeed - no patterns
	entries := []commandEntry{
		{Command: "go test ./...", BaseCmd: "go", IsError: false, HasResult: true},
		{Command: "git status", BaseCmd: "git", IsError: false, HasResult: true},
	}

	patterns := detectPatterns(entries)

	if len(patterns) != 0 {
		t.Errorf("expected 0 patterns, got %d", len(patterns))
	}
}

func TestDetectPatternsLookAheadLimit(t *testing.T) {
	// Error command followed by 5 unrelated commands, then a fix (too far)
	entries := []commandEntry{
		{Command: "go test ./...", BaseCmd: "go", IsError: true, HasResult: true, Output: "FAIL"},
		{Command: "git status", BaseCmd: "git", IsError: false, HasResult: true},
		{Command: "git diff", BaseCmd: "git", IsError: false, HasResult: true},
		{Command: "git log", BaseCmd: "git", IsError: false, HasResult: true},
		{Command: "git add .", BaseCmd: "git", IsError: false, HasResult: true},
		{Command: "ls -la", BaseCmd: "ls", IsError: false, HasResult: true},
		{Command: "go test ./...", BaseCmd: "go", IsError: false, HasResult: true}, // Too far (index 6, beyond window of 5)
	}

	patterns := detectPatterns(entries)

	if len(patterns) != 0 {
		t.Errorf("expected 0 patterns (fix too far away), got %d", len(patterns))
	}
}

func TestDetectPatternsWithinLimit(t *testing.T) {
	// Error followed by fix within the 5-command window
	entries := []commandEntry{
		{Command: "npm install", BaseCmd: "npm", IsError: true, HasResult: true, Output: "ERESOLVE"},
		{Command: "git status", BaseCmd: "git", IsError: false, HasResult: true},
		{Command: "git diff", BaseCmd: "git", IsError: false, HasResult: true},
		{Command: "npm install --legacy-peer-deps", BaseCmd: "npm", IsError: false, HasResult: true},
	}

	patterns := detectPatterns(entries)

	if len(patterns) != 1 {
		t.Fatalf("expected 1 pattern, got %d", len(patterns))
	}
	if patterns[0].FixCommand != "npm install --legacy-peer-deps" {
		t.Errorf("FixCommand = %q, want %q", patterns[0].FixCommand, "npm install --legacy-peer-deps")
	}
}

func TestDetectPatternsSkipsErrorFix(t *testing.T) {
	// Error followed by another error with same base - should not match
	entries := []commandEntry{
		{Command: "go test ./...", BaseCmd: "go", IsError: true, HasResult: true, Output: "FAIL"},
		{Command: "go test -v ./...", BaseCmd: "go", IsError: true, HasResult: true, Output: "FAIL again"},
	}

	patterns := detectPatterns(entries)

	if len(patterns) != 0 {
		t.Errorf("expected 0 patterns (fix also failed), got %d", len(patterns))
	}
}

func TestAggregatePatterns(t *testing.T) {
	byCmd := map[string][]ErrorPattern{
		"go": {
			{BaseCommand: "go", ErrorCommand: "go test ./...", ErrorOutput: "FAIL", FixCommand: "go test -run Foo ./...", Count: 1},
			{BaseCommand: "go", ErrorCommand: "go test ./...", ErrorOutput: "FAIL", FixCommand: "go test -run Bar ./...", Count: 1},
			{BaseCommand: "go", ErrorCommand: "go build ./cmd/...", ErrorOutput: "undefined: X", FixCommand: "go build ./cmd/...", Count: 1},
		},
		"git": {
			{BaseCommand: "git", ErrorCommand: "git push", ErrorOutput: "rejected", FixCommand: "git pull --rebase && git push", Count: 1},
		},
	}

	groups := aggregatePatterns(byCmd)

	if len(groups) != 2 {
		t.Fatalf("expected 2 groups, got %d", len(groups))
	}

	// "go" should be first (3 total vs 1)
	if groups[0].BaseCommand != "go" {
		t.Errorf("groups[0].BaseCommand = %q, want %q", groups[0].BaseCommand, "go")
	}
	if groups[0].TotalCount != 3 {
		t.Errorf("groups[0].TotalCount = %d, want 3", groups[0].TotalCount)
	}

	// "go test ./..." should have count 2 after merge
	found := false
	for _, p := range groups[0].Patterns {
		if p.ErrorCommand == "go test ./..." && p.Count == 2 {
			found = true
		}
	}
	if !found {
		t.Error("expected merged pattern for 'go test ./...' with count 2")
	}

	if groups[1].BaseCommand != "git" {
		t.Errorf("groups[1].BaseCommand = %q, want %q", groups[1].BaseCommand, "git")
	}
}

func TestLooksLikeError(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"FAIL: TestFoo", true},
		{"error: cannot find module", true},
		{"command not found: xyz", true},
		{"No such file or directory", true},
		{"permission denied", true},
		{"ok  \tgithub.com/example/pkg\t0.5s", false},
		{"PASS", false},
		{"", false},
		{"Everything is fine", false},
		{"ERESOLVE unable to resolve dependency tree", true},
		{"exit status 1", true},
		{"panic: runtime error", true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := looksLikeError(tt.input)
			if got != tt.want {
				t.Errorf("looksLikeError(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestSummarizeError(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"empty", "", "command failed"},
		{"simple error", "Error: build failed", "Error: build failed"},
		{"multiline", "some preamble\nError: the real error\nmore stuff", "Error: the real error"},
		{"long line", "Error: " + strings.Repeat("x", 100), "Error: " + strings.Repeat("x", 73) + "..."},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := summarizeError(tt.input)
			if got != tt.want {
				t.Errorf("summarizeError() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestSummarizeFix(t *testing.T) {
	// Same command = retry
	got := summarizeFix("go test ./...", "go test ./...")
	if got != "retrying the same command" {
		t.Errorf("expected retry message, got %q", got)
	}

	// Different command
	got = summarizeFix("go test ./...", "go test -run TestFoo ./...")
	if got != "go test -run TestFoo ./..." {
		t.Errorf("expected fix command, got %q", got)
	}
}

func TestTruncateOutput(t *testing.T) {
	short := "hello"
	if truncateOutput(short, 10) != "hello" {
		t.Error("should not truncate short string")
	}

	long := strings.Repeat("x", 50)
	got := truncateOutput(long, 20)
	if got != strings.Repeat("x", 20)+"..." {
		t.Errorf("truncateOutput() = %q, want %q", got, strings.Repeat("x", 20)+"...")
	}
}

func TestScanIntegration(t *testing.T) {
	dir := t.TempDir()

	// Session with error-correction pattern
	session1 := filepath.Join(dir, "session1.jsonl")
	writeLines(t, session1, []string{
		`{"type":"assistant","message":{"role":"assistant","content":[{"type":"tool_use","id":"toolu_1","name":"Bash","input":{"command":"go test ./..."}}]},"timestamp":"2026-04-01T10:00:00.000Z"}`,
		`{"type":"user","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"toolu_1","content":"FAIL: TestFoo - undefined: bar","is_error":true}]},"timestamp":"2026-04-01T10:00:05.000Z"}`,
		`{"type":"assistant","message":{"role":"assistant","content":[{"type":"tool_use","id":"toolu_2","name":"Bash","input":{"command":"go test -run TestFoo ./..."}}]},"timestamp":"2026-04-01T10:00:10.000Z"}`,
		`{"type":"user","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"toolu_2","content":"ok  \tgithub.com/example/pkg"}]},"timestamp":"2026-04-01T10:00:15.000Z"}`,
	})

	// Session with no errors
	session2 := filepath.Join(dir, "session2.jsonl")
	writeLines(t, session2, []string{
		`{"type":"assistant","message":{"role":"assistant","content":[{"type":"tool_use","id":"toolu_3","name":"Bash","input":{"command":"git status"}}]},"timestamp":"2026-04-01T10:00:00.000Z"}`,
		`{"type":"user","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"toolu_3","content":"On branch master"}]},"timestamp":"2026-04-01T10:00:05.000Z"}`,
	})

	cutoff := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	result := scan([]string{dir}, cutoff)

	if result.SessionsScanned != 2 {
		t.Errorf("sessions scanned = %d, want 2", result.SessionsScanned)
	}
	if result.TotalErrors != 1 {
		t.Errorf("total errors = %d, want 1", result.TotalErrors)
	}
	if len(result.Groups) != 1 {
		t.Fatalf("expected 1 group, got %d", len(result.Groups))
	}
	if result.Groups[0].BaseCommand != "go" {
		t.Errorf("group base command = %q, want %q", result.Groups[0].BaseCommand, "go")
	}
}

func TestGenerateRules(t *testing.T) {
	dir := t.TempDir()
	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(originalDir) }()

	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}

	result := Result{
		SessionsScanned: 5,
		TotalErrors:     3,
		Groups: []PatternGroup{
			{
				BaseCommand: "go",
				TotalCount:  2,
				Patterns: []ErrorPattern{
					{
						BaseCommand:  "go",
						ErrorCommand: "go test ./...",
						ErrorOutput:  "FAIL: undefined: bar",
						FixCommand:   "go test -run TestFoo ./...",
						Count:        2,
					},
				},
			},
			{
				BaseCommand: "git",
				TotalCount:  1,
				Patterns: []ErrorPattern{
					{
						BaseCommand:  "git",
						ErrorCommand: "git push",
						ErrorOutput:  "error: rejected",
						FixCommand:   "git pull --rebase && git push",
						Count:        1,
					},
				},
			},
		},
	}

	if err := generateRules(result); err != nil {
		t.Fatalf("generateRules() error: %v", err)
	}

	path := filepath.Join(dir, ".claude", "rules", "cli-corrections.md")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read generated file: %v", err)
	}

	content := string(data)

	// Check structure
	if !strings.Contains(content, "# CLI Corrections (auto-generated by snip learn)") {
		t.Error("missing header")
	}
	if !strings.Contains(content, "### go failures") {
		t.Error("missing go failures section")
	}
	if !strings.Contains(content, "### git failures") {
		t.Error("missing git failures section")
	}
	if !strings.Contains(content, "go test -run TestFoo ./...") {
		t.Error("missing fix command for go")
	}
	if !strings.Contains(content, "git pull --rebase && git push") {
		t.Error("missing fix command for git")
	}
}

func TestGenerateRulesEmpty(t *testing.T) {
	result := Result{TotalErrors: 0}
	if err := generateRules(result); err != nil {
		t.Fatalf("generateRules() error: %v", err)
	}
	// Should not create a file
	if _, err := os.Stat(filepath.Join(".claude", "rules", "cli-corrections.md")); err == nil {
		t.Error("should not create file with no patterns")
	}
}

func TestParseArgs(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		since    int
		generate bool
		all      bool
	}{
		{"defaults", nil, 30, false, false},
		{"since flag", []string{"--since", "14"}, 14, false, false},
		{"generate flag", []string{"--generate"}, 30, true, false},
		{"all flag", []string{"--all"}, 30, false, true},
		{"all flags", []string{"--all", "--since", "7", "--generate"}, 7, true, true},
		{"since without value", []string{"--since"}, 30, false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := parseArgs(tt.args)
			if opts.Since != tt.since {
				t.Errorf("Since = %d, want %d", opts.Since, tt.since)
			}
			if opts.Generate != tt.generate {
				t.Errorf("Generate = %v, want %v", opts.Generate, tt.generate)
			}
			if opts.All != tt.all {
				t.Errorf("All = %v, want %v", opts.All, tt.all)
			}
		})
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
		{"/usr/bin/git status", "git"},
		{"npm install", "npm"},
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

func TestEmptyScan(t *testing.T) {
	dir := t.TempDir()
	cutoff := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	result := scan([]string{dir}, cutoff)

	if result.SessionsScanned != 0 {
		t.Errorf("sessions scanned = %d, want 0", result.SessionsScanned)
	}
	if result.TotalErrors != 0 {
		t.Errorf("total errors = %d, want 0", result.TotalErrors)
	}
}

func TestStringOrArrayUnmarshal(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"string", `"hello world"`, "hello world"},
		{"array", `[{"type":"text","text":"line1"},{"type":"text","text":"line2"}]`, "line1\nline2"},
		{"empty array", `[]`, ""},
		{"null", `null`, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var s stringOrArray
			if err := s.UnmarshalJSON([]byte(tt.input)); err != nil {
				t.Fatalf("UnmarshalJSON error: %v", err)
			}
			if s.Value != tt.want {
				t.Errorf("Value = %q, want %q", s.Value, tt.want)
			}
		})
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
