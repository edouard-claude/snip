package engine

import (
	"fmt"
	"strings"

	"github.com/edouard-claude/snip/internal/utils"
)

// SummaryInfo holds the data needed to build a summary line.
type SummaryInfo struct {
	FilterName    string
	FilterVersion int
	InjectedArgs  []string
	PipelineNames []string
}

const maxArgDisplayLen = 20
const minLinesForSummary = 3

// BuildSummaryLine constructs a compact summary string from filter metadata.
func BuildSummaryLine(info SummaryInfo) string {
	if len(info.InjectedArgs) == 0 && len(info.PipelineNames) == 0 {
		return ""
	}

	var b strings.Builder
	fmt.Fprintf(&b, "[snip: %s v%d", info.FilterName, info.FilterVersion)

	if len(info.InjectedArgs) > 0 {
		b.WriteString(" | ")
		for i, arg := range info.InjectedArgs {
			if i > 0 {
				b.WriteByte(' ')
			}
			b.WriteByte('+')
			if len(arg) > maxArgDisplayLen {
				b.WriteString(arg[:maxArgDisplayLen-3])
				b.WriteString("...")
			} else {
				b.WriteString(arg)
			}
		}
	}

	if len(info.PipelineNames) > 0 {
		b.WriteString(" | ")
		b.WriteString(strings.Join(info.PipelineNames, ">"))
	}

	b.WriteByte(']')
	return b.String()
}

// ApplySummary prepends a summary line to filtered output while maintaining
// token neutrality. Returns the original output unchanged if the summary
// cannot fit within the token budget.
func ApplySummary(filtered string, summary string) string {
	if summary == "" {
		return filtered
	}

	lines := strings.Split(strings.TrimRight(filtered, "\n"), "\n")
	if len(lines) < minLinesForSummary {
		return filtered
	}

	summaryWithNewline := summary + "\n"
	summaryTokens := utils.EstimateTokens(summaryWithNewline)
	filteredTokens := utils.EstimateTokens(filtered)

	if summaryTokens >= filteredTokens {
		return filtered
	}

	// Trim trailing lines until the summary fits within the original budget
	remaining := lines
	remainingStr := strings.Join(remaining, "\n") + "\n"
	for len(remaining) > 1 && utils.EstimateTokens(remainingStr)+summaryTokens > filteredTokens {
		remaining = remaining[:len(remaining)-1]
		remainingStr = strings.Join(remaining, "\n") + "\n"
	}

	if utils.EstimateTokens(remainingStr)+summaryTokens > filteredTokens {
		return filtered
	}

	return summaryWithNewline + remainingStr
}

// ComputeInjectedArgs returns args present in finalArgs but not in fullArgs.
func ComputeInjectedArgs(fullArgs, finalArgs []string) []string {
	original := make(map[string]int, len(fullArgs))
	for _, a := range fullArgs {
		original[a]++
	}

	var injected []string
	seen := make(map[string]int, len(finalArgs))
	for _, a := range finalArgs {
		seen[a]++
		if seen[a] > original[a] {
			injected = append(injected, a)
		}
	}
	return injected
}
