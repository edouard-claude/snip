package cli

import (
	"reflect"
	"testing"
)

func TestUnproxyableCommands(t *testing.T) {
	tests := []struct {
		command string
		want    bool
	}{
		{"cd", true},
		{"chdir", true},
		{"pushd", true},
		{"popd", true},
		{"source", true},
		{".", true},
		{"export", true},
		{"unset", true},
		{"alias", true},
		{"unalias", true},
		{"readonly", true},
		{"declare", true},
		{"typeset", true},
		{"local", true},
		{"shift", true},
		{"read", true},
		{"mapfile", true},
		{"readarray", true},
		{"let", true},
		{"getopts", true},
		{"set", true},
		{"shopt", true},
		{"setopt", true},
		{"unsetopt", true},
		{"emulate", true},
		{"eval", true},
		{"exec", true},
		{"exit", true},
		{"logout", true},
		{"return", true},
		{"break", true},
		{"continue", true},
		{"wait", true},
		{"bg", true},
		{"fg", true},
		{"disown", true},
		{"jobs", true},
		{"suspend", true},
		{"bindkey", true},
		{"bind", true},
		{"complete", true},
		{"compopt", true},
		{"compinit", true},
		{"zstyle", true},
		{"autoload", true},
		{"zmodload", true},
		{"enable", true},
		{"disable", true},
		{"abbr", true},
		{"functions", true},
		{"hash", true},
		{"trap", true},
		{"umask", true},
		{"ulimit", true},
		{"git", false},
		{"go", false},
		{"docker", false},
		{"echo", false},
		{"printf", false},
		{"pwd", false},
		{"test", false},
	}

	for _, tt := range tests {
		t.Run(tt.command, func(t *testing.T) {
			got := unproxyableReason(tt.command) != ""
			if got != tt.want {
				t.Errorf("unproxyableReason(%q) returned %q, wantBlocked=%v", tt.command, unproxyableReason(tt.command), tt.want)
			}
		})
	}
}

func TestRunRejectsCd(t *testing.T) {
	code := Run([]string{"snip", "cd", "/tmp"})
	if code != 1 {
		t.Errorf("Run(cd) = %d, want 1", code)
	}
}

func TestRunSubcommandMissingSeparator(t *testing.T) {
	code := Run([]string{"snip", "run", "git", "log"})
	if code != 1 {
		t.Errorf("Run(run without --) = %d, want 1", code)
	}
}

func TestRunSubcommandEmptyAfterSeparator(t *testing.T) {
	code := Run([]string{"snip", "run", "--"})
	if code != 1 {
		t.Errorf("Run(run --) = %d, want 1", code)
	}
}

func TestRunSubcommandRejectsUnproxyable(t *testing.T) {
	code := Run([]string{"snip", "run", "--", "cd", "/tmp"})
	if code != 1 {
		t.Errorf("Run(run -- cd) = %d, want 1", code)
	}
}

func TestRunSubcommandRejectsArgsBeforeSeparator(t *testing.T) {
	code := Run([]string{"snip", "run", "foo", "--", "bar"})
	if code != 1 {
		t.Errorf("Run(run foo -- bar) = %d, want 1", code)
	}
}

func TestRunGlobalHelpBeforeSeparator(t *testing.T) {
	code := Run([]string{"snip", "run", "--help", "--", "foo", "bar"})
	if code != 0 {
		t.Errorf("Run(run --help -- foo bar) = %d, want 0", code)
	}
}

func TestRunCommandHelpAfterSeparator(t *testing.T) {
	code := Run([]string{"snip", "run", "--", "git", "--help"})
	if code != 0 {
		t.Errorf("Run(run -- git --help) = %d, want 0", code)
	}
}

func TestRunSubcommandWithFlags(t *testing.T) {
	flags, remaining := ParseFlags([]string{"-v", "run", "--", "git", "log", "-10"})
	if flags.Verbose != 1 {
		t.Errorf("flags.Verbose = %d, want 1", flags.Verbose)
	}
	wantRemaining := []string{"run", "--", "git", "log", "-10"}
	if !reflect.DeepEqual(remaining, wantRemaining) {
		t.Errorf("remaining = %v, want %v", remaining, wantRemaining)
	}
}

func TestCheckMissingSeparator(t *testing.T) {
	code := Run([]string{"snip", "check", "git", "log"})
	if code != 1 {
		t.Errorf("Run(check without --) = %d, want 1", code)
	}
}

func TestCheckEmptyAfterSeparator(t *testing.T) {
	code := Run([]string{"snip", "check", "--"})
	if code != 1 {
		t.Errorf("Run(check --) = %d, want 1", code)
	}
}

func TestCheckShellBuiltin(t *testing.T) {
	code := Run([]string{"snip", "check", "--", "cd", "/tmp"})
	if code != 1 {
		t.Errorf("Run(check -- cd) = %d, want 1", code)
	}
}

func TestCheckShellBuiltinExport(t *testing.T) {
	code := Run([]string{"snip", "check", "--", "export", "FOO=bar"})
	if code != 1 {
		t.Errorf("Run(check -- export) = %d, want 1", code)
	}
}

func TestCheckShellBuiltinSet(t *testing.T) {
	code := Run([]string{"snip", "check", "--", "set", "-e"})
	if code != 1 {
		t.Errorf("Run(check -- set) = %d, want 1", code)
	}
}

func TestCheckShellBuiltinExit(t *testing.T) {
	code := Run([]string{"snip", "check", "--", "exit"})
	if code != 1 {
		t.Errorf("Run(check -- exit) = %d, want 1", code)
	}
}
