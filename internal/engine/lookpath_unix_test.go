//go:build !windows

package engine

import (
	"os"
	"path/filepath"
	"testing"
)

func resetPathCache(t *testing.T) {
	t.Helper()
	pathCacheMu.Lock()
	pathCache = make(map[string]string)
	pathCacheMu.Unlock()
}

func TestLookPathFindsBinaryInPath(t *testing.T) {
	resetPathCache(t)
	got, err := lookPath("sh")
	if err != nil {
		t.Fatalf("lookPath(sh): %v", err)
	}
	if !filepath.IsAbs(got) {
		t.Errorf("expected absolute path, got %q", got)
	}
	info, err := os.Stat(got)
	if err != nil {
		t.Fatalf("stat %q: %v", got, err)
	}
	if info.Mode()&0o111 == 0 {
		t.Errorf("resolved path %q is not executable", got)
	}
}

func TestLookPathAbsoluteInputReturnsAbsolute(t *testing.T) {
	resetPathCache(t)
	got, err := lookPath("/bin/sh")
	if err != nil {
		t.Fatalf("lookPath: %v", err)
	}
	if got != "/bin/sh" {
		t.Errorf("got %q, want /bin/sh", got)
	}
}

func TestLookPathMissingBinaryReturnsError(t *testing.T) {
	resetPathCache(t)
	if _, err := lookPath("snip-does-not-exist-xyz123"); err == nil {
		t.Error("expected error for missing binary")
	}
}

func TestLookPathCachesResolution(t *testing.T) {
	resetPathCache(t)
	first, err := lookPath("sh")
	if err != nil {
		t.Fatal(err)
	}
	// Point PATH at a directory with no sh — cached value should still win.
	tmp := t.TempDir()
	t.Setenv("PATH", tmp)
	second, err := lookPath("sh")
	if err != nil {
		t.Fatalf("cached lookPath: %v", err)
	}
	if first != second {
		t.Errorf("cache miss: first=%q second=%q", first, second)
	}
}

func TestLookPathSkipsDirectories(t *testing.T) {
	resetPathCache(t)
	tmp := t.TempDir()
	// Create a directory named "cmd" in tmp so lookPath finds it but skips it.
	if err := os.Mkdir(filepath.Join(tmp, "cmd"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", tmp)
	if _, err := lookPath("cmd"); err == nil {
		t.Error("expected error when PATH match is a directory, got nil")
	}
}

func TestLookPathSkipsNonExecutable(t *testing.T) {
	resetPathCache(t)
	tmp := t.TempDir()
	// Plain file with no executable bit.
	if err := os.WriteFile(filepath.Join(tmp, "notexec"), []byte("#!/bin/sh\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", tmp)
	if _, err := lookPath("notexec"); err == nil {
		t.Error("expected error when PATH match is not executable, got nil")
	}
}
