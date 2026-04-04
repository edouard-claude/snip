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
			fmt.Fprintf(os.Stderr, "snip: no filter for %q, passing through — you can run %q directly\n", command, command)
		}
		code, _ := Passthrough(command, args)
		return code
	}

	// Check if filter is disabled in configuration
	if !p.isFilterEnabled(f.Name) {
		if !p.QuietNoFilter {
			fmt.Fprintf(os.Stderr, "snip: no filter for %q, passing through — you can run %q directly\n", command, command)
		}
		code, _ := Passthrough(command, args)
		return code
	}

	// Execute command with filtering
	result, err := Execute(command, args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "snip: %v\n", err)
		return 1
	}

	// Build input for pipeline based on filter's stream configuration
	input := buildPipelineInput(f, result)

	// Apply filter pipeline
	output, err := ApplyPipeline(f, input)
	if err != nil {
		fmt.Fprintf(os.Stderr, "snip: filter error: %v\n", err)
		fmt.Fprint(os.Stdout, input) // Fallback to raw output
		return result.ExitCode
	}

	// Print filtered output
	fmt.Fprint(os.Stdout, output)

	// Track token savings if tracking is enabled
	if p.Tracker != nil {
		beforeTokens := utils.EstimateTokens(input)
		afterTokens := utils.EstimateTokens(output)
		if beforeTokens > afterTokens {
			_ = p.Tracker.Track(command, f.Name, beforeTokens, afterTokens, result.Duration.Milliseconds())
		}
	}

	// Handle tee (save raw output on failure)
	_ = tee.MaybeSave(input, result.ExitCode, command, p.TeeConfig)

	return result.ExitCode
}

// isFilterEnabled checks if a filter is enabled in the configuration
func (p *Pipeline) isFilterEnabled(name string) bool {
	if p.FilterEnabled == nil {
		return true
	}
	enabled, exists := p.FilterEnabled[name]
	if !exists {
		return true
	}
	return enabled
}

// Passthrough runs a command directly without filtering.
// Passthrough commands are not tracked because the output goes directly
// to stdout — snip never captures it, so token counts would be 0/0.
func (p *Pipeline) Passthrough(command string, args []string) int {
	code, err := Passthrough(command, args)
	if err != nil && !p.QuietNoFilter {
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
