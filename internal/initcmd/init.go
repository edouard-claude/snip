package initcmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// hookScript reads JSON from stdin (Claude Code PreToolUse protocol),
// rewrites supported commands through snip, and returns updatedInput JSON.
// Requires jq. Falls back silently (exit 0) if snip or jq are missing.
const hookScript = `#!/bin/bash
# snip — CLI Token Killer hook for Claude Code
# PreToolUse hook: reads JSON from stdin, rewrites command through snip

# Graceful degradation: if snip or jq are missing, allow original command
if ! command -v snip &>/dev/null || ! command -v jq &>/dev/null; then
  exit 0
fi

set -euo pipefail

INPUT=$(cat)
CMD=$(echo "$INPUT" | jq -r '.tool_input.command // empty')

# Nothing to rewrite
if [ -z "$CMD" ]; then
  exit 0
fi

# Extract the first command (before && or | or ;)
FIRST_CMD=$(echo "$CMD" | sed 's/[;&|].*//' | xargs)

# Skip if already using snip
case "$FIRST_CMD" in
  snip\ *|*/snip\ *) exit 0 ;;
esac

# Strip leading env var assignments (e.g. CGO_ENABLED=0 go test)
BARE_CMD=$(echo "$FIRST_CMD" | sed 's/^[A-Za-z_][A-Za-z0-9_]*=[^ ]* *//')

# Extract the base command name
BASE=$(echo "$BARE_CMD" | awk '{print $1}')

# Check if this command is supported
REWRITE=""
case "$BASE" in
  git|go|cargo|npm|npx|yarn|pnpm|docker|kubectl|make|pip|pytest|jest|tsc|eslint|rustc)
    # Rewrite: prefix with snip
    REWRITE=$(echo "$CMD" | sed "s|$BARE_CMD|snip $BARE_CMD|")
    ;;
esac

# No match — allow original command unchanged
if [ -z "$REWRITE" ]; then
  exit 0
fi

# Build updatedInput preserving all original fields
ORIGINAL_INPUT=$(echo "$INPUT" | jq -c '.tool_input')
UPDATED_INPUT=$(echo "$ORIGINAL_INPUT" | jq --arg cmd "$REWRITE" '.command = $cmd')

# Return rewrite instruction
jq -n \
  --argjson updated "$UPDATED_INPUT" \
  '{
    "hookSpecificOutput": {
      "hookEventName": "PreToolUse",
      "permissionDecision": "allow",
      "permissionDecisionReason": "snip auto-rewrite",
      "updatedInput": $updated
    }
  }'
`

const hookIdentifier = "snip-rewrite.sh"

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

	hookPath := filepath.Join(hookDir, hookIdentifier)
	if err := os.WriteFile(hookPath, []byte(hookScript), 0755); err != nil {
		return fmt.Errorf("write hook: %w", err)
	}

	// 3. Patch settings.json
	settingsPath := filepath.Join(home, ".claude", "settings.json")
	if err := patchSettings(settingsPath, hookPath); err != nil {
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

	hookPath := filepath.Join(home, ".claude", "hooks", hookIdentifier)
	os.Remove(hookPath)

	// Remove hook entry from settings.json
	settingsPath := filepath.Join(home, ".claude", "settings.json")
	unpatchSettings(settingsPath)

	fmt.Println("snip uninstalled")
	return nil
}

// patchSettings adds the snip hook to Claude Code settings.json.
// Uses the correct array-based PreToolUse format:
//
//	{"hooks": {"PreToolUse": [{"matcher": "Bash", "hooks": [{"type": "command", "command": "/path/to/snip-rewrite.sh"}]}]}}
func patchSettings(path, hookPath string) error {
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

	// Build the hook entry
	snipHookEntry := map[string]any{
		"type":    "command",
		"command": hookPath,
	}

	snipMatcher := map[string]any{
		"matcher": "Bash",
		"hooks":   []any{snipHookEntry},
	}

	// Get or create hooks section
	hooks, _ := settings["hooks"].(map[string]any)
	if hooks == nil {
		hooks = make(map[string]any)
	}

	// Get existing PreToolUse array or create new one
	var preToolUse []any
	if existing, ok := hooks["PreToolUse"]; ok {
		if arr, ok := existing.([]any); ok {
			preToolUse = arr
		}
	}

	// Check if snip hook already exists (idempotent)
	found := false
	for i, entry := range preToolUse {
		if isSnipEntry(entry) {
			preToolUse[i] = snipMatcher // Update in place
			found = true
			break
		}
	}
	if !found {
		preToolUse = append(preToolUse, snipMatcher)
	}

	hooks["PreToolUse"] = preToolUse
	settings["hooks"] = hooks

	out, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal settings: %w", err)
	}

	return os.WriteFile(path, out, 0644)
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

	existing, ok := hooks["PreToolUse"]
	if !ok {
		return
	}
	arr, ok := existing.([]any)
	if !ok {
		return
	}

	// Remove snip entries
	var filtered []any
	for _, entry := range arr {
		if !isSnipEntry(entry) {
			filtered = append(filtered, entry)
		}
	}

	if len(filtered) == 0 {
		delete(hooks, "PreToolUse")
	} else {
		hooks["PreToolUse"] = filtered
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

// isSnipEntry checks if a PreToolUse entry is a snip hook.
func isSnipEntry(entry any) bool {
	m, ok := entry.(map[string]any)
	if !ok {
		return false
	}
	// Check hooks sub-array for snip-rewrite.sh command
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
		if strings.Contains(cmd, hookIdentifier) {
			return true
		}
	}
	return false
}
