package cli

import (
	"testing"
)

func TestParseFlags(t *testing.T) {
	tests := []struct {
		name      string
		args      []string
		wantFlags Flags
		wantArgs  []string
	}{
		{
			name:      "no flags",
			args:      []string{"git", "log"},
			wantFlags: Flags{},
			wantArgs:  []string{"git", "log"},
		},
		{
			name:      "verbose",
			args:      []string{"-v", "git", "log"},
			wantFlags: Flags{Verbose: 1},
			wantArgs:  []string{"git", "log"},
		},
		{
			name:      "double verbose",
			args:      []string{"-vv", "git", "log"},
			wantFlags: Flags{Verbose: 2},
			wantArgs:  []string{"git", "log"},
		},
		{
			name:      "ultra compact",
			args:      []string{"-u", "git", "log"},
			wantFlags: Flags{UltraCompact: true},
			wantArgs:  []string{"git", "log"},
		},
		{
			name:      "version",
			args:      []string{"--version"},
			wantFlags: Flags{Version: true},
			wantArgs:  nil,
		},
		{
			name:      "help",
			args:      []string{"--help"},
			wantFlags: Flags{Help: true},
			wantArgs:  nil,
		},
		{
			name:      "mixed flags and args",
			args:      []string{"-v", "-u", "git", "status"},
			wantFlags: Flags{Verbose: 1, UltraCompact: true},
			wantArgs:  []string{"git", "status"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			flags, args := ParseFlags(tt.args)
			if flags.Verbose != tt.wantFlags.Verbose {
				t.Errorf("verbose = %d, want %d", flags.Verbose, tt.wantFlags.Verbose)
			}
			if flags.UltraCompact != tt.wantFlags.UltraCompact {
				t.Errorf("ultra = %v, want %v", flags.UltraCompact, tt.wantFlags.UltraCompact)
			}
			if flags.Version != tt.wantFlags.Version {
				t.Errorf("version = %v", flags.Version)
			}
			if flags.Help != tt.wantFlags.Help {
				t.Errorf("help = %v", flags.Help)
			}
			if len(args) != len(tt.wantArgs) {
				t.Errorf("args = %v, want %v", args, tt.wantArgs)
			}
		})
	}
}
