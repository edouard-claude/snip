package engine

import (
	"runtime"
	"strings"
	"testing"
)

func TestExecuteEcho(t *testing.T) {
	result, err := Execute("echo", []string{"hello", "world"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ExitCode != 0 {
		t.Errorf("exit code = %d", result.ExitCode)
	}
	if got := strings.TrimSpace(result.Stdout); got != "hello world" {
		t.Errorf("stdout = %q", got)
	}
	if result.Duration <= 0 {
		t.Error("duration should be positive")
	}
}

func TestExecuteStderr(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skip on windows")
	}
	result, err := Execute("sh", []string{"-c", "echo error >&2"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := strings.TrimSpace(result.Stderr); got != "error" {
		t.Errorf("stderr = %q", got)
	}
}

func TestExecuteExitCode(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skip on windows")
	}
	result, err := Execute("sh", []string{"-c", "exit 42"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ExitCode != 42 {
		t.Errorf("exit code = %d, want 42", result.ExitCode)
	}
}

func TestExecuteNotFound(t *testing.T) {
	_, err := Execute("nonexistent-command-xyz", nil)
	if err == nil {
		t.Fatal("expected error for nonexistent command")
	}
}

func TestPassthrough(t *testing.T) {
	code, err := Passthrough("echo", []string{"test"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if code != 0 {
		t.Errorf("exit code = %d", code)
	}
}

func TestPassthroughExitCode(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skip on windows")
	}
	code, err := Passthrough("sh", []string{"-c", "exit 7"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if code != 7 {
		t.Errorf("exit code = %d, want 7", code)
	}
}
