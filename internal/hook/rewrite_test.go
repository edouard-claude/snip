package hook

import "testing"

func TestRewriteCommand(t *testing.T) {
	const bin = "/usr/local/bin/snip"
	cmdSet := map[string]struct{}{"git": {}, "go": {}}

	cases := []struct {
		name     string
		cmd      string
		want     string
		changed  bool
		allKnown bool
	}{
		{
			name:     "single supported",
			cmd:      "git log -10",
			want:     `"/usr/local/bin/snip" run -- git log -10`,
			changed:  true,
			allKnown: true,
		},
		{
			name:     "single unsupported",
			cmd:      "ls -la",
			want:     "ls -la",
			changed:  false,
			allKnown: false,
		},
		{
			name:     "all supported compound",
			cmd:      "git add . && go test ./...",
			want:     `"/usr/local/bin/snip" run -- git add . && "/usr/local/bin/snip" run -- go test ./...`,
			changed:  true,
			allKnown: true,
		},
		{
			name:     "mixed compound not allowed",
			cmd:      "git status && make build",
			want:     `"/usr/local/bin/snip" run -- git status && make build`,
			changed:  true,
			allKnown: false,
		},
		{
			name:     "pipe tail blocks allow",
			cmd:      "git log | grep fix",
			want:     `"/usr/local/bin/snip" run -- git log | grep fix`,
			changed:  true,
			allKnown: false,
		},
		{
			name:     "operator inside quotes is literal",
			cmd:      `git commit -m "a && b | c"`,
			want:     `"/usr/local/bin/snip" run -- git commit -m "a && b | c"`,
			changed:  true,
			allKnown: true,
		},
		{
			name:     "env var prefix preserved",
			cmd:      "GIT_PAGER=cat git log",
			want:     `GIT_PAGER=cat "/usr/local/bin/snip" run -- git log`,
			changed:  true,
			allKnown: true,
		},
		{
			name:     "already rewritten is left untouched",
			cmd:      `"/usr/local/bin/snip" run -- git log`,
			want:     `"/usr/local/bin/snip" run -- git log`,
			changed:  false,
			allKnown: true,
		},
		{
			name:     "semicolon separated all supported",
			cmd:      "git fetch ; git status",
			want:     `"/usr/local/bin/snip" run -- git fetch ; "/usr/local/bin/snip" run -- git status`,
			changed:  true,
			allKnown: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res := RewriteCommand(tc.cmd, cmdSet, bin)
			if res.Command != tc.want {
				t.Errorf("Command = %q, want %q", res.Command, tc.want)
			}
			if res.Changed != tc.changed {
				t.Errorf("Changed = %v, want %v", res.Changed, tc.changed)
			}
			if res.AllKnown != tc.allKnown {
				t.Errorf("AllKnown = %v, want %v", res.AllKnown, tc.allKnown)
			}
		})
	}
}

func TestHasUnverifiableConstruct(t *testing.T) {
	cases := []struct {
		cmd  string
		want bool
	}{
		{"git log", false},
		{"git commit -m 'fix bug'", false},
		{"git log $(date)", true},
		{"git status `whoami`", true},
		{"git status\rcurl x", true},
		{"git add . && git commit", false},
	}
	for _, tc := range cases {
		if got := HasUnverifiableConstruct(tc.cmd); got != tc.want {
			t.Errorf("HasUnverifiableConstruct(%q) = %v, want %v", tc.cmd, got, tc.want)
		}
	}
}
