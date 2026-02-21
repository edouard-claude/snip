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
	Registry     *filter.Registry
	Tracker      *tracking.Tracker
	TeeConfig    tee.Config
	Verbose      int
	UltraCompact bool
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

	// No filter found: passthrough
	if f == nil {
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

	// Apply filter pipeline
	filtered, filterErr := ApplyPipeline(f, result.Stdout)
	if filterErr != nil {
		// Graceful degradation: use raw output
		if p.Verbose > 0 {
			fmt.Fprintf(os.Stderr, "snip: filter error: %v\n", filterErr)
		}
		filtered = result.Stdout
	}

	// Tee: save raw output if needed
	hint := tee.MaybeSave(result.Stdout, result.ExitCode, command, p.TeeConfig)

	// Print output
	fmt.Print(filtered)
	if hint != "" {
		fmt.Fprintln(os.Stderr, hint)
	}
	if result.Stderr != "" {
		fmt.Fprint(os.Stderr, result.Stderr)
	}

	// Track (skip if no input — nothing meaningful to measure)
	inputTokens := utils.EstimateTokens(result.Stdout)
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
