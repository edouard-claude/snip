package engine

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/edouard-claude/snip/internal/filter"
)

func TestQuietNoFilter(t *testing.T) {
	registry := filter.NewRegistry([]filter.Filter{})
	
	p := &Pipeline{
		Registry:      registry,
		QuietNoFilter: true,
	}
	
	// Capture stderr
	oldStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w
	
	code := p.Run("test-command", []string{})
	
	w.Close()
	os.Stderr = oldStderr
	
	var buf bytes.Buffer
	buf.ReadFrom(r)
	
	// Should return exit code 1 and no stderr output
	if code != 1 {
		t.Errorf("expected exit code 1, got %d", code)
	}
	
	if buf.String() != "" {
		t.Errorf("expected no stderr output, got %q", buf.String())
	}
}

func TestQuietNoFilterFalse(t *testing.T) {
	registry := filter.NewRegistry([]filter.Filter{})
	
	p := &Pipeline{
		Registry:      registry,
		QuietNoFilter: false,
	}
	
	// Capture stderr
	oldStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w
	
	code := p.Run("test-command", []string{})
	
	w.Close()
	os.Stderr = oldStderr
	
	var buf bytes.Buffer
	buf.ReadFrom(r)
	
	// Should return exit code 1 and stderr output
	if code != 1 {
		t.Errorf("expected exit code 1, got %d", code)
	}
	
	expected := "snip: no filter for \"test-command\", passing through — you can run \"test-command\" directly\n"
	if buf.String() != expected {
		t.Errorf("expected stderr %q, got %q", expected, buf.String())
	}
}

func TestFilterDisabled(t *testing.T) {
	f := filter.Filter{
		Name: "test-filter",
		Match: filter.Match{
			Command:    "test-command",
			Subcommand: "sub",
		},
		Pipeline: filter.Pipeline{
			{ActionName: "keep_lines", Params: map[string]any{"pattern": `\S`}},
		},
	}

	registry := filter.NewRegistry([]filter.Filter{f})

	p := &Pipeline{
		Registry:      registry,
		FilterEnabled:  map[string]bool{"test-filter": false},
		QuietNoFilter:  false,
	}

	// Capture stderr
	oldStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	code := p.Run("test-command", []string{"sub"})

	w.Close()
	os.Stderr = oldStderr

	var buf bytes.Buffer
	buf.ReadFrom(r)

	// Should return exit code 1 (since test-command doesn't exist) and stderr message
	if code != 1 {
		t.Errorf("expected exit code 1, got %d", code)
	}

	expected := "snip: no filter for \"test-command\", passing through — you can run \"test-command\" directly\n"
	if !strings.Contains(buf.String(), expected) {
		t.Errorf("expected stderr to contain %q, got %q", expected, buf.String())
	}
}

func TestFilterEnabledExplicit(t *testing.T) {
	f := filter.Filter{
		Name: "git-diff",
		Match: filter.Match{
			Command:    "git",
			Subcommand: "diff",
		},
		Pipeline: filter.Pipeline{
			{ActionName: "keep_lines", Params: map[string]any{"pattern": `\S`}},
		},
	}

	registry := filter.NewRegistry([]filter.Filter{f})
	
	p := &Pipeline{
		Registry:      registry,
		FilterEnabled: map[string]bool{"git-diff": true},
	}
	
	// Test that filter would normally run - we'll simulate a successful command
	// by checking that the filter path is taken (which would normally execute git)
	// Since we can't easily mock Execute() without more refactoring, we'll just
	// verify that the filter isn't disabled
	if !p.isFilterEnabled("git-diff") {
		t.Error("expected git-diff filter to be enabled")
	}
}

func TestFilterEnabledMissing(t *testing.T) {
	p := &Pipeline{
		FilterEnabled: map[string]bool{"other-filter": true},
	}
	
	if !p.isFilterEnabled("git-diff") {
		t.Error("expected git-diff filter to be enabled when not in map")
	}
}

func TestFilterEnabledNil(t *testing.T) {
	p := &Pipeline{}
	
	if !p.isFilterEnabled("git-diff") {
		t.Error("expected git-diff filter to be enabled when FilterEnabled is nil")
	}
}