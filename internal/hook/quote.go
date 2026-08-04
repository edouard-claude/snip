package hook

import (
	"fmt"
	"runtime"
	"strings"
)

// quoteSnipBin renders the snip binary path for inclusion in a rewritten
// command, so a path containing spaces still executes as one word.
func quoteSnipBin(path string) string {
	return QuoteBinFor(path, runtime.GOOS)
}

// QuoteBinFor is quoteSnipBin with the target OS injected, so both branches are
// testable from any host.
//
// Go's %q is a Go string literal, not a shell word, and on Windows it is wrong
// twice over: it doubles every backslash of a native path
// (`"C:\\Users\\me\\snip.exe"`), and the quotes it adds make PowerShell parse
// the result as a string expression rather than a command, which fails with
// "Unexpected token 'run' in expression or statement" (issue #150).
//
// A Windows path with no space needs no quoting at all, and unquoted is the one
// form that runs in PowerShell, cmd.exe and a POSIX shell alike. A path that
// does contain a space still needs the quotes; PowerShell would additionally
// need its call operator there, which cmd.exe rejects, so the quotes are
// emitted without guessing which shell is on the other side.
func QuoteBinFor(path, goos string) string {
	if goos == "windows" {
		if !strings.ContainsAny(path, " \t\"") {
			return path
		}
		return `"` + path + `"`
	}
	return fmt.Sprintf("%q", path)
}
