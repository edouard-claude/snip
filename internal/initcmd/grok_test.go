package initcmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGrokDenyRerunNoteDescribesOnlyGrok(t *testing.T) {
	if !strings.Contains(grokDenyRerunNote, "Grok Build hooks cannot rewrite commands in place") {
		t.Fatalf("Grok limitation missing from init note: %q", grokDenyRerunNote)
	}
	if strings.Contains(grokDenyRerunNote, "Codex") {
		t.Fatalf("Grok init note must not describe Codex as deny-and-rerun: %q", grokDenyRerunNote)
	}
}

func TestBuildGrokHookConfig(t *testing.T) {
	cfg := buildGrokHookConfig(`"/usr/local/bin/snip" hook grok`)

	hooks, ok := cfg["hooks"].(map[string]any)
	if !ok {
		t.Fatalf("hooks missing: %v", cfg)
	}
	pre, ok := hooks["PreToolUse"].([]any)
	if !ok || len(pre) != 1 {
		t.Fatalf("PreToolUse = %v, want 1 entry", hooks["PreToolUse"])
	}
	matcher := pre[0].(map[string]any)
	if matcher["matcher"] != grokMatcher {
		t.Errorf("matcher = %v, want %q", matcher["matcher"], grokMatcher)
	}
	inner, ok := matcher["hooks"].([]any)
	if !ok || len(inner) != 1 {
		t.Fatalf("nested hooks = %v, want 1 entry", matcher["hooks"])
	}
	entry := inner[0].(map[string]any)
	if entry["type"] != "command" {
		t.Errorf("type = %v, want command", entry["type"])
	}
	cmd, _ := entry["command"].(string)
	if !strings.HasSuffix(cmd, " hook grok") {
		t.Errorf("command = %q, want suffix ' hook grok'", cmd)
	}
}

func TestInitGrokEndToEnd(t *testing.T) {
	home := t.TempDir()
	filterDir := filepath.Join(home, ".config", "snip", "filters")
	if err := os.MkdirAll(filterDir, 0o755); err != nil {
		t.Fatal(err)
	}

	if err := initGrok("/usr/local/bin/snip", home, filterDir); err != nil {
		t.Fatalf("initGrok: %v", err)
	}

	path := grokHookPath(home)
	cfg := readSettings(t, path)

	pre := cfg["hooks"].(map[string]any)["PreToolUse"].([]any)
	if len(pre) != 1 {
		t.Fatalf("expected 1 PreToolUse entry, got %d", len(pre))
	}
	inner := pre[0].(map[string]any)["hooks"].([]any)
	cmd := inner[0].(map[string]any)["command"].(string)
	if !strings.HasSuffix(cmd, " hook grok") {
		t.Errorf("hook command = %q, want suffix ' hook grok'", cmd)
	}
	// The binary path must be quoted so a space in it cannot break the shell
	// invocation (Grok Build runs the hook command through a shell).
	if !strings.HasPrefix(cmd, `"`) {
		t.Errorf("hook command = %q, want quoted binary path", cmd)
	}
}

// TestInitGrokIdempotent verifies running init twice leaves a single valid
// snip-owned hook file (install = write, so the second run overwrites).
func TestInitGrokIdempotent(t *testing.T) {
	home := t.TempDir()
	filterDir := filepath.Join(home, ".config", "snip", "filters")
	_ = os.MkdirAll(filterDir, 0o755)

	if err := initGrok("/usr/local/bin/snip", home, filterDir); err != nil {
		t.Fatalf("first initGrok: %v", err)
	}
	if err := initGrok("/usr/local/bin/snip", home, filterDir); err != nil {
		t.Fatalf("second initGrok: %v", err)
	}

	cfg := readSettings(t, grokHookPath(home))
	pre := cfg["hooks"].(map[string]any)["PreToolUse"].([]any)
	if len(pre) != 1 {
		t.Fatalf("expected 1 PreToolUse entry after re-init, got %d", len(pre))
	}
}

func TestInitGrokThenUninstall(t *testing.T) {
	home := t.TempDir()
	filterDir := filepath.Join(home, ".config", "snip", "filters")
	_ = os.MkdirAll(filterDir, 0o755)

	if err := initGrok("/usr/local/bin/snip", home, filterDir); err != nil {
		t.Fatalf("initGrok: %v", err)
	}
	path := grokHookPath(home)
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("hook file should exist after init: %v", err)
	}

	t.Setenv("HOME", home)
	if err := uninstallGrok(); err != nil {
		t.Fatalf("uninstallGrok: %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("hook file should be removed after uninstall, stat err = %v", err)
	}
}

// TestUninstallGrokNoHookFile verifies uninstall is a no-op (no error) when
// nothing was installed.
func TestUninstallGrokNoHookFile(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	if err := uninstallGrok(); err != nil {
		t.Fatalf("uninstallGrok on clean home: %v", err)
	}
}

// TestPromptFileShared verifies AGENTS.md is recognized as shared (codex and
// grok both write it) while single-agent files are not.
func TestPromptFileShared(t *testing.T) {
	if !promptFileShared("AGENTS.md") {
		t.Error("AGENTS.md should be shared (codex + grok)")
	}
	for _, f := range []string{"GEMINI.md", ".windsurfrules", ".clinerules"} {
		if promptFileShared(f) {
			t.Errorf("%s should not be shared", f)
		}
	}
}

// TestUninstallGrokKeepsSharedAgentsMD guards the shared-file regression:
// AGENTS.md is written by both codex and grok with byte-identical content, so
// uninstalling grok must not delete a file codex may still rely on.
func TestUninstallGrokKeepsSharedAgentsMD(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Chdir(t.TempDir())

	if err := initPromptAgent("codex", "/usr/local/bin/snip", "/tmp/filters"); err != nil {
		t.Fatalf("initPromptAgent(codex): %v", err)
	}
	if _, err := os.Stat("AGENTS.md"); err != nil {
		t.Fatalf("AGENTS.md should exist after codex prompt init: %v", err)
	}

	if err := uninstallGrok(); err != nil {
		t.Fatalf("uninstallGrok: %v", err)
	}
	if _, err := os.Stat("AGENTS.md"); err != nil {
		t.Errorf("uninstallGrok must not delete the shared AGENTS.md: %v", err)
	}
}

// TestUninstallCodexKeepsSharedAgentsMD is the symmetric guard: uninstalling
// codex must not delete an AGENTS.md that grok's prompt mode may rely on.
func TestUninstallCodexKeepsSharedAgentsMD(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Chdir(t.TempDir())

	if err := initPromptAgent("grok", "/usr/local/bin/snip", "/tmp/filters"); err != nil {
		t.Fatalf("initPromptAgent(grok): %v", err)
	}
	if _, err := os.Stat("AGENTS.md"); err != nil {
		t.Fatalf("AGENTS.md should exist after grok prompt init: %v", err)
	}

	if err := uninstallCodex(); err != nil {
		t.Fatalf("uninstallCodex: %v", err)
	}
	if _, err := os.Stat("AGENTS.md"); err != nil {
		t.Errorf("uninstallCodex must not delete the shared AGENTS.md: %v", err)
	}
}

// TestRemoveLegacyPromptFileStillRemovesUnsharedFile verifies the shared-file
// guard did not disable cleanup of single-agent prompt files.
func TestRemoveLegacyPromptFileStillRemovesUnsharedFile(t *testing.T) {
	t.Chdir(t.TempDir())

	if err := initPromptAgent("gemini", "/usr/local/bin/snip", "/tmp/filters"); err != nil {
		t.Fatalf("initPromptAgent(gemini): %v", err)
	}
	removeLegacyPromptFile("gemini")
	if _, err := os.Stat("GEMINI.md"); !os.IsNotExist(err) {
		t.Errorf("GEMINI.md should be removed (not shared), stat err = %v", err)
	}
}
