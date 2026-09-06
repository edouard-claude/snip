package initcmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPatchSettingsNew(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")
	hookCommand := "/usr/local/bin/snip hook"

	err := patchClaudeSettings(path, hookCommand)
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

	err := patchClaudeSettings(path, hookCommand)
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
	_ = patchClaudeSettings(path, hookCommand)
	_ = patchClaudeSettings(path, hookCommand)

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
	err := patchClaudeSettings(path, hookCommand)
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

func TestUnpatchClaudeSettings(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")
	hookCommand := "/usr/local/bin/snip hook"

	// Patch first
	_ = patchClaudeSettings(path, hookCommand)

	// Unpatch
	if err := unpatchClaudeSettings(path); err != nil {
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
	_ = patchClaudeSettings(path, hookCommand)

	// Verify both present
	settings := readSettings(t, path)
	preToolUse := settings["hooks"].(map[string]any)["PreToolUse"].([]any)
	if len(preToolUse) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(preToolUse))
	}

	// Unpatch -- should remove snip but keep the Write hook
	if err := unpatchClaudeSettings(path); err != nil {
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

	err := patchClaudeSettings(path, hookCommand)
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

	// The command is stored as-is; path normalization happens in Run() before calling patchClaudeSettings
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

// --- parseAgent tests ---

func TestParseAgentDefault(t *testing.T) {
	agent, remaining := parseAgent([]string{"--uninstall"})
	if agent != "claude-code" {
		t.Errorf("agent = %q, want claude-code", agent)
	}
	if len(remaining) != 1 || remaining[0] != "--uninstall" {
		t.Errorf("remaining = %v, want [--uninstall]", remaining)
	}
}

func TestParseAgentFlag(t *testing.T) {
	agent, remaining := parseAgent([]string{"--agent", "cursor"})
	if agent != "cursor" {
		t.Errorf("agent = %q, want cursor", agent)
	}
	if len(remaining) != 0 {
		t.Errorf("remaining = %v, want empty", remaining)
	}
}

func TestParseAgentEquals(t *testing.T) {
	agent, remaining := parseAgent([]string{"--agent=codex", "--uninstall"})
	if agent != "codex" {
		t.Errorf("agent = %q, want codex", agent)
	}
	if len(remaining) != 1 || remaining[0] != "--uninstall" {
		t.Errorf("remaining = %v, want [--uninstall]", remaining)
	}
}

func TestIsValidAgent(t *testing.T) {
	valid := []string{
		"claude-code", "cursor", "codex", "windsurf", "cline",
		"copilot", "gemini", "kilocode", "antigravity", "grok",
	}
	for _, a := range valid {
		if !isValidAgent(a) {
			t.Errorf("expected %q to be valid", a)
		}
	}
	if isValidAgent("unknown") {
		t.Error("expected 'unknown' to be invalid")
	}
}

// --- Cursor hook tests ---

func TestPatchCursorHooksNew(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "hooks.json")
	hookCommand := "/usr/local/bin/snip hook"

	err := patchCursorHooks(path, hookCommand)
	if err != nil {
		t.Fatalf("patch: %v", err)
	}

	config := readSettings(t, path)

	hooks, ok := config["hooks"].(map[string]any)
	if !ok {
		t.Fatal("hooks not found")
	}

	beforeShell, ok := hooks["beforeShellExecution"].([]any)
	if !ok {
		t.Fatal("beforeShellExecution not found or not array")
	}

	if len(beforeShell) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(beforeShell))
	}

	entry := beforeShell[0].(map[string]any)
	if entry["matcher"] != ".*" {
		t.Errorf("matcher = %v, want .*", entry["matcher"])
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

func TestPatchCursorHooksIdempotent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "hooks.json")
	hookCommand := "/usr/local/bin/snip hook"

	_ = patchCursorHooks(path, hookCommand)
	_ = patchCursorHooks(path, hookCommand)

	config := readSettings(t, path)
	hooks := config["hooks"].(map[string]any)
	beforeShell := hooks["beforeShellExecution"].([]any)

	if len(beforeShell) != 1 {
		t.Errorf("expected 1 entry after double patch, got %d", len(beforeShell))
	}
}

func TestPatchCursorHooksExisting(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "hooks.json")
	hookCommand := "/usr/local/bin/snip hook"

	// Write existing hooks with another entry
	existing := map[string]any{
		"hooks": map[string]any{
			"beforeShellExecution": []any{
				map[string]any{
					"matcher": ".*",
					"hooks": []any{
						map[string]any{"type": "command", "command": "other-tool.sh"},
					},
				},
			},
		},
	}
	data, _ := json.MarshalIndent(existing, "", "  ")
	_ = os.WriteFile(path, data, 0644)

	err := patchCursorHooks(path, hookCommand)
	if err != nil {
		t.Fatalf("patch: %v", err)
	}

	config := readSettings(t, path)
	hooks := config["hooks"].(map[string]any)
	beforeShell := hooks["beforeShellExecution"].([]any)

	if len(beforeShell) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(beforeShell))
	}
}

func TestUnpatchCursorHooks(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "hooks.json")
	hookCommand := "/usr/local/bin/snip hook"

	_ = patchCursorHooks(path, hookCommand)

	if err := unpatchCursorHooks(path); err != nil {
		t.Fatalf("unpatch: %v", err)
	}

	config := readSettings(t, path)
	if _, ok := config["hooks"]; ok {
		hooks := config["hooks"].(map[string]any)
		if _, ok := hooks["beforeShellExecution"]; ok {
			t.Error("beforeShellExecution should be removed after unpatch")
		}
	}
}

func TestUnpatchCursorPreservesOtherHooks(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "hooks.json")
	hookCommand := "/usr/local/bin/snip hook"

	// Create hooks with another entry
	existing := map[string]any{
		"hooks": map[string]any{
			"beforeShellExecution": []any{
				map[string]any{
					"matcher": ".*",
					"hooks":   []any{map[string]any{"type": "command", "command": "other.sh"}},
				},
			},
		},
	}
	data, _ := json.MarshalIndent(existing, "", "  ")
	_ = os.WriteFile(path, data, 0644)

	// Add snip
	_ = patchCursorHooks(path, hookCommand)

	// Unpatch
	if err := unpatchCursorHooks(path); err != nil {
		t.Fatalf("unpatch: %v", err)
	}

	config := readSettings(t, path)
	hooks := config["hooks"].(map[string]any)
	beforeShell := hooks["beforeShellExecution"].([]any)
	if len(beforeShell) != 1 {
		t.Fatalf("expected 1 entry after unpatch, got %d", len(beforeShell))
	}
	remaining := beforeShell[0].(map[string]any)
	entryHooks := remaining["hooks"].([]any)
	hook := entryHooks[0].(map[string]any)
	if hook["command"] != "other.sh" {
		t.Errorf("remaining command = %v, want other.sh", hook["command"])
	}
}

// --- Prompt agent tests ---

func TestPromptContent(t *testing.T) {
	content := promptContent("/usr/local/bin/snip")
	if !strings.Contains(content, "/usr/local/bin/snip -- git status") {
		t.Error("prompt content should contain snip binary path with example command")
	}
	if !strings.Contains(content, "# Snip - CLI Token Optimizer") {
		t.Error("prompt content should contain header")
	}
}

func TestPromptAgentFiles(t *testing.T) {
	tests := []struct {
		agent    string
		filename string
	}{
		{"codex", "AGENTS.md"},
		{"grok", "AGENTS.md"},
		{"windsurf", ".windsurfrules"},
		{"cline", ".clinerules"},
		{"copilot", filepath.Join(".github", "copilot-instructions.md")},
		{"gemini", "GEMINI.md"},
		{"kilocode", filepath.Join(".kilocode", "rules", "snip-rules.md")},
		{"antigravity", filepath.Join(".agents", "rules", "snip-rules.md")},
	}
	for _, tt := range tests {
		t.Run(tt.agent, func(t *testing.T) {
			dir := t.TempDir()
			targetPath := filepath.Join(dir, tt.filename)

			// Create parent directories for subdirectory agents
			if parentDir := filepath.Dir(targetPath); parentDir != dir {
				if err := os.MkdirAll(parentDir, 0755); err != nil {
					t.Fatalf("mkdir: %v", err)
				}
			}

			content := promptContent("/usr/local/bin/snip")
			if err := os.WriteFile(targetPath, []byte(content), 0644); err != nil {
				t.Fatalf("write: %v", err)
			}

			data, err := os.ReadFile(targetPath)
			if err != nil {
				t.Fatalf("read: %v", err)
			}
			if !strings.Contains(string(data), "# Snip - CLI Token Optimizer") {
				t.Error("file should contain snip header")
			}
			if !strings.Contains(string(data), "/usr/local/bin/snip") {
				t.Error("file should contain snip binary path")
			}
		})
	}
}

func TestUninstallPromptAgent(t *testing.T) {
	dir := t.TempDir()

	// Create the file in a temp dir to test removal
	targetPath := filepath.Join(dir, "AGENTS.md")
	if err := os.WriteFile(targetPath, []byte("test"), 0644); err != nil {
		t.Fatal(err)
	}

	// Remove it
	if err := os.Remove(targetPath); err != nil {
		t.Fatalf("remove: %v", err)
	}

	// Verify it's gone
	if _, err := os.Stat(targetPath); !os.IsNotExist(err) {
		t.Error("file should be removed")
	}
}

func TestInitPromptAgentSubdirectory(t *testing.T) {
	dir := t.TempDir()

	// Test kilocode: creates .kilocode/rules/snip-rules.md
	targetPath := filepath.Join(dir, ".kilocode", "rules", "snip-rules.md")

	// Simulate initPromptAgent behavior: create parent dirs + write file
	if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	content := promptContent("/usr/local/bin/snip")
	if err := os.WriteFile(targetPath, []byte(content), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(targetPath); err != nil {
		t.Fatal("file should exist after init")
	}

	// Simulate uninstall: remove file + clean empty parents
	_ = os.Remove(targetPath)
	for d := filepath.Dir(targetPath); d != dir && d != string(filepath.Separator); d = filepath.Dir(d) {
		if err := os.Remove(d); err != nil {
			break
		}
	}

	// Verify directory tree cleaned up
	if _, err := os.Stat(filepath.Join(dir, ".kilocode")); !os.IsNotExist(err) {
		t.Error(".kilocode directory should be removed after uninstall")
	}
}

func TestRunUnknownAgent(t *testing.T) {
	err := Run([]string{"--agent", "unknown"})
	if err == nil {
		t.Fatal("expected error for unknown agent")
	}
	if !strings.Contains(err.Error(), "unknown agent") {
		t.Errorf("error = %q, want to contain 'unknown agent'", err.Error())
	}
	if !strings.Contains(err.Error(), "claude-code") {
		t.Errorf("error should list valid agents, got: %q", err.Error())
	}
}

func TestInitClaudeCodeRespectsClaudeConfigDir(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	claudeDir := t.TempDir()
	t.Setenv("CLAUDE_CONFIG_DIR", claudeDir)

	filterDir := t.TempDir()
	err := initClaudeCode("/usr/local/bin/snip", filterDir)
	if err != nil {
		t.Fatalf("initClaudeCode: %v", err)
	}

	settingsPath := filepath.Join(claudeDir, "settings.json")
	if _, err := os.Stat(settingsPath); err != nil {
		t.Errorf("expected settings.json at %s honoring CLAUDE_CONFIG_DIR: %v", settingsPath, err)
	}
}

func TestInitClaudeCodeMigratesLegacyHookUnderClaudeConfigDir(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	claudeDir := t.TempDir()
	t.Setenv("CLAUDE_CONFIG_DIR", claudeDir)

	hooksDir := filepath.Join(claudeDir, "hooks")
	if err := os.MkdirAll(hooksDir, 0755); err != nil {
		t.Fatal(err)
	}
	oldHookPath := filepath.Join(hooksDir, legacyHookFile)
	if err := os.WriteFile(oldHookPath, []byte("#!/bin/bash\nexit 0"), 0755); err != nil {
		t.Fatal(err)
	}

	filterDir := t.TempDir()
	if err := initClaudeCode("/usr/local/bin/snip", filterDir); err != nil {
		t.Fatalf("initClaudeCode: %v", err)
	}

	if _, err := os.Stat(oldHookPath); !os.IsNotExist(err) {
		t.Error("legacy hook script under CLAUDE_CONFIG_DIR should be removed")
	}
}

func TestUninstallClaudeCodeRespectsClaudeConfigDir(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	claudeDir := t.TempDir()
	t.Setenv("CLAUDE_CONFIG_DIR", claudeDir)

	settingsPath := filepath.Join(claudeDir, "settings.json")
	if err := patchClaudeSettings(settingsPath, "/usr/local/bin/snip hook"); err != nil {
		t.Fatal(err)
	}

	if err := uninstallClaudeCode(); err != nil {
		t.Fatalf("uninstallClaudeCode: %v", err)
	}

	settings := readSettings(t, settingsPath)
	if hooks, ok := settings["hooks"].(map[string]any); ok {
		if _, ok := hooks["PreToolUse"]; ok {
			t.Error("expected snip hook removed from CLAUDE_CONFIG_DIR settings")
		}
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

func TestResolveSnipBin(t *testing.T) {
	bin, err := resolveSnipBin()
	if err != nil {
		t.Fatalf("resolveSnipBin: %v", err)
	}
	if bin == "" {
		t.Error("expected non-empty path")
	}
	// Should be an absolute path.
	if !filepath.IsAbs(bin) {
		t.Errorf("expected absolute path, got %q", bin)
	}
}
