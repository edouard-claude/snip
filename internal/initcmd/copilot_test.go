package initcmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildCopilotHookConfig(t *testing.T) {
	cfg := buildCopilotHookConfig(`"/usr/local/bin/snip" hook copilot`)

	if v, _ := cfg["version"].(int); v != 1 {
		t.Errorf("version = %v, want 1", cfg["version"])
	}
	hooks, ok := cfg["hooks"].(map[string]any)
	if !ok {
		t.Fatalf("hooks missing: %v", cfg)
	}
	pre, ok := hooks["preToolUse"].([]any)
	if !ok || len(pre) != 1 {
		t.Fatalf("preToolUse = %v, want 1 entry", hooks["preToolUse"])
	}
	entry := pre[0].(map[string]any)
	if entry["type"] != "command" {
		t.Errorf("type = %v, want command", entry["type"])
	}
	bash, _ := entry["bash"].(string)
	if !strings.HasSuffix(bash, " hook copilot") {
		t.Errorf("bash = %q, want suffix ' hook copilot'", bash)
	}
}

func TestInitCopilotHookEndToEnd(t *testing.T) {
	home := t.TempDir()
	filterDir := filepath.Join(home, ".config", "snip", "filters")
	if err := os.MkdirAll(filterDir, 0o755); err != nil {
		t.Fatal(err)
	}

	if err := initCopilotHook("/usr/local/bin/snip", home, filterDir); err != nil {
		t.Fatalf("initCopilotHook: %v", err)
	}

	path := copilotHookPath(home)
	cfg := readSettings(t, path)

	pre := cfg["hooks"].(map[string]any)["preToolUse"].([]any)
	if len(pre) != 1 {
		t.Fatalf("expected 1 preToolUse entry, got %d", len(pre))
	}
	bash := pre[0].(map[string]any)["bash"].(string)
	if !strings.HasSuffix(bash, " hook copilot") {
		t.Errorf("bash command = %q, want suffix ' hook copilot'", bash)
	}
	// Fail-closed safety: the binary path must be quoted so a space cannot break
	// the invocation and deny every command.
	if !strings.HasPrefix(bash, `"`) {
		t.Errorf("bash command = %q, want quoted binary path", bash)
	}
	// version must round-trip as JSON number 1.
	if v, _ := cfg["version"].(float64); v != 1 {
		t.Errorf("version = %v, want 1", cfg["version"])
	}
}

func TestInitCopilotThenUninstall(t *testing.T) {
	home := t.TempDir()
	filterDir := filepath.Join(home, ".config", "snip", "filters")
	_ = os.MkdirAll(filterDir, 0o755)

	if err := initCopilotHook("/usr/local/bin/snip", home, filterDir); err != nil {
		t.Fatalf("initCopilotHook: %v", err)
	}
	path := copilotHookPath(home)
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("hook file should exist after init: %v", err)
	}

	t.Setenv("HOME", home)
	if err := uninstallCopilot(); err != nil {
		t.Fatalf("uninstallCopilot: %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("hook file should be removed after uninstall, stat err = %v", err)
	}
}

// TestUninstallCopilotNoHookFile verifies uninstall is a no-op (no error) when
// nothing was installed.
func TestUninstallCopilotNoHookFile(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	if err := uninstallCopilot(); err != nil {
		t.Fatalf("uninstallCopilot on clean home: %v", err)
	}
}
