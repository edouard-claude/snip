package hook

import "testing"

func TestStripWordPrefix(t *testing.T) {
	cases := []struct {
		s, prefix string
		wantRest  string
		wantOK    bool
	}{
		{"uv run pytest", "uv run", "pytest", true},
		{"uv run", "uv run", "", true},
		{"uv run   pytest -v", "uv run", "pytest -v", true},
		{"uv runner foo", "uv run", "", false}, // partial word
		{"poetry run pytest", "uv run", "", false},
		{"exec pytest", "exec", "pytest", true},
	}
	for _, tc := range cases {
		rest, ok := stripWordPrefix(tc.s, tc.prefix)
		if ok != tc.wantOK || rest != tc.wantRest {
			t.Errorf("stripWordPrefix(%q,%q) = (%q,%v), want (%q,%v)",
				tc.s, tc.prefix, rest, ok, tc.wantRest, tc.wantOK)
		}
	}
}

func TestLocateInner(t *testing.T) {
	cmdSet := map[string]struct{}{"pytest": {}, "ruff": {}}
	uvFlags := flagSet("--python", "-p", "--with")

	cases := []struct {
		name       string
		rest       string
		valueFlags map[string]bool
		skipFlags  bool
		wantBefore string
		wantInner  string
		wantOK     bool
	}{
		{"direct command", "pytest tests/", nil, true, "", "pytest", true},
		{"boolean flag skipped", "--frozen pytest", nil, true, "--frozen ", "pytest", true},
		{"value flag and value skipped", "--python 3.12 pytest -v", uvFlags, true, "--python 3.12 ", "pytest", true},
		{"with package value skipped", "--with cov pytest", uvFlags, true, "--with cov ", "pytest", true},
		{"flag eq form", "--python=3.12 pytest", uvFlags, true, "--python=3.12 ", "pytest", true},
		{"dash dash separator", "-- pytest", nil, true, "-- ", "pytest", true},
		{"unknown inner fails closed", "echo pytest", nil, true, "", "", false},
		{"no command at all", "--frozen --isolated", uvFlags, true, "", "", false},
		{"empty", "", nil, true, "", "", false},
		// Value of a value-flag that happens to be a known command is consumed,
		// not mistaken for the inner command.
		{"value flag value is a known cmd", "--with ruff pytest", uvFlags, true, "--with ruff ", "pytest", true},
		// skipFlags=false (shell wrappers): a leading flag fails closed so a
		// mode flag like "command -v" never rewrites a non-executing lookup.
		{"no-skip direct command", "pytest -q", nil, false, "", "pytest", true},
		{"no-skip flag fails closed", "-v pytest", nil, false, "", "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			before, inner, ok := LocateInner(tc.rest, cmdSet, tc.valueFlags, tc.skipFlags)
			if ok != tc.wantOK || before != tc.wantBefore || inner != tc.wantInner {
				t.Errorf("LocateInner(%q) = (%q,%q,%v), want (%q,%q,%v)",
					tc.rest, before, inner, ok, tc.wantBefore, tc.wantInner, tc.wantOK)
			}
		})
	}
}

func TestMergeTransparentPrefixesOrdersByLength(t *testing.T) {
	got := MergeTransparentPrefixes([]string{"docker exec ctr", "", "  ssh host  "})
	// User prefixes are included.
	var hasDocker, hasSSH bool
	for _, p := range got {
		if p.Prefix == "docker exec ctr" {
			hasDocker = true
		}
		if p.Prefix == "ssh host" {
			hasSSH = true
		}
	}
	if !hasDocker || !hasSSH {
		t.Fatalf("user prefixes missing: %+v", got)
	}
	// Longest-first ordering: each prefix is at least as long as the next.
	for i := 1; i < len(got); i++ {
		if len(got[i].Prefix) > len(got[i-1].Prefix) {
			t.Fatalf("not ordered longest-first at %d: %q before %q", i, got[i-1].Prefix, got[i].Prefix)
		}
	}
}

func TestMatchTransparentPrefix(t *testing.T) {
	prefixes := MergeTransparentPrefixes([]string{"docker exec ctr"})
	cases := []struct {
		bareCmd  string
		wantPfx  string
		wantRest string
		wantOK   bool
	}{
		{"uv run pytest -v", "uv run", "pytest -v", true},
		{"docker exec ctr pytest", "docker exec ctr", "pytest", true},
		{"poetry run ruff check", "poetry run", "ruff check", true},
		{"uv run", "", "", false}, // no inner command
		{"git status", "", "", false},
	}
	for _, tc := range cases {
		tp, rest, ok := matchTransparentPrefix(tc.bareCmd, prefixes)
		if ok != tc.wantOK || tp.Prefix != tc.wantPfx || rest != tc.wantRest {
			t.Errorf("matchTransparentPrefix(%q) = (%q,%q,%v), want (%q,%q,%v)",
				tc.bareCmd, tp.Prefix, rest, ok, tc.wantPfx, tc.wantRest, tc.wantOK)
		}
	}
}
