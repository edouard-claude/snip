package inspect

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
)

func Run(args []string) int {
	useJSON := false
	checkAll := false
	for _, a := range args {
		switch a {
		case "--json":
			useJSON = true
		case "--all":
			checkAll = true
		case "--help", "-h":
			printUsage()
			return 0
		}
	}

	root, err := resolveRoot()
	if err != nil {
		fmt.Fprintf(os.Stderr, "snip inspect: %v\n", err)
		return 1
	}

	checkers := []Checker{}

	runDead := checkAll
	runAppend := checkAll
	for _, a := range args {
		switch a {
		case "--dead-fields":
			runDead = true
		case "--append-safety":
			runAppend = true
		}
	}

	if !runDead && !runAppend {
		printUsage()
		return 0
	}

	if runDead {
		checkers = append(checkers, DeadFieldChecker{})
	}
	if runAppend {
		checkers = append(checkers, AppendSafetyChecker{})
	}

	var allFindings []Finding
	for _, c := range checkers {
		findings, err := c.Run(root)
		if err != nil {
			fmt.Fprintf(os.Stderr, "snip inspect: %s: %v\n", c.Name(), err)
			return 1
		}
		allFindings = append(allFindings, findings...)
	}

	sort.Slice(allFindings, func(i, j int) bool {
		if allFindings[i].File != allFindings[j].File {
			return allFindings[i].File < allFindings[j].File
		}
		return allFindings[i].Line < allFindings[j].Line
	})

	hasRisky := false
	for _, f := range allFindings {
		if f.Level == "risky" || f.Level == "dead" {
			hasRisky = true
			break
		}
	}

	if useJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		enc.Encode(allFindings)
		if hasRisky {
			return 1
		}
		return 0
	}

	deadFields := filterFindings(allFindings, "dead-field")
	appendIssues := filterFindings(allFindings, "append-safety")

	if len(deadFields) > 0 {
		fmt.Println("--- Dead Fields ---")
		for _, f := range deadFields {
			short := shortenPath(root, f.File)
			fmt.Printf("  DEAD: %s:%d  %s (%s)\n", short, f.Line, f.Field, f.Message)
		}
		fmt.Printf("  Found: %d dead fields\n\n", len(deadFields))
	}

	if len(appendIssues) > 0 {
		fmt.Println("--- Append Safety ---")
		risky := 0
		for _, f := range appendIssues {
			short := shortenPath(root, f.File)
			tag := f.Level
			if f.Level == "risky" {
				tag = "RISKY"
				risky++
			} else {
				tag = "SAFE"
			}
			fmt.Printf("  %s: %s:%d  %s\n", tag, short, f.Line, f.Context)
		}
		fmt.Printf("  Found: %d risky / %d total\n\n", risky, len(appendIssues))
	}

	if len(allFindings) == 0 {
		fmt.Println("No issues found.")
	}

	if hasRisky {
		return 1
	}
	return 0
}

func printUsage() {
	fmt.Println("snip inspect: code quality checks for snip's own source")
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  snip inspect --dead-fields     find tagged struct fields never read")
	fmt.Println("  snip inspect --append-safety   find shared-state append() calls")
	fmt.Println("  snip inspect --all             run both checks")
	fmt.Println("  snip inspect --json            output as JSON (for CI)")
	fmt.Println("  snip inspect --help            show this help")
}

func filterFindings(all []Finding, category string) []Finding {
	var out []Finding
	for _, f := range all {
		if f.Category == category {
			out = append(out, f)
		}
	}
	return out
}

func shortenPath(root, full string) string {
	root = strings.TrimRight(root, "/")
	if strings.HasPrefix(full, root+"/") {
		return full[len(root)+1:]
	}
	return full
}
