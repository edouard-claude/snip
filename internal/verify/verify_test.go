package verify

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/edouard-claude/snip/internal/filter"
)

func TestRunTestsPassingFilter(t *testing.T) {
	filters := []filter.Filter{
		{
			Name: "test-filter",
			Pipeline: filter.Pipeline{
				{ActionName: "remove_lines", Params: map[string]any{"pattern": `^DEBUG`}},
			},
			Tests: []filter.FilterTest{
				{
					Name:     "removes debug lines",
					Input:    "DEBUG: foo\nINFO: bar\nDEBUG: baz\n",
					Expected: "INFO: bar\n",
				},
			},
		},
	}

	summary := RunTests(filters)

	if summary.TotalFilters != 1 {
		t.Errorf("TotalFilters: got %d, want 1", summary.TotalFilters)
	}
	if summary.TestedFilters != 1 {
		t.Errorf("TestedFilters: got %d, want 1", summary.TestedFilters)
	}
	if summary.TotalTests != 1 {
		t.Errorf("TotalTests: got %d, want 1", summary.TotalTests)
	}
	if summary.Passed != 1 {
		t.Errorf("Passed: got %d, want 1", summary.Passed)
	}
	if summary.Failed != 0 {
		t.Errorf("Failed: got %d, want 0", summary.Failed)
	}
}

func TestRunTestsFailingFilter(t *testing.T) {
	filters := []filter.Filter{
		{
			Name: "fail-filter",
			Pipeline: filter.Pipeline{
				{ActionName: "remove_lines", Params: map[string]any{"pattern": `^DEBUG`}},
			},
			Tests: []filter.FilterTest{
				{
					Name:     "wrong expectation",
					Input:    "DEBUG: foo\nINFO: bar\n",
					Expected: "DEBUG: foo\nINFO: bar\n",
				},
			},
		},
	}

	summary := RunTests(filters)

	if summary.Passed != 0 {
		t.Errorf("Passed: got %d, want 0", summary.Passed)
	}
	if summary.Failed != 1 {
		t.Errorf("Failed: got %d, want 1", summary.Failed)
	}
	if len(summary.Results) != 1 {
		t.Fatalf("Results: got %d, want 1", len(summary.Results))
	}
	if summary.Results[0].Passed {
		t.Error("expected test to fail")
	}
}

func TestRunTestsFilterWithNoTests(t *testing.T) {
	filters := []filter.Filter{
		{
			Name: "no-tests",
			Pipeline: filter.Pipeline{
				{ActionName: "strip_ansi"},
			},
		},
	}

	summary := RunTests(filters)

	if summary.TotalFilters != 1 {
		t.Errorf("TotalFilters: got %d, want 1", summary.TotalFilters)
	}
	if summary.TestedFilters != 0 {
		t.Errorf("TestedFilters: got %d, want 0", summary.TestedFilters)
	}
	if summary.TotalTests != 0 {
		t.Errorf("TotalTests: got %d, want 0", summary.TotalTests)
	}
	if len(summary.UntestdFilters) != 1 {
		t.Errorf("UntestdFilters: got %d, want 1", len(summary.UntestdFilters))
	}
	if summary.UntestdFilters[0] != "no-tests" {
		t.Errorf("UntestdFilters[0]: got %q, want %q", summary.UntestdFilters[0], "no-tests")
	}
}

func TestRunTestsRequireAllBehavior(t *testing.T) {
	// RunTests itself does not enforce --require-all; it just reports.
	// The Run function checks UntestdFilters. We test the data here.
	filters := []filter.Filter{
		{
			Name: "tested",
			Pipeline: filter.Pipeline{
				{ActionName: "on_empty", Params: map[string]any{"message": "ok"}},
			},
			Tests: []filter.FilterTest{
				{Name: "empty returns ok", Input: "", Expected: "ok\n"},
			},
		},
		{
			Name:     "untested",
			Pipeline: filter.Pipeline{{ActionName: "strip_ansi"}},
		},
	}

	summary := RunTests(filters)

	if summary.TestedFilters != 1 {
		t.Errorf("TestedFilters: got %d, want 1", summary.TestedFilters)
	}
	if len(summary.UntestdFilters) != 1 {
		t.Errorf("UntestdFilters: got %d, want 1", len(summary.UntestdFilters))
	}
}

func TestRunTestsMultipleTestsPerFilter(t *testing.T) {
	filters := []filter.Filter{
		{
			Name: "multi",
			Pipeline: filter.Pipeline{
				{ActionName: "remove_lines", Params: map[string]any{"pattern": `^#`}},
				{ActionName: "on_empty", Params: map[string]any{"message": "ok"}},
			},
			Tests: []filter.FilterTest{
				{
					Name:     "removes comments",
					Input:    "# comment\ncode line\n",
					Expected: "code line\n",
				},
				{
					Name:     "all comments returns ok",
					Input:    "# comment\n",
					Expected: "ok\n",
				},
			},
		},
	}

	summary := RunTests(filters)

	if summary.TotalTests != 2 {
		t.Errorf("TotalTests: got %d, want 2", summary.TotalTests)
	}
	if summary.Passed != 2 {
		t.Errorf("Passed: got %d, want 2", summary.Passed)
	}
}

func TestApplyTestPipelineChained(t *testing.T) {
	f := &filter.Filter{
		Name: "test",
		Pipeline: filter.Pipeline{
			{ActionName: "remove_lines", Params: map[string]any{"pattern": `^$`}},
			{ActionName: "head", Params: map[string]any{"n": 2}},
		},
	}

	got, err := ApplyTestPipeline(f, "a\n\nb\n\nc\nd\n")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// After remove_lines: a, b, c, d
	// After head(2): a, b, +2 more lines
	expected := "a\nb\n+2 more lines\n"
	if got != expected {
		t.Errorf("got %q, want %q", got, expected)
	}
}

func TestApplyTestPipelineUnknownAction(t *testing.T) {
	f := &filter.Filter{
		Name: "test",
		Pipeline: filter.Pipeline{
			{ActionName: "nonexistent"},
		},
	}

	_, err := ApplyTestPipeline(f, "input")
	if err == nil {
		t.Fatal("expected error for unknown action")
	}
}

func TestRunTestsPipelineError(t *testing.T) {
	filters := []filter.Filter{
		{
			Name: "bad-pipeline",
			Pipeline: filter.Pipeline{
				{ActionName: "keep_lines", Params: map[string]any{}}, // missing pattern
			},
			Tests: []filter.FilterTest{
				{Name: "triggers error", Input: "hello\n", Expected: "hello\n"},
			},
		},
	}

	summary := RunTests(filters)

	if summary.Failed != 1 {
		t.Errorf("Failed: got %d, want 1", summary.Failed)
	}
	if summary.Results[0].Passed {
		t.Error("expected test to fail on pipeline error")
	}
	if summary.Results[0].Got == "" {
		t.Error("expected Got to contain error message")
	}
}

func TestPrintReportAllPassed(t *testing.T) {
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w

	summary := Summary{
		TotalFilters:  2,
		TestedFilters: 2,
		TotalTests:    3,
		Passed:        3,
		Failed:        0,
		Results: []TestResult{
			{FilterName: "filter-a", TestName: "test1", Passed: true},
			{FilterName: "filter-a", TestName: "test2", Passed: true},
			{FilterName: "filter-b", TestName: "test1", Passed: true},
		},
	}

	PrintReport(summary)

	w.Close()
	var buf bytes.Buffer
	io.Copy(&buf, r)
	os.Stdout = old

	output := buf.String()
	if !strings.Contains(output, "filter-a") {
		t.Error("expected output to contain filter-a")
	}
	if !strings.Contains(output, "filter-b") {
		t.Error("expected output to contain filter-b")
	}
	if !strings.Contains(output, "2/2 passed") {
		t.Error("expected '2/2 passed' in output")
	}
	if !strings.Contains(output, "3 passed") {
		t.Error("expected '3 passed' in summary line")
	}
}

func TestPrintReportWithFailures(t *testing.T) {
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w

	summary := Summary{
		TotalFilters:   2,
		TestedFilters:  1,
		TotalTests:     2,
		Passed:         1,
		Failed:         1,
		UntestdFilters: []string{"untested-filter"},
		Results: []TestResult{
			{FilterName: "my-filter", TestName: "good-test", Passed: true},
			{FilterName: "my-filter", TestName: "bad-test", Passed: false, Expected: "expected-out", Got: "actual-out"},
		},
	}

	PrintReport(summary)

	w.Close()
	var buf bytes.Buffer
	io.Copy(&buf, r)
	os.Stdout = old

	output := buf.String()
	if !strings.Contains(output, "FAIL") {
		t.Error("expected FAIL in output")
	}
	if !strings.Contains(output, "1/2 passed") {
		t.Error("expected 1/2 passed in output")
	}
	if !strings.Contains(output, "expected-out") {
		t.Error("expected 'expected-out' in failure details")
	}
	if !strings.Contains(output, "actual-out") {
		t.Error("expected 'actual-out' in failure details")
	}
}

func TestPrintReportEmpty(t *testing.T) {
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w

	summary := Summary{}
	PrintReport(summary)

	w.Close()
	var buf bytes.Buffer
	io.Copy(&buf, r)
	os.Stdout = old

	output := buf.String()
	if !strings.Contains(output, "0 filters") {
		t.Error("expected '0 filters' in output")
	}
}

func TestRunNoArgs(t *testing.T) {
	// Set HOME to a temp dir so config.Load() doesn't find any user config.
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	// Capture stdout.
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	exitCode := Run(nil)

	w.Close()
	var buf bytes.Buffer
	io.Copy(&buf, r)
	os.Stdout = old

	if exitCode != 0 {
		t.Errorf("expected exit 0 for default run, got %d", exitCode)
	}
	output := buf.String()
	if !strings.Contains(output, "snip verify") {
		t.Error("expected output to contain 'snip verify'")
	}
}

func TestRunRequireAll(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	exitCode := Run([]string{"--require-all"})
	if exitCode != 0 {
		t.Errorf("expected exit 0 for --require-all when loading succeeds, got %d", exitCode)
	}
}
