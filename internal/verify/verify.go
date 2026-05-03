package verify

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/edouard-claude/snip/internal/config"
	"github.com/edouard-claude/snip/internal/filter"
)

// TestResult holds the outcome of a single filter test.
type TestResult struct {
	FilterName string
	TestName   string
	Passed     bool
	Expected   string
	Got        string
}

// Summary holds aggregated verify results.
type Summary struct {
	TotalFilters   int
	TestedFilters  int
	TotalTests     int
	Passed         int
	Failed         int
	Results        []TestResult
	UntestdFilters []string // filters with no tests
}

// Run executes the verify command with the given args.
// Returns 0 on success, 1 on failure.
func Run(args []string) int {
	requireAll := false
	for _, arg := range args {
		if arg == "--require-all" {
			requireAll = true
		}
	}

	cfg, err := config.Load()
	if err != nil {
		cfg = config.DefaultConfig()
	}

	filters, err := filter.LoadAll(cfg.Filters.Dirs())
	if err != nil {
		fmt.Fprintf(os.Stderr, "snip verify: load filters: %v\n", err)
		return 1
	}

	summary := RunTests(filters)
	PrintReport(summary)

	if summary.Failed > 0 {
		return 1
	}

	if requireAll && len(summary.UntestdFilters) > 0 {
		fmt.Fprintf(os.Stderr, "\nFAIL: %d filters have no tests\n", len(summary.UntestdFilters))
		return 1
	}

	return 0
}

// RunTests executes all inline tests across the given filters.
func RunTests(filters []filter.Filter) Summary {
	var summary Summary
	summary.TotalFilters = len(filters)

	// Sort filters by name for deterministic output
	sorted := make([]filter.Filter, len(filters))
	copy(sorted, filters)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Name < sorted[j].Name
	})

	for _, f := range sorted {
		if len(f.Tests) == 0 {
			summary.UntestdFilters = append(summary.UntestdFilters, f.Name)
			continue
		}

		summary.TestedFilters++

		for _, tc := range f.Tests {
			result := runSingleTest(&f, tc)
			summary.Results = append(summary.Results, result)
			summary.TotalTests++
			if result.Passed {
				summary.Passed++
			} else {
				summary.Failed++
			}
		}
	}

	return summary
}

// runSingleTest runs one test case against a filter's pipeline.
func runSingleTest(f *filter.Filter, tc filter.FilterTest) TestResult {
	result := TestResult{
		FilterName: f.Name,
		TestName:   tc.Name,
	}

	got, err := ApplyTestPipeline(f, tc.Input)
	if err != nil {
		result.Passed = false
		result.Expected = tc.Expected
		result.Got = fmt.Sprintf("ERROR: %v", err)
		return result
	}

	// Normalize: trim trailing newline for comparison
	gotNorm := strings.TrimRight(got, "\n")
	expectedNorm := strings.TrimRight(tc.Expected, "\n")

	result.Passed = gotNorm == expectedNorm
	result.Expected = expectedNorm
	result.Got = gotNorm
	return result
}

// ApplyTestPipeline runs a filter pipeline on test input, reusing the same
// logic as engine.ApplyPipeline but without the engine dependency.
func ApplyTestPipeline(f *filter.Filter, input string) (string, error) {
	lines := strings.Split(input, "\n")
	// Remove trailing empty line from split (matches engine.ApplyPipeline behavior)
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	// Strip CR from CRLF line endings (matches engine.ApplyPipeline behavior).
	for i, l := range lines {
		if len(l) > 0 && l[len(l)-1] == '\r' {
			lines[i] = l[:len(l)-1]
		}
	}

	ar := filter.ActionResult{
		Lines:    lines,
		Metadata: make(map[string]any),
	}

	for i, action := range f.Pipeline {
		fn, ok := filter.GetAction(action.ActionName)
		if !ok {
			return "", fmt.Errorf("unknown action %q at pipeline[%d]", action.ActionName, i)
		}

		var err error
		ar, err = fn(ar, action.Params)
		if err != nil {
			return "", fmt.Errorf("pipeline[%d] %s: %w", i, action.ActionName, err)
		}
	}

	return strings.Join(ar.Lines, "\n") + "\n", nil
}

// PrintReport prints the verify results to stdout.
func PrintReport(s Summary) {
	fmt.Println("snip verify - filter test results")
	fmt.Println()

	// Group results by filter
	type filterGroup struct {
		name    string
		total   int
		passed  int
		failed  []TestResult
	}

	groups := make(map[string]*filterGroup)
	var order []string

	for _, r := range s.Results {
		g, ok := groups[r.FilterName]
		if !ok {
			g = &filterGroup{name: r.FilterName}
			groups[r.FilterName] = g
			order = append(order, r.FilterName)
		}
		g.total++
		if r.Passed {
			g.passed++
		} else {
			g.failed = append(g.failed, r)
		}
	}

	// Sort by filter name
	sort.Strings(order)

	// Find max filter name length for alignment
	maxNameLen := 0
	for _, name := range order {
		if len(name) > maxNameLen {
			maxNameLen = len(name)
		}
	}

	for _, name := range order {
		g := groups[name]
		dots := strings.Repeat(".", maxNameLen-len(name)+3)
		if g.passed == g.total {
			fmt.Printf("%s %s %d/%d passed\n", name, dots, g.passed, g.total)
		} else {
			fmt.Printf("%s %s FAIL (%d/%d passed)\n", name, dots, g.passed, g.total)
			for _, f := range g.failed {
				fmt.Printf("  test %q: expected %q, got %q\n", f.TestName, f.Expected, f.Got)
			}
		}
	}

	fmt.Printf("\n%d filters, %d tested, %d tests, %d passed, %d failed\n",
		s.TotalFilters, s.TestedFilters, s.TotalTests, s.Passed, s.Failed)
}
