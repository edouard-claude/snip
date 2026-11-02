package engine

import (
	"testing"

	"github.com/edouard-claude/snip/internal/filter"
	"github.com/edouard-claude/snip/internal/hook"
)

// runnerTestPipeline builds a pipeline where the user-configured transparent
// prefix "fakenpx" also has a command-level filter of its own, like the
// built-in npx filter, next to a filter for the inner command vitest. The two
// filters keep different lines so the output tells which one applied.
func runnerTestPipeline(enabled map[string]bool) (*Pipeline, *[]string) {
	filters := []filter.Filter{
		{
			Name:  "fakenpx",
			Match: filter.Match{Command: "fakenpx"},
			Pipeline: filter.Pipeline{
				{ActionName: "keep_lines", Params: map[string]any{"pattern": `^npx:`}},
			},
		},
		{
			Name:  "vitest",
			Match: filter.Match{Command: "vitest", ExcludeFlags: []string{"--version"}},
			Pipeline: filter.Pipeline{
				{ActionName: "keep_lines", Params: map[string]any{"pattern": `^vitest:`}},
			},
		},
	}
	var executed []string
	p := &Pipeline{
		Registry:            filter.NewRegistry(filters),
		FilterEnabled:       enabled,
		TransparentPrefixes: hook.MergeTransparentPrefixes([]string{"fakenpx"}),
		execute: func(command string, args []string) (*Result, error) {
			executed = append([]string{command}, args...)
			return &Result{Stdout: "npx: Need to install the following packages\nvitest: 3 passed\n"}, nil
		},
	}
	return p, &executed
}

// A transparent prefix does not change what runs, so the inner command's
// filter wins over the runner's own filter (issue #202).
func TestPipelineRunPrefersInnerFilterOverRunnerFilter(t *testing.T) {
	cases := []struct {
		name     string
		enabled  map[string]bool
		args     []string
		wantOut  string
		wantExec []string
	}{
		{
			name:     "inner filter applies although the runner has a filter",
			args:     []string{"vitest", "run"},
			wantOut:  "vitest: 3 passed\n",
			wantExec: []string{"fakenpx", "vitest", "run"},
		},
		{
			name:     "disabling the runner filter does not block the inner one",
			enabled:  map[string]bool{"fakenpx": false},
			args:     []string{"vitest", "run"},
			wantOut:  "vitest: 3 passed\n",
			wantExec: []string{"fakenpx", "vitest", "run"},
		},
		{
			name:     "unknown inner command stays on the runner filter",
			args:     []string{"cowsay", "hi"},
			wantOut:  "npx: Need to install the following packages\n",
			wantExec: []string{"fakenpx", "cowsay", "hi"},
		},
		{
			name:     "inner filter excluded by flags falls back to the runner filter",
			args:     []string{"vitest", "--version"},
			wantOut:  "npx: Need to install the following packages\n",
			wantExec: []string{"fakenpx", "vitest", "--version"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p, executed := runnerTestPipeline(tc.enabled)
			out := captureStdout(t, func() { p.Run("fakenpx", tc.args) })
			if !sliceEq(*executed, tc.wantExec) {
				t.Fatalf("executed %q, want %q (nil means the command passed through unfiltered)", *executed, tc.wantExec)
			}
			if out != tc.wantOut {
				t.Errorf("output = %q, want %q", out, tc.wantOut)
			}
		})
	}
}

// A disabled inner filter means "do not filter this tool's output": the
// command passes through rather than fall back to the runner's filter.
func TestPipelineRunDisabledInnerFilterPassesThrough(t *testing.T) {
	p, executed := runnerTestPipeline(map[string]bool{"vitest": false})
	p.QuietNoFilter = true

	// fakenpx is not a real binary, so the passthrough fails fast with exit 1
	// without ever reaching the filtered-execution stub.
	code := p.Run("fakenpx", []string{"vitest", "run"})

	if *executed != nil {
		t.Fatalf("executed %q through the filtered path, want a plain passthrough", *executed)
	}
	if code != 1 {
		t.Errorf("exit code = %d, want 1 from the failed passthrough of a missing binary", code)
	}
}
