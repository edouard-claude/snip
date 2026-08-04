package initcmd

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPatchCodexHooksNew(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "hooks.json")
	hookCommand := "/usr/local/bin/snip hook codex"

	if err := patchCodexHooks(path, hookCommand); err != nil {
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
	if entry["matcher"] != "Bash" {
		t.Errorf("matcher = %v, want Bash", entry["matcher"])
	}
	entryHooks := entry["hooks"].([]any)
	hook := entryHooks[0].(map[string]any)
	if hook["command"] != hookCommand {
		t.Errorf("command = %v, want %s", hook["command"], hookCommand)
	}
}

func TestPatchCodexHooksWhitespaceOnlyFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "hooks.json")
	if err := os.WriteFile(path, []byte(" \n\t"), 0o600); err != nil {
		t.Fatal(err)
	}

	hookCommand := "/usr/local/bin/snip hook codex"
	if err := patchCodexHooks(path, hookCommand); err != nil {
		t.Fatalf("patch whitespace-only hooks.json: %v", err)
	}

	cfg := readSettings(t, path)
	hooks, ok := cfg["hooks"].(map[string]any)
	if !ok {
		t.Fatal("hooks not found")
	}
	preToolUse, ok := hooks["PreToolUse"].([]any)
	if !ok || len(preToolUse) != 1 {
		t.Fatalf("PreToolUse = %#v, want one entry", hooks["PreToolUse"])
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Errorf("mode = %o, want 600", got)
	}
}

func TestPatchCodexHooksIdempotent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "hooks.json")
	hookCommand := "/usr/local/bin/snip hook codex"

	_ = patchCodexHooks(path, hookCommand)
	_ = patchCodexHooks(path, hookCommand)

	cfg := readSettings(t, path)
	hooks := cfg["hooks"].(map[string]any)
	preToolUse := hooks["PreToolUse"].([]any)
	if len(preToolUse) != 1 {
		t.Errorf("expected 1 entry after double patch, got %d", len(preToolUse))
	}
}

func TestPatchCodexHooksPreservesForeignEntries(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "hooks.json")

	existing := map[string]any{
		"hooks": map[string]any{
			"PreToolUse": []any{
				map[string]any{
					"matcher": "Bash",
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

	if err := patchCodexHooks(path, "/usr/local/bin/snip hook codex"); err != nil {
		t.Fatalf("patch: %v", err)
	}

	cfg := readSettings(t, path)
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

func TestUnpatchCodexHooksRemovesOnlySnip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "hooks.json")

	existing := map[string]any{
		"hooks": map[string]any{
			"PreToolUse": []any{
				map[string]any{
					"matcher": "Bash",
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

	_ = patchCodexHooks(path, "/usr/local/bin/snip hook codex")
	if err := unpatchCodexHooks(path); err != nil {
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

func TestCheckCodexHooksEnabledMissingConfigIsAllowed(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")

	if err := checkCodexHooksEnabled(path); err != nil {
		t.Fatalf("check: %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("a missing config.toml must not be created")
	}
}

func TestCheckCodexHooksEnabledNeverWritesConfig(t *testing.T) {
	// Codex enables hooks by default and still honours the deprecated
	// codex_hooks alias, so snip has nothing to add to any of these.
	configs := map[string]string{
		"no features section": "model = 'gpt-5.6-terra'\n",
		"canonical key":       "# user comment\n[features]\nhooks = true\n",
		"legacy alias":        "[features]\ncodex_hooks = true # keep this comment\n",
		"both keys":           "[features]\nhooks = true\ncodex_hooks = true\n",
		"dotted legacy key":   "features.codex_hooks = true\n",
		"inline table":        "features = { codex_hooks = true }\n",
	}

	for name, original := range configs {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "config.toml")
			if err := os.WriteFile(path, []byte(original), 0o600); err != nil {
				t.Fatal(err)
			}
			before, err := os.Stat(path)
			if err != nil {
				t.Fatal(err)
			}

			if err := checkCodexHooksEnabled(path); err != nil {
				t.Fatalf("check: %v", err)
			}

			got, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if string(got) != original {
				t.Errorf("config.toml was rewritten:\n%s", got)
			}
			if after, err := os.Stat(path); err == nil && before.ModTime() != after.ModTime() {
				t.Error("config.toml was touched")
			}
			if _, err := os.Stat(path + ".bak"); !os.IsNotExist(err) {
				t.Error("no backup should be written: snip never edits config.toml")
			}
		})
	}
}

func TestCheckCodexHooksEnabledExplicitOptOutRefused(t *testing.T) {
	for _, setting := range []string{"hooks", "codex_hooks"} {
		t.Run(setting, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "config.toml")
			original := []byte("[features]\n" + setting + " = false\n")
			if err := os.WriteFile(path, original, 0o600); err != nil {
				t.Fatal(err)
			}

			err := checkCodexHooksEnabled(path)
			if !errors.Is(err, errCodexHooksExplicitlyDisabled) {
				t.Errorf("err = %v, want errCodexHooksExplicitlyDisabled", err)
			}
			got, readErr := os.ReadFile(path)
			if readErr != nil || string(got) != string(original) {
				t.Errorf("config changed after a refused install: %q (%v)", got, readErr)
			}
		})
	}
}

func TestCheckCodexHooksEnabledNonBooleanIsNotAnOptOut(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(path, []byte("[features]\nhooks = \"false\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := checkCodexHooksEnabled(path); err != nil {
		t.Errorf("a non-boolean value is not an opt-out, got %v", err)
	}
}

// An unreadable or malformed config.toml must not be read as "no opt-out":
// installing over an opt-out we simply failed to see is the outcome this
// check exists to prevent.
func TestCheckCodexHooksEnabledUnreadableConfigIsAnError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	// A directory in place of the file is an unreadable path on every OS,
	// unlike chmod 000 which root and Windows both ignore.
	if err := os.Mkdir(path, 0o755); err != nil {
		t.Fatal(err)
	}

	err := checkCodexHooksEnabled(path)
	if err == nil {
		t.Fatal("expected an error for an unreadable config.toml")
	}
	if errors.Is(err, errCodexHooksExplicitlyDisabled) {
		t.Errorf("err = %v, want a read error", err)
	}
}

func TestCheckCodexHooksEnabledMalformedConfigIsAnError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(path, []byte("oops =\n[features]\nhooks = false\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := checkCodexHooksEnabled(path); err == nil {
		t.Fatal("expected an error for a config.toml that does not parse")
	}
}

func TestInitCodexEndToEnd(t *testing.T) {
	home := t.TempDir()
	filterDir := filepath.Join(home, ".config", "snip", "filters")
	if err := os.MkdirAll(filterDir, 0o755); err != nil {
		t.Fatal(err)
	}

	if err := initCodex("/usr/local/bin/snip", home, filterDir); err != nil {
		t.Fatalf("initCodex: %v", err)
	}

	hooks := readSettings(t, codexHooksPath(home))
	preToolUse := hooks["hooks"].(map[string]any)["PreToolUse"].([]any)
	if len(preToolUse) != 1 {
		t.Fatalf("expected 1 PreToolUse entry, got %d", len(preToolUse))
	}
	entryHooks := preToolUse[0].(map[string]any)["hooks"].([]any)
	cmd := entryHooks[0].(map[string]any)["command"].(string)
	if cmd != `"/usr/local/bin/snip" hook codex` {
		t.Errorf("hook command = %q, want quoted binary path", cmd)
	}

	if _, err := os.Stat(codexConfigPath(home)); !os.IsNotExist(err) {
		t.Error("init should not create config.toml: Codex hooks are enabled by default")
	}
}

func TestInitCodexQuotesPathContainingSpaces(t *testing.T) {
	home := t.TempDir()
	filterDir := filepath.Join(home, ".config", "snip", "filters")
	if err := initCodex("/tmp/bin with space/snip", home, filterDir); err != nil {
		t.Fatalf("initCodex: %v", err)
	}
	hooks := readSettings(t, codexHooksPath(home))
	cmd := hooks["hooks"].(map[string]any)["PreToolUse"].([]any)[0].(map[string]any)["hooks"].([]any)[0].(map[string]any)["command"]
	if cmd != `"/tmp/bin with space/snip" hook codex` {
		t.Errorf("hook command = %q, want quoted path", cmd)
	}
}

func TestInitCodexRefusesDisabledHooksBeforeWritingHook(t *testing.T) {
	home := t.TempDir()
	configPath := codexConfigPath(home)
	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(configPath, []byte("[features]\nhooks = false\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	err := initCodex("/usr/local/bin/snip", home, filepath.Join(home, "filters"))
	if !errors.Is(err, errCodexHooksExplicitlyDisabled) {
		t.Fatalf("err = %v, want errCodexHooksExplicitlyDisabled", err)
	}
	if _, err := os.Stat(codexHooksPath(home)); !os.IsNotExist(err) {
		t.Error("hooks.json was written despite an explicit hooks=false opt-out")
	}
}

func TestInitCodexLeavesLegacyConfigUntouched(t *testing.T) {
	home := t.TempDir()
	configPath := codexConfigPath(home)
	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		t.Fatal(err)
	}
	original := []byte("[features]\ncodex_hooks = true # still honoured by Codex\n")
	if err := os.WriteFile(configPath, original, 0o600); err != nil {
		t.Fatal(err)
	}

	if err := initCodex("/usr/local/bin/snip", home, filepath.Join(home, "filters")); err != nil {
		t.Fatalf("initCodex: %v", err)
	}
	if _, err := os.Stat(codexHooksPath(home)); err != nil {
		t.Fatalf("hooks.json was not installed: %v", err)
	}
	got, err := os.ReadFile(configPath)
	if err != nil || string(got) != string(original) {
		t.Errorf("config changed during install: %q (%v)", got, err)
	}
	if _, err := os.Stat(configPath + ".bak"); !os.IsNotExist(err) {
		t.Error("install wrote a config.toml backup it does not need")
	}
}

// An install must fail closed when config.toml cannot be read: an opt-out we
// could not parse is still an opt-out.
func TestInitCodexRefusesUnreadableConfigBeforeWritingHook(t *testing.T) {
	home := t.TempDir()
	configPath := codexConfigPath(home)
	if err := os.MkdirAll(configPath, 0o755); err != nil {
		t.Fatal(err)
	}

	if err := initCodex("/usr/local/bin/snip", home, filepath.Join(home, "filters")); err == nil {
		t.Fatal("expected an error for an unreadable config.toml")
	}
	if _, err := os.Stat(codexHooksPath(home)); !os.IsNotExist(err) {
		t.Error("hooks.json was written despite an unreadable config.toml")
	}
}

func TestInitCodexThenUninstallLeavesConfigUntouched(t *testing.T) {
	home := t.TempDir()
	filterDir := filepath.Join(home, ".config", "snip", "filters")
	_ = os.MkdirAll(filterDir, 0o755)
	configPath := codexConfigPath(home)
	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		t.Fatal(err)
	}
	original := []byte("# preserve this exact config\n[features]\nhooks = true\n")
	if err := os.WriteFile(configPath, original, 0o600); err != nil {
		t.Fatal(err)
	}

	if err := initCodex("/usr/local/bin/snip", home, filterDir); err != nil {
		t.Fatalf("initCodex: %v", err)
	}

	t.Setenv("HOME", home)
	if err := uninstallCodex(); err != nil {
		t.Fatalf("uninstallCodex: %v", err)
	}

	hooks := readSettings(t, codexHooksPath(home))
	if h, ok := hooks["hooks"].(map[string]any); ok {
		if _, ok := h["PreToolUse"]; ok {
			t.Error("PreToolUse should be removed after uninstall")
		}
	}

	got, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(original) {
		t.Errorf("config.toml changed during init/uninstall. got:\n%s", got)
	}
}

func TestDetectLegacyAgentsMD(t *testing.T) {
	dir := t.TempDir()
	if got := detectLegacyAgentsMD(dir); got != "" {
		t.Errorf("detectLegacyAgentsMD on empty dir = %q, want empty", got)
	}

	// Foreign AGENTS.md
	foreign := filepath.Join(dir, "AGENTS.md")
	if err := os.WriteFile(foreign, []byte("# My project rules"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := detectLegacyAgentsMD(dir); got != "" {
		t.Errorf("foreign AGENTS.md misdetected as legacy: %q", got)
	}

	// Snip-template AGENTS.md
	if err := os.WriteFile(foreign, []byte(promptContent("/usr/local/bin/snip")), 0o644); err != nil {
		t.Fatal(err)
	}
	got := detectLegacyAgentsMD(dir)
	if got == "" {
		t.Error("snip AGENTS.md not detected as legacy")
	}
}

func TestParseModeFlag(t *testing.T) {
	mode, remaining := parseMode([]string{"--mode", "prompt", "--uninstall"})
	if mode != "prompt" {
		t.Errorf("mode = %q, want prompt", mode)
	}
	if len(remaining) != 1 || remaining[0] != "--uninstall" {
		t.Errorf("remaining = %v, want [--uninstall]", remaining)
	}

	mode, remaining = parseMode([]string{"--mode=hook"})
	if mode != "hook" {
		t.Errorf("mode = %q, want hook", mode)
	}
	if len(remaining) != 0 {
		t.Errorf("remaining = %v, want empty", remaining)
	}

	mode, _ = parseMode([]string{})
	if mode != "" {
		t.Errorf("mode = %q, want empty", mode)
	}
}

func TestRunRejectsUnknownMode(t *testing.T) {
	err := Run([]string{"--agent", "codex", "--mode", "wat"})
	if err == nil {
		t.Fatal("expected error for unknown mode")
	}
	if !strings.Contains(err.Error(), "unknown mode") {
		t.Errorf("err = %q, want to contain 'unknown mode'", err.Error())
	}
}
