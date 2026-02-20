package tee

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func testConfig(dir string) Config {
	return Config{
		Enabled:     true,
		Mode:        "failures",
		MaxFiles:    3,
		MaxFileSize: 1 << 20,
		Dir:         dir,
	}
}

func TestMaybeSaveOnFailure(t *testing.T) {
	dir := t.TempDir()
	cfg := testConfig(dir)
	raw := strings.Repeat("error output\n", 100) // >500 chars

	hint := MaybeSave(raw, 1, "git push", cfg)
	if hint == "" {
		t.Fatal("expected hint, got empty")
	}
	if !strings.Contains(hint, "[full output:") {
		t.Errorf("unexpected hint: %q", hint)
	}

	// Verify file exists
	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 {
		t.Errorf("expected 1 file, got %d", len(entries))
	}
}

func TestMaybeSaveNoSaveOnSuccess(t *testing.T) {
	dir := t.TempDir()
	cfg := testConfig(dir)
	raw := strings.Repeat("output\n", 100)

	hint := MaybeSave(raw, 0, "git push", cfg)
	if hint != "" {
		t.Errorf("expected no save on success, got %q", hint)
	}
}

func TestMaybeSaveSmallOutput(t *testing.T) {
	dir := t.TempDir()
	cfg := testConfig(dir)

	hint := MaybeSave("small", 1, "cmd", cfg)
	if hint != "" {
		t.Errorf("expected no save for small output, got %q", hint)
	}
}

func TestMaybeSaveDisabled(t *testing.T) {
	dir := t.TempDir()
	cfg := testConfig(dir)
	cfg.Enabled = false
	raw := strings.Repeat("error\n", 100)

	hint := MaybeSave(raw, 1, "cmd", cfg)
	if hint != "" {
		t.Errorf("expected no save when disabled, got %q", hint)
	}
}

func TestMaybeSaveModeAlways(t *testing.T) {
	dir := t.TempDir()
	cfg := testConfig(dir)
	cfg.Mode = "always"
	raw := strings.Repeat("output\n", 100)

	hint := MaybeSave(raw, 0, "cmd", cfg)
	if hint == "" {
		t.Error("expected save in always mode on success")
	}
}

func TestMaybeSaveEnvDisable(t *testing.T) {
	dir := t.TempDir()
	cfg := testConfig(dir)
	raw := strings.Repeat("error\n", 100)

	t.Setenv("SNIP_TEE", "0")
	hint := MaybeSave(raw, 1, "cmd", cfg)
	if hint != "" {
		t.Errorf("expected no save with SNIP_TEE=0, got %q", hint)
	}
}

func TestRotateFiles(t *testing.T) {
	dir := t.TempDir()

	// Create 5 log files
	for i := range 5 {
		path := filepath.Join(dir, strings.Repeat("a", i+1)+".log")
		_ = os.WriteFile(path, []byte("data"), 0644)
	}

	rotateFiles(dir, 3)

	entries, _ := os.ReadDir(dir)
	logCount := 0
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".log") {
			logCount++
		}
	}
	if logCount != 3 {
		t.Errorf("expected 3 files after rotation, got %d", logCount)
	}
}
