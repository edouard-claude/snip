package cli

import "strings"

// Flags holds parsed global flags.
type Flags struct {
	Verbose      int
	UltraCompact bool
	SkipEnv      bool
	Version      bool
	Help         bool
}

// ParseFlags extracts global flags from args and returns remaining args.
func ParseFlags(args []string) (Flags, []string) {
	var flags Flags
	var remaining []string

	for _, arg := range args {
		switch {
		case arg == "-vv":
			flags.Verbose = 2
		case arg == "-v":
			if flags.Verbose < 1 {
				flags.Verbose = 1
			}
		case arg == "-u":
			flags.UltraCompact = true
		case arg == "--skip-env":
			flags.SkipEnv = true
		case arg == "--version":
			flags.Version = true
		case arg == "--help" || arg == "-h":
			flags.Help = true
		case isStackedVerboseFlag(arg):
			flags.Verbose = strings.Count(arg, "v")
		default:
			remaining = append(remaining, arg)
		}
	}

	return flags, remaining
}

// isStackedVerboseFlag detects flags like -vvv, -vvvv (only 'v' chars after dash).
func isStackedVerboseFlag(arg string) bool {
	if !strings.HasPrefix(arg, "-") || strings.HasPrefix(arg, "--") {
		return false
	}
	trimmed := strings.TrimLeft(arg, "-")
	return len(trimmed) > 0 && strings.Trim(trimmed, "v") == ""
}
