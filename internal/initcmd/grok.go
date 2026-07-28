package initcmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// grokHookSubcommand is the snip subsubcommand Grok Build invokes.
const grokHookSubcommand = "hook grok"

// grokMatcher is the PreToolUse matcher written to the hook config. The field
// is a regex tested against the tool name; it covers Grok Build's native shell
// tool (run_terminal_cmd) and the Claude-style alias (Bash) it also accepts.
const grokMatcher = "Bash|run_terminal_cmd"

// grokHookFile is the snip-owned hook config filename. Grok Build loads every
// *.json under its hooks directory, so snip writes a dedicated file rather
// than merging into a shared one (install = write, uninstall = remove).
const grokHookFile = "snip.json"

// grokHooksDir returns the Grok Build global hooks directory for the given
// home. Global hooks are always trusted (no /hooks-trust needed).
func grokHooksDir(home string) string {
	return filepath.Join(home, ".grok", "hooks")
}

// grokHookPath returns the snip hook config path for the given home.
func grokHookPath(home string) string {
	return filepath.Join(grokHooksDir(home), grokHookFile)
}

// initGrok installs the snip Grok Build hook: writes ~/.grok/hooks/snip.json
// with a PreToolUse entry that pipes shell commands through `snip hook grok`.
func initGrok(snipBin, home, filterDir string) error {
	// Grok Build runs the hook command through a shell; the binary path is
	// quoted so a space in it cannot break the invocation. Grok Build hooks are
	// fail-open (a crash or malformed output lets the command run unfiltered),
	// so a broken hook never blocks the agent.
	hookCommand := fmt.Sprintf("%q %s", snipBin, grokHookSubcommand)
	path := grokHookPath(home)
	if err := writeGrokHookConfig(path, hookCommand); err != nil {
		return fmt.Errorf("write grok hook: %w", err)
	}

	fmt.Println("snip init complete:")
	fmt.Printf("  agent: grok\n")
	fmt.Printf("  hook: %s\n", hookCommand)
	fmt.Printf("  filters: %s\n", filterDir)
	fmt.Printf("  config: %s\n", path)
	fmt.Println()
	fmt.Println("note: Grok Build hooks cannot rewrite commands in place, so snip denies")
	fmt.Println("      matched commands with a re-run suggestion.")
	fmt.Println("      Verify the hook is loaded with `grok inspect` or /hooks-list.")
	fmt.Println("      For the prompt-injection setup instead use:")
	fmt.Println("        snip init --agent grok --mode prompt")
	return nil
}

// buildGrokHookConfig returns the Grok Build hook config object that pipes
// matched shell commands through hookCommand.
func buildGrokHookConfig(hookCommand string) map[string]any {
	return map[string]any{
		"hooks": map[string]any{
			"PreToolUse": []any{
				map[string]any{
					"matcher": grokMatcher,
					"hooks": []any{
						map[string]any{"type": "command", "command": hookCommand},
					},
				},
			},
		},
	}
}

// writeGrokHookConfig writes the snip-owned Grok Build hook file, creating the
// hooks directory if needed.
func writeGrokHookConfig(path, hookCommand string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create hooks dir: %w", err)
	}
	out, err := json.MarshalIndent(buildGrokHookConfig(hookCommand), "", "  ")
	if err != nil {
		return fmt.Errorf("marshal hook config: %w", err)
	}
	return os.WriteFile(path, out, 0o644)
}

// uninstallGrok removes the snip Grok Build hook file and, best-effort, a
// legacy prompt-injection file left by `--mode prompt`.
func uninstallGrok() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("get home dir: %w", err)
	}

	// Remove the snip-owned hook file (snip owns this filename entirely).
	if err := os.Remove(grokHookPath(home)); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove grok hook: %w", err)
	}

	// Best-effort cleanup of a prompt-injection file, only when it still
	// matches the snip template (don't touch a file the user has edited).
	removeLegacyPromptFile("grok")

	fmt.Println("snip uninstalled (grok)")
	return nil
}
