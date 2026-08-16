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
			// A producer whose output feeds a pipe stage is left raw (#111): its
			// exact count/content must reach the consumer unmodified.
			name:     "pipe tail leaves producer raw",
			cmd:      "git log | grep fix",
			want:     "git log | grep fix",
			changed:  false,
			allKnown: false,
		},
		{
			// A producer redirected to a file is left raw (#111): the file must
			// receive the real output, not snip's compacted view.
			name:     "redirect leaves producer raw",
			cmd:      "git log > out.txt",
			want:     "git log > out.txt",
			changed:  false,
			allKnown: true,
		},
		{
			// A raw redirected producer must not block auto-allow of a wrapped,
			// known sibling group (#111): both git and go are attested.
			name:     "redirect with known sibling still allowed",
			cmd:      "git log > out.txt && go test ./...",
			want:     `git log > out.txt && "/usr/local/bin/snip" run -- go test ./...`,
			changed:  true,
			allKnown: true,
		},
		{
			// A piped producer stays raw while a known non-pipe sibling is still
			// wrapped; the uninspected pipe tail keeps auto-allow off (#88/#111).
			name:     "piped producer raw but known sibling wrapped",
			cmd:      "git log | grep x && go test ./...",
			want:     `git log | grep x && "/usr/local/bin/snip" run -- go test ./...`,
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
			res := RewriteCommand(tc.cmd, cmdSet, nil, bin)
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

// TestRewriteTransparentPrefix covers runner-prefix unwrapping: "uv run pytest"
// is rewritten so the pytest filter applies, while the runner prefix is left in
// place. The inner command must be a known snip command for the rewrite (and any
// auto-allow) to happen; an unknown inner program falls through (#88).
func TestRewriteTransparentPrefix(t *testing.T) {
	const bin = "/usr/local/bin/snip"
	cmdSet := map[string]struct{}{"git": {}, "pytest": {}, "ruff": {}}
	builtin := MergeTransparentPrefixes(nil)
	withDocker := MergeTransparentPrefixes([]string{"docker exec ctr"})

	cases := []struct {
		name     string
		cmd      string
		prefixes []TransparentPrefix
		want     string
		changed  bool
		allKnown bool
	}{
		{
			name:     "uv run pytest",
			cmd:      "uv run pytest tests/",
			prefixes: builtin,
			want:     `uv run "/usr/local/bin/snip" run -- pytest tests/`,
			changed:  true,
			allKnown: true,
		},
		{
			name:     "uv run with value flag",
			cmd:      "uv run --python 3.12 pytest -v",
			prefixes: builtin,
			want:     `uv run --python 3.12 "/usr/local/bin/snip" run -- pytest -v`,
			changed:  true,
			allKnown: true,
		},
		{
			name:     "uv run with boolean flag",
			cmd:      "uv run --frozen pytest",
			prefixes: builtin,
			want:     `uv run --frozen "/usr/local/bin/snip" run -- pytest`,
			changed:  true,
			allKnown: true,
		},
		{
			name:     "poetry run ruff (builtin)",
			cmd:      "poetry run ruff check .",
			prefixes: builtin,
			want:     `poetry run "/usr/local/bin/snip" run -- ruff check .`,
			changed:  true,
			allKnown: true,
		},
		{
			name:     "user-configured docker exec",
			cmd:      "docker exec ctr pytest -q",
			prefixes: withDocker,
			want:     `docker exec ctr "/usr/local/bin/snip" run -- pytest -q`,
			changed:  true,
			allKnown: true,
		},
		{
			name:     "compound runner and direct",
			cmd:      "uv run pytest && ruff check",
			prefixes: builtin,
			want:     `uv run "/usr/local/bin/snip" run -- pytest && "/usr/local/bin/snip" run -- ruff check`,
			changed:  true,
			allKnown: true,
		},
		{
			// Inner command is not a known snip command: the runner (poetry) is
			// also unknown, so nothing is rewritten and nothing is auto-allowed.
			name:     "unknown inner not rewritten",
			cmd:      "poetry run bash -c 'rm -rf /'",
			prefixes: builtin,
			want:     "poetry run bash -c 'rm -rf /'",
			changed:  false,
			allKnown: false,
		},
		{
			// Unrecognized value flag: its value (highest) is treated as the inner
			// command, is unknown, so it fails closed rather than guessing.
			name:     "unknown value flag fails closed",
			cmd:      "poetry run --resolution highest pytest",
			prefixes: builtin,
			want:     "poetry run --resolution highest pytest",
			changed:  false,
			allKnown: false,
		},
		{
			name:     "prefix alone is not rewritten",
			cmd:      "poetry run",
			prefixes: builtin,
			want:     "poetry run",
			changed:  false,
			allKnown: false,
		},
		{
			// "uv run echo pytest": echo is not a known command, so pytest (an
			// argument to echo) must not be treated as the target.
			name:     "argument not mistaken for command",
			cmd:      "poetry run echo pytest",
			prefixes: builtin,
			want:     "poetry run echo pytest",
			changed:  false,
			allKnown: false,
		},
		{
			// Env-var prefix is preserved ahead of the runner.
			name:     "env prefix preserved",
			cmd:      "PYTHONPATH=. uv run pytest",
			prefixes: builtin,
			want:     `PYTHONPATH=. uv run "/usr/local/bin/snip" run -- pytest`,
			changed:  true,
			allKnown: true,
		},
		{
			// "command -v pytest" is a non-executing lookup, not an exec wrapper:
			// the mode flag must block the rewrite (shell wrapper, SkipFlags=false).
			name:     "command -v lookup not rewritten",
			cmd:      "command -v pytest",
			prefixes: builtin,
			want:     "command -v pytest",
			changed:  false,
			allKnown: false,
		},
		{
			name:     "command direct is rewritten",
			cmd:      "command pytest -q",
			prefixes: builtin,
			want:     `command "/usr/local/bin/snip" run -- pytest -q`,
			changed:  true,
			allKnown: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res := RewriteCommand(tc.cmd, cmdSet, tc.prefixes, bin)
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

func TestRewriteEmbedsPluginConfigFlag(t *testing.T) {
	// Issue #169: the rewritten command runs in the agent's shell, which does
	// not inherit the hook's environment. When the hook itself was started
	// with SNIP_PLUGIN_CONFIG, the path must travel inside the rewritten
	// command as a portable flag, or the plugin layer silently vanishes at
	// run time.
	t.Setenv("SNIP_PLUGIN_CONFIG", "/plug/snip/config.toml")

	cmdSet := map[string]struct{}{"git": {}}
	res := RewriteCommand("git status", cmdSet, nil, "/usr/local/bin/snip")
	if !res.Changed {
		t.Fatal("expected rewrite")
	}
	want := `"/usr/local/bin/snip" --plugin-config "/plug/snip/config.toml" run -- git status`
	if res.Command != want {
		t.Errorf("got  %q\nwant %q", res.Command, want)
	}
}

func TestRewriteNoPluginConfigFlagWhenUnset(t *testing.T) {
	t.Setenv("SNIP_PLUGIN_CONFIG", "")

	cmdSet := map[string]struct{}{"git": {}}
	res := RewriteCommand("git status", cmdSet, nil, "/usr/local/bin/snip")
	if !res.Changed {
		t.Fatal("expected rewrite")
	}
	want := `"/usr/local/bin/snip" run -- git status`
	if res.Command != want {
		t.Errorf("got  %q\nwant %q", res.Command, want)
	}
}
