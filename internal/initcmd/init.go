package initcmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

const hookScript = `#!/bin/bash
# snip — CLI Token Killer hook for Claude Code
# Rewrites supported commands to run through snip

COMMAND="$1"
shift

case "$COMMAND" in
  git|go|cargo|npm|npx|yarn|pnpm|docker|kubectl|make|pip|pytest|jest|tsc|eslint|rustc)
    exec snip "$COMMAND" "$@"
    ;;
  *)
    exec "$COMMAND" "$@"
    ;;
esac
`

// Run installs the snip integration for Claude Code.
func Run(args []string) error {
	for _, arg := range args {
		if arg == "--uninstall" {
			return Uninstall()
		}
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("get home dir: %w", err)
	}

	// 1. Create filter directory
	filterDir := filepath.Join(home, ".config", "snip", "filters")
	if err := os.MkdirAll(filterDir, 0755); err != nil {
		return fmt.Errorf("create filter dir: %w", err)
	}

	// 2. Write hook script
	hookDir := filepath.Join(home, ".claude", "hooks")
	if err := os.MkdirAll(hookDir, 0755); err != nil {
		return fmt.Errorf("create hook dir: %w", err)
	}

	hookPath := filepath.Join(hookDir, "snip-rewrite.sh")
	if err := os.WriteFile(hookPath, []byte(hookScript), 0755); err != nil {
		return fmt.Errorf("write hook: %w", err)
	}

	// 3. Patch settings.json
	settingsPath := filepath.Join(home, ".claude", "settings.json")
	if err := patchSettings(settingsPath); err != nil {
		return fmt.Errorf("patch settings: %w", err)
	}

	fmt.Println("snip init complete:")
	fmt.Printf("  hook: %s\n", hookPath)
	fmt.Printf("  filters: %s\n", filterDir)
	fmt.Printf("  settings: %s\n", settingsPath)
	return nil
}

// Uninstall removes snip integration.
func Uninstall() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("get home dir: %w", err)
	}

	hookPath := filepath.Join(home, ".claude", "hooks", "snip-rewrite.sh")
	os.Remove(hookPath)

	// Remove hook entry from settings.json
	settingsPath := filepath.Join(home, ".claude", "settings.json")
	unpatchSettings(settingsPath)

	fmt.Println("snip uninstalled")
	return nil
}

func unpatchSettings(path string) {
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	var settings map[string]any
	if err := json.Unmarshal(data, &settings); err != nil {
		return
	}
	hooks, _ := settings["hooks"].(map[string]any)
	if hooks == nil {
		return
	}
	preToolUse, _ := hooks["PreToolUse"].(map[string]any)
	if preToolUse == nil {
		return
	}
	delete(preToolUse, "Bash")
	if len(preToolUse) == 0 {
		delete(hooks, "PreToolUse")
	}
	if len(hooks) == 0 {
		delete(settings, "hooks")
	}
	out, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return
	}
	os.WriteFile(path, out, 0644)
}

func patchSettings(path string) error {
	var settings map[string]any

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			settings = make(map[string]any)
		} else {
			return fmt.Errorf("read settings: %w", err)
		}
	} else {
		// Backup
		backupPath := path + ".bak"
		os.WriteFile(backupPath, data, 0644)

		if err := json.Unmarshal(data, &settings); err != nil {
			return fmt.Errorf("parse settings: %w", err)
		}
	}

	// Add or update hooks section (merge, don't overwrite)
	hooks, _ := settings["hooks"].(map[string]any)
	if hooks == nil {
		hooks = make(map[string]any)
	}

	preToolUse, _ := hooks["PreToolUse"].(map[string]any)
	if preToolUse == nil {
		preToolUse = make(map[string]any)
	}
	preToolUse["Bash"] = map[string]any{
		"command": "snip-rewrite.sh",
	}
	hooks["PreToolUse"] = preToolUse
	settings["hooks"] = hooks

	out, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal settings: %w", err)
	}

	return os.WriteFile(path, out, 0644)
}
