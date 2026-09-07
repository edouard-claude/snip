package initcmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/edouard-claude/snip/internal/hook"
)

func TestPatchAntigravityHooksNew(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "hooks.json")
	hookCommand := hook.QuoteBinFor("/usr/local/bin/snip", runtime.GOOS) + " hook antigravity"

	err := patchAntigravityHooks(path, hookCommand)
	if err != nil {
		t.Fatalf("patch: %v", err)
	}

	settings := readSettings(t, path)

	hooks, ok := settings["snip-hooks"].(map[string]any)
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
	if entry["matcher"] != "run_command" {
		t.Errorf("matcher = %v, want run_command", entry["matcher"])
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

func TestPatchAntigravityHooksExisting(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "hooks.json")
	hookCommand := hook.QuoteBinFor("/usr/local/bin/snip", runtime.GOOS) + " hook antigravity"

	// Write existing hooks with other hooks
	existing := map[string]any{
		"other-hooks": map[string]any{
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
		"snip-hooks": map[string]any{
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

	err := patchAntigravityHooks(path, hookCommand)
	if err != nil {
		t.Fatalf("patch: %v", err)
	}

	settings := readSettings(t, path)

	// Existing settings preserved
	if _, ok := settings["other-hooks"]; !ok {
		t.Error("existing settings not preserved")
	}

	hooks := settings["snip-hooks"].(map[string]any)

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
	if second["matcher"] != "run_command" {
		t.Errorf("second matcher = %v, want run_command", second["matcher"])
	}
}

func TestPatchAntigravityHooksIdempotent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "hooks.json")

	cases := []struct {
		name      string
		path      string
		runtimeOS string
	}{
		{
			name:      "Windows path with spaces",
			path:      "C:\\Program Files\\snip\\snip.exe",
			runtimeOS: "windows",
		},
		{
			name:      "Windows path without spaces",
			path:      "C:\\snip\\snip.exe",
			runtimeOS: "windows",
		},
		{
			name:      "Unix path",
			path:      "/usr/local/bin/snip",
			runtimeOS: "linux",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			hookCommand := hook.QuoteBinFor(tc.path, tc.runtimeOS) + " hook antigravity"

			// Patch twice
			_ = patchAntigravityHooks(path, hookCommand)
			_ = patchAntigravityHooks(path, hookCommand)

			settings := readSettings(t, path)
			hooks := settings["snip-hooks"].(map[string]any)
			preToolUse := hooks["PreToolUse"].([]any)

			// Should still be exactly 1 entry, not duplicated
			if len(preToolUse) != 1 {
				t.Errorf("expected 1 entry after double patch, got %d", len(preToolUse))
			}
		})
	}
}

func TestUnpatchAntigravityHooks(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "hooks.json")

	cases := []struct {
		name      string
		path      string
		runtimeOS string
	}{
		{
			name:      "Windows path with spaces",
			path:      "C:\\Program Files\\snip\\snip.exe",
			runtimeOS: "windows",
		},
		{
			name:      "Windows path without spaces",
			path:      "C:\\snip\\snip.exe",
			runtimeOS: "windows",
		},
		{
			name:      "Unix path with spaces",
			path:      "/usr/local/bin/snip",
			runtimeOS: "linux",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			hookCommand := hook.QuoteBinFor(tc.path, tc.runtimeOS) + " hook antigravity"

			// Patch first
			_ = patchAntigravityHooks(path, hookCommand)

			// Unpatch
			if err := unpatchAntigravityHooks(path); err != nil {
				t.Fatalf("unpatch: %v", err)
			}

			settings := readSettings(t, path)

			// hooks section should be gone entirely
			if _, ok := settings["snip-hooks"]; ok {
				hooks := settings["snip-hooks"].(map[string]any)
				if _, ok := hooks["PreToolUse"]; ok {
					t.Error("PreToolUse should be removed after unpatch")
				}
			}
		})
	}
}

func TestUnpatchAntigravityHooksPreservesOtherHooks(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "hooks.json")

	cases := []struct {
		name      string
		path      string
		runtimeOS string
	}{
		{
			name:      "Windows path with spaces",
			path:      "C:\\Program Files\\snip\\snip.exe",
			runtimeOS: "windows",
		},
		{
			name:      "Windows path without spaces",
			path:      "C:\\snip\\snip.exe",
			runtimeOS: "windows",
		},
		{
			name:      "Unix path with spaces",
			path:      "/usr/local/bin/snip",
			runtimeOS: "linux",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			hookCommand := hook.QuoteBinFor(tc.path, tc.runtimeOS) + " hook antigravity"

			// Create settings with snip + another hook
			existing := map[string]any{
				"snip-hooks": map[string]any{
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
			_ = patchAntigravityHooks(path, hookCommand)

			// Verify both present
			settings := readSettings(t, path)
			preToolUse := settings["snip-hooks"].(map[string]any)["PreToolUse"].([]any)
			if len(preToolUse) != 2 {
				t.Fatalf("expected 2 entries, got %d", len(preToolUse))
			}

			// Unpatch -- should remove snip but keep the Write hook
			if err := unpatchAntigravityHooks(path); err != nil {
				t.Fatalf("unpatch: %v", err)
			}

			settings = readSettings(t, path)
			hooks := settings["snip-hooks"].(map[string]any)
			preToolUse = hooks["PreToolUse"].([]any)
			if len(preToolUse) != 1 {
				t.Fatalf("expected 1 entry after unpatch, got %d", len(preToolUse))
			}
			remaining := preToolUse[0].(map[string]any)
			if remaining["matcher"] != "Write" {
				t.Errorf("remaining matcher = %v, want Write", remaining["matcher"])
			}
		})
	}
}

func TestPatchAntigravityHooksWindowsPath(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "hooks.json")
	// Simulate a Windows-style snip hook command
	hookCommand := hook.QuoteBinFor(`C:\Users\joedoe\go\bin\snip.exe`, runtime.GOOS) + ` hook antigravity`

	err := patchAntigravityHooks(path, hookCommand)
	if err != nil {
		t.Fatalf("patch: %v", err)
	}

	settings := readSettings(t, path)
	hooks := settings["snip-hooks"].(map[string]any)
	preToolUse := hooks["PreToolUse"].([]any)
	entry := preToolUse[0].(map[string]any)
	entryHooks := entry["hooks"].([]any)
	hook := entryHooks[0].(map[string]any)
	cmd := hook["command"].(string)

	// The command is stored as-is; path normalization happens in Run() before calling patchAntigravityHooks
	if cmd != hookCommand {
		t.Errorf("command = %v, want %s", cmd, hookCommand)
	}
}
