package filter

import (
	"sort"
	"strings"
)

// Registry holds loaded filters indexed for fast command matching.
type Registry struct {
	byKey   map[string][]Filter // key = "command" or "command:subcommand"
	filters []Filter
}

// NewRegistry builds a registry from a list of filters.
func NewRegistry(filters []Filter) *Registry {
	r := &Registry{
		byKey:   make(map[string][]Filter),
		filters: filters,
	}
	for _, f := range filters {
		key := f.Match.Command
		if f.Match.Subcommand != "" {
			key += ":" + f.Match.Subcommand
		}
		r.byKey[key] = append(r.byKey[key], f)
	}
	return r
}

// Match finds the first filter matching the given command, subcommand, and args.
func (r *Registry) Match(command, subcommand string, args []string) *Filter {
	// Try exact match first (command:subcommand)
	if subcommand != "" {
		key := command + ":" + subcommand
		if candidates, ok := r.byKey[key]; ok {
			for i := range candidates {
				if matchesFlags(&candidates[i], args) {
					return &candidates[i]
				}
			}
		}
	}

	// Try command-only match
	if candidates, ok := r.byKey[command]; ok {
		for i := range candidates {
			if matchesFlags(&candidates[i], args) {
				return &candidates[i]
			}
		}
	}

	return nil
}

// ShouldInject computes final args with injections, respecting skip_if_present.
func (r *Registry) ShouldInject(f *Filter, args []string) ([]string, bool) {
	if f.Inject == nil {
		return args, false
	}

	// Check skip_if_present
	for _, skip := range f.Inject.SkipIfPresent {
		for _, arg := range args {
			if strings.HasPrefix(arg, skip) {
				return args, false
			}
		}
	}

	// Apply injected args — insert before "--" separator if present
	result := make([]string, 0, len(args)+len(f.Inject.Args))
	dashDashIdx := -1
	for i, a := range args {
		if a == "--" {
			dashDashIdx = i
			break
		}
	}
	if dashDashIdx >= 0 {
		result = append(result, args[:dashDashIdx]...)
		result = append(result, f.Inject.Args...)
		result = append(result, args[dashDashIdx:]...)
	} else {
		result = append(result, args...)
		result = append(result, f.Inject.Args...)
	}

	// Apply defaults (only if flag not already present)
	for flag, val := range f.Inject.Defaults {
		found := false
		for _, arg := range result {
			if strings.HasPrefix(arg, flag) {
				found = true
				break
			}
		}
		if !found {
			result = append(result, flag, val)
		}
	}

	return result, true
}

// HasAnyFilter returns true if any filter is registered for the given command
// and subcommand, regardless of flag constraints. Use this to distinguish
// "no filter at all" from "filter exists but was excluded by flags".
func (r *Registry) HasAnyFilter(command, subcommand string) bool {
	if subcommand != "" {
		if _, ok := r.byKey[command+":"+subcommand]; ok {
			return true
		}
	}
	_, ok := r.byKey[command]
	return ok
}

// Commands returns a sorted, unique list of base command names in the registry.
// Keys like "git:log" are split on ":" and only the base command "git" is kept.
func (r *Registry) Commands() []string {
	seen := make(map[string]struct{})
	for key := range r.byKey {
		base, _, _ := strings.Cut(key, ":")
		seen[base] = struct{}{}
	}
	cmds := make([]string, 0, len(seen))
	for cmd := range seen {
		cmds = append(cmds, cmd)
	}
	sort.Strings(cmds)
	return cmds
}

func matchesFlags(f *Filter, args []string) bool {
	// Check exclude_flags: skip if user passed any excluded flag
	for _, exclude := range f.Match.ExcludeFlags {
		for _, arg := range args {
			if strings.HasPrefix(arg, exclude) {
				return false
			}
		}
	}

	// Check require_flags: skip if user missing required flag
	for _, require := range f.Match.RequireFlags {
		found := false
		for _, arg := range args {
			if strings.HasPrefix(arg, require) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	return true
}
