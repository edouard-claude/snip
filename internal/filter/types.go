package filter

import (
	"slices"
)

// Filter represents a declarative YAML filter for a command.
type Filter struct {
	Name        string       `yaml:"name"`
	Version     int          `yaml:"version"`
	Description string       `yaml:"description"`
	Match       Match        `yaml:"match"`
	Inject      *Inject      `yaml:"inject,omitempty"`
	Streams     []string     `yaml:"streams,omitempty"` // "stdout", "stderr"; defaults to ["stdout"]
	Pipeline    Pipeline     `yaml:"pipeline"`
	OnError     string       `yaml:"on_error"` // "passthrough", "empty", "template"
	Tests       []FilterTest `yaml:"tests,omitempty"`
}

// FilterTest defines an inline test case for a filter.
type FilterTest struct {
	Name     string `yaml:"name"`
	Input    string `yaml:"input"`
	Expected string `yaml:"expected"`
}

// HasStream returns true if the filter includes the given stream name.
// When Streams is empty (default), only "stdout" is included.
func (f *Filter) HasStream(name string) bool {
	if len(f.Streams) == 0 {
		return name == "stdout"
	}
	return slices.Contains(f.Streams, name)
}

// Clone returns a deep copy of the filter. The Pipeline and its Params
// maps are independently allocated so mutations to the clone do not
// affect the original.
func (f *Filter) Clone() *Filter {
	if f == nil {
		return nil
	}
	clone := *f
	clone.Pipeline = make(Pipeline, len(f.Pipeline))
	for i, a := range f.Pipeline {
		clone.Pipeline[i] = Action{
			ActionName: a.ActionName,
			Params:     cloneParams(a.Params),
		}
	}
	return &clone
}

// cloneParams returns a deep copy of the params map, preserving nil.
func cloneParams(src map[string]any) map[string]any {
	if src == nil {
		return nil
	}
	dst := make(map[string]any, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

// Match defines which command a filter applies to.
type Match struct {
	Command      string   `yaml:"command"`
	Subcommand   string   `yaml:"subcommand,omitempty"`
	ExcludeFlags []string `yaml:"exclude_flags,omitempty"`
	RequireFlags []string `yaml:"require_flags,omitempty"`
}

// Inject defines args to inject before execution.
type Inject struct {
	Args          []string          `yaml:"args,omitempty"`
	Defaults      map[string]string `yaml:"defaults,omitempty"`
	SkipIfPresent []string          `yaml:"skip_if_present,omitempty"`
}

// Action represents a single step in a filter pipeline.
type Action struct {
	ActionName string         `yaml:"action"`
	Params     map[string]any `yaml:",inline"`
}

// Pipeline is an ordered sequence of actions.
type Pipeline []Action

// ActionResult is the data passed between pipeline actions.
type ActionResult struct {
	Lines    []string
	Metadata map[string]any
}

// ActionFunc is the signature for built-in action implementations.
type ActionFunc func(input ActionResult, params map[string]any) (ActionResult, error)
