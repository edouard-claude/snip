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
	projectDir := t.TempDir()
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

	projectDir := t.TempDir()
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

	projectDir := t.TempDir()
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

	projectDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(projectDir, ".snip"), 0o755); err != nil {
		t.Fatal(err)
	}
	projectContent := `[filters.bypass]
commands = ["psql", "jq"]
`
	if err := os.WriteFile(filepath.Join(projectDir, ".snip", "config.toml"), []byte(projectContent), 0o644); err != nil {
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

	projectDir := t.TempDir()
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
