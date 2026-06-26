package initcmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	// hookIdentifier is used to detect snip entries in settings/hooks JSON.
	hookIdentifier = "snip hook"
	// legacyHookFile is the old bash hook script filename (for migration).
	legacyHookFile = "snip-rewrite.sh"
)

// validAgents lists all supported agent names.
var validAgents = []string{
	"claude-code", "cursor", "codex", "pi", "windsurf", "cline",
	"copilot", "gemini", "kilocode", "antigravity",
}

// promptAgentFiles maps prompt-injection agents to their target file paths.
// Paths may include subdirectories (created automatically on init).
var promptAgentFiles = map[string]string{
	"codex":       "AGENTS.md",
	"windsurf":    ".windsurfrules",
	"cline":       ".clinerules",
	"copilot":     filepath.Join(".github", "copilot-instructions.md"),
	"gemini":      "GEMINI.md",
	"kilocode":    filepath.Join(".kilocode", "rules", "snip-rules.md"),
	"antigravity": filepath.Join(".agents", "rules", "snip-rules.md"),
}

// parseAgent extracts the --agent value from args.
// Returns the agent name and the remaining args (without --agent).
func parseAgent(args []string) (string, []string) {
	agent := "claude-code"
	var remaining []string

	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--agent" && i+1 < len(args) {
			agent = args[i+1]
			i++ // skip value
		} else if strings.HasPrefix(arg, "--agent=") {
			agent = strings.TrimPrefix(arg, "--agent=")
		} else {
			remaining = append(remaining, arg)
		}
	}
	return agent, remaining
}

// parseMode extracts the --mode value from args. Empty string means no
// --mode flag was provided; the caller decides the per-agent default.
func parseMode(args []string) (string, []string) {
	mode := ""
	var remaining []string

	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--mode" && i+1 < len(args) {
			mode = args[i+1]
			i++
		} else if strings.HasPrefix(arg, "--mode=") {
			mode = strings.TrimPrefix(arg, "--mode=")
		} else {
			remaining = append(remaining, arg)
		}
	}
	return mode, remaining
}

// isValidAgent checks if the given agent name is supported.
func isValidAgent(name string) bool {
	for _, a := range validAgents {
		if a == name {
			return true
		}
	}
	return false
}

// Run installs the snip integration for the specified agent.
func Run(args []string) error {
	agent, remaining := parseAgent(args)
	mode, remaining := parseMode(remaining)

	if !isValidAgent(agent) {
		return fmt.Errorf("unknown agent %q, valid agents: %s", agent, strings.Join(validAgents, ", "))
	}

	if mode != "" && mode != "hook" && mode != "prompt" {
		return fmt.Errorf("unknown mode %q, valid modes: hook, prompt", mode)
	}

	for _, arg := range remaining {
		if arg == "--uninstall" {
			return Uninstall(agent)
		}
	}

	// Resolve the absolute path of the running snip binary.
	snipBin, err := resolveSnipBin()
	if err != nil {
		return err
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("get home dir: %w", err)
	}

	// Create filter directory (shared by all agents)
	filterDir := filepath.Join(home, ".config", "snip", "filters")
	if err := os.MkdirAll(filterDir, 0755); err != nil {
		return fmt.Errorf("create filter dir: %w", err)
	}

	switch agent {
	case "claude-code":
		return initClaudeCode(snipBin, home, filterDir)
	case "cursor":
		return initCursor(snipBin, home, filterDir)
	case "codex":
		// Codex defaults to runtime hooks; --mode prompt selects the legacy
		// AGENTS.md prompt-injection path for older Codex releases.
		if mode == "prompt" {
			return initPromptAgent(agent, snipBin, filterDir)
		}
		return initCodex(snipBin, home, filterDir)
	case "pi":
		return initPi(snipBin, home, filterDir)
	case "copilot":
		// Copilot CLI defaults to its GA runtime hook; --mode prompt selects the
		// legacy .github/copilot-instructions.md prompt-injection path.
		if mode == "prompt" {
			return initPromptAgent(agent, snipBin, filterDir)
		}
		return initCopilotHook(snipBin, home, filterDir)
	case "windsurf", "cline", "gemini", "kilocode", "antigravity":
		return initPromptAgent(agent, snipBin, filterDir)
	}
	return nil
}

// resolveSnipBin returns the absolute, symlink-resolved, slash-normalized path
// to the running snip binary.
func resolveSnipBin() (string, error) {
	snipBin, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("resolve executable: %w", err)
	}
	snipBin, err = filepath.EvalSymlinks(snipBin)
	if err != nil {
		return "", fmt.Errorf("eval symlinks: %w", err)
	}
	snipBin, err = filepath.Abs(snipBin)
	if err != nil {
		return "", fmt.Errorf("abs path: %w", err)
	}
	return filepath.ToSlash(snipBin), nil
}

// initClaudeCode installs the snip hook for Claude Code.
func initClaudeCode(snipBin, home, filterDir string) error {
	// Migrate: remove old bash hook script if present
	oldHookPath := filepath.Join(home, ".claude", "hooks", legacyHookFile)
	if _, err := os.Stat(oldHookPath); err == nil {
		_ = os.Remove(oldHookPath)
		fmt.Printf("  migrated: removed old %s\n", legacyHookFile)
	}

	hookCommand := snipBin + " hook"
	settingsPath := filepath.Join(home, ".claude", "settings.json")
	if err := patchClaudeSettings(settingsPath, hookCommand); err != nil {
		return fmt.Errorf("patch settings: %w", err)
	}

	fmt.Println("snip init complete:")
	fmt.Printf("  agent: claude-code\n")
	fmt.Printf("  hook: %s\n", hookCommand)
	fmt.Printf("  filters: %s\n", filterDir)
	fmt.Printf("  settings: %s\n", settingsPath)
	return nil
}

// initCursor installs the snip hook for Cursor.
func initCursor(snipBin, home, filterDir string) error {
	hookCommand := snipBin + " hook"
	hooksPath := filepath.Join(home, ".cursor", "hooks.json")
	if err := patchCursorHooks(hooksPath, hookCommand); err != nil {
		return fmt.Errorf("patch cursor hooks: %w", err)
	}

	fmt.Println("snip init complete:")
	fmt.Printf("  agent: cursor\n")
	fmt.Printf("  hook: %s\n", hookCommand)
	fmt.Printf("  filters: %s\n", filterDir)
	fmt.Printf("  hooks: %s\n", hooksPath)
	return nil
}

// initPromptAgent creates a prompt-injection file for prompt-based agents.
func initPromptAgent(agent, snipBin, filterDir string) error {
	filename := promptAgentFiles[agent]
	targetPath := filepath.Join(".", filename)

	// Create parent directories if needed (e.g. .kilocode/rules/)
	if dir := filepath.Dir(targetPath); dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("create dir %s: %w", dir, err)
		}
	}

	content := promptContent(snipBin)
	if err := os.WriteFile(targetPath, []byte(content), 0644); err != nil {
		return fmt.Errorf("write %s: %w", filename, err)
	}

	fmt.Println("snip init complete:")
	fmt.Printf("  agent: %s\n", agent)
	fmt.Printf("  file: %s\n", targetPath)
	fmt.Printf("  filters: %s\n", filterDir)
	return nil
}

// promptContent returns the markdown instructions for prompt-injection agents.
func promptContent(snipBin string) string {
	return fmt.Sprintf(`# Snip - CLI Token Optimizer

When executing shell commands, always prefix them with the snip proxy to reduce token usage.

## Usage

Instead of running commands directly:
`+"```"+`
git status
go test ./...
`+"```"+`

Prefix with snip:
`+"```"+`
%s -- git status
%s -- go test ./...
`+"```"+`

This applies to all shell commands. Snip filters verbose output while preserving errors and essential information.
`, snipBin, snipBin)
}

// Uninstall removes snip integration for the given agent.
func Uninstall(agent string) error {
	if !isValidAgent(agent) {
		return fmt.Errorf("unknown agent %q, valid agents: %s", agent, strings.Join(validAgents, ", "))
	}

	switch agent {
	case "claude-code":
		return uninstallClaudeCode()
	case "cursor":
		return uninstallCursor()
	case "codex":
		return uninstallCodex()
	case "pi":
		return uninstallPi()
	case "copilot":
		return uninstallCopilot()
	case "windsurf", "cline", "gemini", "kilocode", "antigravity":
		return uninstallPromptAgent(agent)
	}
	return nil
}

// uninstallClaudeCode removes snip integration from Claude Code.
func uninstallClaudeCode() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("get home dir: %w", err)
	}

	// Remove legacy bash script if present
	oldHookPath := filepath.Join(home, ".claude", "hooks", legacyHookFile)
	_ = os.Remove(oldHookPath)

	settingsPath := filepath.Join(home, ".claude", "settings.json")
	if err := unpatchClaudeSettings(settingsPath); err != nil {
		return fmt.Errorf("unpatch settings: %w", err)
	}

	fmt.Println("snip uninstalled (claude-code)")
	return nil
}

// uninstallCursor removes snip integration from Cursor.
func uninstallCursor() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("get home dir: %w", err)
	}

	hooksPath := filepath.Join(home, ".cursor", "hooks.json")
	if err := unpatchCursorHooks(hooksPath); err != nil {
		return fmt.Errorf("unpatch cursor hooks: %w", err)
	}

	fmt.Println("snip uninstalled (cursor)")
	return nil
}

// uninstallPromptAgent removes the prompt-injection file for the given agent.
// For agents with subdirectory paths, it also removes empty parent directories.
func uninstallPromptAgent(agent string) error {
	filename := promptAgentFiles[agent]
	targetPath := filepath.Join(".", filename)

	if err := os.Remove(targetPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove %s: %w", filename, err)
	}

	// Clean up empty parent directories (best-effort, stops at ".")
	for dir := filepath.Dir(targetPath); dir != "." && dir != string(filepath.Separator); dir = filepath.Dir(dir) {
		if err := os.Remove(dir); err != nil {
			break // directory not empty or other error, stop
		}
	}

	fmt.Printf("snip uninstalled (%s)\n", agent)
	return nil
}

// patchClaudeSettings adds the snip hook to Claude Code settings.json.
// hookCommand is the full command string (e.g. "/usr/local/bin/snip hook").
func patchClaudeSettings(path, hookCommand string) error {
	var settings map[string]any

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			settings = make(map[string]any)
		} else {
			return fmt.Errorf("read settings: %w", err)
		}
	} else {
		// Backup (best-effort)
		backupPath := path + ".bak"
		_ = os.WriteFile(backupPath, data, 0644)

		if err := json.Unmarshal(data, &settings); err != nil {
			return fmt.Errorf("parse settings: %w", err)
		}
	}

	snipHookEntry := map[string]any{
		"type":    "command",
		"command": hookCommand,
	}

	snipMatcher := map[string]any{
		"matcher": "Bash",
		"hooks":   []any{snipHookEntry},
	}

	// Get or create hooks section
	hooks, _ := settings["hooks"].(map[string]any)
	if hooks == nil {
		hooks = make(map[string]any)
	}

	// Get existing PreToolUse array or create new one
	var preToolUse []any
	if existing, ok := hooks["PreToolUse"]; ok {
		if arr, ok := existing.([]any); ok {
			preToolUse = arr
		}
	}

	// Check if snip hook already exists (idempotent)
	found := false
	for i, entry := range preToolUse {
		if isSnipEntry(entry) {
			preToolUse[i] = snipMatcher // Update in place
			found = true
			break
		}
	}
	if !found {
		preToolUse = append(preToolUse, snipMatcher)
	}

	hooks["PreToolUse"] = preToolUse
	settings["hooks"] = hooks

	// Ensure parent directory exists for fresh installations
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("create settings dir: %w", err)
	}

	out, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal settings: %w", err)
	}

	return os.WriteFile(path, out, 0644)
}

func unpatchClaudeSettings(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read settings: %w", err)
	}
	var settings map[string]any
	if err := json.Unmarshal(data, &settings); err != nil {
		return fmt.Errorf("parse settings: %w", err)
	}
	hooks, _ := settings["hooks"].(map[string]any)
	if hooks == nil {
		return nil
	}

	existing, ok := hooks["PreToolUse"]
	if !ok {
		return nil
	}
	arr, ok := existing.([]any)
	if !ok {
		return nil
	}

	// Remove snip entries
	var filtered []any
	for _, entry := range arr {
		if !isSnipEntry(entry) {
			filtered = append(filtered, entry)
		}
	}

	if len(filtered) == 0 {
		delete(hooks, "PreToolUse")
	} else {
		hooks["PreToolUse"] = filtered
	}
	if len(hooks) == 0 {
		delete(settings, "hooks")
	}

	out, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal settings: %w", err)
	}

	return os.WriteFile(path, out, 0644)
}

// isSnipEntry checks if a hook array entry contains a snip hook.
// Matches both the new "snip hook" command and the legacy "snip-rewrite.sh" path.
// Detection relies on the "command" field inside hook entries, which is the only
// format snip has ever written. If a third-party tool installed hooks using a
// different field name, those entries would not be detected here.
func isSnipEntry(entry any) bool {
	m, ok := entry.(map[string]any)
	if !ok {
		return false
	}
	hooksRaw, ok := m["hooks"]
	if !ok {
		return false
	}
	hooksArr, ok := hooksRaw.([]any)
	if !ok {
		return false
	}
	for _, h := range hooksArr {
		hm, ok := h.(map[string]any)
		if !ok {
			continue
		}
		cmd, _ := hm["command"].(string)
		if strings.Contains(cmd, hookIdentifier) || strings.Contains(cmd, legacyHookFile) {
			return true
		}
	}
	return false
}

// isSnipCursorEntry checks if a Cursor beforeShellExecution entry is a snip hook.
func isSnipCursorEntry(entry any) bool {
	m, ok := entry.(map[string]any)
	if !ok {
		return false
	}
	hooksRaw, ok := m["hooks"]
	if !ok {
		return false
	}
	hooksArr, ok := hooksRaw.([]any)
	if !ok {
		return false
	}
	for _, h := range hooksArr {
		hm, ok := h.(map[string]any)
		if !ok {
			continue
		}
		cmd, _ := hm["command"].(string)
		if strings.Contains(cmd, hookIdentifier) {
			return true
		}
	}
	return false
}

// patchCursorHooks adds the snip hook to Cursor's hooks.json.
func patchCursorHooks(path, hookCommand string) error {
	var config map[string]any

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			config = make(map[string]any)
		} else {
			return fmt.Errorf("read hooks: %w", err)
		}
	} else {
		// Backup (best-effort)
		backupPath := path + ".bak"
		_ = os.WriteFile(backupPath, data, 0644)

		if err := json.Unmarshal(data, &config); err != nil {
			return fmt.Errorf("parse hooks: %w", err)
		}
	}

	snipHookEntry := map[string]any{
		"type":    "command",
		"command": hookCommand,
	}

	snipMatcher := map[string]any{
		"matcher": ".*",
		"hooks":   []any{snipHookEntry},
	}

	// Get or create hooks section
	hooks, _ := config["hooks"].(map[string]any)
	if hooks == nil {
		hooks = make(map[string]any)
	}

	// Get existing beforeShellExecution array or create new one
	var beforeShell []any
	if existing, ok := hooks["beforeShellExecution"]; ok {
		if arr, ok := existing.([]any); ok {
			beforeShell = arr
		}
	}

	// Check if snip hook already exists (idempotent)
	found := false
	for i, entry := range beforeShell {
		if isSnipCursorEntry(entry) {
			beforeShell[i] = snipMatcher
			found = true
			break
		}
	}
	if !found {
		beforeShell = append(beforeShell, snipMatcher)
	}

	hooks["beforeShellExecution"] = beforeShell
	config["hooks"] = hooks

	// Ensure parent directory exists
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("create hooks dir: %w", err)
	}

	out, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal hooks: %w", err)
	}

	return os.WriteFile(path, out, 0644)
}

// unpatchCursorHooks removes the snip hook from Cursor's hooks.json.
func unpatchCursorHooks(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read hooks: %w", err)
	}
	var config map[string]any
	if err := json.Unmarshal(data, &config); err != nil {
		return fmt.Errorf("parse hooks: %w", err)
	}
	hooks, _ := config["hooks"].(map[string]any)
	if hooks == nil {
		return nil
	}

	existing, ok := hooks["beforeShellExecution"]
	if !ok {
		return nil
	}
	arr, ok := existing.([]any)
	if !ok {
		return nil
	}

	// Remove snip entries
	var filtered []any
	for _, entry := range arr {
		if !isSnipCursorEntry(entry) {
			filtered = append(filtered, entry)
		}
	}

	if len(filtered) == 0 {
		delete(hooks, "beforeShellExecution")
	} else {
		hooks["beforeShellExecution"] = filtered
	}
	if len(hooks) == 0 {
		delete(config, "hooks")
	}

	out, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal hooks: %w", err)
	}

	return os.WriteFile(path, out, 0644)
}
