package initcmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/edouard-claude/snip/internal/testutil"
)

func TestPatchPiSettingsNew(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")
	hookCommand := "/usr/local/bin/snip hook pi"

	if err := patchPiSettings(path, hookCommand); err != nil {
		t.Fatalf("patch: %v", err)
	}

	cfg := readSettings(t, path)
	hooks, ok := cfg["hooks"].(map[string]any)
	if !ok {
		t.Fatal("hooks not found")
	}
	preToolUse, ok := hooks["PreToolUse"].([]any)
	if !ok {
		t.Fatal("PreToolUse not found or not array")
	}
	if len(preToolUse) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(preToolUse))
	}
	entry := preToolUse[0].(map[string]any)
	if entry["matcher"] != "bash" {
		t.Errorf("matcher = %v, want bash (lowercase)", entry["matcher"])
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

func TestPatchPiSettingsIdempotent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")
	hookCommand := "/usr/local/bin/snip hook pi"

	_ = patchPiSettings(path, hookCommand)
	_ = patchPiSettings(path, hookCommand)

	cfg := readSettings(t, path)
	hooks := cfg["hooks"].(map[string]any)
	preToolUse := hooks["PreToolUse"].([]any)
	if len(preToolUse) != 1 {
		t.Errorf("expected 1 entry after double patch, got %d", len(preToolUse))
	}
}

func TestPatchPiSettingsPreservesForeignEntries(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")

	existing := map[string]any{
		"defaultProvider": "anthropic",
		"hooks": map[string]any{
			"PreToolUse": []any{
				map[string]any{
					"matcher": "write",
					"hooks": []any{
						map[string]any{"type": "command", "command": "/opt/other/guard"},
					},
				},
			},
		},
	}
	data, _ := json.MarshalIndent(existing, "", "  ")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}

	if err := patchPiSettings(path, "/usr/local/bin/snip hook pi"); err != nil {
		t.Fatalf("patch: %v", err)
	}

	cfg := readSettings(t, path)
	if cfg["defaultProvider"] != "anthropic" {
		t.Errorf("foreign top-level key dropped: defaultProvider = %v", cfg["defaultProvider"])
	}
	preToolUse := cfg["hooks"].(map[string]any)["PreToolUse"].([]any)
	if len(preToolUse) != 2 {
		t.Fatalf("expected 2 entries (foreign + snip), got %d", len(preToolUse))
	}
	first := preToolUse[0].(map[string]any)
	firstHooks := first["hooks"].([]any)
	firstHook := firstHooks[0].(map[string]any)
	if firstHook["command"] != "/opt/other/guard" {
		t.Errorf("foreign entry not preserved: %v", firstHook["command"])
	}
}

func TestUnpatchPiSettingsRemovesOnlySnip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")

	existing := map[string]any{
		"hooks": map[string]any{
			"PreToolUse": []any{
				map[string]any{
					"matcher": "write",
					"hooks": []any{
						map[string]any{"type": "command", "command": "/opt/other/guard"},
					},
				},
			},
		},
	}
	data, _ := json.MarshalIndent(existing, "", "  ")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}

	_ = patchPiSettings(path, "/usr/local/bin/snip hook pi")
	if err := unpatchPiSettings(path); err != nil {
		t.Fatalf("unpatch: %v", err)
	}

	cfg := readSettings(t, path)
	preToolUse := cfg["hooks"].(map[string]any)["PreToolUse"].([]any)
	if len(preToolUse) != 1 {
		t.Fatalf("expected 1 entry after unpatch, got %d", len(preToolUse))
	}
	remaining := preToolUse[0].(map[string]any)
	hookEntry := remaining["hooks"].([]any)[0].(map[string]any)
	if hookEntry["command"] != "/opt/other/guard" {
		t.Errorf("foreign entry not preserved: %v", hookEntry["command"])
	}
}

func TestUnpatchPiSettingsRemovesEmptyHooksSection(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")

	_ = patchPiSettings(path, "/usr/local/bin/snip hook pi")
	if err := unpatchPiSettings(path); err != nil {
		t.Fatalf("unpatch: %v", err)
	}

	cfg := readSettings(t, path)
	if _, ok := cfg["hooks"]; ok {
		t.Error("empty hooks section should be removed after uninstall")
	}
}

func TestInitPiEndToEnd(t *testing.T) {
	home := t.TempDir()
	filterDir := filepath.Join(home, ".config", "snip", "filters")
	if err := os.MkdirAll(filterDir, 0o755); err != nil {
		t.Fatal(err)
	}

	if err := initPi("/usr/local/bin/snip", home, filterDir); err != nil {
		t.Fatalf("initPi: %v", err)
	}

	settings := readSettings(t, piSettingsPath(home))
	preToolUse := settings["hooks"].(map[string]any)["PreToolUse"].([]any)
	if len(preToolUse) != 1 {
		t.Fatalf("expected 1 PreToolUse entry, got %d", len(preToolUse))
	}
	entry := preToolUse[0].(map[string]any)
	if entry["matcher"] != "bash" {
		t.Errorf("matcher = %v, want bash", entry["matcher"])
	}
	entryHooks := entry["hooks"].([]any)
	cmd := entryHooks[0].(map[string]any)["command"].(string)
	if !strings.HasSuffix(cmd, " hook pi") {
		t.Errorf("hook command = %q, want suffix ' hook pi'", cmd)
	}
}

func TestInitPiThenUninstallSymmetric(t *testing.T) {
	home := t.TempDir()
	filterDir := filepath.Join(home, ".config", "snip", "filters")
	_ = os.MkdirAll(filterDir, 0o755)

	if err := initPi("/usr/local/bin/snip", home, filterDir); err != nil {
		t.Fatalf("initPi: %v", err)
	}

	testutil.SetHome(t, home)
	if err := uninstallPi(); err != nil {
		t.Fatalf("uninstallPi: %v", err)
	}

	settings := readSettings(t, piSettingsPath(home))
	if h, ok := settings["hooks"].(map[string]any); ok {
		if _, ok := h["PreToolUse"]; ok {
			t.Error("PreToolUse should be removed after uninstall")
		}
	}
}

func TestIsSnipPiEntryRejectsForeign(t *testing.T) {
	foreign := map[string]any{
		"matcher": "bash",
		"hooks": []any{
			map[string]any{"type": "command", "command": "/opt/other/guard"},
		},
	}
	if isSnipPiEntry(foreign) {
		t.Error("foreign entry misdetected as snip")
	}
}

func TestIsSnipPiEntryAcceptsSnip(t *testing.T) {
	snip := map[string]any{
		"matcher": "bash",
		"hooks": []any{
			map[string]any{"type": "command", "command": "/usr/local/bin/snip hook pi"},
		},
	}
	if !isSnipPiEntry(snip) {
		t.Error("snip entry not detected")
	}
}
