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
