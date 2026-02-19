package filter

// Filter represents a declarative YAML filter for a command.
type Filter struct {
	Name        string   `yaml:"name"`
	Version     int      `yaml:"version"`
	Description string   `yaml:"description"`
	Match       Match    `yaml:"match"`
	Inject      *Inject  `yaml:"inject,omitempty"`
	Pipeline    Pipeline `yaml:"pipeline"`
	OnError     string   `yaml:"on_error"` // "passthrough", "empty", "template"
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
