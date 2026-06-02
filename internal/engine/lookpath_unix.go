//go:build !windows

package engine

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

var (
	pathCache   = make(map[string]string)
	pathCacheMu sync.RWMutex
)

// lookPath resolves a command name to an absolute executable path without
// using Go's standard exec.LookPath. The stdlib path uses the faccessat2
// syscall (via internal/syscall/unix.Eaccess since Go 1.25.8), which is
// unimplemented on Android kernels and crashes Termux processes with
// SIGSYS. We walk $PATH with os.Stat instead and check the executable
// bits ourselves.
//
// Results are cached in-process to avoid re-walking $PATH on every call.
func lookPath(name string) (string, error) {
	if strings.Contains(name, "/") {
		return filepath.Abs(name)
	}

	pathCacheMu.RLock()
	cached, ok := pathCache[name]
	pathCacheMu.RUnlock()
	if ok {
		return cached, nil
	}

	pathEnv := os.Getenv("PATH")
	for _, dir := range filepath.SplitList(pathEnv) {
		if dir == "" {
			dir = "."
		}
		fullPath := filepath.Join(dir, name)
		info, err := os.Stat(fullPath)
		if err != nil {
			continue
		}
		if info.IsDir() {
			continue
		}
		if info.Mode()&0o111 != 0 {
			pathCacheMu.Lock()
			pathCache[name] = fullPath
			pathCacheMu.Unlock()
			return fullPath, nil
		}
	}
	return "", fmt.Errorf("executable file not found in $PATH: %s", name)
}
