package filter

import (
	"slices"
	"testing"
)

// Global options sit between the command and its subcommand (git -C <dir>
// status). The subcommand match keys on the first argument, so they must be
// split off before matching and replayed at execution time (issue #201).
func TestSplitGlobalFlags(t *testing.T) {
	cases := []struct {
		name        string
		command     string
		args        []string
		wantGlobals []string
		wantRest    []string
	}{
		{
			name: "no global option", command: "git",
			args: []string{"status"}, wantGlobals: nil, wantRest: []string{"status"},
		},
		{
			name: "-C consumes its directory", command: "git",
			args:        []string{"-C", "../repo", "status"},
			wantGlobals: []string{"-C", "../repo"}, wantRest: []string{"status"},
		},
		{
			name: "-c consumes its key=value", command: "git",
			args:        []string{"-c", "core.pager=cat", "log", "-3"},
			wantGlobals: []string{"-c", "core.pager=cat"}, wantRest: []string{"log", "-3"},
		},
		{
			name: "--no-pager stands alone", command: "git",
			args:        []string{"--no-pager", "diff"},
			wantGlobals: []string{"--no-pager"}, wantRest: []string{"diff"},
		},
		{
			name: "--git-dir=<path> inline value", command: "git",
			args:        []string{"--git-dir=/x/.git", "status"},
			wantGlobals: []string{"--git-dir=/x/.git"}, wantRest: []string{"status"},
		},
		{
			name: "--git-dir <path> separate value", command: "git",
			args:        []string{"--git-dir", "/x/.git", "status"},
			wantGlobals: []string{"--git-dir", "/x/.git"}, wantRest: []string{"status"},
		},
		{
			name: "several global options in a row", command: "git",
			args:        []string{"-C", "r", "--no-pager", "-c", "a=b", "log"},
			wantGlobals: []string{"-C", "r", "--no-pager", "-c", "a=b"}, wantRest: []string{"log"},
		},
		{
			name: "path-qualified command uses the base name", command: "/usr/bin/git",
			args:        []string{"-C", "r", "status"},
			wantGlobals: []string{"-C", "r"}, wantRest: []string{"status"},
		},
		{
			name: "global options with no subcommand", command: "git",
			args:        []string{"-C", "r"},
			wantGlobals: []string{"-C", "r"}, wantRest: []string{},
		},
		{
			// An option snip does not know may take a value or not: guessing
			// could mistake its value for the subcommand, so the split is
			// abandoned and the match proceeds (and fails) as before.
			name: "unknown leading flag fails closed", command: "git",
			args: []string{"--frobnicate", "status"}, wantGlobals: nil, wantRest: []string{"--frobnicate", "status"},
		},
		{
			name: "value flag without its value fails closed", command: "git",
			args: []string{"-C"}, wantGlobals: nil, wantRest: []string{"-C"},
		},
		{
			// --version, --help and --html-path exit without running a
			// subcommand, so they are not global options to skip.
			name: "--version is not a global option", command: "git",
			args: []string{"--version"}, wantGlobals: nil, wantRest: []string{"--version"},
		},
		{
			name: "command without a global option table", command: "npm",
			args: []string{"-C", "x", "install"}, wantGlobals: nil, wantRest: []string{"-C", "x", "install"},
		},
		{
			name: "empty args", command: "git",
			args: nil, wantGlobals: nil, wantRest: nil,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			globals, rest := SplitGlobalFlags(tc.command, tc.args)
			if !slices.Equal(globals, tc.wantGlobals) {
				t.Errorf("globals = %q, want %q", globals, tc.wantGlobals)
			}
			if !slices.Equal(rest, tc.wantRest) {
				t.Errorf("rest = %q, want %q", rest, tc.wantRest)
			}
		})
	}
}
