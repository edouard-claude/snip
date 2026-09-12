package hook

import (
	"bytes"
	"strings"
	"testing"
)

// TestWindowsBinQuotingPerAgent pins issue #187: the shell a rewritten command
// runs in is the agent's, not the host's. On Windows, Claude Code's Bash tool
// is Git Bash, where a bare `C:\Users\...` path loses its backslashes and
// fails with exit 127, while Codex hands the command to PowerShell or cmd.exe,
// where the quoted literal is the form that fails (#150, #174).
func TestWindowsBinQuotingPerAgent(t *testing.T) {
	const bin = `C:\Users\me\.local\bin\snip.exe`
	commands := []string{"git"}

	old := hostGOOS
	hostGOOS = "windows"
	t.Cleanup(func() { hostGOOS = old })

	t.Run("claude code gets a bash-safe literal", func(t *testing.T) {
		var out bytes.Buffer
		if err := Run(strings.NewReader(makePayload("Bash", "git status")), &out, commands, nil, bin); err != nil {
			t.Fatalf("Run: %v", err)
		}
		got := extractRewrittenCommand(t, out.String())
		want := `"C:\\Users\\me\\.local\\bin\\snip.exe" run -- git status`
		if got != want {
			t.Errorf("command = %q, want %q", got, want)
		}

		// Re-running the hook on its own output must not wrap it twice.
		out.Reset()
		if err := Run(strings.NewReader(makePayload("Bash", got)), &out, commands, nil, bin); err != nil {
			t.Fatalf("Run (second pass): %v", err)
		}
		if out.Len() != 0 {
			t.Errorf("second pass rewrote again: %s", out.String())
		}
	})

	t.Run("codex keeps the bare windows path", func(t *testing.T) {
		var out bytes.Buffer
		if err := RunCodex(strings.NewReader(makePayload("Bash", "git status")), &out, commands, nil, bin); err != nil {
			t.Fatalf("RunCodex: %v", err)
		}
		_, updated := extractCodexRewrite(t, out.String())
		want := `C:\Users\me\.local\bin\snip.exe run -- git status`
		if updated["command"] != want {
			t.Errorf("command = %q, want %q", updated["command"], want)
		}
	})
}
