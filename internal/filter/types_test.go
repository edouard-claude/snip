package filter

import (
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
on_error: "passthrough"
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
	if f.Match.Subcommand != "log" {
		t.Errorf("match.subcommand = %q", f.Match.Subcommand)
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
	if f.OnError != "passthrough" {
		t.Errorf("on_error = %q", f.OnError)
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
		Match:   Match{Command: "git", Subcommand: "log"},
		OnError: "passthrough",
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
