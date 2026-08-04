package initcmd

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/edouard-claude/snip/internal/hook"
	toml "github.com/pelletier/go-toml/v2"
)

// codexHookSubcommand is the snip subsubcommand Codex invokes.
const codexHookSubcommand = "hook codex"

// codexConfigDir returns the Codex config directory for the given home.
func codexConfigDir(home string) string {
	return filepath.Join(home, ".codex")
}

// codexHooksPath returns the Codex hooks.json path for the given home.
func codexHooksPath(home string) string {
	return filepath.Join(codexConfigDir(home), "hooks.json")
}

// codexConfigPath returns the Codex config.toml path for the given home.
func codexConfigPath(home string) string {
	return filepath.Join(codexConfigDir(home), "config.toml")
}

// initCodex installs the snip Codex hook in ~/.codex/hooks.json. Codex hooks
// are enabled by default, so config.toml is only read to honour an explicit
// opt-out and is never written to.
func initCodex(snipBin, home, filterDir string) error {
	if err := checkCodexHooksEnabled(codexConfigPath(home)); err != nil {
		return err
	}

	if err := os.MkdirAll(codexConfigDir(home), 0o755); err != nil {
		return fmt.Errorf("create codex config dir: %w", err)
	}

	// Reuse the command-rewrite quoting so the installed hook works with the
	// platform shell, including cmd.exe on Windows.
	hookCommand := hook.QuoteBinFor(snipBin, runtime.GOOS) + " " + codexHookSubcommand

	hooksPath := codexHooksPath(home)
	if err := patchCodexHooks(hooksPath, hookCommand); err != nil {
		return fmt.Errorf("patch codex hooks: %w", err)
	}

	legacyAgentsMd := detectLegacyAgentsMD(".")

	fmt.Println("snip init complete:")
	fmt.Printf("  agent: codex\n")
	fmt.Printf("  hook: %s\n", hookCommand)
	fmt.Printf("  filters: %s\n", filterDir)
	fmt.Printf("  hooks: %s\n", hooksPath)
	fmt.Println()
	fmt.Println("note: Codex hooks require Codex CLI 0.131.0 or later for PreToolUse")
	fmt.Println("      updatedInput. Supported commands are transparently")
	fmt.Println("      rewritten through snip. For older Codex releases use:")
	fmt.Println("      snip init --agent codex --mode prompt")
	if legacyAgentsMd != "" {
		fmt.Printf("      legacy %s detected — remove with `snip init --agent codex --uninstall`\n", legacyAgentsMd)
		fmt.Println("      or delete it manually if it has been edited or committed")
	}

	return nil
}

// uninstallCodex removes only the snip entry from ~/.codex/hooks.json. It
// leaves config.toml untouched so it cannot disable the user's other Codex
// lifecycle hooks. It also removes a legacy AGENTS.md prompt file if it still
// matches the snip template.
func uninstallCodex() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("get home dir: %w", err)
	}

	hooksPath := codexHooksPath(home)
	if err := unpatchCodexHooks(hooksPath); err != nil {
		return fmt.Errorf("unpatch codex hooks: %w", err)
	}

	// Best-effort cleanup of a legacy AGENTS.md. Skipped when the file is shared
	// with another prompt-mode agent (grok), which the helper reports on.
	removeLegacyPromptFile("codex")

	fmt.Println("snip uninstalled (codex)")
	return nil
}

// patchCodexHooks adds the snip hook to ~/.codex/hooks.json. Idempotent:
// existing snip entries are updated in place; foreign entries are preserved.
func patchCodexHooks(path, hookCommand string) error {
	config, mode, err := readJSONMap(path)
	if err != nil {
		return err
	}

	snipMatcher := map[string]any{
		"matcher": "Bash",
		"hooks": []any{
			map[string]any{"type": "command", "command": hookCommand},
		},
	}

	hooks, _ := config["hooks"].(map[string]any)
	if hooks == nil {
		hooks = make(map[string]any)
	}

	var preToolUse []any
	if existing, ok := hooks["PreToolUse"]; ok {
		if arr, ok := existing.([]any); ok {
			preToolUse = arr
		}
	}

	found := false
	for i, entry := range preToolUse {
		if isSnipCodexEntry(entry) {
			preToolUse[i] = snipMatcher
			found = true
			break
		}
	}
	if !found {
		preToolUse = append(preToolUse, snipMatcher)
	}

	hooks["PreToolUse"] = preToolUse
	config["hooks"] = hooks

	return writeJSONMap(path, config, mode)
}

// unpatchCodexHooks removes snip entries from ~/.codex/hooks.json.
func unpatchCodexHooks(path string) error {
	config, mode, err := readJSONMap(path)
	if err != nil {
		return err
	}
	if config == nil {
		return nil
	}

	hooks, _ := config["hooks"].(map[string]any)
	if hooks == nil {
		return nil
	}

	existing, ok := hooks["PreToolUse"]
	if !ok {
		return nil
	}
	arr, ok := existing.([]any)
	if !ok {
		return nil
	}

	var filtered []any
	for _, entry := range arr {
		if !isSnipCodexEntry(entry) {
			filtered = append(filtered, entry)
		}
	}

	if len(filtered) == 0 {
		delete(hooks, "PreToolUse")
	} else {
		hooks["PreToolUse"] = filtered
	}
	if len(hooks) == 0 {
		delete(config, "hooks")
	}

	return writeJSONMap(path, config, mode)
}

// isSnipCodexEntry reports whether a Codex PreToolUse entry was installed by
// snip. Detection looks for "snip hook codex" inside any nested hook command.
func isSnipCodexEntry(entry any) bool {
	m, ok := entry.(map[string]any)
	if !ok {
		return false
	}
	hooksRaw, ok := m["hooks"]
	if !ok {
		return false
	}
	hooksArr, ok := hooksRaw.([]any)
	if !ok {
		return false
	}
	for _, h := range hooksArr {
		hm, ok := h.(map[string]any)
		if !ok {
			continue
		}
		cmd, _ := hm["command"].(string)
		if strings.Contains(cmd, codexHookSubcommand) {
			return true
		}
	}
	return false
}

// errCodexHooksExplicitlyDisabled is returned when the user has disabled
// Codex hooks in config.toml. We refuse to silently override their choice.
var errCodexHooksExplicitlyDisabled = errors.New(
	"~/.codex/config.toml has [features].hooks = false (or deprecated codex_hooks = false); " +
		"set it to true (or remove the line) and re-run snip init. " +
		"Older snip releases wrote codex_hooks = false themselves on uninstall, " +
		"so the line may not be yours")

// checkCodexHooksEnabled reports whether snip may install its Codex hook.
//
// config.toml is only read, never written: Codex enables hooks by default, so
// the file has nothing snip needs to add. The deprecated codex_hooks alias is
// still honoured by Codex, so an existing one is left in place rather than
// renamed. A config that cannot be read or parsed is an error, not an implicit
// "no opt-out": silently installing over an unreadable opt-out is the one
// outcome this check exists to prevent.
func checkCodexHooksEnabled(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read codex config.toml: %w", err)
	}

	cfg := make(map[string]any)
	if err := toml.Unmarshal(data, &cfg); err != nil {
		return fmt.Errorf("parse codex config.toml: %w", err)
	}

	features, _ := cfg["features"].(map[string]any)
	if featureFlagDisabled(features, "hooks") || featureFlagDisabled(features, "codex_hooks") {
		return errCodexHooksExplicitlyDisabled
	}
	return nil
}

// featureFlagDisabled reports whether key is present in features and set to
// the boolean false. A missing key or a non-boolean value is not an opt-out.
func featureFlagDisabled(features map[string]any, key string) bool {
	value, present := features[key]
	disabled, isBool := value.(bool)
	return present && isBool && !disabled
}

// detectLegacyAgentsMD returns the path to AGENTS.md in dir if its content
// matches the snip prompt-injection template, or "" otherwise. Used to print
// a migration hint without touching files the user may have edited.
func detectLegacyAgentsMD(dir string) string {
	path := filepath.Join(dir, "AGENTS.md")
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	if !strings.Contains(string(data), "# Snip - CLI Token Optimizer") {
		return ""
	}
	return path
}

// readJSONMap reads a JSON file as map[string]any, returning the discovered
// file mode. A missing file yields an empty map and the default mode.
// Writes a .bak alongside if the file is non-empty.
func readJSONMap(path string) (map[string]any, os.FileMode, error) {
	mode := os.FileMode(0o644)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return make(map[string]any), mode, nil
		}
		return nil, mode, fmt.Errorf("read %s: %w", filepath.Base(path), err)
	}
	if info, statErr := os.Stat(path); statErr == nil {
		mode = info.Mode().Perm()
	}
	_ = os.WriteFile(path+".bak", data, mode)

	m := make(map[string]any)
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) > 0 {
		if err := json.Unmarshal(trimmed, &m); err != nil {
			return nil, mode, fmt.Errorf("parse %s: %w", filepath.Base(path), err)
		}
	}
	return m, mode, nil
}

// writeJSONMap writes the given map to path with indented JSON, creating the
// parent directory if needed.
func writeJSONMap(path string, m map[string]any, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create dir: %w", err)
	}
	out, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}
	return os.WriteFile(path, out, mode)
}
