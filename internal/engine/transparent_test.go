package engine

import (
	"testing"

	"github.com/edouard-claude/snip/internal/config"
	"github.com/edouard-claude/snip/internal/filter"
	"github.com/edouard-claude/snip/internal/hook"
)

// transparentTestPipeline builds a pipeline whose registry knows "git" (with a
// git-log subcommand filter) and "pytest", plus the built-in transparent runner
// prefixes and a custom single-word user prefix "myrunner".
func transparentTestPipeline() *Pipeline {
	filters := []filter.Filter{
		{Name: "git-log", Match: filter.Match{Command: "git", Subcommand: "log"}},
		{Name: "pytest", Match: filter.Match{Command: "pytest"}},
	}
	return &Pipeline{
		Registry:            filter.NewRegistry(filters),
		TransparentPrefixes: hook.MergeTransparentPrefixes([]string{"myrunner"}),
	}
}

func sliceEq(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestUnwrapTransparent(t *testing.T) {
	p := transparentTestPipeline()

	cases := []struct {
		name          string
		command       string
		args          []string
		wantOK        bool
		wantInner     string
		wantInnerArgs []string
		wantRunner    []string
	}{
		{
			name: "uv run with inner subcommand and args",
			command: "uv", args: []string{"run", "git", "log", "-5"},
			wantOK: true, wantInner: "git", wantInnerArgs: []string{"log", "-5"}, wantRunner: []string{"run"},
		},
		{
			name: "uv run skips value-taking flag and its value",
			command: "uv", args: []string{"run", "--python", "3.12", "pytest"},
			wantOK: true, wantInner: "pytest", wantInnerArgs: nil, wantRunner: []string{"run", "--python", "3.12"},
		},
		{
			name: "uv run handles flag=value form",
			command: "uv", args: []string{"run", "--python=3.12", "pytest", "-x"},
			wantOK: true, wantInner: "pytest", wantInnerArgs: []string{"-x"}, wantRunner: []string{"run", "--python=3.12"},
		},
		{
			// A single-word user prefix puts the inner command at args[0], so the
			// runner prefix is empty. (The built-in single-word prefixes exec/
			// noglob/nocorrect/command are shell builtins: exec is intercepted by
			// unproxyableReason before the pipeline, and the others aren't real
			// binaries, so a custom prefix is the realistic single-word case.)
			name: "single-word prefix puts inner at args[0]",
			command: "myrunner", args: []string{"git", "log"},
			wantOK: true, wantInner: "git", wantInnerArgs: []string{"log"}, wantRunner: []string{},
		},
		{
			name: "path-qualified runner still matches by base name",
			command: "/usr/local/bin/uv", args: []string{"run", "git", "log"},
			wantOK: true, wantInner: "git", wantInnerArgs: []string{"log"}, wantRunner: []string{"run"},
		},
		{
			name: "unknown inner command fails closed",
			command: "uv", args: []string{"run", "unknowncmd", "x"},
			wantOK: false,
		},
		{
			name: "flag before inner on a non-skip prefix fails closed",
			command: "command", args: []string{"-v", "git"},
			wantOK: false,
		},
		{
			name: "no transparent prefix",
			command: "git", args: []string{"log"},
			wantOK: false,
		},
		{
			name: "prefix alone with no inner command",
			command: "uv", args: []string{"run"},
			wantOK: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			inner, innerArgs, runner, ok := p.unwrapTransparent(tc.command, tc.args)
			if ok != tc.wantOK {
				t.Fatalf("ok = %v, want %v (inner=%q innerArgs=%v runner=%v)", ok, tc.wantOK, inner, innerArgs, runner)
			}
			if !ok {
				return
			}
			if inner != tc.wantInner {
				t.Errorf("inner = %q, want %q", inner, tc.wantInner)
			}
			if !sliceEq(innerArgs, tc.wantInnerArgs) {
				t.Errorf("innerArgs = %v, want %v", innerArgs, tc.wantInnerArgs)
			}
			if !sliceEq(runner, tc.wantRunner) {
				t.Errorf("runner = %v, want %v", runner, tc.wantRunner)
			}
		})
	}
}

// TestUnwrapTransparentDisabledWhenNoPrefixes verifies unwrapping is inert when
// no transparent prefixes are configured.
func TestUnwrapTransparentDisabledWhenNoPrefixes(t *testing.T) {
	p := &Pipeline{
		Registry:            filter.NewRegistry([]filter.Filter{{Name: "git-log", Match: filter.Match{Command: "git", Subcommand: "log"}}}),
		TransparentPrefixes: nil,
	}
	if _, _, _, ok := p.unwrapTransparent("uv", []string{"run", "git", "log"}); ok {
		t.Error("expected no unwrap when TransparentPrefixes is empty")
	}
}

// TestIsBypassed verifies the bypass-list check used for both the outer command
// and the unwrapped inner command (so `snip uv run git ...` is bypassed when
// "git" is bypassed, mirroring the outer check).
func TestIsBypassed(t *testing.T) {
	p := &Pipeline{
		Config: &config.Config{
			Filters: config.FiltersConfig{
				Bypass: config.FilterBypassConfig{Commands: []string{"git", "docker"}},
			},
		},
	}
	if !p.isBypassed("git") {
		t.Error("git should be bypassed")
	}
	if p.isBypassed("go") {
		t.Error("go should not be bypassed")
	}

	// A nil Config never bypasses.
	if (&Pipeline{}).isBypassed("git") {
		t.Error("nil Config must not bypass")
	}
}
