package filter

import (
	"testing"
)

func TestParseFilterValid(t *testing.T) {
	yaml := `
name: "test"
version: 1
description: "test filter"
match:
  command: "echo"
pipeline:
  - action: "keep_lines"
    pattern: "\\S"
on_error: "passthrough"
`
	f, err := ParseFilter([]byte(yaml))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if f.Name != "test" {
		t.Errorf("name = %q", f.Name)
	}
	if f.Match.Command != "echo" {
		t.Errorf("match.command = %q", f.Match.Command)
	}
}

func TestParseFilterMissingName(t *testing.T) {
	yaml := `
match:
  command: "echo"
pipeline: []
`
	_, err := ParseFilter([]byte(yaml))
	if err == nil {
		t.Fatal("expected error for missing name")
	}
}

func TestParseFilterMissingCommand(t *testing.T) {
	yaml := `
name: "test"
pipeline: []
`
	_, err := ParseFilter([]byte(yaml))
	if err == nil {
		t.Fatal("expected error for missing command")
	}
}

func TestParseFilterUnknownAction(t *testing.T) {
	yaml := `
name: "test"
match:
  command: "echo"
pipeline:
  - action: "nonexistent_action"
`
	_, err := ParseFilter([]byte(yaml))
	if err == nil {
		t.Fatal("expected error for unknown action")
	}
}

func TestParseFilterInvalidYAML(t *testing.T) {
	_, err := ParseFilter([]byte("}{invalid"))
	if err == nil {
		t.Fatal("expected error for invalid YAML")
	}
}
