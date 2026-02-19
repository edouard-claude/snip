package filter

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadUserFilters(t *testing.T) {
	dir := t.TempDir()

	validYAML := `
name: "user-filter"
version: 1
match:
  command: "echo"
pipeline:
  - action: "keep_lines"
    pattern: "\\S"
on_error: "passthrough"
`
	if err := os.WriteFile(filepath.Join(dir, "echo.yaml"), []byte(validYAML), 0644); err != nil {
		t.Fatal(err)
	}

	filters, err := LoadUserFilters(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(filters) != 1 {
		t.Fatalf("got %d filters, want 1", len(filters))
	}
	if filters[0].Name != "user-filter" {
		t.Errorf("name = %q", filters[0].Name)
	}
}

func TestLoadUserFiltersMissingDir(t *testing.T) {
	filters, err := LoadUserFilters("/tmp/nonexistent-snip-filters-test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if filters != nil {
		t.Errorf("expected nil, got %v", filters)
	}
}

func TestLoadUserFiltersSkipsInvalid(t *testing.T) {
	dir := t.TempDir()

	// Invalid filter (no name)
	if err := os.WriteFile(filepath.Join(dir, "bad.yaml"), []byte("pipeline: []"), 0644); err != nil {
		t.Fatal(err)
	}

	filters, err := LoadUserFilters(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(filters) != 0 {
		t.Errorf("expected 0 filters, got %d", len(filters))
	}
}

func TestLoadAllUserOverridesEmbedded(t *testing.T) {
	dir := t.TempDir()

	// Create user filter that would override an embedded one
	userYAML := `
name: "custom"
version: 1
match:
  command: "custom"
pipeline:
  - action: "head"
    n: 5
on_error: "passthrough"
`
	if err := os.WriteFile(filepath.Join(dir, "custom.yaml"), []byte(userYAML), 0644); err != nil {
		t.Fatal(err)
	}

	filters, err := LoadAll(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have user filter
	found := false
	for _, f := range filters {
		if f.Name == "custom" {
			found = true
		}
	}
	if !found {
		t.Error("user filter not found in merged results")
	}
}
