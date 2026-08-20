package filter

import (
	"slices"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestFilterYAMLRoundtrip(t *testing.T) {
	input := `
name: "test-filter"
version: 1
description: "A test filter"
match:
  command: "git"
  subcommand: "log"
  exclude_flags: ["--format"]
inject:
  args: ["--oneline"]
  defaults:
    "-n": "10"
  skip_if_present: ["--format"]
pipeline:
  - action: "keep_lines"
    pattern: "\\S"
  - action: "head"
    n: 5
`
	var f Filter
	if err := yaml.Unmarshal([]byte(input), &f); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if f.Name != "test-filter" {
		t.Errorf("name = %q, want 'test-filter'", f.Name)
	}
	if f.Match.Command != "git" {
		t.Errorf("match.command = %q", f.Match.Command)
	}
	if !f.Match.Subcommand.IsPresent() || f.Match.Subcommand.String() != "log" {
		t.Errorf("match.subcommand = %q", f.Match.Subcommand.String())
	}
	if len(f.Match.ExcludeFlags) != 1 || f.Match.ExcludeFlags[0] != "--format" {
		t.Errorf("exclude_flags = %v", f.Match.ExcludeFlags)
	}
	if f.Inject == nil {
		t.Fatal("inject is nil")
	}
	if len(f.Inject.Args) != 1 {
		t.Errorf("inject.args = %v", f.Inject.Args)
	}
	if f.Inject.Defaults["-n"] != "10" {
		t.Errorf("inject.defaults = %v", f.Inject.Defaults)
	}
	if len(f.Pipeline) != 2 {
		t.Fatalf("pipeline len = %d, want 2", len(f.Pipeline))
	}
	if f.Pipeline[0].ActionName != "keep_lines" {
		t.Errorf("pipeline[0].action = %q", f.Pipeline[0].ActionName)
	}
	if f.Pipeline[1].ActionName != "head" {
		t.Errorf("pipeline[1].action = %q", f.Pipeline[1].ActionName)
	}
}

func TestMatchSubcommandYAMLScalarListAndPresence(t *testing.T) {
	tests := []struct {
		name    string
		yaml    string
		present bool
		values  []string
	}{
		{
			name: "omitted",
			yaml: `command: "npm"`,
		},
		{
			name:    "legacy scalar",
			yaml:    "command: npm\nsubcommand: install\n",
			present: true,
			values:  []string{"install"},
		},
		{
			name:    "list",
			yaml:    "command: npm\nsubcommand: [install, add, i]\n",
			present: true,
			values:  []string{"install", "add", "i"},
		},
		{
			name:    "bare command entry",
			yaml:    "command: yarn\nsubcommand: [\"\", install]\n",
			present: true,
			values:  []string{"", "install"},
		},
		{
			name:    "empty scalar is explicit bare command",
			yaml:    "command: yarn\nsubcommand: \"\"\n",
			present: true,
			values:  []string{""},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var m Match
			if err := yaml.Unmarshal([]byte(tt.yaml), &m); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			if m.Subcommand.IsPresent() != tt.present {
				t.Fatalf("present = %v, want %v", m.Subcommand.IsPresent(), tt.present)
			}
			if !slices.Equal(m.Subcommand.Values(), tt.values) {
				t.Errorf("values = %v, want %v", m.Subcommand.Values(), tt.values)
			}
		})
	}
}

func TestMatchSubcommandConstructors(t *testing.T) {
	single := NewSubcommand("install")
	if !single.IsPresent() || !slices.Equal(single.Values(), []string{"install"}) {
		t.Fatalf("single constructor = present %v values %v", single.IsPresent(), single.Values())
	}

	multi := NewSubcommands("", "install", "add")
	if !multi.IsPresent() || !slices.Equal(multi.Values(), []string{"", "install", "add"}) {
		t.Fatalf("multi constructor = present %v values %v", multi.IsPresent(), multi.Values())
	}

	values := multi.Values()
	values[0] = "mutated"
	if got := multi.Values()[0]; got != "" {
		t.Errorf("Values exposed internal slice, first value = %q", got)
	}
}

func TestActionResultEmpty(t *testing.T) {
	ar := ActionResult{Lines: nil, Metadata: nil}
	if len(ar.Lines) != 0 {
		t.Error("expected empty lines")
	}
}

func TestHasStreamDefault(t *testing.T) {
	f := Filter{Name: "test"}
	if !f.HasStream("stdout") {
		t.Error("default should include stdout")
	}
	if f.HasStream("stderr") {
		t.Error("default should not include stderr")
	}
}

func TestHasStreamExplicit(t *testing.T) {
	f := Filter{Name: "test", Streams: []string{"stderr"}}
	if f.HasStream("stdout") {
		t.Error("should not include stdout")
	}
	if !f.HasStream("stderr") {
		t.Error("should include stderr")
	}
}

func TestHasStreamBoth(t *testing.T) {
	f := Filter{Name: "test", Streams: []string{"stdout", "stderr"}}
	if !f.HasStream("stdout") {
		t.Error("should include stdout")
	}
	if !f.HasStream("stderr") {
		t.Error("should include stderr")
	}
}

func TestStreamsYAMLParsing(t *testing.T) {
	input := `
name: "test"
streams: ["stdout", "stderr"]
match:
  command: "bun"
pipeline: []
`
	var f Filter
	if err := yaml.Unmarshal([]byte(input), &f); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(f.Streams) != 2 {
		t.Fatalf("streams len = %d, want 2", len(f.Streams))
	}
	if f.Streams[0] != "stdout" || f.Streams[1] != "stderr" {
		t.Errorf("streams = %v", f.Streams)
	}
}

func TestFilterCloneDeepCopy(t *testing.T) {
	original := Filter{
		Name:    "test-clone",
		Version: 2,
		Match:   Match{Command: "git", Subcommand: NewSubcommand("log")},
		Pipeline: Pipeline{
			{ActionName: "head", Params: map[string]any{"n": 10}},
			{ActionName: "keep_lines", Params: map[string]any{"pattern": `\S`}},
		},
	}

	clone := original.Clone()

	// Verify value fields
	if clone.Name != original.Name {
		t.Errorf("name: got %q, want %q", clone.Name, original.Name)
	}
	if clone.Version != original.Version {
		t.Errorf("version: got %d, want %d", clone.Version, original.Version)
	}

	// Verify pipeline was deep-copied
	if len(clone.Pipeline) != len(original.Pipeline) {
		t.Fatalf("pipeline len: got %d, want %d", len(clone.Pipeline), len(original.Pipeline))
	}
	if clone.Pipeline[0].ActionName != original.Pipeline[0].ActionName {
		t.Errorf("pipeline[0] action: got %q, want %q", clone.Pipeline[0].ActionName, original.Pipeline[0].ActionName)
	}

	// Mutate clone — original should be unaffected
	clone.Pipeline[0].Params["n"] = 999

	if original.Pipeline[0].Params["n"] != 10 {
		t.Errorf("original head n = %v, want 10 (was corrupted by clone mutation)", original.Pipeline[0].Params["n"])
	}
	if clone.Pipeline[0].Params["n"] != 999 {
		t.Errorf("clone head n = %v, want 999", clone.Pipeline[0].Params["n"])
	}

	// Mutate via applyGlobalLimit on clone — original should stay clean
	clone.Pipeline = append(clone.Pipeline, Action{ActionName: "truncate_lines", Params: map[string]any{"max": 80}})
	if len(original.Pipeline) != 2 {
		t.Errorf("original pipeline len = %d, want 2 (corrupted by clone append)", len(original.Pipeline))
	}
	if len(clone.Pipeline) != 3 {
		t.Errorf("clone pipeline len = %d, want 3", len(clone.Pipeline))
	}
}

func TestFilterCloneNil(t *testing.T) {
	var f *Filter
	clone := f.Clone()
	if clone != nil {
		t.Error("Clone of nil should return nil")
	}
}

func TestFilterCloneEmptyPipeline(t *testing.T) {
	f := Filter{
		Name:     "test-empty",
		Match:    Match{Command: "ls"},
		Pipeline: Pipeline{},
	}
	clone := f.Clone()
	if len(clone.Pipeline) != 0 {
		t.Errorf("expected empty pipeline, got %d actions", len(clone.Pipeline))
	}
}

func TestFilterClonePreservesNilParams(t *testing.T) {
	f := Filter{
		Name: "test-nil-params",
		Pipeline: Pipeline{
			{ActionName: "head", Params: nil},
			{ActionName: "keep_lines", Params: map[string]any{"pattern": `\S`}},
		},
	}
	clone := f.Clone()

	// Nil Params must stay nil on clone (not become empty map)
	if clone.Pipeline[0].Params != nil {
		t.Errorf("clone[0].Params = %v, want nil", clone.Pipeline[0].Params)
	}
	// Non-nil Params must be deep-copied
	if clone.Pipeline[1].Params == nil {
		t.Fatal("clone[1].Params is nil, want non-nil copy")
	}
	if clone.Pipeline[1].Params["pattern"] != `\S` {
		t.Errorf("clone[1].Params[pattern] = %v, want \\S", clone.Pipeline[1].Params["pattern"])
	}
	// Clone must not share the params map with original
	originalParams := f.Pipeline[1].Params
	clone.Pipeline[1].Params["pattern"] = "modified"
	if originalParams["pattern"] != `\S` {
		t.Error("clone mutation leaked into original Params map")
	}
}

func TestYAMLBackwardCompatUntaggedFields(t *testing.T) {
	// Bundled filters keep their descriptive `on_error:` lines and the
	// Filter struct has no matching field: yaml.v3 must silently ignore
	// unknown keys (same for `description`, whose field has no yaml tag).
	input := `
name: "backward-compat"
version: 1
description: "This field has no yaml tag anymore"
match:
  command: "echo"
pipeline: []
on_error: "passthrough"
`
	var f Filter
	if err := yaml.Unmarshal([]byte(input), &f); err != nil {
		t.Fatalf("yaml.Unmarshal should succeed even with untagged fields: %v", err)
	}
	if f.Name != "backward-compat" {
		t.Errorf("Name = %q, want backward-compat", f.Name)
	}
	if f.Match.Command != "echo" {
		t.Errorf("Match.Command = %q, want echo", f.Match.Command)
	}
}
