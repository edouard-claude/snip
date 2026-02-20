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
	hookPath := filepath.Join(dir, "snip-rewrite.sh")

	err := patchSettings(path, hookPath)
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
	if hook["command"] != hookPath {
		t.Errorf("command = %v, want %s", hook["command"], hookPath)
	}
}

func TestPatchSettingsExisting(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")
	hookPath := filepath.Join(dir, "snip-rewrite.sh")

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
	os.WriteFile(path, data, 0644)

	err := patchSettings(path, hookPath)
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
	hookPath := filepath.Join(dir, "snip-rewrite.sh")

	// Patch twice
	patchSettings(path, hookPath)
	patchSettings(path, hookPath)

	settings := readSettings(t, path)
	hooks := settings["hooks"].(map[string]any)
	preToolUse := hooks["PreToolUse"].([]any)

	// Should still be exactly 1 entry, not duplicated
	if len(preToolUse) != 1 {
		t.Errorf("expected 1 entry after double patch, got %d", len(preToolUse))
	}
}

func TestUnpatchSettings(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")
	hookPath := filepath.Join(dir, "snip-rewrite.sh")

	// Patch first
	patchSettings(path, hookPath)

	// Unpatch
	unpatchSettings(path)

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
	hookPath := filepath.Join(dir, "snip-rewrite.sh")

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
	os.WriteFile(path, data, 0644)

	// Add snip
	patchSettings(path, hookPath)

	// Verify both present
	settings := readSettings(t, path)
	preToolUse := settings["hooks"].(map[string]any)["PreToolUse"].([]any)
	if len(preToolUse) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(preToolUse))
	}

	// Unpatch — should remove snip but keep the Write hook
	unpatchSettings(path)

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
