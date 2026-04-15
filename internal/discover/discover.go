package discover

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/edouard-claude/snip/internal/config"
	"github.com/edouard-claude/snip/internal/display"
	"github.com/edouard-claude/snip/internal/filter"
	"github.com/edouard-claude/snip/internal/hook"
	"github.com/edouard-claude/snip/internal/utils"
)

// sessionLine represents a single JSONL entry from a Claude Code session file.
type sessionLine struct {
	Type    string `json:"type"`
	Message *struct {
		Role    string            `json:"role"`
		Content json.RawMessage   `json:"content"`
	} `json:"message,omitempty"`
	Timestamp string `json:"timestamp,omitempty"`
}

// contentItem represents one element of the message content array.
type contentItem struct {
	Type  string          `json:"type"`
	Name  string          `json:"name"`
	Input json.RawMessage `json:"input,omitempty"`
}

// bashInput holds the command field from a Bash tool_use input.
type bashInput struct {
	Command string `json:"command"`
}

// CommandStat tracks command occurrence counts.
type CommandStat struct {
	Name  string
	Count int
}

// Result holds the discover analysis output.
type Result struct {
	SessionsScanned  int
	TotalCommands    int
	Supported        []CommandStat
	Unsupported      []CommandStat
	SupportedCount   int
	UnsupportedCount int
}

// Options configures the discover scan.
type Options struct {
	All   bool
	Since int // days
}

// Run executes the discover command with the given CLI args.
func Run(args []string) error {
	opts := parseArgs(args)

	cfg, err := config.Load()
	if err != nil {
		cfg = config.DefaultConfig()
	}

	filters, err := filter.LoadAll(cfg.Filters.Dirs())
	if err != nil {
		return fmt.Errorf("load filters: %w", err)
	}

	registry := filter.NewRegistry(filters)
	supportedCmds := registry.Commands()
	cmdSet := make(map[string]struct{}, len(supportedCmds))
	for _, c := range supportedCmds {
		cmdSet[c] = struct{}{}
	}

	projectDirs, err := findProjectDirs(opts)
	if err != nil {
		return fmt.Errorf("find project dirs: %w", err)
	}

	if len(projectDirs) == 0 {
		fmt.Fprintln(os.Stderr, "snip discover: no Claude Code session directories found")
		return nil
	}

	cutoff := time.Now().AddDate(0, 0, -opts.Since)
	result := scan(projectDirs, cmdSet, cutoff)
	printResult(result)
	return nil
}

// parseArgs extracts --all and --since flags from args.
func parseArgs(args []string) Options {
	opts := Options{Since: 7}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--all":
			opts.All = true
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
// project directory name format: slashes become dashes, leading slash stripped.
func cwdToProjectName(cwd string) string {
	// Claude Code uses the absolute path with "/" replaced by "-"
	// e.g. /Users/edouard/Code/go/snip -> -Users-edouard-Code-go-snip
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

	// Current project only
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
			// Check for subagent files inside session directories
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

// scan processes all session files and classifies commands.
func scan(projectDirs []string, supportedCmds map[string]struct{}, cutoff time.Time) Result {
	files := findSessionFiles(projectDirs)

	supported := make(map[string]int)
	unsupported := make(map[string]int)
	sessions := 0
	totalCmds := 0

	for _, file := range files {
		commands := extractCommands(file, cutoff)
		if len(commands) > 0 {
			sessions++
		}
		for _, cmd := range commands {
			totalCmds++
			if _, ok := supportedCmds[cmd]; ok {
				supported[cmd]++
			} else {
				unsupported[cmd]++
			}
		}
	}

	return Result{
		SessionsScanned:  sessions,
		TotalCommands:    totalCmds,
		Supported:        mapToStats(supported),
		Unsupported:      mapToStats(unsupported),
		SupportedCount:   sumMap(supported),
		UnsupportedCount: sumMap(unsupported),
	}
}

// extractCommands reads a JSONL file and returns base command names from
// Bash tool_use entries. Malformed lines are silently skipped.
func extractCommands(path string, cutoff time.Time) []string {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer func() { _ = f.Close() }()

	var commands []string
	scanner := bufio.NewScanner(f)
	// Increase buffer for potentially large JSONL lines
	scanner.Buffer(make([]byte, 0, 256*1024), 2*1024*1024)

	for scanner.Scan() {
		line := scanner.Bytes()
		cmd := parseBashCommand(line, cutoff)
		if cmd != "" {
			commands = append(commands, cmd)
		}
	}
	return commands
}

// parseBashCommand extracts the base command from a JSONL line if it represents
// a Bash tool_use entry. Returns "" if the line is not relevant.
func parseBashCommand(line []byte, cutoff time.Time) string {
	var entry sessionLine
	if err := json.Unmarshal(line, &entry); err != nil {
		return ""
	}

	if entry.Type != "assistant" || entry.Message == nil || entry.Message.Role != "assistant" {
		return ""
	}

	// Check timestamp
	if entry.Timestamp != "" {
		ts, err := time.Parse(time.RFC3339Nano, entry.Timestamp)
		if err == nil && ts.Before(cutoff) {
			return ""
		}
	}

	// Parse content array
	var content []contentItem
	if err := json.Unmarshal(entry.Message.Content, &content); err != nil {
		return ""
	}

	for _, item := range content {
		if item.Type != "tool_use" || item.Name != "Bash" {
			continue
		}
		var bi bashInput
		if err := json.Unmarshal(item.Input, &bi); err != nil {
			continue
		}
		if bi.Command == "" {
			continue
		}
		return extractBaseCommand(bi.Command)
	}
	return ""
}

// extractBaseCommand parses a shell command string to extract the base command
// name, handling env vars, pipes, semicolons, and newlines.
func extractBaseCommand(cmd string) string {
	// Use the hook package's parsing utilities for consistency
	firstLine := cmd
	if idx := strings.IndexByte(firstLine, '\n'); idx >= 0 {
		firstLine = firstLine[:idx]
	}
	firstSegment := hook.ExtractFirstSegment(firstLine)
	_, _, bareCmd := hook.ParseSegment(firstSegment)
	base := hook.BaseCommand(bareCmd)

	// Strip path prefix (e.g. /usr/bin/git -> git)
	if idx := strings.LastIndexByte(base, '/'); idx >= 0 {
		base = base[idx+1:]
	}

	// Strip quotes
	base = strings.Trim(base, "'\"")

	return base
}

// mapToStats converts a map[string]int to a sorted slice of CommandStat.
func mapToStats(m map[string]int) []CommandStat {
	stats := make([]CommandStat, 0, len(m))
	for name, count := range m {
		stats = append(stats, CommandStat{Name: name, Count: count})
	}
	sort.Slice(stats, func(i, j int) bool {
		if stats[i].Count != stats[j].Count {
			return stats[i].Count > stats[j].Count
		}
		return stats[i].Name < stats[j].Name
	})
	return stats
}

func sumMap(m map[string]int) int {
	total := 0
	for _, v := range m {
		total += v
	}
	return total
}

// printResult outputs the discover report to stdout.
func printResult(r Result) {
	tty := display.IsTerminal()

	fmt.Println()
	if tty {
		fmt.Println(display.HeaderStyle.Render("  snip — Discover Report"))
		fmt.Println(display.DimStyle.Render("  " + display.FormatSeparator(30)))
	} else {
		fmt.Println("  snip — Discover Report")
		fmt.Println("  " + display.FormatSeparator(30))
	}
	fmt.Println()

	if r.TotalCommands == 0 {
		fmt.Println("  No Bash commands found in the scanned sessions.")
		return
	}

	supportedPct := float64(r.SupportedCount) / float64(r.TotalCommands) * 100

	printKPI := func(label, value string, styled bool) {
		if tty {
			styledValue := value
			if !styled {
				styledValue = display.StatStyle.Render(value)
			}
			fmt.Printf("  %s  %s\n", display.DimStyle.Render(fmt.Sprintf("%-20s", label)), styledValue)
		} else {
			fmt.Printf("  %-20s  %s\n", label, value)
		}
	}

	printKPI("Sessions scanned", fmt.Sprintf("%d", r.SessionsScanned), false)
	printKPI("Commands found", fmt.Sprintf("%d", r.TotalCommands), false)
	printKPI("Filter coverage", display.ColorSavings(supportedPct), true)

	bar := display.ColorBar(r.SupportedCount, r.TotalCommands, 20)
	fmt.Println()
	if tty {
		fmt.Printf("  %s %s\n", bar, display.DimStyle.Render(fmt.Sprintf("%.0f%%", supportedPct)))
	} else {
		fmt.Printf("  %s %.0f%%\n", bar, supportedPct)
	}
	fmt.Println()

	// Subtract 9 to leave room for the Count column and separator; prevents
	// long Windows paths from overflowing into the adjacent column.
	maxCmd := display.TerminalWidth() - 9
	if maxCmd < 20 {
		maxCmd = 20
	}

	printSection := func(title string, count int, pct float64, stats []CommandStat) {
		if tty {
			fmt.Println(display.DimStyle.Render(fmt.Sprintf("  %s — %d commands (%.0f%%)", title, count, pct)))
		} else {
			fmt.Printf("  %s — %d commands (%.0f%%)\n", title, count, pct)
		}
		fmt.Println()

		if len(stats) > 0 {
			rows := make([][]string, 0, len(stats))
			for _, s := range stats {
				rows = append(rows, []string{utils.Truncate(s.Name, maxCmd), fmt.Sprintf("%d", s.Count)})
			}
			fmt.Print(display.FormatTable([]string{"Command", "Count"}, rows))
			fmt.Println()
		}
	}

	printSection("Supported", r.SupportedCount, supportedPct, r.Supported)
	printSection("Unsupported", r.UnsupportedCount, 100-supportedPct, r.Unsupported)

	if tty {
		fmt.Println(display.StatStyle.Render(fmt.Sprintf("  %.0f%% of your commands already have snip filters.", supportedPct)))
	} else {
		fmt.Printf("  %.0f%% of your commands already have snip filters.\n", supportedPct)
	}
	fmt.Println()
}
