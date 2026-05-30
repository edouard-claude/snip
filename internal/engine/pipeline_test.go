package engine

import (
	"bytes"
	"io"
	"os"
	"runtime"
	"strings"
	"testing"

	"github.com/edouard-claude/snip/internal/config"
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

func TestPipelineRunSilentWhenSiblingSubcommandHasFilter(t *testing.T) {
	// Reproduces issue #56: snip has filters for "true:foo" but the user runs
	// "true bar". Before the fix, snip printed the misleading
	// "no filter for true" message. Expected behavior: stay silent — a filter
	// exists for the base command, just not for this subcommand.
	if runtime.GOOS == "windows" {
		t.Skip("skipping: no 'true' command on Windows")
	}

	f := filter.Filter{
		Name:    "true-foo",
		Version: 1,
		Match:   filter.Match{Command: "true", Subcommand: "foo"},
		OnError: "passthrough",
		Pipeline: filter.Pipeline{
			{ActionName: "keep_lines", Params: map[string]any{"pattern": `.`}},
		},
	}
	reg := filter.NewRegistry([]filter.Filter{f})
	p := &Pipeline{
		Registry:      reg,
		QuietNoFilter: false,
	}

	oldStderr := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stderr = w
	t.Cleanup(func() { os.Stderr = oldStderr })

	p.Run("true", []string{"bar"})

	_ = w.Close()
	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)

	if strings.Contains(buf.String(), "no filter for") {
		t.Errorf("expected silent stderr when sibling subcommand filter exists, got: %q", buf.String())
	}
}

func TestApplyOverrideHead(t *testing.T) {
	f := filterForTest("test-filter",
		filter.Pipeline{
			{ActionName: "keep_lines", Params: map[string]any{"pattern": `\S`}},
			{ActionName: "head", Params: map[string]any{"n": 10}},
		},
	)

	o := config.FilterOverride{Head: 25}
	applyOverride(&f, &o)

	if f.Pipeline[1].Params["n"] != 25 {
		t.Errorf("head n = %v, want 25", f.Pipeline[1].Params["n"])
	}
}

func TestApplyOverrideStreamModeFull(t *testing.T) {
	f := filterForTest("test-filter",
		filter.Pipeline{
			{ActionName: "keep_lines", Params: map[string]any{"pattern": `\S`}},
			{ActionName: "head", Params: map[string]any{"n": 10}},
		},
	)

	o := config.FilterOverride{StreamMode: "full"}
	applyOverride(&f, &o)

	if f.Pipeline != nil {
		t.Errorf("pipeline should be nil after stream_mode=full, got %v", f.Pipeline)
	}
}

func TestApplyGlobalLimit(t *testing.T) {
	f := filterForTest("test-filter",
		filter.Pipeline{
			{ActionName: "keep_lines", Params: map[string]any{"pattern": `\S`}},
		},
	)

	origLen := len(f.Pipeline)
	g := config.FilterGlobalConfig{MaxLines: 50, MaxLineLength: 120}
	applyGlobalLimit(&f, &g)

	if len(f.Pipeline) != origLen+2 {
		t.Fatalf("pipeline len = %d, want %d", len(f.Pipeline), origLen+2)
	}
	lastTwo := f.Pipeline[len(f.Pipeline)-2:]
	if lastTwo[0].ActionName != "head" || lastTwo[0].Params["n"] != 50 {
		t.Errorf("expected head(50), got %v", lastTwo[0])
	}
	if lastTwo[1].ActionName != "truncate_lines" || lastTwo[1].Params["max"] != 120 {
		t.Errorf("expected truncate_lines(120), got %v", lastTwo[1])
	}
}

func TestApplyOverrideDoesNotMutateRegistry(t *testing.T) {
	// Verify that Clone() prevents mutations from leaking into the registry.
	original := filterForTest("test-override",
		filter.Pipeline{
			{ActionName: "head", Params: map[string]any{"n": 10}},
			{ActionName: "tail", Params: map[string]any{"n": 5}},
		},
	)

	// Simulate what Pipeline.Run does: match, then clone.
	cloned := original.Clone()

	o := config.FilterOverride{Head: 99, Tail: 88}
	applyOverride(cloned, &o)

	// Original should be unchanged
	if original.Pipeline[0].Params["n"] != 10 {
		t.Errorf("original head n = %v, want 10 (was corrupted)", original.Pipeline[0].Params["n"])
	}
	if original.Pipeline[1].Params["n"] != 5 {
		t.Errorf("original tail n = %v, want 5 (was corrupted)", original.Pipeline[1].Params["n"])
	}
	// Clone should be changed
	if cloned.Pipeline[0].Params["n"] != 99 {
		t.Errorf("cloned head n = %v, want 99", cloned.Pipeline[0].Params["n"])
	}
	if cloned.Pipeline[1].Params["n"] != 88 {
		t.Errorf("cloned tail n = %v, want 88", cloned.Pipeline[1].Params["n"])
	}
}

func TestApplyGlobalLimitDoesNotMutateRegistry(t *testing.T) {
	original := filterForTest("test-global",
		filter.Pipeline{
			{ActionName: "keep_lines", Params: map[string]any{"pattern": `\S`}},
		},
	)

	cloned := original.Clone()
	origLen := len(original.Pipeline)

	g := config.FilterGlobalConfig{MaxLines: 50, MaxLineLength: 120}
	applyGlobalLimit(cloned, &g)

	// Original pipeline should still have same length
	if len(original.Pipeline) != origLen {
		t.Errorf("original pipeline grew from %d to %d (registry corrupted)", origLen, len(original.Pipeline))
	}
	// Clone should have extra actions
	if len(cloned.Pipeline) != origLen+2 {
		t.Errorf("cloned pipeline len = %d, want %d", len(cloned.Pipeline), origLen+2)
	}
}

func TestBypassCommand(t *testing.T) {
	// Test that bypass list sends command through passthrough unconditionally.
	// We use "echo" since it exists everywhere and is test-friendly.
	f := filterForTest("echo", filter.Pipeline{
		{ActionName: "keep_lines", Params: map[string]any{"pattern": `\S`}},
	})
	reg := filter.NewRegistry([]filter.Filter{f})

	cfg := &config.Config{
		Filters: config.FiltersConfig{
			Bypass: config.FilterBypassConfig{
				Commands: []string{"echo"},
			},
		},
	}

	p := &Pipeline{
		Registry: reg,
		Config:   cfg,
	}

	// "echo" is in bypass — should passthrough (raw output, not filtered)
	code := p.Run("echo", []string{"hello bypass!"})
	if code != 0 {
		t.Errorf("bypass exit code = %d, want 0", code)
	}
}

func filterForTest(name string, pipeline filter.Pipeline) filter.Filter {
	return filter.Filter{
		Name:     name,
		Version:  1,
		Match:    filter.Match{Command: name},
		Pipeline: pipeline,
		OnError:  "passthrough",
	}
}

func TestApplyGlobalLimit_MaxOutputBytes(t *testing.T) {
	f := &filter.Filter{Pipeline: filter.Pipeline{}}
	g := &config.FilterGlobalConfig{MaxOutputBytes: 100}

	applyGlobalLimit(f, g)

	if len(f.Pipeline) != 1 {
		t.Fatalf("expected 1 action, got %d", len(f.Pipeline))
	}
	if f.Pipeline[0].ActionName != "truncate_bytes" {
		t.Errorf("action name = %q, want truncate_bytes", f.Pipeline[0].ActionName)
	}
	max, _ := f.Pipeline[0].Params["max"].(int)
	if max != 100 {
		t.Errorf("max = %d, want 100", max)
	}
}

func TestApplyGlobalLimit_ZeroIsNoOp(t *testing.T) {
	f := &filter.Filter{Pipeline: filter.Pipeline{}}
	g := &config.FilterGlobalConfig{} // all zeros

	applyGlobalLimit(f, g)

	if len(f.Pipeline) != 0 {
		t.Errorf("expected 0 actions for zero limits, got %d", len(f.Pipeline))
	}
}

func TestApplyOverrideAppendsWhenActionNotInPipeline(t *testing.T) {
	f := &filter.Filter{
		Pipeline: filter.Pipeline{
			{ActionName: "keep_lines", Params: map[string]any{"pattern": `\S`}},
		},
	}
	o := config.FilterOverride{Head: 25}
	applyOverride(f, &o)

	if len(f.Pipeline) != 2 {
		t.Fatalf("pipeline len = %d, want 2", len(f.Pipeline))
	}
	last := f.Pipeline[len(f.Pipeline)-1]
	if last.ActionName != "head" {
		t.Errorf("last action name = %q, want head", last.ActionName)
	}
	if last.Params["n"] != 25 {
		t.Errorf("head n = %v, want 25", last.Params["n"])
	}
}

func TestApplyOverrideAppendsMultipleUnmatchedActions(t *testing.T) {
	f := &filter.Filter{Pipeline: filter.Pipeline{}}
	o := config.FilterOverride{Head: 25, TruncateLines: 120, KeepLines: "error|warn"}
	applyOverride(f, &o)

	if len(f.Pipeline) != 3 {
		t.Fatalf("pipeline len = %d, want 3", len(f.Pipeline))
	}
	names := map[string]bool{}
	for _, a := range f.Pipeline {
		names[a.ActionName] = true
	}
	for _, want := range []string{"head", "truncate_lines", "keep_lines"} {
		if !names[want] {
			t.Errorf("pipeline missing action %q", want)
		}
	}
}

func TestApplyOverrideDoesNotDuplicateExistingActions(t *testing.T) {
	f := &filter.Filter{
		Pipeline: filter.Pipeline{
			{ActionName: "keep_lines", Params: map[string]any{"pattern": `\S`}},
			{ActionName: "head", Params: map[string]any{"n": 10}},
		},
	}
	o := config.FilterOverride{Head: 25}
	applyOverride(f, &o)

	if len(f.Pipeline) != 2 {
		t.Fatalf("pipeline len = %d, want 2 (no duplicate)", len(f.Pipeline))
	}
	if f.Pipeline[1].Params["n"] != 25 {
		t.Errorf("head n = %v, want 25 (updated in place)", f.Pipeline[1].Params["n"])
	}
}

func TestApplyOverrideZeroValueIsNoOp(t *testing.T) {
	f := &filter.Filter{
		Pipeline: filter.Pipeline{
			{ActionName: "head", Params: map[string]any{"n": 10}},
		},
	}
	o := config.FilterOverride{Head: 0}
	applyOverride(f, &o)

	if len(f.Pipeline) != 1 {
		t.Fatalf("pipeline len = %d, want 1", len(f.Pipeline))
	}
	if f.Pipeline[0].Params["n"] != 10 {
		t.Errorf("head n = %v, want 10 unchanged", f.Pipeline[0].Params["n"])
	}
}

func TestApplyOverrideAppendPreservesCloneIsolation(t *testing.T) {
	original := &filter.Filter{
		Pipeline: filter.Pipeline{
			{ActionName: "keep_lines", Params: map[string]any{"pattern": `\S`}},
		},
	}
	clone := original.Clone()
	o := config.FilterOverride{Head: 25}
	applyOverride(clone, &o)

	if len(original.Pipeline) != 1 {
		t.Errorf("original pipeline len = %d, want 1 (should not be mutated)", len(original.Pipeline))
	}
	if len(clone.Pipeline) != 2 {
		t.Errorf("clone pipeline len = %d, want 2", len(clone.Pipeline))
	}
}
