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

func TestMaybeSaveProjectMarkerFound(t *testing.T) {
	repo := t.TempDir()
	if err := os.Mkdir(filepath.Join(repo, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	sub := filepath.Join(repo, "a", "b")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Chdir(sub)

	fallback := t.TempDir()
	cfg := testConfig(fallback)
	cfg.ProjectMarker = ".git"
	raw := strings.Repeat("error output\n", 100)

	hint := MaybeSave(raw, 1, "git push", cfg)
	if hint == "" {
		t.Fatal("expected hint, got empty")
	}

	teeDir := filepath.Join(repo, ".snip", "tee")
	entries, err := os.ReadDir(teeDir)
	if err != nil {
		t.Fatalf("expected tee dir at %s: %v", teeDir, err)
	}
	logCount := 0
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".log") {
			logCount++
		}
	}
	if logCount != 1 {
		t.Errorf("expected 1 log file in %s, got %d", teeDir, logCount)
	}
	if fbEntries, _ := os.ReadDir(fallback); len(fbEntries) != 0 {
		t.Errorf("expected fallback dir empty, got %d files", len(fbEntries))
	}

	// A .gitignore must guard the project tee dir against git add -A.
	gi, err := os.ReadFile(filepath.Join(teeDir, ".gitignore"))
	if err != nil {
		t.Fatalf("expected .gitignore in project tee dir: %v", err)
	}
	if string(gi) != "*\n" {
		t.Errorf("unexpected .gitignore content: %q", string(gi))
	}
}

func TestMaybeSaveProjectMarkerUnwritableFallsBack(t *testing.T) {
	repo := t.TempDir()
	if err := os.Mkdir(filepath.Join(repo, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Chdir(repo)

	if err := os.Chmod(repo, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(repo, 0o755) })

	fallback := t.TempDir()
	cfg := testConfig(fallback)
	cfg.ProjectMarker = ".git"
	raw := strings.Repeat("error output\n", 100)

	hint := MaybeSave(raw, 1, "git push", cfg)
	if hint == "" {
		t.Fatal("expected hint from fallback dir, got empty")
	}

	entries, _ := os.ReadDir(fallback)
	if len(entries) != 1 {
		t.Errorf("expected 1 file in fallback dir, got %d", len(entries))
	}
}

func TestMaybeSaveProjectMarkerNotFound(t *testing.T) {
	t.Chdir(t.TempDir())

	fallback := t.TempDir()
	cfg := testConfig(fallback)
	cfg.ProjectMarker = ".snip-test-marker-does-not-exist"
	raw := strings.Repeat("error output\n", 100)

	hint := MaybeSave(raw, 1, "git push", cfg)
	if hint == "" {
		t.Fatal("expected hint, got empty")
	}

	entries, _ := os.ReadDir(fallback)
	if len(entries) != 1 {
		t.Errorf("expected 1 file in fallback dir, got %d", len(entries))
	}
}

func TestMaybeSaveEnvDirOverridesProjectMarker(t *testing.T) {
	repo := t.TempDir()
	if err := os.Mkdir(filepath.Join(repo, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Chdir(repo)

	envDir := t.TempDir()
	t.Setenv("SNIP_TEE_DIR", envDir)

	cfg := testConfig(t.TempDir())
	cfg.ProjectMarker = ".git"
	raw := strings.Repeat("error output\n", 100)

	hint := MaybeSave(raw, 1, "git push", cfg)
	if hint == "" {
		t.Fatal("expected hint, got empty")
	}

	entries, _ := os.ReadDir(envDir)
	if len(entries) != 1 {
		t.Errorf("expected 1 file in SNIP_TEE_DIR, got %d", len(entries))
	}
	if _, err := os.Stat(filepath.Join(repo, ".snip")); !os.IsNotExist(err) {
		t.Errorf("expected no .snip dir in repo when SNIP_TEE_DIR is set")
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

func TestDefaultConfig(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	cfg := DefaultConfig()
	if !cfg.Enabled {
		t.Error("expected Enabled to be true by default")
	}
	if cfg.Mode != "failures" {
		t.Errorf("expected Mode=failures, got %s", cfg.Mode)
	}
	if cfg.MaxFiles != 20 {
		t.Errorf("expected MaxFiles=20, got %d", cfg.MaxFiles)
	}
	if cfg.MaxFileSize != 1<<20 {
		t.Errorf("expected MaxFileSize=1MB, got %d", cfg.MaxFileSize)
	}
}
