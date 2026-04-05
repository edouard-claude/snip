package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.Tee.Mode != "failures" {
		t.Errorf("expected tee mode 'failures', got %q", cfg.Tee.Mode)
	}
	if cfg.Tee.MaxFiles != 20 {
		t.Errorf("expected max_files 20, got %d", cfg.Tee.MaxFiles)
	}
	if !cfg.Display.Color {
		t.Error("expected color enabled by default")
	}
	if cfg.Tracking.DBPath == "" {
		t.Error("expected non-empty db path")
	}
}

func TestLoadMissingFile(t *testing.T) {
	t.Setenv("SNIP_CONFIG", "/tmp/nonexistent-snip-config-test.toml")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Tee.Mode != "failures" {
		t.Errorf("expected defaults when file missing, got tee.mode=%q", cfg.Tee.Mode)
	}
}

func TestDefaultConfigQuietAndEnable(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.Display.QuietNoFilter != false {
		t.Error("expected QuietNoFilter false by default")
	}
	if cfg.Filters.Enable != nil {
		t.Errorf("expected Filters.Enable nil by default, got %v", cfg.Filters.Enable)
	}
}

func TestLoadConfigWithEnable(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")

	content := `
[display]
quiet_no_filter = true

[filters.enable]
git-diff = false
git-status = true
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	t.Setenv("SNIP_CONFIG", path)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !cfg.Display.QuietNoFilter {
		t.Error("expected QuietNoFilter true")
	}
	if cfg.Filters.Enable == nil {
		t.Fatal("expected non-nil Filters.Enable")
	}
	if cfg.Filters.Enable["git-diff"] != false {
		t.Error("expected git-diff disabled")
	}
	if cfg.Filters.Enable["git-status"] != true {
		t.Error("expected git-status enabled")
	}
}

func TestLoadConfigEmptyEnable(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")

	content := `
[filters.enable]
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	t.Setenv("SNIP_CONFIG", path)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// nil or empty map both mean "all enabled"
	if len(cfg.Filters.Enable) != 0 {
		t.Errorf("expected nil or empty Filters.Enable, got %v", cfg.Filters.Enable)
	}
}

func TestLoadFromFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")

	content := `
[tracking]
db_path = "/custom/path.db"

[tee]
mode = "always"
max_files = 5
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	t.Setenv("SNIP_CONFIG", path)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Tracking.DBPath != "/custom/path.db" {
		t.Errorf("expected custom db path, got %q", cfg.Tracking.DBPath)
	}
	if cfg.Tee.Mode != "always" {
		t.Errorf("expected tee mode 'always', got %q", cfg.Tee.Mode)
	}
	if cfg.Tee.MaxFiles != 5 {
		t.Errorf("expected max_files 5, got %d", cfg.Tee.MaxFiles)
	}
}
