package engine

import (
	"strings"
	"testing"

	"github.com/edouard-claude/snip/internal/filter"
)

func TestApplyPipelineKeepLines(t *testing.T) {
	f := &filter.Filter{
		Name: "test",
		Pipeline: filter.Pipeline{
			{ActionName: "keep_lines", Params: map[string]any{"pattern": `\S`}},
		},
	}

	input := "hello\n\nworld\n\n"
	out, err := ApplyPipeline(f, input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) != 2 {
		t.Errorf("got %d lines, want 2: %v", len(lines), lines)
	}
}

func TestApplyPipelineChained(t *testing.T) {
	f := &filter.Filter{
		Name: "test",
		Pipeline: filter.Pipeline{
			{ActionName: "keep_lines", Params: map[string]any{"pattern": `\S`}},
			{ActionName: "head", Params: map[string]any{"n": 2}},
		},
	}

	input := "a\nb\nc\nd\ne\n"
	out, err := ApplyPipeline(f, input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) != 3 { // 2 kept + overflow msg
		t.Errorf("got %d lines: %v", len(lines), lines)
	}
}

func TestApplyPipelineUnknownAction(t *testing.T) {
	f := &filter.Filter{
		Name: "test",
		Pipeline: filter.Pipeline{
			{ActionName: "nonexistent"},
		},
	}

	_, err := ApplyPipeline(f, "input")
	if err == nil {
		t.Fatal("expected error for unknown action")
	}
}

func TestApplyPipelineEmptyInput(t *testing.T) {
	f := &filter.Filter{
		Name: "test",
		Pipeline: filter.Pipeline{
			{ActionName: "keep_lines", Params: map[string]any{"pattern": `\S`}},
		},
	}

	out, err := ApplyPipeline(f, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.TrimSpace(out) != "" {
		t.Errorf("expected empty output, got %q", out)
	}
}

func TestApplyPipelineGracefulDegradation(t *testing.T) {
	f := &filter.Filter{
		Name: "test",
		Pipeline: filter.Pipeline{
			{ActionName: "keep_lines", Params: map[string]any{}}, // Missing pattern
		},
	}

	_, err := ApplyPipeline(f, "hello\nworld\n")
	if err == nil {
		t.Fatal("expected error for missing pattern")
	}
}

func TestIsFilterEnabledNilMap(t *testing.T) {
	p := &Pipeline{FilterEnabled: nil}
	for _, name := range []string{"git-diff", "git-status", "unknown"} {
		if !p.isFilterEnabled(name) {
			t.Errorf("nil map: expected %q enabled", name)
		}
	}
}

func TestIsFilterEnabledExplicit(t *testing.T) {
	p := &Pipeline{FilterEnabled: map[string]bool{
		"git-diff":   false,
		"git-status": true,
	}}
	if p.isFilterEnabled("git-diff") {
		t.Error("expected git-diff disabled")
	}
	if !p.isFilterEnabled("git-status") {
		t.Error("expected git-status enabled")
	}
	if !p.isFilterEnabled("unknown") {
		t.Error("expected unlisted filter enabled by default")
	}
}

func TestIsFilterEnabledEmptyMap(t *testing.T) {
	p := &Pipeline{FilterEnabled: map[string]bool{}}
	for _, name := range []string{"git-diff", "git-status", "unknown"} {
		if !p.isFilterEnabled(name) {
			t.Errorf("empty map: expected %q enabled", name)
		}
	}
}

func TestBuildPipelineInputDefault(t *testing.T) {
	f := &filter.Filter{Name: "test"}
	r := &Result{Stdout: "out\n", Stderr: "err\n"}
	got := buildPipelineInput(f, r)
	if got != "out\n" {
		t.Errorf("default streams: got %q, want %q", got, "out\n")
	}
}

func TestBuildPipelineInputStderrOnly(t *testing.T) {
	f := &filter.Filter{Name: "test", Streams: []string{"stderr"}}
	r := &Result{Stdout: "out\n", Stderr: "err\n"}
	got := buildPipelineInput(f, r)
	if got != "err\n" {
		t.Errorf("stderr only: got %q, want %q", got, "err\n")
	}
}

func TestBuildPipelineInputBoth(t *testing.T) {
	f := &filter.Filter{Name: "test", Streams: []string{"stdout", "stderr"}}
	r := &Result{Stdout: "out\n", Stderr: "err\n"}
	got := buildPipelineInput(f, r)
	if got != "out\nerr\n" {
		t.Errorf("both streams: got %q, want %q", got, "out\nerr\n")
	}
}
