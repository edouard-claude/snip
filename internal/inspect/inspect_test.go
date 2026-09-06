package inspect

import (
	"bytes"
	"io"
	"os"
	"testing"
)

func TestRunHelp(t *testing.T) {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	code := Run([]string{"--help"})

	w.Close()
	var buf bytes.Buffer
	io.Copy(&buf, r)
	os.Stdout = old

	if code != 0 {
		t.Errorf("expected exit code 0, got %d", code)
	}
}

func TestRunNoArgs(t *testing.T) {
	old := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	code := Run(nil)

	w.Close()
	var buf bytes.Buffer
	io.Copy(&buf, r)
	os.Stderr = old

	// Should find the Go module root (this project) and run fine.
	if code != 0 {
		t.Errorf("expected exit code 0, got %d", code)
	}
}

func TestAppendSafetyName(t *testing.T) {
	var a AppendSafetyChecker
	if a.Name() != "append-safety" {
		t.Errorf("expected 'append-safety', got %q", a.Name())
	}
}

func TestDeadFieldsName(t *testing.T) {
	var d DeadFieldChecker
	if d.Name() != "dead-fields" {
		t.Errorf("expected 'dead-fields', got %q", d.Name())
	}
}