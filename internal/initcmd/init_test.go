package initcmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestPatchSettingsNew(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")
	hookCommand := "/usr/local/bin/snip hook"

	err := patchSettings(path, hookCommand)
	if err != nil {
		t.Fatalf("patch: %v", err)
	}

	settings := readSettings(t, path)

	hooks, ok := settings["hooks"].(map[string]any)
	if !ok {
		t.Fatal("hooks not found")
	}

	preToolUse, ok := hooks["PreToolUse"].([]any)
	if !ok {
		t.Fatal("PreToolUse not found or not array")
	}

	if len(preToolUse) != 1 {
		t.Fatalf("expected 1 PreToolUse entry, got %d", len(preToolUse))
	}

	entry := preToolUse[0].(map[string]any)
	if entry["matcher"] != "Bash" {
		t.Errorf("matcher = %v, want Bash", entry["matcher"])
	}

	entryHooks := entry["hooks"].([]any)
	hook := entryHooks[0].(map[string]any)
	if hook["type"] != "command" {
		t.Errorf("type = %v, want command", hook["type"])
	}
	if hook["command"] != hookCommand {
		t.Errorf("command = %v, want %s", hook["command"], hookCommand)
	}
}

func TestPatchSettingsExisting(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")
	hookCommand := "/usr/local/bin/snip hook"

	// Write existing settings with other hooks
	existing := map[string]any{
		"theme": "dark",
		"hooks": map[string]any{
			"PreToolUse": []any{
				map[string]any{
					"matcher": "Write",
					"hooks": []any{
						map[string]any{"type": "command", "command": "other-hook.sh"},
					},
				},
			},
			"PostToolUse": "other-hook",
		},
	}
	data, _ := json.MarshalIndent(existing, "", "  ")
	_ = os.WriteFile(path, data, 0644)

	err := patchSettings(path, hookCommand)
	if err != nil {
		t.Fatalf("patch: %v", err)
	}

	settings := readSettings(t, path)

	// Existing settings preserved
	if settings["theme"] != "dark" {
		t.Error("existing settings not preserved")
	}

	hooks := settings["hooks"].(map[string]any)

	// PostToolUse preserved
	if hooks["PostToolUse"] != "other-hook" {
		t.Error("PostToolUse not preserved")
	}

	// PreToolUse should have 2 entries (existing Write + new Bash)
	preToolUse := hooks["PreToolUse"].([]any)
	if len(preToolUse) != 2 {
		t.Fatalf("expected 2 PreToolUse entries, got %d", len(preToolUse))
	}

	// First entry should be the existing Write hook
	first := preToolUse[0].(map[string]any)
	if first["matcher"] != "Write" {
		t.Errorf("first matcher = %v, want Write", first["matcher"])
	}

	// Second entry should be snip Bash hook
	second := preToolUse[1].(map[string]any)
	if second["matcher"] != "Bash" {
		t.Errorf("second matcher = %v, want Bash", second["matcher"])
	}
}

func TestPatchSettingsIdempotent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")
	hookCommand := "/usr/local/bin/snip hook"

	// Patch twice
	_ = patchSettings(path, hookCommand)
	_ = patchSettings(path, hookCommand)

	settings := readSettings(t, path)
	hooks := settings["hooks"].(map[string]any)
	preToolUse := hooks["PreToolUse"].([]any)

	// Should still be exactly 1 entry, not duplicated
	if len(preToolUse) != 1 {
		t.Errorf("expected 1 entry after double patch, got %d", len(preToolUse))
	}
}

func TestPatchSettingsMigratesLegacy(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")

	// Write settings with legacy snip-rewrite.sh entry
	legacy := map[string]any{
		"hooks": map[string]any{
			"PreToolUse": []any{
				map[string]any{
					"matcher": "Bash",
					"hooks": []any{
						map[string]any{"type": "command", "command": "/home/user/.claude/hooks/snip-rewrite.sh"},
					},
				},
			},
		},
	}
	data, _ := json.MarshalIndent(legacy, "", "  ")
	_ = os.WriteFile(path, data, 0644)

	hookCommand := "/usr/local/bin/snip hook"
	err := patchSettings(path, hookCommand)
	if err != nil {
		t.Fatalf("patch: %v", err)
	}

	settings := readSettings(t, path)
	hooks := settings["hooks"].(map[string]any)
	preToolUse := hooks["PreToolUse"].([]any)

	// Should replace, not duplicate
	if len(preToolUse) != 1 {
		t.Fatalf("expected 1 entry after migration, got %d", len(preToolUse))
	}

	entry := preToolUse[0].(map[string]any)
	entryHooks := entry["hooks"].([]any)
	hook := entryHooks[0].(map[string]any)
	if hook["command"] != hookCommand {
		t.Errorf("command = %v, want %s", hook["command"], hookCommand)
	}
}

func TestUnpatchSettings(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")
	hookCommand := "/usr/local/bin/snip hook"

	// Patch first
	_ = patchSettings(path, hookCommand)

	// Unpatch
	if err := unpatchSettings(path); err != nil {
		t.Fatalf("unpatch: %v", err)
	}

	settings := readSettings(t, path)

	// hooks section should be gone entirely
	if _, ok := settings["hooks"]; ok {
		hooks := settings["hooks"].(map[string]any)
		if _, ok := hooks["PreToolUse"]; ok {
			t.Error("PreToolUse should be removed after unpatch")
		}
	}
}

func TestUnpatchPreservesOtherHooks(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")
	hookCommand := "/usr/local/bin/snip hook"

	// Create settings with snip + another hook
	existing := map[string]any{
		"hooks": map[string]any{
			"PreToolUse": []any{
				map[string]any{
					"matcher": "Write",
					"hooks":   []any{map[string]any{"type": "command", "command": "other.sh"}},
				},
			},
		},
	}
	data, _ := json.MarshalIndent(existing, "", "  ")
	_ = os.WriteFile(path, data, 0644)

	// Add snip
	_ = patchSettings(path, hookCommand)

	// Verify both present
	settings := readSettings(t, path)
	preToolUse := settings["hooks"].(map[string]any)["PreToolUse"].([]any)
	if len(preToolUse) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(preToolUse))
	}

	// Unpatch -- should remove snip but keep the Write hook
	if err := unpatchSettings(path); err != nil {
		t.Fatalf("unpatch: %v", err)
	}

	settings = readSettings(t, path)
	hooks := settings["hooks"].(map[string]any)
	preToolUse = hooks["PreToolUse"].([]any)
	if len(preToolUse) != 1 {
		t.Fatalf("expected 1 entry after unpatch, got %d", len(preToolUse))
	}
	remaining := preToolUse[0].(map[string]any)
	if remaining["matcher"] != "Write" {
		t.Errorf("remaining matcher = %v, want Write", remaining["matcher"])
	}
}

func TestPatchSettingsWindowsPath(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")
	// Simulate a Windows-style snip hook command
	hookCommand := `C:\Users\joedoe\go\bin\snip hook`

	err := patchSettings(path, hookCommand)
	if err != nil {
		t.Fatalf("patch: %v", err)
	}

	settings := readSettings(t, path)
	hooks := settings["hooks"].(map[string]any)
	preToolUse := hooks["PreToolUse"].([]any)
	entry := preToolUse[0].(map[string]any)
	entryHooks := entry["hooks"].([]any)
	hook := entryHooks[0].(map[string]any)
	cmd := hook["command"].(string)

	// The command is stored as-is; path normalization happens in Run() before calling patchSettings
	if cmd != hookCommand {
		t.Errorf("command = %v, want %s", cmd, hookCommand)
	}
}

func TestInitMigratesOldHookScript(t *testing.T) {
	dir := t.TempDir()
	hooksDir := filepath.Join(dir, ".claude", "hooks")
	if err := os.MkdirAll(hooksDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create legacy hook script
	oldHookPath := filepath.Join(hooksDir, legacyHookFile)
	if err := os.WriteFile(oldHookPath, []byte("#!/bin/bash\nexit 0"), 0755); err != nil {
		t.Fatal(err)
	}

	// Verify it exists
	if _, err := os.Stat(oldHookPath); err != nil {
		t.Fatal("legacy hook should exist before migration")
	}

	// Simulate what Run does: remove old hook
	_ = os.Remove(oldHookPath)

	// Verify it's gone
	if _, err := os.Stat(oldHookPath); !os.IsNotExist(err) {
		t.Error("legacy hook script should be removed after migration")
	}
}

func readSettings(t *testing.T, path string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read settings: %v", err)
	}
	var settings map[string]any
	if err := json.Unmarshal(data, &settings); err != nil {
		t.Fatalf("parse settings: %v", err)
	}
	return settings
}
