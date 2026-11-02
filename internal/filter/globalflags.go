package filter

import (
	"path/filepath"
	"strings"
)

// globalFlagSpec lists the options a command accepts before its subcommand.
// value flags consume the next token, or carry it inline as --flag=value;
// bare flags stand alone.
type globalFlagSpec struct {
	value map[string]bool
	bare  map[string]bool
}

// globalFlags is the per-command table of such options. The subcommand match
// keys on the first argument, so without it a leading global option hides the
// subcommand and the filter never applies (issue #201).
//
// Options that make the command exit without running a subcommand (git
// --version, --help, --html-path) are deliberately absent: with no subcommand
// to filter they must keep failing closed.
var globalFlags = map[string]globalFlagSpec{
	"git": {
		value: optionSet(
			"-C", "-c", "--git-dir", "--work-tree", "--namespace", "--super-prefix",
			"--exec-path", "--config-env", "--list-cmds", "--attr-source",
		),
		bare: optionSet(
			"-p", "--paginate", "-P", "--no-pager", "--bare",
			"--no-replace-objects", "--no-lazy-fetch", "--no-optional-locks", "--no-advice",
			"--literal-pathspecs", "--glob-pathspecs", "--noglob-pathspecs", "--icase-pathspecs",
		),
	},
}

// SplitGlobalFlags separates the global options a command accepts before its
// subcommand from the subcommand and its arguments: ("git", ["-C", "../repo",
// "status"]) yields ["-C", "../repo"] and ["status"]. The caller matches on
// rest and replays globals in front of the subcommand at execution time.
//
// It fails closed, returning nil and args unchanged, for a command without an
// option table, an unknown leading flag (whose value could be mistaken for the
// subcommand) and a value flag missing its value: the match then proceeds, and
// fails, exactly as it did before.
func SplitGlobalFlags(command string, args []string) (globals, rest []string) {
	spec, ok := globalFlags[baseName(command)]
	if !ok {
		return nil, args
	}
	n := 0
	for n < len(args) && strings.HasPrefix(args[n], "-") {
		flag := args[n]
		name, _, inline := strings.Cut(flag, "=")
		switch {
		case spec.bare[flag]:
			n++
		case inline && spec.value[name]:
			n++
		case spec.value[flag] && n+1 < len(args):
			n += 2
		default:
			return nil, args
		}
	}
	if n == 0 {
		return nil, args
	}
	return args[:n], args[n:]
}

// baseName normalizes a path-qualified command (/usr/bin/git) to its base
// name, guarding the empty and root paths the way Registry.Match does.
func baseName(command string) string {
	if command == "" || command == "/" || command == "\\" {
		return command
	}
	return filepath.Base(command)
}

// optionSet builds a set from option names.
func optionSet(options ...string) map[string]bool {
	set := make(map[string]bool, len(options))
	for _, o := range options {
		set[o] = true
	}
	return set
}
