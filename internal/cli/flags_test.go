package cli

import (
	"reflect"
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
			name:      "built-in command help is preserved",
			args:      []string{"proxy", "--help"},
			wantFlags: Flags{Help: true},
			wantArgs:  []string{"proxy"},
		},
		{
			name:      "built-in command keeps its own flags",
			args:      []string{"gain", "--daily"},
			wantFlags: Flags{},
			wantArgs:  []string{"gain", "--daily"},
		},
		{
			name:      "mixed flags and args",
			args:      []string{"-v", "-u", "git", "status"},
			wantFlags: Flags{Verbose: 1, UltraCompact: true},
			wantArgs:  []string{"git", "status"},
		},
		{
			name:      "command help flag is passed through",
			args:      []string{"npx", "-y", "chrome-devtools-mcp@latest", "--help"},
			wantFlags: Flags{},
			wantArgs:  []string{"npx", "-y", "chrome-devtools-mcp@latest", "--help"},
		},
		{
			name:      "global flags stop parsing at command",
			args:      []string{"-v", "npx", "-y", "chrome-devtools-mcp@latest", "--help"},
			wantFlags: Flags{Verbose: 1},
			wantArgs:  []string{"npx", "-y", "chrome-devtools-mcp@latest", "--help"},
		},
		{
			name:      "global flags still allow built-in command help",
			args:      []string{"-v", "proxy", "--help"},
			wantFlags: Flags{Verbose: 1, Help: true},
			wantArgs:  []string{"proxy"},
		},
		// "--" separator: everything after it is passed verbatim to the command.
		{
			name:      "double dash passes remaining verbatim",
			args:      []string{"--", "opencode", "--help"},
			wantFlags: Flags{},
			wantArgs:  []string{"opencode", "--help"},
		},
		{
			name:      "snip flags before double dash, command flags after",
			args:      []string{"-v", "--", "go", "test", "--version"},
			wantFlags: Flags{Verbose: 1},
			wantArgs:  []string{"go", "test", "--version"},
		},
		{
			name:      "double dash alone produces empty remaining",
			args:      []string{"--"},
			wantFlags: Flags{},
			wantArgs:  nil,
		},
		{
			name:      "double dash before --help prevents snip help",
			args:      []string{"--", "--help"},
			wantFlags: Flags{},
			wantArgs:  []string{"--help"},
		},
		{
			name:      "double dash before -v prevents snip verbose",
			args:      []string{"--", "-v", "git", "log"},
			wantFlags: Flags{},
			wantArgs:  []string{"-v", "git", "log"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			flags, args := ParseFlags(tt.args)
			if !reflect.DeepEqual(flags, tt.wantFlags) {
				t.Errorf("flags = %+v, want %+v", flags, tt.wantFlags)
			}
			if !reflect.DeepEqual(args, tt.wantArgs) {
				t.Errorf("args = %v, want %v", args, tt.wantArgs)
			}
		})
	}
}
