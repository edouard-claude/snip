package hook

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func extractDenyReason(t *testing.T, output string) string {
	t.Helper()
	var result map[string]any
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("output is not valid JSON: %v\noutput: %s", err, output)
	}
	hookOut, ok := result["hookSpecificOutput"].(map[string]any)
	if !ok {
		t.Fatalf("missing hookSpecificOutput: %s", output)
	}
	if hookOut["hookEventName"] != "PreToolUse" {
		t.Errorf("hookEventName = %v, want PreToolUse", hookOut["hookEventName"])
	}
	if hookOut["permissionDecision"] != "deny" {
		t.Errorf("permissionDecision = %v, want deny", hookOut["permissionDecision"])
	}
	if _, ok := hookOut["updatedInput"]; ok {
		t.Errorf("updatedInput must not be set in Codex response (Codex ignores it): %s", output)
	}
	reason, _ := hookOut["permissionDecisionReason"].(string)
	if reason == "" {
		t.Fatalf("permissionDecisionReason is empty")
	}
	return reason
}

func TestRunCodexDeniesSupportedCommand(t *testing.T) {
	commands := []string{"git", "go"}
	snipBin := "/usr/local/bin/snip"

	input := makePayload("Bash", "git log -10")
	var out bytes.Buffer
	if err := RunCodex(strings.NewReader(input), &out, commands, nil, snipBin); err != nil {
		t.Fatalf("RunCodex: %v", err)
	}
	if out.Len() == 0 {
		t.Fatal("expected deny output for supported command, got empty")
	}

	reason := extractDenyReason(t, out.String())
	wantSuggestion := `"/usr/local/bin/snip" run -- git log -10`
	if !strings.Contains(reason, wantSuggestion) {
		t.Errorf("reason = %q, want it to contain %q", reason, wantSuggestion)
	}
}

func TestRunCodexUnsupportedPassthrough(t *testing.T) {
	commands := []string{"git", "go"}
	snipBin := "/usr/local/bin/snip"

	input := makePayload("Bash", "ls -la")
	var out bytes.Buffer
	if err := RunCodex(strings.NewReader(input), &out, commands, nil, snipBin); err != nil {
		t.Fatalf("RunCodex: %v", err)
	}
	if out.Len() != 0 {
		t.Errorf("expected no output for unsupported command, got: %s", out.String())
	}
}

func TestRunCodexAlreadyRewritten(t *testing.T) {
	commands := []string{"git"}
	snipBin := "/usr/local/bin/snip"

	already := `"/usr/local/bin/snip" run -- git status`
	input := makePayload("Bash", already)
	var out bytes.Buffer
	if err := RunCodex(strings.NewReader(input), &out, commands, nil, snipBin); err != nil {
		t.Fatalf("RunCodex: %v", err)
	}
	if out.Len() != 0 {
		t.Errorf("expected no output for already-rewritten command, got: %s", out.String())
	}
}

func TestRunCodexMultiSegmentSuggestionIncludesTail(t *testing.T) {
	commands := []string{"git"}
	snipBin := "/usr/local/bin/snip"

	input := makePayload("Bash", "git add . && git commit -m 'fix'")
	var out bytes.Buffer
	if err := RunCodex(strings.NewReader(input), &out, commands, nil, snipBin); err != nil {
		t.Fatalf("RunCodex: %v", err)
	}

	reason := extractDenyReason(t, out.String())
	wantSuggestion := `"/usr/local/bin/snip" run -- git add . && git commit -m 'fix'`
	if !strings.Contains(reason, wantSuggestion) {
		t.Errorf("reason = %q, want it to contain %q", reason, wantSuggestion)
	}
}

func TestRunCodexEnvVarPrefix(t *testing.T) {
	commands := []string{"go"}
	snipBin := "/usr/local/bin/snip"

	input := makePayload("Bash", "CGO_ENABLED=0 go test ./...")
	var out bytes.Buffer
	if err := RunCodex(strings.NewReader(input), &out, commands, nil, snipBin); err != nil {
		t.Fatalf("RunCodex: %v", err)
	}

	reason := extractDenyReason(t, out.String())
	wantSuggestion := `CGO_ENABLED=0 "/usr/local/bin/snip" run -- go test ./...`
	if !strings.Contains(reason, wantSuggestion) {
		t.Errorf("reason = %q, want it to contain %q", reason, wantSuggestion)
	}
}

func TestRunCodexNonBashTool(t *testing.T) {
	commands := []string{"git"}
	snipBin := "/usr/local/bin/snip"

	payload := map[string]any{
		"tool_name":  "Read",
		"tool_input": map[string]any{"path": "/tmp/foo"},
	}
	data, _ := json.Marshal(payload)

	var out bytes.Buffer
	if err := RunCodex(strings.NewReader(string(data)), &out, commands, nil, snipBin); err != nil {
		t.Fatalf("RunCodex: %v", err)
	}
	if out.Len() != 0 {
		t.Errorf("expected no output for non-Bash tool, got: %s", out.String())
	}
}

func TestRunCodexEmptyCommand(t *testing.T) {
	commands := []string{"git"}
	snipBin := "/usr/local/bin/snip"

	input := makePayload("Bash", "")
	var out bytes.Buffer
	if err := RunCodex(strings.NewReader(input), &out, commands, nil, snipBin); err != nil {
		t.Fatalf("RunCodex: %v", err)
	}
	if out.Len() != 0 {
		t.Errorf("expected no output for empty command, got: %s", out.String())
	}
}

func TestRunCodexMalformedJSON(t *testing.T) {
	commands := []string{"git"}
	snipBin := "/usr/local/bin/snip"

	var out bytes.Buffer
	if err := RunCodex(strings.NewReader("{invalid json"), &out, commands, nil, snipBin); err != nil {
		t.Fatalf("RunCodex must not error on malformed JSON: %v", err)
	}
	if out.Len() != 0 {
		t.Errorf("expected no output for malformed JSON, got: %s", out.String())
	}
}
