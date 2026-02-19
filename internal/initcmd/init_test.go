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

	err := patchSettings(path)
	if err != nil {
		t.Fatalf("patch: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	var settings map[string]any
	if err := json.Unmarshal(data, &settings); err != nil {
		t.Fatal(err)
	}

	hooks, ok := settings["hooks"].(map[string]any)
	if !ok {
		t.Fatal("hooks not found")
	}
	if _, ok := hooks["PreToolUse"]; !ok {
		t.Error("PreToolUse hook not found")
	}
}

func TestPatchSettingsExisting(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")

	// Write existing settings
	existing := map[string]any{
		"theme": "dark",
		"hooks": map[string]any{
			"PostToolUse": "other-hook",
		},
	}
	data, _ := json.Marshal(existing)
	os.WriteFile(path, data, 0644)

	err := patchSettings(path)
	if err != nil {
		t.Fatalf("patch: %v", err)
	}

	result, _ := os.ReadFile(path)
	var settings map[string]any
	json.Unmarshal(result, &settings)

	// Existing settings preserved
	if settings["theme"] != "dark" {
		t.Error("existing settings not preserved")
	}

	// Hook added
	hooks := settings["hooks"].(map[string]any)
	if _, ok := hooks["PreToolUse"]; !ok {
		t.Error("PreToolUse not added")
	}
}

func TestPatchSettingsIdempotent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")

	// Patch twice
	patchSettings(path)
	patchSettings(path)

	data, _ := os.ReadFile(path)
	var settings map[string]any
	json.Unmarshal(data, &settings)

	hooks := settings["hooks"].(map[string]any)
	if _, ok := hooks["PreToolUse"]; !ok {
		t.Error("PreToolUse hook not found after double patch")
	}
}
