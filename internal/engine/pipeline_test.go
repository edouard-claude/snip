package engine

import (
	"bytes"
	"io"
	"os"
	"runtime"
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

func TestApplyPipelineStripsCRFromCRLFLineEndings(t *testing.T) {
	// Regression: cygwin's ls.exe under cmd.exe emits CRLF line endings.
	// Without CR stripping, "$" anchors do not match (line ends with \r),
	// so dot-entry removal fails, and reformatted lines carry a trailing CR
	// into captured filenames — which corrupts the rendered output. (#44)
	f := &filter.Filter{
		Name: "test",
		Pipeline: filter.Pipeline{
			{ActionName: "remove_lines", Params: map[string]any{"pattern": `\.$`}},
			{ActionName: "replace", Params: map[string]any{
				"pattern":     `^name\s+(\S+)$`,
				"replacement": "[$1]",
			}},
		},
	}

	input := "skip me .\r\nname .mcp.json\r\nname README.md\r\n"
	out, err := ApplyPipeline(f, input)
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if strings.Contains(out, "\r") {
		t.Errorf("output still contains CR: %q", out)
	}
	if strings.Contains(out, "skip me") {
		t.Errorf("dot-entry not removed (anchors broken by CR): %q", out)
	}
	if !strings.Contains(out, "[.mcp.json]") {
		t.Errorf("expected reformatted .mcp.json, got: %q", out)
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

func TestPipelineRunSilentWhenFilterExcludedByFlags(t *testing.T) {
	// p.Run("true", ...) executes the real "true" binary, which doesn't exist on Windows.
	if runtime.GOOS == "windows" {
		t.Skip("skipping: no 'true' command on Windows")
	}

	// Test mechanism: the filter requires --json, but Run() is called with no flags.
	// Therefore Match() returns nil (flag mismatch), yet HasAnyFilter() still returns
	// true (a filter *exists* for "true"). The expected behavior is silence on stderr;
	// before the fix in #36, a misleading "no filter for true" message was printed.
	f := filter.Filter{
		Name:    "true-json",
		Version: 1,
		Match:   filter.Match{Command: "true", RequireFlags: []string{"--json"}},
		OnError: "passthrough",
		Pipeline: filter.Pipeline{
			{ActionName: "keep_lines", Params: map[string]any{"pattern": `.`}},
		},
	}
	reg := filter.NewRegistry([]filter.Filter{f})
	p := &Pipeline{
		Registry:      reg,
		QuietNoFilter: false, // messages enabled - bug would print here
	}

	// Capture stderr by swapping os.Stderr with a pipe.
	// NOTE: this is not safe under t.Parallel() since os.Stderr is global.
	oldStderr := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stderr = w
	t.Cleanup(func() { os.Stderr = oldStderr })

	p.Run("true", []string{})

	_ = w.Close()
	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)

	if strings.Contains(buf.String(), "no filter for") {
		t.Errorf("expected silent stderr when filter exists but excluded by flags, got: %q", buf.String())
	}
}
