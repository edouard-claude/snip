package filter

import (
	"fmt"

	"go.yaml.in/yaml/v3"
)

// ParseFilter parses YAML bytes into a Filter struct.
func ParseFilter(data []byte) (*Filter, error) {
	var f Filter
	if err := yaml.Unmarshal(data, &f); err != nil {
		return nil, fmt.Errorf("parse filter: %w", err)
	}
	if err := ValidateFilter(&f); err != nil {
		return nil, err
	}
	return &f, nil
}

// validStreams lists the allowed stream names.
var validStreams = map[string]bool{"stdout": true, "stderr": true}

// ValidateFilter checks required fields and action validity.
func ValidateFilter(f *Filter) error {
	if f.Name == "" {
		return fmt.Errorf("validate filter: missing 'name'")
	}
	if f.Match.Command == "" {
		return fmt.Errorf("validate filter %q: missing 'match.command'", f.Name)
	}
	if f.Match.Subcommand.IsPresent() && len(f.Match.Subcommand.Values()) == 0 {
		return fmt.Errorf("validate filter %q: 'match.subcommand' must not be an empty list", f.Name)
	}
	for _, s := range f.Streams {
		if !validStreams[s] {
			return fmt.Errorf("validate filter %q: unknown stream %q (valid: stdout, stderr)", f.Name, s)
		}
	}
	for i, action := range f.Pipeline {
		if action.ActionName == "" {
			return fmt.Errorf("validate filter %q: pipeline[%d] missing 'action'", f.Name, i)
		}
		if _, ok := GetAction(action.ActionName); !ok {
			return fmt.Errorf("validate filter %q: pipeline[%d] unknown action %q", f.Name, i, action.ActionName)
		}
	}
	return nil
}
