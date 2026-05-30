package engine

import (
	"fmt"
	"os"
	"strings"

	"github.com/edouard-claude/snip/internal/config"
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
	Config        *config.Config // merged user+project config (nil if not loaded)
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

	// Check if command is in the project-level bypass list.
	// Bypassed commands pass through unfiltered regardless of filter match.
	if p.Config != nil {
		for _, cmd := range p.Config.Filters.Bypass.Commands {
			if cmd == command {
				return p.Passthrough(command, args)
			}
		}
	}

	// Match filter
	f := p.Registry.Match(command, subcommand, filterArgs)

	// No filter found: passthrough.
	// Only print a hint when no filter is registered for the base command at
	// all. If a filter exists but was excluded by flags (e.g. go test -v) or
	// only covers other subcommands (e.g. git-commit but not git checkout),
	// stay silent to avoid the misleading "no filter for git" message (#56).
	if f == nil {
		if !p.QuietNoFilter && !p.Registry.HasAnyFilterForCommand(command) {
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

	// Apply project-level filter overrides to the matched filter pipeline.
	// Clone the filter first to avoid mutating the registry's shared pointer.
	if p.Config != nil {
		f = f.Clone()
		if override, ok := p.Config.Filters.Override[f.Name]; ok {
			applyOverride(f, &override)
		}
		if p.Config.Filters.Global.MaxLines > 0 || p.Config.Filters.Global.MaxLineLength > 0 || p.Config.Filters.Global.MaxOutputBytes > 0 {
			applyGlobalLimit(f, &p.Config.Filters.Global)
		}
	}

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
	// Strip CR from CRLF line endings so anchors like $ and patterns like \.\.?$
	// behave correctly when the wrapped tool emits Windows-style line endings
	// (e.g. cygwin's ls.exe under cmd.exe).
	for i, l := range lines {
		if len(l) > 0 && l[len(l)-1] == '\r' {
			lines[i] = l[:len(l)-1]
		}
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

// applyOverride modifies a filter's pipeline actions based on project config
// overrides. When StreamMode is "full", the entire pipeline is cleared (full
// passthrough). Otherwise, matching pipeline actions are updated with the
// override values. Override targets not found in the existing pipeline are
// appended as new actions (consistent with applyGlobalLimit's behavior).
func applyOverride(f *filter.Filter, o *config.FilterOverride) {
	if o.StreamMode == "full" {
		f.Pipeline = nil
		return
	}
	applied := map[string]bool{}
	for i, action := range f.Pipeline {
		switch action.ActionName {
		case "head":
			if o.Head > 0 {
				f.Pipeline[i].Params["n"] = o.Head
				applied["head"] = true
			}
		case "tail":
			if o.Tail > 0 {
				f.Pipeline[i].Params["n"] = o.Tail
				applied["tail"] = true
			}
		case "truncate_lines":
			if o.TruncateLines > 0 {
				f.Pipeline[i].Params["max"] = o.TruncateLines
				applied["truncate_lines"] = true
			}
		case "keep_lines":
			if o.KeepLines != "" {
				f.Pipeline[i].Params["pattern"] = o.KeepLines
				applied["keep_lines"] = true
			}
		case "remove_lines":
			if o.RemoveLines != "" {
				f.Pipeline[i].Params["pattern"] = o.RemoveLines
				applied["remove_lines"] = true
			}
		}
	}
	if o.Head > 0 && !applied["head"] {
		f.Pipeline = append(f.Pipeline, filter.Action{
			ActionName: "head",
			Params:     map[string]any{"n": o.Head},
		})
	}
	if o.Tail > 0 && !applied["tail"] {
		f.Pipeline = append(f.Pipeline, filter.Action{
			ActionName: "tail",
			Params:     map[string]any{"n": o.Tail},
		})
	}
	if o.TruncateLines > 0 && !applied["truncate_lines"] {
		f.Pipeline = append(f.Pipeline, filter.Action{
			ActionName: "truncate_lines",
			Params:     map[string]any{"max": o.TruncateLines},
		})
	}
	if o.KeepLines != "" && !applied["keep_lines"] {
		f.Pipeline = append(f.Pipeline, filter.Action{
			ActionName: "keep_lines",
			Params:     map[string]any{"pattern": o.KeepLines},
		})
	}
	if o.RemoveLines != "" && !applied["remove_lines"] {
		f.Pipeline = append(f.Pipeline, filter.Action{
			ActionName: "remove_lines",
			Params:     map[string]any{"pattern": o.RemoveLines},
		})
	}
}

// applyGlobalLimit appends global limits (max_lines, max_line_length, max_output_bytes)
// to the end of a filter's pipeline. These act as a final safety cap on all
// filtered output.
func applyGlobalLimit(f *filter.Filter, g *config.FilterGlobalConfig) {
	if g.MaxLines > 0 {
		f.Pipeline = append(f.Pipeline, filter.Action{
			ActionName: "head",
			Params:     map[string]any{"n": g.MaxLines},
		})
	}
	if g.MaxLineLength > 0 {
		f.Pipeline = append(f.Pipeline, filter.Action{
			ActionName: "truncate_lines",
			Params:     map[string]any{"max": g.MaxLineLength},
		})
	}
	if g.MaxOutputBytes > 0 {
		f.Pipeline = append(f.Pipeline, filter.Action{
			ActionName: "truncate_bytes",
			Params:     map[string]any{"max": g.MaxOutputBytes},
		})
	}
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
