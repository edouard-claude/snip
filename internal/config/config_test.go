package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/edouard-claude/snip/internal/trust"
)

// canonicalTempDir returns t.TempDir() with symlinks resolved. macOS's
// $TMPDIR (/var/folders/...) is a symlink to /private/var/folders/...,
// while os.Getwd() in production returns the resolved form. Without this,
// trust store keys (built from raw TempDir paths via filepath.Abs) would
// not match what LoadMerged looks up via projectConfigPath after Chdir.
func canonicalTempDir(t *testing.T) string {
	t.Helper()
	d := t.TempDir()
	resolved, err := filepath.EvalSymlinks(d)
	if err != nil {
		t.Fatal(err)
	}
	return resolved
}

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

func TestExpandTildeInPaths(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("cannot get home dir")
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")

	content := `
[tracking]
db_path = "~/.local/share/snip/tracking.db"

[filters]
dir = "~/.config/snip/filters"
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	t.Setenv("SNIP_CONFIG", path)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expectedDB := filepath.Join(home, ".local/share/snip/tracking.db")
	if cfg.Tracking.DBPath != expectedDB {
		t.Errorf("db_path: got %q, want %q", cfg.Tracking.DBPath, expectedDB)
	}

	expectedDir := filepath.Join(home, ".config/snip/filters")
	dirs := cfg.Filters.Dirs()
	if len(dirs) != 1 || dirs[0] != expectedDir {
		t.Errorf("filters.dir: got %v, want [%q]", dirs, expectedDir)
	}
}

func TestExpandTildeNoTilde(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")

	content := `
[tracking]
db_path = "/absolute/path/tracking.db"

[filters]
dir = "/absolute/path/filters"
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	t.Setenv("SNIP_CONFIG", path)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Tracking.DBPath != "/absolute/path/tracking.db" {
		t.Errorf("db_path: got %q, want absolute path", cfg.Tracking.DBPath)
	}
	dirs := cfg.Filters.Dirs()
	if len(dirs) != 1 || dirs[0] != "/absolute/path/filters" {
		t.Errorf("filters.dir: got %v, want [\"/absolute/path/filters\"]", dirs)
	}
}

func TestLoadConfigMultipleDirs(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")

	content := `
[filters]
dir = ["~/.config/snip/filters", "/project/.snip"]
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	t.Setenv("SNIP_CONFIG", path)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	home, _ := os.UserHomeDir()
	dirs := cfg.Filters.Dirs()
	if len(dirs) != 2 {
		t.Fatalf("expected 2 dirs, got %d: %v", len(dirs), dirs)
	}
	if dirs[0] != filepath.Join(home, ".config/snip/filters") {
		t.Errorf("dirs[0]: got %q, want tilde-expanded path", dirs[0])
	}
	if dirs[1] != "/project/.snip" {
		t.Errorf("dirs[1]: got %q, want %q", dirs[1], "/project/.snip")
	}
}

func TestLoadConfigEnvVarExpansion(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")

	t.Setenv("SNIP_TEST_PROJECT", "/my/project")

	content := `
[filters]
dir = ["~/.config/snip/filters", "${env.SNIP_TEST_PROJECT}/.snip"]
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	t.Setenv("SNIP_CONFIG", path)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	dirs := cfg.Filters.Dirs()
	if len(dirs) != 2 {
		t.Fatalf("expected 2 dirs, got %d: %v", len(dirs), dirs)
	}
	if dirs[1] != "/my/project/.snip" {
		t.Errorf("dirs[1]: got %q, want %q", dirs[1], "/my/project/.snip")
	}
}

func TestExpandEnvVarsUnset(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")

	// Use a var name that is guaranteed to not exist
	content := `
[filters]
dir = "${env.SNIP_TEST_NONEXISTENT_VAR_XYZ}/.snip"
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	t.Setenv("SNIP_CONFIG", path)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	dirs := cfg.Filters.Dirs()
	if len(dirs) != 1 {
		t.Fatalf("expected 1 dir, got %d", len(dirs))
	}
	if dirs[0] != "/.snip" {
		t.Errorf("got %q, want %q", dirs[0], "/.snip")
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

func TestLoadMergedNoProjectConfig(t *testing.T) {
	// When no .snip/config.toml exists in the directory tree,
	// LoadMerged should return the user config unchanged.
	dir := t.TempDir()
	home := filepath.Join(dir, "home")
	if err := os.MkdirAll(filepath.Join(home, ".config", "snip"), 0o755); err != nil {
		t.Fatal(err)
	}
	userPath := filepath.Join(home, ".config", "snip", "config.toml")
	content := `[display]
quiet_no_filter = true
`
	if err := os.WriteFile(userPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	oldHome := os.Getenv("HOME")
	t.Setenv("HOME", home)
	t.Setenv("SNIP_CONFIG", userPath)
	_ = oldHome

	// Change into a temp dir with no .snip/ so projectConfigPath returns ""
	workDir := t.TempDir()
	oldWd, _ := os.Getwd()
	_ = os.Chdir(workDir)
	t.Cleanup(func() { _ = os.Chdir(oldWd) })

	cfg, err := LoadMerged()
	if err != nil {
		t.Fatalf("LoadMerged: %v", err)
	}
	if !cfg.Display.QuietNoFilter {
		t.Error("expected user QuietNoFilter preserved")
	}
}

func TestLoadMergedNilEnableNoPanic(t *testing.T) {
	// Bug: LoadMerged panicked when user config had no [filters.enable]
	// and project config set one with mode="project".
	dir := t.TempDir()
	home := filepath.Join(dir, "home")
	filterDir := filepath.Join(home, ".config", "snip", "filters")
	if err := os.MkdirAll(filterDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// User config: no [filters.enable] section
	userContent := `[display]
quiet_no_filter = true
`
	userPath := filepath.Join(home, ".config", "snip", "config.toml")
	if err := os.WriteFile(userPath, []byte(userContent), 0o644); err != nil {
		t.Fatal(err)
	}

	// Project config: has [filters.enable] with mode="project"
	projectDir := canonicalTempDir(t)
	if err := os.MkdirAll(filepath.Join(projectDir, ".snip"), 0o755); err != nil {
		t.Fatal(err)
	}
	projectContent := `mode = "project"

[filters.enable]
git-diff = false
`
	if err := os.WriteFile(filepath.Join(projectDir, ".snip", "config.toml"), []byte(projectContent), 0o644); err != nil {
		t.Fatal(err)
	}

	// Trust the project config so the trust gate passes
	projectCfgPath := filepath.Join(projectDir, ".snip", "config.toml")
	trustStore := make(trust.Store)
	hash, err := trust.HashFile(projectCfgPath)
	if err != nil {
		t.Fatal(err)
	}
	trustStore[projectCfgPath] = hash
	if err := trust.SaveTo(trustStore, filepath.Join(home, ".config", "snip", "trusted.json")); err != nil {
		t.Fatal(err)
	}

	t.Setenv("HOME", home)
	t.Setenv("SNIP_CONFIG", userPath)
	oldWd, _ := os.Getwd()
	_ = os.Chdir(projectDir)
	t.Cleanup(func() { _ = os.Chdir(oldWd) })

	cfg, err := LoadMerged()
	if err != nil {
		t.Fatalf("LoadMerged: %v", err)
	}
	if cfg.Filters.Enable == nil {
		t.Fatal("expected non-nil Filters.Enable after merge")
	}
	if cfg.Filters.Enable["git-diff"] != false {
		t.Error("expected git-diff disabled by project")
	}
}

func TestLoadMergedMergePrecedence(t *testing.T) {
	// Project mode="project" should override user enable/disable + global + overrides.
	dir := t.TempDir()
	home := filepath.Join(dir, "home")
	if err := os.MkdirAll(filepath.Join(home, ".config", "snip", "filters"), 0o755); err != nil {
		t.Fatal(err)
	}

	userContent := `[filters.enable]
git-log = true
git-diff = true

[filters.global]
max_lines = 100
`
	userPath := filepath.Join(home, ".config", "snip", "config.toml")
	if err := os.WriteFile(userPath, []byte(userContent), 0o644); err != nil {
		t.Fatal(err)
	}

	projectDir := canonicalTempDir(t)
	if err := os.MkdirAll(filepath.Join(projectDir, ".snip"), 0o755); err != nil {
		t.Fatal(err)
	}
	projectContent := `mode = "project"

[filters.enable]
git-diff = false
curl = false

[filters.global]
max_lines = 50
max_line_length = 80

[filters.override.pyright]
stream_mode = "full"
`
	if err := os.WriteFile(filepath.Join(projectDir, ".snip", "config.toml"), []byte(projectContent), 0o644); err != nil {
		t.Fatal(err)
	}

	// Trust the project config so the trust gate passes
	projectCfgPath := filepath.Join(projectDir, ".snip", "config.toml")
	trustStore := make(trust.Store)
	hash, err := trust.HashFile(projectCfgPath)
	if err != nil {
		t.Fatal(err)
	}
	trustStore[projectCfgPath] = hash
	if err := trust.SaveTo(trustStore, filepath.Join(home, ".config", "snip", "trusted.json")); err != nil {
		t.Fatal(err)
	}

	t.Setenv("HOME", home)
	t.Setenv("SNIP_CONFIG", userPath)
	oldWd, _ := os.Getwd()
	_ = os.Chdir(projectDir)
	t.Cleanup(func() { _ = os.Chdir(oldWd) })

	cfg, err := LoadMerged()
	if err != nil {
		t.Fatalf("LoadMerged: %v", err)
	}

	// Enable: project keys override user
	if cfg.Filters.Enable["git-diff"] != false {
		t.Error("project should disable git-diff")
	}
	if cfg.Filters.Enable["curl"] != false {
		t.Error("project should disable curl")
	}
	if cfg.Filters.Enable["git-log"] != true {
		t.Error("user git-log (not in project) should stay enabled")
	}

	// Global: project wins entirely
	if cfg.Filters.Global.MaxLines != 50 {
		t.Errorf("global.MaxLines = %d, want 50", cfg.Filters.Global.MaxLines)
	}
	if cfg.Filters.Global.MaxLineLength != 80 {
		t.Errorf("global.MaxLineLength = %d, want 80", cfg.Filters.Global.MaxLineLength)
	}

	// Override: project wins
	if cfg.Filters.Override["pyright"].StreamMode != "full" {
		t.Error("expected pyright override stream_mode=full")
	}

	// Mode: should be "project"
	if cfg.Mode != "project" {
		t.Errorf("mode = %q, want 'project'", cfg.Mode)
	}
}

func TestLoadMergedGlobalOverrideGate(t *testing.T) {
	// When project config only sets max_line_length (no max_lines, no stream_mode),
	// the global override should still apply.
	dir := t.TempDir()
	home := filepath.Join(dir, "home")
	if err := os.MkdirAll(filepath.Join(home, ".config", "snip", "filters"), 0o755); err != nil {
		t.Fatal(err)
	}

	userContent := `[filters.global]
max_line_length = 200
`
	userPath := filepath.Join(home, ".config", "snip", "config.toml")
	if err := os.WriteFile(userPath, []byte(userContent), 0o644); err != nil {
		t.Fatal(err)
	}

	projectDir := canonicalTempDir(t)
	if err := os.MkdirAll(filepath.Join(projectDir, ".snip"), 0o755); err != nil {
		t.Fatal(err)
	}
	projectContent := `mode = "project"

[filters.global]
max_line_length = 80
max_output_bytes = 1048576
`
	if err := os.WriteFile(filepath.Join(projectDir, ".snip", "config.toml"), []byte(projectContent), 0o644); err != nil {
		t.Fatal(err)
	}

	// Trust the project config so the trust gate passes
	projectCfgPath := filepath.Join(projectDir, ".snip", "config.toml")
	trustStore := make(trust.Store)
	hash, err := trust.HashFile(projectCfgPath)
	if err != nil {
		t.Fatal(err)
	}
	trustStore[projectCfgPath] = hash
	if err := trust.SaveTo(trustStore, filepath.Join(home, ".config", "snip", "trusted.json")); err != nil {
		t.Fatal(err)
	}

	t.Setenv("HOME", home)
	t.Setenv("SNIP_CONFIG", userPath)
	oldWd, _ := os.Getwd()
	_ = os.Chdir(projectDir)
	t.Cleanup(func() { _ = os.Chdir(oldWd) })

	cfg, err := LoadMerged()
	if err != nil {
		t.Fatalf("LoadMerged: %v", err)
	}
	if cfg.Filters.Global.MaxLineLength != 80 {
		t.Errorf("MaxLineLength = %d, want 80 (project override)", cfg.Filters.Global.MaxLineLength)
	}
	if cfg.Filters.Global.MaxOutputBytes != 1048576 {
		t.Errorf("MaxOutputBytes = %d, want 1048576 (project override)", cfg.Filters.Global.MaxOutputBytes)
	}
}

func TestLoadMergedGlobalOverrideGateOutputBytesOnly(t *testing.T) {
	dir := t.TempDir()
	home := filepath.Join(dir, "home")
	if err := os.MkdirAll(filepath.Join(home, ".config", "snip", "filters"), 0o755); err != nil {
		t.Fatal(err)
	}

	userContent := `[filters.global]
max_line_length = 200
`
	userPath := filepath.Join(home, ".config", "snip", "config.toml")
	if err := os.WriteFile(userPath, []byte(userContent), 0o644); err != nil {
		t.Fatal(err)
	}

	projectDir := canonicalTempDir(t)
	if err := os.MkdirAll(filepath.Join(projectDir, ".snip"), 0o755); err != nil {
		t.Fatal(err)
	}
	projectContent := `mode = "project"

[filters.global]
max_output_bytes = 1048576
`
	if err := os.WriteFile(filepath.Join(projectDir, ".snip", "config.toml"), []byte(projectContent), 0o644); err != nil {
		t.Fatal(err)
	}

	projectCfgPath := filepath.Join(projectDir, ".snip", "config.toml")
	trustStore := make(trust.Store)
	hash, err := trust.HashFile(projectCfgPath)
	if err != nil {
		t.Fatal(err)
	}
	trustStore[projectCfgPath] = hash
	if err := trust.SaveTo(trustStore, filepath.Join(home, ".config", "snip", "trusted.json")); err != nil {
		t.Fatal(err)
	}

	t.Setenv("HOME", home)
	t.Setenv("SNIP_CONFIG", userPath)
	oldWd, _ := os.Getwd()
	_ = os.Chdir(projectDir)
	t.Cleanup(func() { _ = os.Chdir(oldWd) })

	cfg, err := LoadMerged()
	if err != nil {
		t.Fatalf("LoadMerged: %v", err)
	}
	if cfg.Filters.Global.MaxOutputBytes != 1048576 {
		t.Errorf("MaxOutputBytes = %d, want 1048576", cfg.Filters.Global.MaxOutputBytes)
	}
}

func TestLoadMergedBypassMerge(t *testing.T) {
	// Bypass list should merge from both user and project.
	dir := t.TempDir()
	home := filepath.Join(dir, "home")
	if err := os.MkdirAll(filepath.Join(home, ".config", "snip", "filters"), 0o755); err != nil {
		t.Fatal(err)
	}

	userContent := `[filters.bypass]
commands = ["curl", "wget"]
`
	userPath := filepath.Join(home, ".config", "snip", "config.toml")
	if err := os.WriteFile(userPath, []byte(userContent), 0o644); err != nil {
		t.Fatal(err)
	}

	projectDir := canonicalTempDir(t)
	if err := os.MkdirAll(filepath.Join(projectDir, ".snip"), 0o755); err != nil {
		t.Fatal(err)
	}
	projectContent := `[filters.bypass]
commands = ["psql", "jq"]
`
	if err := os.WriteFile(filepath.Join(projectDir, ".snip", "config.toml"), []byte(projectContent), 0o644); err != nil {
		t.Fatal(err)
	}

	// Trust the project config so the trust gate passes
	projectCfgPath := filepath.Join(projectDir, ".snip", "config.toml")
	trustStore := make(trust.Store)
	hash, err := trust.HashFile(projectCfgPath)
	if err != nil {
		t.Fatal(err)
	}
	trustStore[projectCfgPath] = hash
	if err := trust.SaveTo(trustStore, filepath.Join(home, ".config", "snip", "trusted.json")); err != nil {
		t.Fatal(err)
	}

	t.Setenv("HOME", home)
	t.Setenv("SNIP_CONFIG", userPath)
	oldWd, _ := os.Getwd()
	_ = os.Chdir(projectDir)
	t.Cleanup(func() { _ = os.Chdir(oldWd) })

	cfg, err := LoadMerged()
	if err != nil {
		t.Fatalf("LoadMerged: %v", err)
	}
	if len(cfg.Filters.Bypass.Commands) != 4 {
		t.Fatalf("bypass commands len = %d, want 4: %v", len(cfg.Filters.Bypass.Commands), cfg.Filters.Bypass.Commands)
	}
	has := func(s string) bool {
		for _, c := range cfg.Filters.Bypass.Commands {
			if c == s {
				return true
			}
		}
		return false
	}
	for _, want := range []string{"curl", "wget", "psql", "jq"} {
		if !has(want) {
			t.Errorf("bypass missing %q", want)
		}
	}
}

func TestLoadMergedOverrideNilMaps(t *testing.T) {
	// When user has no Override map and project adds overrides,
	// the code should initialize the map without panicking.
	dir := t.TempDir()
	home := filepath.Join(dir, "home")
	if err := os.MkdirAll(filepath.Join(home, ".config", "snip", "filters"), 0o755); err != nil {
		t.Fatal(err)
	}

	userContent := `[display]
quiet_no_filter = true
`
	userPath := filepath.Join(home, ".config", "snip", "config.toml")
	if err := os.WriteFile(userPath, []byte(userContent), 0o644); err != nil {
		t.Fatal(err)
	}

	projectDir := canonicalTempDir(t)
	if err := os.MkdirAll(filepath.Join(projectDir, ".snip"), 0o755); err != nil {
		t.Fatal(err)
	}
	projectContent := `mode = "project"

[filters.override.ls]
head = 0

[filters.override.pytest]
head = 200
`
	if err := os.WriteFile(filepath.Join(projectDir, ".snip", "config.toml"), []byte(projectContent), 0o644); err != nil {
		t.Fatal(err)
	}

	// Trust the project config so the trust gate passes
	projectCfgPath := filepath.Join(projectDir, ".snip", "config.toml")
	trustStore := make(trust.Store)
	hash, err := trust.HashFile(projectCfgPath)
	if err != nil {
		t.Fatal(err)
	}
	trustStore[projectCfgPath] = hash
	if err := trust.SaveTo(trustStore, filepath.Join(home, ".config", "snip", "trusted.json")); err != nil {
		t.Fatal(err)
	}

	t.Setenv("HOME", home)
	t.Setenv("SNIP_CONFIG", userPath)
	oldWd, _ := os.Getwd()
	_ = os.Chdir(projectDir)
	t.Cleanup(func() { _ = os.Chdir(oldWd) })

	cfg, err := LoadMerged()
	if err != nil {
		t.Fatalf("LoadMerged: %v", err)
	}
	if cfg.Filters.Override == nil {
		t.Fatal("expected non-nil Override map")
	}
	if cfg.Filters.Override["ls"].Head != 0 {
		t.Errorf("ls head = %d, want 0", cfg.Filters.Override["ls"].Head)
	}
	if cfg.Filters.Override["pytest"].Head != 200 {
		t.Errorf("pytest head = %d, want 200", cfg.Filters.Override["pytest"].Head)
	}
}

func TestLoadMergedUntrustedProjectConfigIgnored(t *testing.T) {
	baseDir := canonicalTempDir(t)
	home := filepath.Join(baseDir, "home")
	projectDir := filepath.Join(baseDir, "project")

	if err := os.MkdirAll(filepath.Join(home, ".config", "snip", "filters"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(projectDir, ".snip"), 0o755); err != nil {
		t.Fatal(err)
	}

	userContent := "[filters.enable]\ngit-diff = true\n"
	userPath := filepath.Join(home, ".config", "snip", "config.toml")
	if err := os.WriteFile(userPath, []byte(userContent), 0o644); err != nil {
		t.Fatal(err)
	}

	projectContent := "mode = \"project\"\n\n[filters.enable]\ngit-diff = false\n"
	projectPath := filepath.Join(projectDir, ".snip", "config.toml")
	if err := os.WriteFile(projectPath, []byte(projectContent), 0o644); err != nil {
		t.Fatal(err)
	}

	t.Setenv("HOME", home)
	t.Setenv("SNIP_CONFIG", userPath)
	oldWd, _ := os.Getwd()
	_ = os.Chdir(projectDir)
	t.Cleanup(func() { _ = os.Chdir(oldWd) })

	cfg, err := LoadMerged()
	if err != nil {
		t.Fatalf("LoadMerged: %v", err)
	}
	if cfg.Filters.Enable["git-diff"] != true {
		t.Error("untrusted project config should be ignored, git-diff should be true from user")
	}
}

func TestLoadMergedTrustedProjectConfigApplied(t *testing.T) {
	baseDir := canonicalTempDir(t)
	home := filepath.Join(baseDir, "home")
	projectDir := filepath.Join(baseDir, "project")

	if err := os.MkdirAll(filepath.Join(home, ".config", "snip", "filters"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(projectDir, ".snip"), 0o755); err != nil {
		t.Fatal(err)
	}

	userContent := "[filters.enable]\ngit-log = true\n"
	userPath := filepath.Join(home, ".config", "snip", "config.toml")
	if err := os.WriteFile(userPath, []byte(userContent), 0o644); err != nil {
		t.Fatal(err)
	}

	projectContent := "mode = \"project\"\n\n[filters.enable]\ngit-log = false\n"
	projectPath := filepath.Join(projectDir, ".snip", "config.toml")
	if err := os.WriteFile(projectPath, []byte(projectContent), 0o644); err != nil {
		t.Fatal(err)
	}

	store := make(trust.Store)
	hash, err := trust.HashFile(projectPath)
	if err != nil {
		t.Fatal(err)
	}
	abs, _ := filepath.Abs(projectPath)
	store[abs] = hash
	trustedPath := filepath.Join(home, ".config", "snip", "trusted.json")
	if err := trust.SaveTo(store, trustedPath); err != nil {
		t.Fatal(err)
	}

	t.Setenv("HOME", home)
	t.Setenv("SNIP_CONFIG", userPath)
	oldWd, _ := os.Getwd()
	_ = os.Chdir(projectDir)
	t.Cleanup(func() { _ = os.Chdir(oldWd) })

	cfg, err := LoadMerged()
	if err != nil {
		t.Fatalf("LoadMerged: %v", err)
	}
	if cfg.Filters.Enable["git-log"] != false {
		t.Error("trusted project config should override user, git-log should be false")
	}
}

func TestLoadMergedMissingTrustStoreReturnsUserOnly(t *testing.T) {
	baseDir := canonicalTempDir(t)
	home := filepath.Join(baseDir, "home")
	projectDir := filepath.Join(baseDir, "project")

	if err := os.MkdirAll(filepath.Join(home, ".config", "snip", "filters"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(projectDir, ".snip"), 0o755); err != nil {
		t.Fatal(err)
	}

	userContent := "[filters.enable]\ngit-diff = true\n"
	userPath := filepath.Join(home, ".config", "snip", "config.toml")
	if err := os.WriteFile(userPath, []byte(userContent), 0o644); err != nil {
		t.Fatal(err)
	}

	projectContent := "mode = \"project\"\n\n[filters.enable]\ngit-diff = false\n"
	projectPath := filepath.Join(projectDir, ".snip", "config.toml")
	if err := os.WriteFile(projectPath, []byte(projectContent), 0o644); err != nil {
		t.Fatal(err)
	}

	trustedPath := filepath.Join(home, ".config", "snip", "trusted.json")
	_ = os.Remove(trustedPath)

	t.Setenv("HOME", home)
	t.Setenv("SNIP_CONFIG", userPath)
	oldWd, _ := os.Getwd()
	_ = os.Chdir(projectDir)
	t.Cleanup(func() { _ = os.Chdir(oldWd) })

	cfg, err := LoadMerged()
	if err != nil {
		t.Fatalf("LoadMerged: %v", err)
	}
	if cfg.Filters.Enable["git-diff"] != true {
		t.Error("missing trust store should fall back to user config only")
	}
}

func TestLoadMergedMalformedTrustedConfigReturnsError(t *testing.T) {
	baseDir := canonicalTempDir(t)
	home := filepath.Join(baseDir, "home")
	projectDir := filepath.Join(baseDir, "project")

	if err := os.MkdirAll(filepath.Join(home, ".config", "snip", "filters"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(projectDir, ".snip"), 0o755); err != nil {
		t.Fatal(err)
	}

	userContent := "[display]\nquiet_no_filter = true\n"
	userPath := filepath.Join(home, ".config", "snip", "config.toml")
	if err := os.WriteFile(userPath, []byte(userContent), 0o644); err != nil {
		t.Fatal(err)
	}

	projectContent := "mode = \"project\"\nmissing_bracket = true\n\n[filters.enable\ngit-diff = false\n"
	projectPath := filepath.Join(projectDir, ".snip", "config.toml")
	if err := os.WriteFile(projectPath, []byte(projectContent), 0o644); err != nil {
		t.Fatal(err)
	}

	store := make(trust.Store)
	hash, err := trust.HashFile(projectPath)
	if err != nil {
		t.Fatal(err)
	}
	abs, _ := filepath.Abs(projectPath)
	store[abs] = hash
	trustedPath := filepath.Join(home, ".config", "snip", "trusted.json")
	if err := trust.SaveTo(store, trustedPath); err != nil {
		t.Fatal(err)
	}

	t.Setenv("HOME", home)
	t.Setenv("SNIP_CONFIG", userPath)
	oldWd, _ := os.Getwd()
	_ = os.Chdir(projectDir)
	t.Cleanup(func() { _ = os.Chdir(oldWd) })

	cfg, err := LoadMerged()
	if err == nil {
		t.Fatal("expected error for malformed trusted project config")
	}
	if cfg != nil {
		t.Error("expected nil config on malformed project config")
	}
}

func TestLoadMergedTrustStoreLoadErrorReturnsUserOnly(t *testing.T) {
	baseDir := canonicalTempDir(t)
	home := filepath.Join(baseDir, "home")
	projectDir := filepath.Join(baseDir, "project")

	if err := os.MkdirAll(filepath.Join(home, ".config", "snip", "filters"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(projectDir, ".snip"), 0o755); err != nil {
		t.Fatal(err)
	}

	userContent := "[filters.enable]\ngit-diff = true\n"
	userPath := filepath.Join(home, ".config", "snip", "config.toml")
	if err := os.WriteFile(userPath, []byte(userContent), 0o644); err != nil {
		t.Fatal(err)
	}

	projectContent := "mode = \"project\"\n\n[filters.enable]\ngit-diff = false\n"
	projectPath := filepath.Join(projectDir, ".snip", "config.toml")
	if err := os.WriteFile(projectPath, []byte(projectContent), 0o644); err != nil {
		t.Fatal(err)
	}

	trustedPath := filepath.Join(home, ".config", "snip", "trusted.json")
	if err := os.WriteFile(trustedPath, []byte("not valid json{{{"), 0o644); err != nil {
		t.Fatal(err)
	}

	t.Setenv("HOME", home)
	t.Setenv("SNIP_CONFIG", userPath)
	oldWd, _ := os.Getwd()
	_ = os.Chdir(projectDir)
	t.Cleanup(func() { _ = os.Chdir(oldWd) })

	cfg, err := LoadMerged()
	if err != nil {
		t.Fatalf("LoadMerged: %v", err)
	}
	if cfg.Filters.Enable["git-diff"] != true {
		t.Error("corrupt trust store should fall back to user config only")
	}
}

func TestLoadMergedTrustRevokedAfterFileModified(t *testing.T) {
	// After snip trust, if the project config is modified (hash changes),
	// LoadMerged must fall back to user config only — the trust is revoked.
	baseDir := canonicalTempDir(t)
	home := filepath.Join(baseDir, "home")
	projectDir := filepath.Join(baseDir, "project")

	if err := os.MkdirAll(filepath.Join(home, ".config", "snip", "filters"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(projectDir, ".snip"), 0o755); err != nil {
		t.Fatal(err)
	}

	userContent := "[filters.enable]\ngit-diff = true\n"
	userPath := filepath.Join(home, ".config", "snip", "config.toml")
	if err := os.WriteFile(userPath, []byte(userContent), 0o644); err != nil {
		t.Fatal(err)
	}

	projectContent := "mode = \"project\"\n\n[filters.enable]\ngit-diff = false\n"
	projectPath := filepath.Join(projectDir, ".snip", "config.toml")
	if err := os.WriteFile(projectPath, []byte(projectContent), 0o644); err != nil {
		t.Fatal(err)
	}

	// Trust the original content
	store := make(trust.Store)
	hash, err := trust.HashFile(projectPath)
	if err != nil {
		t.Fatal(err)
	}
	abs, _ := filepath.Abs(projectPath)
	store[abs] = hash
	trustedPath := filepath.Join(home, ".config", "snip", "trusted.json")
	if err := trust.SaveTo(store, trustedPath); err != nil {
		t.Fatal(err)
	}

	// Modify the file — hash no longer matches
	modifiedContent := "mode = \"project\"\n\n[filters.enable]\ngit-diff = true\n"
	if err := os.WriteFile(projectPath, []byte(modifiedContent), 0o644); err != nil {
		t.Fatal(err)
	}

	t.Setenv("HOME", home)
	t.Setenv("SNIP_CONFIG", userPath)
	oldWd, _ := os.Getwd()
	_ = os.Chdir(projectDir)
	t.Cleanup(func() { _ = os.Chdir(oldWd) })

	cfg, err := LoadMerged()
	if err != nil {
		t.Fatalf("LoadMerged: %v", err)
	}
	// Hash mismatch — project config should be rejected, user config wins
	if cfg.Filters.Enable["git-diff"] != true {
		t.Error("modified config should fail trust check, git-diff should stay true (user)")
	}
}

func TestLoadMergedSymlinkedProjectConfig(t *testing.T) {
	// A symlinked .snip/config.toml should still work with the trust
	// gate — trust.IsTrusted resolves paths via filepath.Abs.
	baseDir := canonicalTempDir(t)
	home := filepath.Join(baseDir, "home")
	realProjectDir := filepath.Join(baseDir, "real-project")
	symProjectDir := filepath.Join(baseDir, "sym-project")

	if err := os.MkdirAll(filepath.Join(home, ".config", "snip", "filters"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(realProjectDir, ".snip"), 0o755); err != nil {
		t.Fatal(err)
	}

	userContent := "[filters.enable]\ngit-log = true\n"
	userPath := filepath.Join(home, ".config", "snip", "config.toml")
	if err := os.WriteFile(userPath, []byte(userContent), 0o644); err != nil {
		t.Fatal(err)
	}

	projectContent := "mode = \"project\"\n\n[filters.enable]\ngit-log = false\n"
	projectPath := filepath.Join(realProjectDir, ".snip", "config.toml")
	if err := os.WriteFile(projectPath, []byte(projectContent), 0o644); err != nil {
		t.Fatal(err)
	}

	// Symlink the project dir
	if err := os.MkdirAll(symProjectDir, 0o755); err != nil {
		t.Fatal(err)
	}
	symSnipDir := filepath.Join(symProjectDir, ".snip")
	if err := os.Symlink(filepath.Join(realProjectDir, ".snip"), symSnipDir); err != nil {
		t.Fatal(err)
	}

	// Trust the symlinked path (what projectConfigPath returns after Stat
	// follows the symlink to the real file).
	symProjectPath := filepath.Join(symProjectDir, ".snip", "config.toml")
	abs, _ := filepath.Abs(symProjectPath)
	store := make(trust.Store)
	hash, err := trust.HashFile(symProjectPath)
	if err != nil {
		t.Fatal(err)
	}
	store[abs] = hash
	if err := trust.SaveTo(store, filepath.Join(home, ".config", "snip", "trusted.json")); err != nil {
		t.Fatal(err)
	}

	t.Setenv("HOME", home)
	t.Setenv("SNIP_CONFIG", userPath)
	oldWd, _ := os.Getwd()
	// CWD is the symlinked dir — projectConfigPath walks up from here
	_ = os.Chdir(symProjectDir)
	t.Cleanup(func() { _ = os.Chdir(oldWd) })

	cfg, err := LoadMerged()
	if err != nil {
		t.Fatalf("LoadMerged: %v", err)
	}
	if cfg.Filters.Enable == nil {
		t.Fatal("expected non-nil Filters.Enable")
	}
	if cfg.Filters.Enable["git-log"] != false {
		t.Error("symlinked project config should be trusted, git-log should be false")
	}
}

func TestLoadMergedCorruptUserConfigReturnsError(t *testing.T) {
	baseDir := canonicalTempDir(t)
	home := filepath.Join(baseDir, "home")
	if err := os.MkdirAll(filepath.Join(home, ".config", "snip", "filters"), 0o755); err != nil {
		t.Fatal(err)
	}

	userContent := "this is not valid toml {{{"
	userPath := filepath.Join(home, ".config", "snip", "config.toml")
	if err := os.WriteFile(userPath, []byte(userContent), 0o644); err != nil {
		t.Fatal(err)
	}

	t.Setenv("HOME", home)
	t.Setenv("SNIP_CONFIG", userPath)

	cfg, err := LoadMerged()
	if err == nil {
		t.Fatal("expected error for corrupt user config")
	}
	if cfg != nil {
		t.Error("expected nil config on corrupt user config")
	}
}

func TestLoadMergedProjectConfigWithoutProjectMode(t *testing.T) {
	// Project config with mode="user" (or no mode) should NOT override user
	// settings. Only the bypass list merges unconditionally.
	dir := t.TempDir()
	home := filepath.Join(dir, "home")
	if err := os.MkdirAll(filepath.Join(home, ".config", "snip", "filters"), 0o755); err != nil {
		t.Fatal(err)
	}

	userContent := "[filters.enable]\ngit-diff = true\n[filters.global]\nmax_lines = 100\n"
	userPath := filepath.Join(home, ".config", "snip", "config.toml")
	if err := os.WriteFile(userPath, []byte(userContent), 0o644); err != nil {
		t.Fatal(err)
	}

	projectDir := canonicalTempDir(t)
	if err := os.MkdirAll(filepath.Join(projectDir, ".snip"), 0o755); err != nil {
		t.Fatal(err)
	}
	// mode is "user" — project should NOT override
	projectContent := "mode = \"user\"\n\n[filters.enable]\ngit-diff = false\n[filters.global]\nmax_lines = 200\n"
	projectPath := filepath.Join(projectDir, ".snip", "config.toml")
	if err := os.WriteFile(projectPath, []byte(projectContent), 0o644); err != nil {
		t.Fatal(err)
	}

	// Trust the project config
	store := make(trust.Store)
	hash, err := trust.HashFile(projectPath)
	if err != nil {
		t.Fatal(err)
	}
	abs, _ := filepath.Abs(projectPath)
	store[abs] = hash
	if err := trust.SaveTo(store, filepath.Join(home, ".config", "snip", "trusted.json")); err != nil {
		t.Fatal(err)
	}

	t.Setenv("HOME", home)
	t.Setenv("SNIP_CONFIG", userPath)
	oldWd, _ := os.Getwd()
	_ = os.Chdir(projectDir)
	t.Cleanup(func() { _ = os.Chdir(oldWd) })

	cfg, err := LoadMerged()
	if err != nil {
		t.Fatalf("LoadMerged: %v", err)
	}
	// User settings should be preserved since mode is not "project"
	if cfg.Filters.Enable["git-diff"] != true {
		t.Error("user git-diff should stay true when project mode is user")
	}
	if cfg.Filters.Global.MaxLines != 100 {
		t.Errorf("user max_lines = %d, want 100 (project not overriding)", cfg.Filters.Global.MaxLines)
	}
}

func TestLoadMergedMinimalProjectConfig(t *testing.T) {
	// Project config with mode="project" but NO enable, global, or override
	// settings. Must not crash or produce unexpected state.
	dir := t.TempDir()
	home := filepath.Join(dir, "home")
	if err := os.MkdirAll(filepath.Join(home, ".config", "snip", "filters"), 0o755); err != nil {
		t.Fatal(err)
	}

	userContent := "[filters.enable]\ngit-diff = true\n"
	userPath := filepath.Join(home, ".config", "snip", "config.toml")
	if err := os.WriteFile(userPath, []byte(userContent), 0o644); err != nil {
		t.Fatal(err)
	}

	projectDir := canonicalTempDir(t)
	if err := os.MkdirAll(filepath.Join(projectDir, ".snip"), 0o755); err != nil {
		t.Fatal(err)
	}
	projectContent := "mode = \"project\"\n"
	projectPath := filepath.Join(projectDir, ".snip", "config.toml")
	if err := os.WriteFile(projectPath, []byte(projectContent), 0o644); err != nil {
		t.Fatal(err)
	}

	store := make(trust.Store)
	hash, err := trust.HashFile(projectPath)
	if err != nil {
		t.Fatal(err)
	}
	abs, _ := filepath.Abs(projectPath)
	store[abs] = hash
	if err := trust.SaveTo(store, filepath.Join(home, ".config", "snip", "trusted.json")); err != nil {
		t.Fatal(err)
	}

	t.Setenv("HOME", home)
	t.Setenv("SNIP_CONFIG", userPath)
	oldWd, _ := os.Getwd()
	_ = os.Chdir(projectDir)
	t.Cleanup(func() { _ = os.Chdir(oldWd) })

	cfg, err := LoadMerged()
	if err != nil {
		t.Fatalf("LoadMerged: %v", err)
	}
	if cfg.Mode != "project" {
		t.Errorf("mode = %q, want project", cfg.Mode)
	}
	// User settings preserved (project had nothing to override)
	if cfg.Filters.Enable["git-diff"] != true {
		t.Error("user git-diff should be preserved with minimal project config")
	}
}
