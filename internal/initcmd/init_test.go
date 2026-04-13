package initcmd

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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
	_ = os.WriteFile(path, data, 0644)

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
	_ = patchSettings(path, hookPath)
	_ = patchSettings(path, hookPath)

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
	_ = patchSettings(path, hookPath)

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
	_ = os.WriteFile(path, data, 0644)

	// Add snip
	_ = patchSettings(path, hookPath)

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

func runHookScript(t *testing.T, cmd string) string {
	t.Helper()

	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("bash not available")
	}
	if _, err := exec.LookPath("jq"); err != nil {
		t.Skip("jq not available")
	}

	dir := t.TempDir()
	snipPath := filepath.Join(dir, "snip")
	if err := os.WriteFile(snipPath, []byte("#!/bin/sh\nexit 0\n"), 0755); err != nil {
		t.Fatalf("write fake snip: %v", err)
	}
	hookPath := filepath.Join(dir, "snip-rewrite.sh")
	if err := os.WriteFile(hookPath, []byte(generateHookScript(snipPath)), 0755); err != nil {
		t.Fatalf("write hook: %v", err)
	}

	payload, _ := json.Marshal(map[string]any{
		"tool_name":  "Bash",
		"tool_input": map[string]any{"command": cmd},
	})

	proc := exec.Command("bash", hookPath)
	proc.Env = append(os.Environ(), "PATH="+dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	proc.Stdin = strings.NewReader(string(payload))
	output, runErr := proc.Output()
	if runErr != nil {
		t.Fatalf("hook exited non-zero: %v", runErr)
	}

	var result map[string]any
	if err := json.Unmarshal(output, &result); err != nil {
		t.Fatalf("hook output is not valid JSON: %v\noutput: %s", err, output)
	}

	hookOut, _ := result["hookSpecificOutput"].(map[string]any)
	updated, _ := hookOut["updatedInput"].(map[string]any)
	rewritten, _ := updated["command"].(string)
	return rewritten
}

// TestHookScriptMultilineCommand verifies that the installed hook script handles
// multiline commands (e.g. git commit with a heredoc) without error.
// Previously, xargs was used to trim whitespace from FIRST_CMD and would fail
// with exit 1 on unmatched quotes present in heredoc body lines.
func TestHookScriptMultilineCommand(t *testing.T) {
	// Simulate the JSON Claude Code sends for a heredoc-style git commit.
	// The multiline command contains an unmatched `)"` on the last line,
	// which caused xargs to exit 1 (unmatched double quote).
	cmd := "git add file.go && git commit -m \"$(cat <<'EOF'\n   fix: something\n\n   Co-Authored-By: Bot <bot@example.com>\n   EOF\n   )\""
	rewritten := runHookScript(t, cmd)

	if !strings.HasPrefix(rewritten, "snip -- git add ") {
		t.Errorf("expected rewritten command to start with 'snip -- git add', got: %s", rewritten)
	}
}

func TestHookScriptInlinePythonDoesNotRewriteQuotedSemicolons(t *testing.T) {
	cmd := "git commit -m \"$(python3 -c \\\"from pathlib import Path; import sys; print(Path('.').name); print(sys.version)\\\")\" && git status"
	rewritten := runHookScript(t, cmd)

	if !strings.HasPrefix(rewritten, "snip -- git commit ") {
		t.Fatalf("expected rewritten command to start with 'snip -- git commit', got: %s", rewritten)
	}
	if strings.Count(rewritten, "snip --") != 1 {
		t.Fatalf("expected exactly one snip injection, got: %s", rewritten)
	}
	if strings.Contains(rewritten, "; snip") {
		t.Fatalf("expected inline python to stay unchanged, got: %s", rewritten)
	}
	if strings.Contains(rewritten, "python3 snip") {
		t.Fatalf("expected inline python command to stay unchanged, got: %s", rewritten)
	}
}

// TestPatchSettingsWindowsPath verifies that patchSettings normalizes backslashes
// to forward slashes in the hook command path written to settings.json, so the
// path is valid for bash on all platforms including Windows.
func TestPatchSettingsWindowsPath(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")
	// Simulate a Windows-style backslash path
	hookPath := `C:\Users\joedoe\.claude\hooks\snip-rewrite.sh`

	err := patchSettings(path, hookPath)
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

	if strings.Contains(cmd, `\`) {
		t.Errorf("command should not contain backslashes, got: %s", cmd)
	}
	if !strings.Contains(cmd, "/") {
		t.Errorf("command should contain forward slashes, got: %s", cmd)
	}
}

// TestGenerateHookScriptEmbedsBin verifies that generateHookScript embeds the
// absolute snip binary path as SNIP_BIN in the generated script, and does not
// reference bare "snip" as a command so the hook works regardless of $PATH.
func TestGenerateHookScriptEmbedsBin(t *testing.T) {
	snipBin := "/usr/local/bin/snip"
	script := generateHookScript(snipBin)

	if !strings.Contains(script, `SNIP_BIN="/usr/local/bin/snip"`) {
		t.Errorf("script should contain SNIP_BIN variable with path, got script:\n%s", script)
	}

	// Should not call bare 'snip' as a command (outside of comments)
	for _, line := range strings.Split(script, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "#") {
			continue
		}
		if strings.HasPrefix(trimmed, "snip ") || trimmed == "snip" {
			t.Errorf("script should not reference bare 'snip' command, found line: %s", line)
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
