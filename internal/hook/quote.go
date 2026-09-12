package hook

import (
	"fmt"
	"os"
	"runtime"
	"strings"
)

// runInvocation renders `<quotedBin> [--plugin-config <path>] run -- ` for a
// rewritten or suggested command. The hook process may carry a plugin
// configuration through SNIP_PLUGIN_CONFIG (set by an agent plugin's hook
// entry), but the rewritten command executes later in the agent's shell,
// which does not inherit the hook's environment. Embedding the path as a
// flag keeps the plugin layer alive at run time (issue #169). A flag is
// used instead of a `VAR=x` env prefix so PowerShell keeps working (#150).
func runInvocation(quotedBin string, shell Shell) string {
	inv := quotedBin
	if p := os.Getenv("SNIP_PLUGIN_CONFIG"); p != "" {
		inv += " --plugin-config " + quoteBin(p, shell)
	}
	return inv + " run -- "
}

// Shell is the shell a rewritten command will execute in. The agent decides
// it, not the host OS: Claude Code's Bash tool runs Git Bash on Windows, while
// Codex hands the command to PowerShell or cmd.exe there.
type Shell int

const (
	// ShellHost is the shell agents use on the host OS: PowerShell or cmd.exe
	// on Windows, a POSIX shell elsewhere.
	ShellHost Shell = iota
	// ShellPOSIX is a POSIX shell whatever the host OS (issue #187).
	ShellPOSIX
)

// hostGOOS is runtime.GOOS, overridable in tests to exercise the Windows
// branches from any host.
var hostGOOS = runtime.GOOS

// quoteSnipBin renders the snip binary path for the host shell.
func quoteSnipBin(path string) string {
	return quoteBin(path, ShellHost)
}

// quoteBin renders a path for inclusion in a rewritten command targeting
// shell, so a path containing spaces still executes as one word.
func quoteBin(path string, shell Shell) string {
	return QuoteBinFor(path, hostGOOS, shell)
}

// QuoteBinFor is quoteBin with the host OS injected, so every branch is
// testable from any host.
//
// Go's %q is a Go string literal, not a shell word. For a POSIX shell that is
// harmless: double quotes turn the doubled backslashes of a native Windows
// path back into single ones, so `"C:\\Users\\me\\snip.exe"` executes
// `C:\Users\me\snip.exe` in Git Bash. The bare path would not: bash eats an
// unquoted backslash (`\U` -> `U`), which is how issue #187 produced exit 127.
//
// For a Windows shell the same literal is wrong twice over: it names no
// existing file (the backslashes stay doubled), and the quotes make
// PowerShell parse the result as a string expression rather than a command,
// which fails with "Unexpected token 'run' in expression or statement"
// (issue #150). A Windows path with no space needs no quoting at all, and
// unquoted is the one form PowerShell and cmd.exe both run. A path that does
// contain a space still needs the quotes; PowerShell would additionally need
// its call operator there, which cmd.exe rejects, so the quotes are emitted
// without guessing which of the two is on the other side.
func QuoteBinFor(path, goos string, shell Shell) string {
	if goos == "windows" && shell == ShellHost {
		if !strings.ContainsAny(path, " \t\"") {
			return path
		}
		return `"` + path + `"`
	}
	return fmt.Sprintf("%q", path)
}
