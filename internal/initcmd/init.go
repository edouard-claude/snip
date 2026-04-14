package initcmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	// hookIdentifier is used to detect snip entries in settings.json.
	hookIdentifier = "snip hook"
	// legacyHookFile is the old bash hook script filename (for migration).
	legacyHookFile = "snip-rewrite.sh"
)

// Run installs the snip integration for Claude Code.
func Run(args []string) error {
	for _, arg := range args {
		if arg == "--uninstall" {
			return Uninstall()
		}
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("get home dir: %w", err)
	}

	// Resolve the absolute path of the running snip binary.
	snipBin, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolve executable: %w", err)
	}
	snipBin, err = filepath.EvalSymlinks(snipBin)
	if err != nil {
		return fmt.Errorf("eval symlinks: %w", err)
	}
	snipBin, err = filepath.Abs(snipBin)
	if err != nil {
		return fmt.Errorf("abs path: %w", err)
	}
	snipBin = filepath.ToSlash(snipBin)

	// 1. Create filter directory
	filterDir := filepath.Join(home, ".config", "snip", "filters")
	if err := os.MkdirAll(filterDir, 0755); err != nil {
		return fmt.Errorf("create filter dir: %w", err)
	}

	// 2. Migrate: remove old bash hook script if present
	oldHookPath := filepath.Join(home, ".claude", "hooks", legacyHookFile)
	if _, err := os.Stat(oldHookPath); err == nil {
		_ = os.Remove(oldHookPath)
		fmt.Printf("  migrated: removed old %s\n", legacyHookFile)
	}

	// 3. Patch settings.json with "snip hook" command
	hookCommand := snipBin + " hook"
	settingsPath := filepath.Join(home, ".claude", "settings.json")
	if err := patchSettings(settingsPath, hookCommand); err != nil {
		return fmt.Errorf("patch settings: %w", err)
	}

	fmt.Println("snip init complete:")
	fmt.Printf("  hook: %s\n", hookCommand)
	fmt.Printf("  filters: %s\n", filterDir)
	fmt.Printf("  settings: %s\n", settingsPath)
	return nil
}

// Uninstall removes snip integration.
func Uninstall() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("get home dir: %w", err)
	}

	// Remove legacy bash script if present
	oldHookPath := filepath.Join(home, ".claude", "hooks", legacyHookFile)
	_ = os.Remove(oldHookPath)

	// Remove hook entry from settings.json
	settingsPath := filepath.Join(home, ".claude", "settings.json")
	if err := unpatchSettings(settingsPath); err != nil {
		return fmt.Errorf("unpatch settings: %w", err)
	}

	fmt.Println("snip uninstalled")
	return nil
}

// patchSettings adds the snip hook to Claude Code settings.json.
// hookCommand is the full command string (e.g. "/usr/local/bin/snip hook").
func patchSettings(path, hookCommand string) error {
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

func unpatchSettings(path string) error {
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

// isSnipEntry checks if a PreToolUse entry is a snip hook.
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
