package engine

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/edouard-claude/snip/internal/config"
	"github.com/edouard-claude/snip/internal/filter"
	"github.com/edouard-claude/snip/internal/hook"
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
	// TransparentPrefixes are runner wrappers (uv run, poetry run, ...) whose
	// inner command's filter should apply when run directly via `snip <prefix>
	// <cmd>`. Empty disables transparent unwrapping on the direct-run path.
	TransparentPrefixes []hook.TransparentPrefix
}

// Run executes a command through the full pipeline.
func (p *Pipeline) Run(command string, args []string) int {
	// Check if command is in the project-level bypass list.
	// Bypassed commands pass through unfiltered regardless of filter match.
	if p.isBypassed(command) {
		return p.Passthrough(command, args)
	}

	// Resolve the filter target. fcmd/fargs are the command whose filter applies
	// and its args; runnerPrefix is the transparent wrapper tokens (e.g. "run",
	// "--python", "3.12") that precede the inner command and must be replayed at
	// execution time. For a normal command these are (command, args, nil).
	fcmd, fargs := command, args
	var runnerPrefix []string

	subcommand, filterArgs := splitFirstArg(fargs)
	f := p.Registry.Match(fcmd, subcommand, filterArgs)

	// No direct filter: if command starts with a transparent runner prefix
	// (uv run, poetry run, ...), unwrap to the inner command so its filter
	// applies while the full wrapper still executes (issue #95).
	if f == nil && len(p.TransparentPrefixes) > 0 {
		if inner, innerArgs, runner, ok := p.unwrapTransparent(command, args); ok {
			// Honor the bypass list for the inner command too: a bypassed inner
			// runs the full wrapper unfiltered, mirroring the outer check above.
			if p.isBypassed(inner) {
				return p.Passthrough(command, args)
			}
			isub, ifilter := splitFirstArg(innerArgs)
			if f2 := p.Registry.Match(inner, isub, ifilter); f2 != nil {
				f, fcmd, fargs, runnerPrefix = f2, inner, innerArgs, runner
			}
		}
	}

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

	// Compute injected args for the (inner) filter, then reattach the runner
	// prefix so the full wrapper still executes.
	fullArgs := args
	finalArgs := fargs
	if injected, ok := p.Registry.ShouldInject(f, fargs); ok {
		finalArgs = injected
	}
	if runnerPrefix != nil {
		exec := make([]string, 0, len(runnerPrefix)+1+len(finalArgs))
		exec = append(exec, runnerPrefix...)
		exec = append(exec, fcmd)
		exec = append(exec, finalArgs...)
		finalArgs = exec
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

	// Safety net: a filter that strips every line would send empty output to
	// the LLM, which is worse than the raw result and triggers wasteful retry
	// loops (issue #85). Fall back to raw unless the input was itself empty.
	// Filters with a legitimately empty result use the on_empty action to emit
	// a message, so they never reach this state.
	if shouldRestoreRaw(filtered, pipelineInput) {
		if p.Verbose > 0 {
			fmt.Fprintf(os.Stderr, "snip: filter %q produced empty output, using raw\n", f.Name)
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

// isBypassed reports whether command is in the project-level bypass list, which
// forces unfiltered passthrough regardless of any filter match.
func (p *Pipeline) isBypassed(command string) bool {
	if p.Config == nil {
		return false
	}
	for _, cmd := range p.Config.Filters.Bypass.Commands {
		if cmd == command {
			return true
		}
	}
	return false
}

// splitFirstArg returns the first arg (the subcommand) and the remaining args.
// Both are empty for a no-argument command.
func splitFirstArg(args []string) (first string, rest []string) {
	if len(args) > 0 {
		return args[0], args[1:]
	}
	return "", nil
}

// unwrapTransparent detects a transparent runner prefix (e.g. "uv run") at the
// head of command+args and locates the inner command so its filter applies
// while the full wrapper still executes. It returns the inner command token,
// the inner command's own args, and the runner tokens within args that precede
// the inner command (e.g. ["run", "--python", "3.12"]). ok is false when no
// prefix matches or the first non-flag token is not a known snip command — fail
// closed, mirroring the hook's LocateInner so an argument is never mistaken for
// the inner command.
func (p *Pipeline) unwrapTransparent(command string, args []string) (inner string, innerArgs, runnerArgs []string, ok bool) {
	full := make([]string, 0, len(args)+1)
	full = append(full, command)
	full = append(full, args...)

	for _, tp := range p.TransparentPrefixes {
		pre := strings.Fields(tp.Prefix)
		if len(pre) == 0 || len(full) <= len(pre) || !prefixMatches(full, pre) {
			continue
		}
		for i := len(pre); i < len(full); i++ {
			t := full[i]
			if strings.HasPrefix(t, "-") {
				if !tp.SkipFlags {
					break // flag before the inner command: fail closed for this prefix
				}
				if tp.ValueFlags[t] && !strings.Contains(t, "=") {
					i++ // value-taking flag also consumes the next token
				}
				continue
			}
			// HasAnyFilterForCommand base-normalizes the token, so a path- or
			// ./-qualified inner command still matches its filter.
			if p.Registry.HasAnyFilterForCommand(t) {
				// full[0] is command, so the args index is i-1.
				return t, full[i+1:], args[:i-1], true
			}
			break // first non-flag token is not a known command: fail closed
		}
	}
	return "", nil, nil, false
}

// prefixMatches reports whether full begins with the prefix tokens, comparing
// the leading runner by base name so /usr/bin/uv still matches the "uv" prefix.
func prefixMatches(full, pre []string) bool {
	if filepath.Base(full[0]) != pre[0] {
		return false
	}
	for i := 1; i < len(pre); i++ {
		if full[i] != pre[i] {
			return false
		}
	}
	return true
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

// shouldRestoreRaw reports whether a filter produced an empty (whitespace-only)
// result from non-empty input. In that case the engine restores the raw output
// rather than sending nothing to the LLM (issue #85).
func shouldRestoreRaw(filtered, raw string) bool {
	return strings.TrimSpace(filtered) == "" && strings.TrimSpace(raw) != ""
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
		if action.Params == nil {
			f.Pipeline[i].Params = make(map[string]any)
		}
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
