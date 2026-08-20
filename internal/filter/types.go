package filter

import (
	"fmt"
	"slices"

	"gopkg.in/yaml.v3"
)

// Filter represents a declarative YAML filter for a command.
type Filter struct {
	Name        string       `yaml:"name"`
	Version     int          `yaml:"version"`
	Description string       // parsed from YAML but unused by behavior code
	Match       Match        `yaml:"match"`
	Inject      *Inject      `yaml:"inject,omitempty"`
	Streams     []string     `yaml:"streams,omitempty"`
	Pipeline    Pipeline     `yaml:"pipeline"`
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
	Command      string          `yaml:"command"`
	Subcommand   MatchSubcommand `yaml:"subcommand,omitempty"`
	ExcludeFlags []string        `yaml:"exclude_flags,omitempty"`
	RequireFlags []string        `yaml:"require_flags,omitempty"`
}

// MatchSubcommand preserves whether match.subcommand was omitted while
// accepting the legacy scalar YAML form and the list form.
type MatchSubcommand struct {
	present bool
	values  []string
}

// NewSubcommand returns an explicit single subcommand match for direct Go use.
func NewSubcommand(value string) MatchSubcommand {
	return MatchSubcommand{present: true, values: []string{value}}
}

// NewSubcommands returns an explicit multi-subcommand match for direct Go use.
func NewSubcommands(values ...string) MatchSubcommand {
	return MatchSubcommand{present: true, values: append([]string(nil), values...)}
}

// IsPresent reports whether match.subcommand was explicitly supplied.
func (s MatchSubcommand) IsPresent() bool {
	return s.present
}

// Values returns the exact subcommands supplied for matching.
func (s MatchSubcommand) Values() []string {
	return append([]string(nil), s.values...)
}

// String returns the first configured subcommand for legacy assertions and logs.
func (s MatchSubcommand) String() string {
	if len(s.values) == 0 {
		return ""
	}
	return s.values[0]
}

// IsZero allows yaml omitempty to treat an omitted subcommand as empty.
func (s MatchSubcommand) IsZero() bool {
	return !s.present
}

// UnmarshalYAML accepts match.subcommand as either a scalar string or a list of strings.
func (s *MatchSubcommand) UnmarshalYAML(value *yaml.Node) error {
	s.present = true
	s.values = nil

	switch value.Kind {
	case yaml.ScalarNode:
		var subcommand string
		if err := value.Decode(&subcommand); err != nil {
			return err
		}
		s.values = []string{subcommand}
		return nil
	case yaml.SequenceNode:
		s.values = make([]string, 0, len(value.Content))
		for i, node := range value.Content {
			var subcommand string
			if err := node.Decode(&subcommand); err != nil {
				return fmt.Errorf("subcommand[%d]: %w", i, err)
			}
			s.values = append(s.values, subcommand)
		}
		return nil
	default:
		return fmt.Errorf("subcommand must be a string or list of strings")
	}
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

// PipelineActionNames returns the ordered list of action names in the pipeline.
func (f *Filter) PipelineActionNames() []string {
	names := make([]string, len(f.Pipeline))
	for i, a := range f.Pipeline {
		names[i] = a.ActionName
	}
	return names
}

// ActionFunc is the signature for built-in action implementations.
type ActionFunc func(input ActionResult, params map[string]any) (ActionResult, error)
