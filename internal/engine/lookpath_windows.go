//go:build windows

package engine

import "os/exec"

// lookPath resolves a command name to an absolute executable path. On
// Windows we defer to os/exec.LookPath, which knows about PATHEXT and
// the .exe/.bat/.cmd resolution that the manual !windows implementation
// does not perform. The Android faccessat2 bug that motivates the unix
// variant does not exist on Windows.
func lookPath(name string) (string, error) {
	return exec.LookPath(name)
}
