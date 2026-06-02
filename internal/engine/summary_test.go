package engine

import (
	"strings"
	"testing"

	"github.com/edouard-claude/snip/internal/utils"
)

func TestBuildSummaryLineInjectionOnly(t *testing.T) {
	got := BuildSummaryLine(SummaryInfo{
		FilterName:    "git-status",
		FilterVersion: 2,
		InjectedArgs:  []string{"--porcelain"},
	})
	want := "[snip: git-status v2 | +--porcelain]"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestBuildSummaryLinePipelineOnly(t *testing.T) {
	got := BuildSummaryLine(SummaryInfo{
		FilterName:    "find",
		FilterVersion: 1,
		PipelineNames: []string{"compact_path", "head"},
	})
	want := "[snip: find v1 | compact_path>head]"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestBuildSummaryLineBoth(t *testing.T) {
	got := BuildSummaryLine(SummaryInfo{
		FilterName:    "git-log",
		FilterVersion: 1,
		InjectedArgs:  []string{"--no-merges"},
		PipelineNames: []string{"keep_lines", "truncate_lines", "format_template"},
	})
	want := "[snip: git-log v1 | +--no-merges | keep_lines>truncate_lines>format_template]"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestBuildSummaryLineEmpty(t *testing.T) {
	got := BuildSummaryLine(SummaryInfo{
		FilterName:    "noop",
		FilterVersion: 1,
	})
	if got != "" {
		t.Errorf("expected empty string, got %q", got)
	}
}

func TestBuildSummaryLineLongArgTruncated(t *testing.T) {
	got := BuildSummaryLine(SummaryInfo{
		FilterName:    "git-log",
		FilterVersion: 1,
		InjectedArgs:  []string{"--pretty=format:%h %s (%ar) <%an>"},
	})
	want := "[snip: git-log v1 | +--pretty=format:%...]"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestBuildSummaryLineMultipleArgs(t *testing.T) {
	got := BuildSummaryLine(SummaryInfo{
		FilterName:    "git-log",
		FilterVersion: 1,
		InjectedArgs:  []string{"--no-merges", "-n", "10"},
		PipelineNames: []string{"keep_lines"},
	})
	want := "[snip: git-log v1 | +--no-merges +-n +10 | keep_lines]"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestApplySummaryEmptySkipped(t *testing.T) {
	filtered := "line1\nline2\nline3\n"
	got := ApplySummary(filtered, "")
	if got != filtered {
		t.Errorf("expected unchanged output, got %q", got)
	}
}

func TestApplySummarySingleLineSkipped(t *testing.T) {
	filtered := "line1\n"
	got := ApplySummary(filtered, "[snip: test v1 | +--flag]")
	if got != filtered {
		t.Errorf("expected unchanged output, got %q", got)
	}
}

func TestApplySummaryTwoLinesSkipped(t *testing.T) {
	filtered := "line1\nline2\n"
	got := ApplySummary(filtered, "[snip: test v1 | +--flag]")
	if got != filtered {
		t.Errorf("expected unchanged output, got %q", got)
	}
}

func TestApplySummarySufficientOutput(t *testing.T) {
	filtered := "line1\nline2\nline3\nline4\nline5\nline6\nline7\nline8\nline9\nline10\n"
	summary := "[snip: test v1 | +--x]"
	got := ApplySummary(filtered, summary)
	if !strings.HasPrefix(got, summary+"\n") {
		t.Errorf("expected output to start with summary, got %q", got)
	}
}

func TestApplySummaryLargerThanOutputSkipped(t *testing.T) {
	filtered := "ab\ncd\nef\n"
	summary := "[snip: very-long-filter-name-here v999 | +--extremely-long-flag | action1>action2>action3>action4]"
	got := ApplySummary(filtered, summary)
	if got != filtered {
		t.Errorf("expected unchanged output, got %q", got)
	}
}

func TestApplySummaryTokenNeutral(t *testing.T) {
	inputs := []string{
		"line1\nline2\nline3\nline4\nline5\nline6\nline7\nline8\nline9\nline10\n",
		strings.Repeat("a long line with content\n", 20),
		"short\nmedium length line\na very long line that has quite a lot of content in it\nfourth\nfifth\nsixth\n",
	}
	summaries := []string{
		"[snip: git-status v2 | +--porcelain | keep_lines>group_by]",
		"[snip: test v1 | +--flag]",
		"[snip: go-test v3 | +-json | keep_lines>aggregate>format_template]",
	}

	for _, filtered := range inputs {
		for _, summary := range summaries {
			result := ApplySummary(filtered, summary)
			originalTokens := utils.EstimateTokens(filtered)
			resultTokens := utils.EstimateTokens(result)

			if resultTokens > originalTokens {
				t.Errorf("token neutrality violated: original=%d, result=%d, summary=%q, filtered=%q",
					originalTokens, resultTokens, summary, filtered[:min(50, len(filtered))])
			}
		}
	}
}

func TestComputeInjectedArgsBasic(t *testing.T) {
	got := ComputeInjectedArgs([]string{"status"}, []string{"status", "--porcelain"})
	if len(got) != 1 || got[0] != "--porcelain" {
		t.Errorf("got %v, want [--porcelain]", got)
	}
}

func TestComputeInjectedArgsNoChange(t *testing.T) {
	got := ComputeInjectedArgs([]string{"log", "--oneline"}, []string{"log", "--oneline"})
	if len(got) != 0 {
		t.Errorf("got %v, want empty", got)
	}
}

func TestComputeInjectedArgsDefaults(t *testing.T) {
	got := ComputeInjectedArgs([]string{"log"}, []string{"log", "-n", "10", "--no-merges"})
	want := []string{"-n", "10", "--no-merges"}
	if len(got) != len(want) {
		t.Errorf("got %v, want %v", got, want)
		return
	}
	for i := range got {
		if got[i] != want[i] {
			t.Errorf("got[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestComputeInjectedArgsDuplicatePreserved(t *testing.T) {
	got := ComputeInjectedArgs([]string{"diff", "--stat"}, []string{"diff", "--stat"})
	if len(got) != 0 {
		t.Errorf("got %v, want empty", got)
	}
}
