package initcmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// copilotHookSubcommand is the snip subsubcommand the GitHub Copilot CLI invokes.
const copilotHookSubcommand = "hook copilot"

// snipPromptMarker is the header snip writes into every prompt-injection file;
// uninstall only removes such files when this marker is still present.
const snipPromptMarker = "# Snip - CLI Token Optimizer"

// copilotHookFile is the snip-owned hook config filename. The Copilot CLI loads
// every *.json under its hooks directory, so snip writes a dedicated file rather
// than merging into a shared one (install = write, uninstall = remove).
const copilotHookFile = "snip.json"

// copilotHooksDir returns the Copilot CLI hooks directory for the given home.
func copilotHooksDir(home string) string {
	return filepath.Join(home, ".copilot", "hooks")
}

// copilotHookPath returns the snip hook config path for the given home.
func copilotHookPath(home string) string {
	return filepath.Join(copilotHooksDir(home), copilotHookFile)
}

// initCopilotHook installs the snip GitHub Copilot CLI hook: writes
// ~/.copilot/hooks/snip.json with a preToolUse entry that pipes every command
// through `snip hook copilot`. snip is tool-name agnostic and emits nothing for
// non-command tools, so no matcher is needed.
func initCopilotHook(snipBin, home, filterDir string) error {
	// Copilot CLI preToolUse hooks are fail-closed: a non-zero exit denies the
	// command. snip always exits 0 (graceful degradation), and the binary path
	// is quoted so a space in it cannot break the bash invocation and deny every
	// command.
	hookCommand := fmt.Sprintf("%q %s", snipBin, copilotHookSubcommand)
	path := copilotHookPath(home)
	if err := writeCopilotHookConfig(path, hookCommand); err != nil {
		return fmt.Errorf("write copilot hook: %w", err)
	}

	fmt.Println("snip init complete:")
	fmt.Printf("  agent: copilot\n")
	fmt.Printf("  hook: %s\n", hookCommand)
	fmt.Printf("  filters: %s\n", filterDir)
	fmt.Printf("  config: %s\n", path)
	fmt.Println()
	fmt.Println("note: GitHub Copilot CLI hooks are GA. The hook is fail-closed, so")
	fmt.Println("      keep the snip binary on disk at the path above; snip itself")
	fmt.Println("      always exits 0 (it never blocks a command). For the older")
	fmt.Println("      prompt-injection setup use:  snip init --agent copilot --mode prompt")
	fmt.Println("      VS Code's agent hooks (Preview) reuse this same handler; wire")
	fmt.Println("      `snip hook copilot` manually as a PreToolUse command hook there.")
	return nil
}

// buildCopilotHookConfig returns the Copilot CLI hook config object that pipes
// matched commands through hookCommand.
func buildCopilotHookConfig(hookCommand string) map[string]any {
	return map[string]any{
		"version": 1,
		"hooks": map[string]any{
			"preToolUse": []any{
				map[string]any{"type": "command", "bash": hookCommand},
			},
		},
	}
}

// writeCopilotHookConfig writes the snip-owned Copilot CLI hook file, creating
// the hooks directory if needed.
func writeCopilotHookConfig(path, hookCommand string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create hooks dir: %w", err)
	}
	out, err := json.MarshalIndent(buildCopilotHookConfig(hookCommand), "", "  ")
	if err != nil {
		return fmt.Errorf("marshal hook config: %w", err)
	}
	return os.WriteFile(path, out, 0o644)
}

// uninstallCopilot removes the snip Copilot CLI hook file and, best-effort, a
// legacy prompt-injection file left by `--mode prompt`.
func uninstallCopilot() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("get home dir: %w", err)
	}

	// Remove the snip-owned hook file (snip owns this filename entirely).
	if err := os.Remove(copilotHookPath(home)); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove copilot hook: %w", err)
	}

	// Best-effort cleanup of a legacy prompt-injection file, only when it still
	// matches the snip template (don't touch a file the user has edited).
	removeLegacyPromptFile("copilot")

	fmt.Println("snip uninstalled (copilot)")
	return nil
}

// removeLegacyPromptFile deletes the prompt-injection file for agent if it still
// contains the snip template marker, then prunes any empty parent directories.
func removeLegacyPromptFile(agent string) {
	filename, ok := promptAgentFiles[agent]
	if !ok {
		return
	}
	targetPath := filepath.Join(".", filename)
	data, err := os.ReadFile(targetPath)
	if err != nil {
		return
	}
	if !strings.Contains(string(data), snipPromptMarker) {
		return
	}
	if err := os.Remove(targetPath); err != nil {
		return
	}
	for dir := filepath.Dir(targetPath); dir != "." && dir != string(filepath.Separator); dir = filepath.Dir(dir) {
		if err := os.Remove(dir); err != nil {
			break
		}
	}
}
