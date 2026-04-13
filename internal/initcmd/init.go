package initcmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// generateHookScript returns the snip-rewrite.sh content with the absolute
// path of the snip binary embedded, so the hook works regardless of $PATH.
func generateHookScript(snipBin string) string {
	return `#!/bin/bash
# snip — CLI Token Killer hook for Claude Code
# PreToolUse hook: reads JSON from stdin, rewrites command through snip

SNIP_BIN="` + snipBin + `"

# Graceful degradation: if snip or jq are missing, allow original command
if ! command -v "$SNIP_BIN" &>/dev/null || ! command -v jq &>/dev/null; then
  echo "snip: invalid setup — binary not found at $SNIP_BIN" >&2
  exit 0
fi

set -euo pipefail

# If anything fails, fall back to allowing the original command unchanged.
# This prevents Claude Code from seeing "PreToolUse:Bash hook error".
trap 'exit 0' ERR

leading_ws_len() {
  local input="$1"
  local len=${#input}
  local i=0

  while [ $i -lt $len ]; do
    case "${input:$i:1}" in
      [[:space:]]) i=$((i + 1)) ;;
      *) break ;;
    esac
  done

  printf '%s' "$i"
}

trailing_ws_len() {
  local input="$1"
  local i=$((${#input} - 1))
  local count=0

  while [ $i -ge 0 ]; do
    case "${input:$i:1}" in
      [[:space:]])
        count=$((count + 1))
        i=$((i - 1))
        ;;
      *) break ;;
    esac
  done

  printf '%s' "$count"
}

extract_first_segment() {
  local input="$1"
  local len=${#input}
  local i=0
  local quote=""
  local ch

  while [ $i -lt $len ]; do
    ch="${input:$i:1}"

    if [ -n "$quote" ]; then
      if [ "$ch" = "\\" ] && [ "$quote" = '"' ]; then
        i=$((i + 2))
        continue
      fi

      if [ "$ch" = "$quote" ]; then
        quote=""
      fi

      i=$((i + 1))
      continue
    fi

    case "$ch" in
      "'") quote="'" ;;
      '"') quote='"' ;;
      ';'|'|'|'&') break ;;
    esac

    i=$((i + 1))
  done

  printf '%s' "${input:0:i}"
}

INPUT=$(cat)
CMD=$(echo "$INPUT" | jq -r '.tool_input.command // empty')

# Nothing to rewrite
if [ -z "$CMD" ]; then
  exit 0
fi

# Extract the first command segment, ignoring separators inside quotes.
# head -1 keeps heredoc bodies out of the scan.
FIRST_LINE=$(printf '%s\n' "$CMD" | head -1)
FIRST_SEGMENT=$(extract_first_segment "$FIRST_LINE")
LEADING_WS_LEN=$(leading_ws_len "$FIRST_SEGMENT")
TRAILING_WS_LEN=$(trailing_ws_len "$FIRST_SEGMENT")
FIRST_CMD_LEN=$((${#FIRST_SEGMENT} - LEADING_WS_LEN - TRAILING_WS_LEN))
if [ $FIRST_CMD_LEN -lt 0 ]; then
  FIRST_CMD_LEN=0
fi
FIRST_PREFIX="${FIRST_SEGMENT:0:LEADING_WS_LEN}"
FIRST_CMD="${FIRST_SEGMENT:LEADING_WS_LEN:FIRST_CMD_LEN}"
FIRST_SUFFIX_START=$((LEADING_WS_LEN + FIRST_CMD_LEN))
FIRST_SUFFIX="${FIRST_SEGMENT:FIRST_SUFFIX_START}"

# Skip if already using snip
if [ "$FIRST_CMD" = "$SNIP_BIN" ]; then
  exit 0
fi

# Strip leading env var assignments (e.g. CGO_ENABLED=0 go test)
ENV_PREFIX=$(printf '%s' "$FIRST_CMD" | sed -E 's/^(([A-Za-z_][A-Za-z0-9_]*=[^[:space:]]+[[:space:]]*)*).*/\1/')
BARE_CMD="${FIRST_CMD:${#ENV_PREFIX}}"

# Extract the base command name
BASE=$(echo "$BARE_CMD" | awk '{print $1}')

# Check if this command is supported
REWRITE=""
case "$BASE" in
  git|go|cargo|npm|npx|yarn|pnpm|docker|kubectl|make|pip|pytest|jest|tsc|eslint|rustc)
    # Rewrite: prefix with "$SNIP_BIN --" so flags like --help or --version in the
    # original command are passed verbatim to the underlying tool, not parsed
    # by snip itself.
    REST="${CMD:${#FIRST_SEGMENT}}"
    REWRITE="${FIRST_PREFIX}${ENV_PREFIX}\"$SNIP_BIN\" -- ${BARE_CMD}${FIRST_SUFFIX}${REST}"
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
}

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

	// Resolve the absolute path of the running snip binary so the hook script
	// can call it directly without relying on $PATH.
	snipBin, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolve executable: %w", err)
	}
	snipBin, err = filepath.EvalSymlinks(snipBin)
	if err != nil {
		return fmt.Errorf("eval symlinks: %w", err)
	}
	snipBin = filepath.ToSlash(snipBin)

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
	if err := os.WriteFile(hookPath, []byte(generateHookScript(snipBin)), 0755); err != nil {
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
	_ = os.Remove(hookPath)

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
		// Backup (best-effort)
		backupPath := path + ".bak"
		_ = os.WriteFile(backupPath, data, 0644)

		if err := json.Unmarshal(data, &settings); err != nil {
			return fmt.Errorf("parse settings: %w", err)
		}
	}

	// Build the hook entry — normalize to forward slashes for bash on all platforms.
	// strings.ReplaceAll is used instead of filepath.ToSlash because the latter
	// only replaces the OS separator, leaving backslashes intact on Linux/macOS.
	snipHookEntry := map[string]any{
		"type":    "command",
		"command": strings.ReplaceAll(hookPath, `\`, `/`),
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
	_ = os.WriteFile(path, out, 0644)
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
