package cli

import (
	"io"
	"testing"
)

func executeRootForTest(t *testing.T, args []string) (int, error, Flags) {
	t.Helper()

	flags := Flags{}
	code := 0
	cmd := newRootCommand(&flags, &code)
	cmd.SetArgs(args)
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)

	err := cmd.Execute()
	return code, err, flags
}

func TestRootCommand_BasicPaths(t *testing.T) {
	tests := []struct {
		name      string
		args      []string
		wantCode  int
		wantError bool
	}{
		{
			name:      "no args shows help",
			args:      nil,
			wantCode:  0,
			wantError: false,
		},
		{
			name:      "version short-circuits",
			args:      []string{"--version"},
			wantCode:  0,
			wantError: false,
		},
		{
			name:      "unproxyable command is rejected",
			args:      []string{"cd", "/tmp"},
			wantCode:  1,
			wantError: false,
		},
		{
			name:      "proxy help uses cobra help",
			args:      []string{"proxy", "--help"},
			wantCode:  0,
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotCode, err, _ := executeRootForTest(t, tt.args)
			if (err != nil) != tt.wantError {
				t.Fatalf("Execute error = %v, wantError=%v", err, tt.wantError)
			}
			if gotCode != tt.wantCode {
				t.Fatalf("exit code = %d, want %d", gotCode, tt.wantCode)
			}
		})
	}
}

func TestRootCommand_GlobalFlagParsingStopsAtCommand(t *testing.T) {
	gotCode, err, flags := executeRootForTest(t, []string{"cd", "--version"})
	if err != nil {
		t.Fatalf("Execute error = %v", err)
	}
	if gotCode != 1 {
		t.Fatalf("exit code = %d, want 1", gotCode)
	}
	if flags.Verbose != 0 {
		t.Fatalf("verbose = %d, want 0", flags.Verbose)
	}
}

func TestRootCommand_DoubleDashPassesThrough(t *testing.T) {
	gotCode, err, _ := executeRootForTest(t, []string{"--", "cd", "--version"})
	if err != nil {
		t.Fatalf("Execute error = %v", err)
	}
	if gotCode != 1 {
		t.Fatalf("exit code = %d, want 1", gotCode)
	}
}

func TestRootCommand_VerboseStackedFlag(t *testing.T) {
	gotCode, err, flags := executeRootForTest(t, []string{"-vv", "cd", "/tmp"})
	if err != nil {
		t.Fatalf("Execute error = %v", err)
	}
	if gotCode != 1 {
		t.Fatalf("exit code = %d, want 1", gotCode)
	}
	if flags.Verbose != 2 {
		t.Fatalf("verbose = %d, want 2", flags.Verbose)
	}
}
