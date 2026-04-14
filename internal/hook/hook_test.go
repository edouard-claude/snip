package hook

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func makePayload(toolName, command string) string {
	payload := map[string]any{
		"tool_name":  toolName,
		"tool_input": map[string]any{"command": command},
	}
	data, _ := json.Marshal(payload)
	return string(data)
}

func extractRewrittenCommand(t *testing.T, output string) string {
	t.Helper()
	var result map[string]any
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("output is not valid JSON: %v\noutput: %s", err, output)
	}
	hookOut := result["hookSpecificOutput"].(map[string]any)
	updated := hookOut["updatedInput"].(map[string]any)
	return updated["command"].(string)
}

func TestRunRewriteSupported(t *testing.T) {
	commands := []string{"git", "go"}
	snipBin := "/usr/local/bin/snip"

	input := makePayload("Bash", "git log -10")
	var out bytes.Buffer
	err := Run(strings.NewReader(input), &out, commands, snipBin)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if out.Len() == 0 {
		t.Fatal("expected output for supported command, got empty")
	}

	rewritten := extractRewrittenCommand(t, out.String())
	want := `"/usr/local/bin/snip" -- git log -10`
	if rewritten != want {
		t.Errorf("rewritten = %q, want %q", rewritten, want)
	}
}

func TestRunUnsupportedPassthrough(t *testing.T) {
	commands := []string{"git", "go"}
	snipBin := "/usr/local/bin/snip"

	input := makePayload("Bash", "ls -la")
	var out bytes.Buffer
	err := Run(strings.NewReader(input), &out, commands, snipBin)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if out.Len() != 0 {
		t.Errorf("expected no output for unsupported command, got: %s", out.String())
	}
}

func TestRunAlreadyRewritten(t *testing.T) {
	commands := []string{"git"}
	snipBin := "/usr/local/bin/snip"

	alreadyRewritten := `"/usr/local/bin/snip" -- git status`
	input := makePayload("Bash", alreadyRewritten)
	var out bytes.Buffer
	err := Run(strings.NewReader(input), &out, commands, snipBin)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if out.Len() != 0 {
		t.Errorf("expected no output for already-rewritten command, got: %s", out.String())
	}
}

func TestRunMultiSegment(t *testing.T) {
	commands := []string{"git"}
	snipBin := "/usr/local/bin/snip"

	input := makePayload("Bash", "git add . && git commit -m 'fix'")
	var out bytes.Buffer
	err := Run(strings.NewReader(input), &out, commands, snipBin)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	rewritten := extractRewrittenCommand(t, out.String())
	want := `"/usr/local/bin/snip" -- git add . && git commit -m 'fix'`
	if rewritten != want {
		t.Errorf("rewritten = %q, want %q", rewritten, want)
	}
}

func TestRunEnvVarPrefix(t *testing.T) {
	commands := []string{"go"}
	snipBin := "/usr/local/bin/snip"

	input := makePayload("Bash", "CGO_ENABLED=0 go test ./...")
	var out bytes.Buffer
	err := Run(strings.NewReader(input), &out, commands, snipBin)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	rewritten := extractRewrittenCommand(t, out.String())
	want := `CGO_ENABLED=0 "/usr/local/bin/snip" -- go test ./...`
	if rewritten != want {
		t.Errorf("rewritten = %q, want %q", rewritten, want)
	}
}

func TestRunEmptyCommand(t *testing.T) {
	commands := []string{"git"}
	snipBin := "/usr/local/bin/snip"

	input := makePayload("Bash", "")
	var out bytes.Buffer
	err := Run(strings.NewReader(input), &out, commands, snipBin)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if out.Len() != 0 {
		t.Errorf("expected no output for empty command, got: %s", out.String())
	}
}

func TestRunNonBashTool(t *testing.T) {
	commands := []string{"git"}
	snipBin := "/usr/local/bin/snip"

	payload := map[string]any{
		"tool_name":  "Read",
		"tool_input": map[string]any{"path": "/tmp/foo"},
	}
	data, _ := json.Marshal(payload)

	var out bytes.Buffer
	err := Run(strings.NewReader(string(data)), &out, commands, snipBin)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if out.Len() != 0 {
		t.Errorf("expected no output for non-Bash tool, got: %s", out.String())
	}
}

func TestRunMalformedJSON(t *testing.T) {
	commands := []string{"git"}
	snipBin := "/usr/local/bin/snip"

	var out bytes.Buffer
	err := Run(strings.NewReader("{invalid json"), &out, commands, snipBin)
	if err != nil {
		t.Fatalf("Run should not return error on malformed JSON: %v", err)
	}

	if out.Len() != 0 {
		t.Errorf("expected no output for malformed JSON, got: %s", out.String())
	}
}

func TestRunPermissionDecision(t *testing.T) {
	commands := []string{"git"}
	snipBin := "/usr/local/bin/snip"

	input := makePayload("Bash", "git status")
	var out bytes.Buffer
	_ = Run(strings.NewReader(input), &out, commands, snipBin)

	var result map[string]any
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("parse output: %v", err)
	}

	hookOut := result["hookSpecificOutput"].(map[string]any)
	if hookOut["permissionDecision"] != "allow" {
		t.Errorf("permissionDecision = %v, want allow", hookOut["permissionDecision"])
	}
	if hookOut["hookEventName"] != "PreToolUse" {
		t.Errorf("hookEventName = %v, want PreToolUse", hookOut["hookEventName"])
	}
}

func TestRunMultipleEnvVars(t *testing.T) {
	commands := []string{"make"}
	snipBin := "/usr/local/bin/snip"

	input := makePayload("Bash", "FOO=1 BAR=2 make build")
	var out bytes.Buffer
	err := Run(strings.NewReader(input), &out, commands, snipBin)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	rewritten := extractRewrittenCommand(t, out.String())
	want := `FOO=1 BAR=2 "/usr/local/bin/snip" -- make build`
	if rewritten != want {
		t.Errorf("rewritten = %q, want %q", rewritten, want)
	}
}
