package learn

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/edouard-claude/snip/internal/hook"
)

// sessionLine represents a single JSONL entry from a Claude Code session file.
type sessionLine struct {
	Type    string `json:"type"`
	Message *struct {
		Role    string          `json:"role"`
		Content json.RawMessage `json:"content"`
	} `json:"message,omitempty"`
	Timestamp string `json:"timestamp,omitempty"`
}

// contentItem represents one element of the message content array.
type contentItem struct {
	Type      string          `json:"type"`
	Name      string          `json:"name,omitempty"`
	ID        string          `json:"id,omitempty"`
	Input     json.RawMessage `json:"input,omitempty"`
	Content   stringOrArray   `json:"content,omitempty"`
	IsError   bool            `json:"is_error,omitempty"`
	ToolUseID string          `json:"tool_use_id,omitempty"`
}

// stringOrArray handles the content field which can be a string or an array.
type stringOrArray struct {
	Value string
}

func (s *stringOrArray) UnmarshalJSON(data []byte) error {
	// Try string first
	var str string
	if err := json.Unmarshal(data, &str); err == nil {
		s.Value = str
		return nil
	}
	// Try array of objects with text field
	var items []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if err := json.Unmarshal(data, &items); err == nil {
		var parts []string
		for _, item := range items {
			if item.Text != "" {
				parts = append(parts, item.Text)
			}
		}
		s.Value = strings.Join(parts, "\n")
		return nil
	}
	return nil
}

// bashInput holds the command field from a Bash tool_use input.
type bashInput struct {
	Command string `json:"command"`
}

// commandEntry represents a Bash command extracted from a session with its result.
type commandEntry struct {
	Command    string // full command string
	BaseCmd    string // extracted base command name
	ToolUseID  string // tool_use id for matching results
	Output     string // tool result content (if found)
	IsError    bool   // whether the tool result indicated an error
	HasResult  bool   // whether a result was matched
	Timestamp  string // timestamp from the JSONL entry
}

// ErrorPattern represents a detected error-correction pattern.
type ErrorPattern struct {
	BaseCommand    string // base command name (e.g. "go", "git")
	ErrorCommand   string // the command that failed
	ErrorOutput    string // truncated error output
	FixCommand     string // the command that succeeded
	Count          int    // number of times this pattern occurred
}

// PatternGroup aggregates similar error patterns by base command.
type PatternGroup struct {
	BaseCommand string
	Patterns    []ErrorPattern
	TotalCount  int
}

// Result holds the learn analysis output.
type Result struct {
	SessionsScanned int
	TotalErrors     int
	Groups          []PatternGroup
}

// Options configures the learn scan.
type Options struct {
	Since    int  // days (default 30)
	Generate bool // write rules file
	All      bool // scan all projects
}

// errorIndicators are substrings that suggest a command output contains an error.
var errorIndicators = []string{
	"command not found",
	"No such file or directory",
	"permission denied",
	"Permission denied",
	"Error:",
	"error:",
	"FAIL",
	"FATAL",
	"fatal:",
	"panic:",
	"cannot find",
	"not found",
	"undefined:",
	"syntax error",
	"SyntaxError",
	"TypeError",
	"ReferenceError",
	"ModuleNotFoundError",
	"ImportError",
	"ERESOLVE",
	"ENOENT",
	"exit status",
	"compilation failed",
	"build failed",
}

// Run executes the learn command with the given CLI args.
func Run(args []string) error {
	opts := parseArgs(args)

	projectDirs, err := findProjectDirs(opts)
	if err != nil {
		return fmt.Errorf("find project dirs: %w", err)
	}

	if len(projectDirs) == 0 {
		fmt.Fprintln(os.Stderr, "snip learn: no Claude Code session directories found")
		return nil
	}

	cutoff := time.Now().AddDate(0, 0, -opts.Since)
	result := scan(projectDirs, cutoff)
	printResult(result)

	if opts.Generate {
		return generateRules(result)
	}

	if !opts.Generate && result.TotalErrors > 0 {
		fmt.Println("Use --generate to create .claude/rules/cli-corrections.md")
	}

	return nil
}

// parseArgs extracts flags from args.
func parseArgs(args []string) Options {
	opts := Options{Since: 30}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--since":
			if i+1 < len(args) {
				n := 0
				for _, c := range args[i+1] {
					if c >= '0' && c <= '9' {
						n = n*10 + int(c-'0')
					} else {
						break
					}
				}
				if n > 0 {
					opts.Since = n
				}
				i++
			}
		case "--generate":
			opts.Generate = true
		case "--all":
			opts.All = true
		}
	}
	return opts
}

// claudeProjectsDir returns the base Claude Code projects directory.
func claudeProjectsDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".claude", "projects")
}

// cwdToProjectName converts a working directory path to the Claude Code
// project directory name format.
func cwdToProjectName(cwd string) string {
	return strings.ReplaceAll(cwd, string(os.PathSeparator), "-")
}

// findProjectDirs returns the list of Claude Code project directories to scan.
func findProjectDirs(opts Options) ([]string, error) {
	base := claudeProjectsDir()
	if base == "" {
		return nil, nil
	}

	if opts.All {
		entries, err := os.ReadDir(base)
		if err != nil {
			if os.IsNotExist(err) {
				return nil, nil
			}
			return nil, fmt.Errorf("read projects dir: %w", err)
		}
		var dirs []string
		for _, e := range entries {
			if e.IsDir() {
				dirs = append(dirs, filepath.Join(base, e.Name()))
			}
		}
		return dirs, nil
	}

	cwd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("get working directory: %w", err)
	}
	projectName := cwdToProjectName(cwd)
	projectDir := filepath.Join(base, projectName)
	if _, err := os.Stat(projectDir); os.IsNotExist(err) {
		return nil, nil
	}
	return []string{projectDir}, nil
}

// findSessionFiles returns all JSONL files under the given project directories,
// including subagent files nested in session subdirectories.
func findSessionFiles(projectDirs []string) []string {
	var files []string
	for _, dir := range projectDirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			path := filepath.Join(dir, e.Name())
			if !e.IsDir() && strings.HasSuffix(e.Name(), ".jsonl") {
				files = append(files, path)
			}
			if e.IsDir() {
				subagentDir := filepath.Join(path, "subagents")
				subEntries, err := os.ReadDir(subagentDir)
				if err != nil {
					continue
				}
				for _, se := range subEntries {
					if !se.IsDir() && strings.HasSuffix(se.Name(), ".jsonl") {
						files = append(files, filepath.Join(subagentDir, se.Name()))
					}
				}
			}
		}
	}
	return files
}

// scan processes all session files and detects error-correction patterns.
func scan(projectDirs []string, cutoff time.Time) Result {
	files := findSessionFiles(projectDirs)

	allPatterns := make(map[string][]ErrorPattern)
	sessions := 0
	totalErrors := 0

	for _, file := range files {
		entries := extractCommandEntries(file, cutoff)
		if len(entries) == 0 {
			continue
		}
		sessions++

		patterns := detectPatterns(entries)
		totalErrors += len(patterns)
		for _, p := range patterns {
			allPatterns[p.BaseCommand] = append(allPatterns[p.BaseCommand], p)
		}
	}

	// Aggregate patterns by base command and similar error messages
	groups := aggregatePatterns(allPatterns)

	return Result{
		SessionsScanned: sessions,
		TotalErrors:     totalErrors,
		Groups:          groups,
	}
}

// extractCommandEntries reads a JSONL file and returns an ordered list of
// command entries with their results. It correlates tool_use and tool_result
// entries by tool_use_id.
func extractCommandEntries(path string, cutoff time.Time) []commandEntry {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer func() { _ = f.Close() }()

	var entries []commandEntry
	// Map tool_use_id to index in entries slice for result matching
	idToIndex := make(map[string]int)

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 256*1024), 2*1024*1024)

	for scanner.Scan() {
		line := scanner.Bytes()

		var sl sessionLine
		if err := json.Unmarshal(line, &sl); err != nil {
			continue
		}

		if sl.Message == nil {
			continue
		}

		// Check timestamp
		if sl.Timestamp != "" {
			ts, err := time.Parse(time.RFC3339Nano, sl.Timestamp)
			if err == nil && ts.Before(cutoff) {
				continue
			}
		}

		var content []contentItem
		if err := json.Unmarshal(sl.Message.Content, &content); err != nil {
			continue
		}

		for _, item := range content {
			switch {
			case item.Type == "tool_use" && item.Name == "Bash":
				var bi bashInput
				if err := json.Unmarshal(item.Input, &bi); err != nil || bi.Command == "" {
					continue
				}
				entry := commandEntry{
					Command:   bi.Command,
					BaseCmd:   extractBaseCommand(bi.Command),
					ToolUseID: item.ID,
					Timestamp: sl.Timestamp,
				}
				idToIndex[item.ID] = len(entries)
				entries = append(entries, entry)

			case item.Type == "tool_result" && item.ToolUseID != "":
				idx, ok := idToIndex[item.ToolUseID]
				if !ok {
					continue
				}
				entries[idx].HasResult = true
				entries[idx].Output = item.Content.Value
				entries[idx].IsError = item.IsError || looksLikeError(item.Content.Value)
			}
		}
	}

	return entries
}

// extractBaseCommand parses a shell command string to extract the base command name.
func extractBaseCommand(cmd string) string {
	firstLine := cmd
	if idx := strings.IndexByte(firstLine, '\n'); idx >= 0 {
		firstLine = firstLine[:idx]
	}
	firstSegment := hook.ExtractFirstSegment(firstLine)
	_, _, bareCmd := hook.ParseSegment(firstSegment)
	base := hook.BaseCommand(bareCmd)

	if idx := strings.LastIndexByte(base, '/'); idx >= 0 {
		base = base[idx+1:]
	}
	base = strings.Trim(base, "'\"")
	return base
}

// looksLikeError checks if command output contains common error indicators.
func looksLikeError(output string) bool {
	for _, indicator := range errorIndicators {
		if strings.Contains(output, indicator) {
			return true
		}
	}
	return false
}

// detectPatterns finds error-correction sequences within a session's commands.
// An error-correction pattern is when:
// 1. Command A fails (is_error or error indicators in output)
// 2. Command B follows within the next 5 commands
// 3. Command B has the same base command and succeeds
func detectPatterns(entries []commandEntry) []ErrorPattern {
	var patterns []ErrorPattern

	for i, entry := range entries {
		if !entry.IsError {
			continue
		}

		// Look ahead up to 5 commands for a successful retry
		limit := i + 6
		if limit > len(entries) {
			limit = len(entries)
		}

		for j := i + 1; j < limit; j++ {
			fix := entries[j]
			if fix.BaseCmd != entry.BaseCmd {
				continue
			}
			if fix.IsError {
				continue
			}
			// Found a correction: same base command, succeeded
			patterns = append(patterns, ErrorPattern{
				BaseCommand:  entry.BaseCmd,
				ErrorCommand: entry.Command,
				ErrorOutput:  truncateOutput(entry.Output, 200),
				FixCommand:   fix.Command,
				Count:        1,
			})
			break
		}
	}

	return patterns
}

// truncateOutput truncates a string to maxLen characters, appending "..." if truncated.
func truncateOutput(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// aggregatePatterns groups error patterns by base command and merges similar ones.
func aggregatePatterns(byCmd map[string][]ErrorPattern) []PatternGroup {
	var groups []PatternGroup

	for baseCmd, patterns := range byCmd {
		// Deduplicate by grouping patterns with identical error commands
		merged := make(map[string]*ErrorPattern)
		for i := range patterns {
			p := patterns[i]
			key := p.ErrorCommand
			if existing, ok := merged[key]; ok {
				existing.Count++
			} else {
				cp := p
				merged[key] = &cp
			}
		}

		var groupPatterns []ErrorPattern
		total := 0
		for _, p := range merged {
			groupPatterns = append(groupPatterns, *p)
			total += p.Count
		}

		sort.Slice(groupPatterns, func(i, j int) bool {
			if groupPatterns[i].Count != groupPatterns[j].Count {
				return groupPatterns[i].Count > groupPatterns[j].Count
			}
			return groupPatterns[i].ErrorCommand < groupPatterns[j].ErrorCommand
		})

		groups = append(groups, PatternGroup{
			BaseCommand: baseCmd,
			Patterns:    groupPatterns,
			TotalCount:  total,
		})
	}

	sort.Slice(groups, func(i, j int) bool {
		if groups[i].TotalCount != groups[j].TotalCount {
			return groups[i].TotalCount > groups[j].TotalCount
		}
		return groups[i].BaseCommand < groups[j].BaseCommand
	})

	return groups
}

// printResult outputs the learn report to stdout.
func printResult(r Result) {
	fmt.Println("snip learn - CLI error pattern analysis")
	fmt.Println()
	fmt.Printf("Scanned: %d sessions, %d errors with corrections\n", r.SessionsScanned, r.TotalErrors)
	fmt.Println()

	if r.TotalErrors == 0 {
		fmt.Println("No error-correction patterns found.")
		return
	}

	fmt.Println("Common patterns:")
	for _, g := range r.Groups {
		fmt.Printf("  %s (%d occurrences)\n", g.BaseCommand, g.TotalCount)
		for _, p := range g.Patterns {
			errorSummary := summarizeError(p.ErrorOutput)
			fixSummary := summarizeFix(p.ErrorCommand, p.FixCommand)
			fmt.Printf("    Error: %s -> fixed by %s\n", errorSummary, fixSummary)
		}
		fmt.Println()
	}
}

// summarizeError extracts a short error description from output.
func summarizeError(output string) string {
	if output == "" {
		return "command failed"
	}

	// Look for the first line that contains an error indicator
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		for _, indicator := range errorIndicators {
			if strings.Contains(line, indicator) {
				if len(line) > 80 {
					return line[:80] + "..."
				}
				return line
			}
		}
	}

	// Fall back to first non-empty line
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			if len(line) > 80 {
				return line[:80] + "..."
			}
			return line
		}
	}

	return "command failed"
}

// summarizeFix compares the error and fix commands to describe the correction.
func summarizeFix(errorCmd, fixCmd string) string {
	if errorCmd == fixCmd {
		return "retrying the same command"
	}
	// Show the fix command, truncated
	if len(fixCmd) > 60 {
		return fixCmd[:60] + "..."
	}
	return fixCmd
}

// generateRules writes a .claude/rules/cli-corrections.md file.
func generateRules(r Result) error {
	if r.TotalErrors == 0 {
		fmt.Println("No patterns to generate rules from.")
		return nil
	}

	dir := filepath.Join(".claude", "rules")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create rules directory: %w", err)
	}

	path := filepath.Join(dir, "cli-corrections.md")

	var b strings.Builder
	b.WriteString("# CLI Corrections (auto-generated by snip learn)\n\n")
	b.WriteString("## Common patterns detected in your coding sessions\n\n")

	for _, g := range r.Groups {
		fmt.Fprintf(&b, "### %s failures\n", g.BaseCommand)

		for _, p := range g.Patterns {
			errorSummary := summarizeError(p.ErrorOutput)
			fmt.Fprintf(&b, "- When `%s` fails with: %s\n", g.BaseCommand, errorSummary)
			fmt.Fprintf(&b, "  - Fix: `%s`\n", truncateOutput(p.FixCommand, 120))
		}

		b.WriteString("\n")
	}

	if err := os.WriteFile(path, []byte(b.String()), 0644); err != nil {
		return fmt.Errorf("write rules file: %w", err)
	}

	fmt.Printf("Generated %s\n", path)
	return nil
}
