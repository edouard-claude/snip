package engine

import (
	"fmt"
	"os"
	"strings"

	"github.com/edouard-claude/snip/internal/filter"
	"github.com/edouard-claude/snip/internal/tee"
	"github.com/edouard-claude/snip/internal/tracking"
	"github.com/edouard-claude/snip/internal/utils"
)

// Pipeline orchestrates command execution, filtering, tracking, and tee.
type Pipeline struct {
	Registry      *filter.Registry
	Tracker       *tracking.Tracker
	TeeConfig     tee.Config
	Verbose       int
	UltraCompact  bool
	QuietNoFilter bool
	FilterEnabled map[string]bool
}

// Run executes a command through the full pipeline.
func (p *Pipeline) Run(command string, args []string) int {
	// Extract subcommand (first non-flag arg)
	subcommand := ""
	filterArgs := args
	if len(args) > 0 {
		subcommand = args[0]
		filterArgs = args[1:]
	}

	// Match filter
	f := p.Registry.Match(command, subcommand, filterArgs)

	// No filter found: passthrough with hint so LLMs know snip is unnecessary
	if f == nil {
		if !p.QuietNoFilter {
			fmt.Fprintf(os.Stderr, "snip: no filter for %q, passing through -- you can run %q directly\n", command, command)
		}
		return p.Passthrough(command, args)
	}

	// Filter disabled via config: treat as no filter
	if !p.isFilterEnabled(f.Name) {
		if !p.QuietNoFilter {
			fmt.Fprintf(os.Stderr, "snip: filter %q disabled, passing through\n", f.Name)
		}
		return p.Passthrough(command, args)
	}

	// Compute injected args
	fullArgs := args
	finalArgs := args
	if injected, ok := p.Registry.ShouldInject(f, args); ok {
		finalArgs = injected
	}

	// Start SQLite init concurrently with command execution
	if p.Tracker != nil {
		p.Tracker.WarmUp()
	}

	// Start timing
	timed := tracking.Start(p.Tracker)

	// Execute command
	result, err := Execute(command, finalArgs)
	if err != nil {
		// Execution failed entirely — fallback to passthrough
		if p.Verbose > 0 {
			fmt.Fprintf(os.Stderr, "snip: execute error: %v\n", err)
		}
		code, _ := Passthrough(command, fullArgs)
		return code
	}

	// Build pipeline input from selected streams
	pipelineInput := buildPipelineInput(f, result)

	// Apply filter pipeline
	filtered, filterErr := ApplyPipeline(f, pipelineInput)
	if filterErr != nil {
		// Graceful degradation: use raw output
		if p.Verbose > 0 {
			fmt.Fprintf(os.Stderr, "snip: filter error: %v\n", filterErr)
		}
		filtered = pipelineInput
	}

	// Tee: save raw output if needed
	hint := tee.MaybeSave(pipelineInput, result.ExitCode, command, p.TeeConfig)

	// Print output
	fmt.Print(filtered)
	if hint != "" {
		fmt.Fprintln(os.Stderr, hint)
	}
	// Only re-emit stderr if it was not included in the filtered streams
	if result.Stderr != "" && !f.HasStream("stderr") {
		fmt.Fprint(os.Stderr, result.Stderr)
	}

	// Track (skip if no input — nothing meaningful to measure)
	inputTokens := utils.EstimateTokens(pipelineInput)
	if inputTokens > 0 {
		originalCmd := command + " " + strings.Join(fullArgs, " ")
		snipCmd := command + " " + strings.Join(finalArgs, " ")
		outputTokens := utils.EstimateTokens(filtered)
		if err := timed.Track(originalCmd, snipCmd, inputTokens, outputTokens); err != nil {
			fmt.Fprintf(os.Stderr, "snip: tracking error: %v\n", err)
		}
	}

	return result.ExitCode
}

// Passthrough runs a command directly without filtering.
// Passthrough commands are not tracked because the output goes directly
// to stdout — snip never captures it, so token counts would be 0/0.
func (p *Pipeline) Passthrough(command string, args []string) int {
	code, err := Passthrough(command, args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "snip: %v\n", err)
		return 1
	}

	return code
}

// isFilterEnabled returns whether a filter is enabled. A nil map means all
// enabled; a missing entry defaults to enabled; only explicit false disables.
func (p *Pipeline) isFilterEnabled(name string) bool {
	if p.FilterEnabled == nil {
		return true
	}
	enabled, ok := p.FilterEnabled[name]
	if !ok {
		return true
	}
	return enabled
}

// ApplyPipeline executes filter actions sequentially.
func ApplyPipeline(f *filter.Filter, input string) (string, error) {
	lines := strings.Split(input, "\n")
	// Remove trailing empty line from split
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}

	result := filter.ActionResult{
		Lines:    lines,
		Metadata: make(map[string]any),
	}

	for i, action := range f.Pipeline {
		fn, ok := filter.GetAction(action.ActionName)
		if !ok {
			return "", fmt.Errorf("unknown action %q at pipeline[%d]", action.ActionName, i)
		}

		var err error
		result, err = fn(result, action.Params)
		if err != nil {
			return "", fmt.Errorf("pipeline[%d] %s: %w", i, action.ActionName, err)
		}
	}

	return strings.Join(result.Lines, "\n") + "\n", nil
}

// buildPipelineInput assembles the text to filter based on the filter's
// streams configuration. When both stdout and stderr are selected, stderr
// is appended after stdout so the pipeline processes them as a single block.
func buildPipelineInput(f *filter.Filter, result *Result) string {
	hasStdout := f.HasStream("stdout")
	hasStderr := f.HasStream("stderr")

	switch {
	case hasStdout && hasStderr:
		return result.Stdout + result.Stderr
	case hasStderr:
		return result.Stderr
	default:
		return result.Stdout
	}
}
