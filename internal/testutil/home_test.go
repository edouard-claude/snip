package testutil

import (
	"os"
	"testing"
)

func TestSetHome(t *testing.T) {
	dir := t.TempDir()
	SetHome(t, dir)

	got, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("UserHomeDir: %v", err)
	}
	if got != dir {
		t.Errorf("UserHomeDir() = %q, want %q", got, dir)
	}
}
