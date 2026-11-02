package engine

import (
	"testing"

	"github.com/edouard-claude/snip/internal/filter"
)

// git -C <dir> status must reach the git-status filter: the global options are
// skipped for the subcommand match, replayed in front of the subcommand at
// execution time, and the injected flags land after the subcommand, where git
// expects them (issue #201).
func TestPipelineRunSkipsGlobalFlagsBeforeSubcommand(t *testing.T) {
	f := filter.Filter{
		Name:    "git-status",
		Version: 1,
		Match:   filter.Match{Command: "git", Subcommand: filter.NewSubcommand("status")},
		Inject:  &filter.Inject{Args: []string{"--porcelain"}},
		Pipeline: filter.Pipeline{
			{ActionName: "keep_lines", Params: map[string]any{"pattern": `^ M`}},
		},
	}
	var got []string
	p := &Pipeline{
		Registry: filter.NewRegistry([]filter.Filter{f}),
		execute: func(command string, args []string) (*Result, error) {
			got = append([]string{command}, args...)
			return &Result{Stdout: " M a.go\n?? b.go\n"}, nil
		},
	}

	out := captureStdout(t, func() { p.Run("git", []string{"-C", "../repo", "status"}) })

	want := []string{"git", "-C", "../repo", "status", "--porcelain"}
	if !sliceEq(got, want) {
		t.Fatalf("executed %q, want %q (nil means the filter never matched and the command ran unfiltered)", got, want)
	}
	if out != " M a.go\n" {
		t.Errorf("filtered output = %q, want %q", out, " M a.go\n")
	}
}

// The same applies to the inner command of a transparent runner prefix.
func TestPipelineRunSkipsGlobalFlagsOfInnerCommand(t *testing.T) {
	p := transparentTestPipeline()
	var got []string
	p.execute = func(command string, args []string) (*Result, error) {
		got = append([]string{command}, args...)
		return &Result{Stdout: "abc123 first commit\n"}, nil
	}

	out := captureStdout(t, func() { p.Run("myrunner", []string{"git", "-C", "../repo", "log", "-3"}) })

	want := []string{"myrunner", "git", "-C", "../repo", "log", "-3"}
	if !sliceEq(got, want) {
		t.Fatalf("executed %q, want %q", got, want)
	}
	if out != "abc123 first commit\n" {
		t.Errorf("output = %q, want the git-log filter's passthrough of the raw line", out)
	}
}
