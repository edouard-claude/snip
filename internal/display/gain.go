package display

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/edouard-claude/snip/internal/tracking"
	"github.com/edouard-claude/snip/internal/utils"
)

// RunGain executes the gain (token savings report) command.
func RunGain(tracker *tracking.Tracker, args []string) error {
	if tracker == nil {
		PrintError("no tracking data (run some commands first)")
		return nil
	}

	// Parse args
	showDaily := false
	showJSON := false
	showCSV := false
	historyN := 0
	days := 7

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--daily":
			showDaily = true
		case "--weekly":
			showDaily = true
			days = 7
		case "--monthly":
			showDaily = true
			days = 30
		case "--json":
			showJSON = true
		case "--csv":
			showCSV = true
		case "--history":
			if i+1 < len(args) {
				fmt.Sscanf(args[i+1], "%d", &historyN)
				i++
			} else {
				historyN = 10
			}
		}
	}

	summary, err := tracker.GetSummary()
	if err != nil {
		return fmt.Errorf("get summary: %w", err)
	}

	if showJSON {
		return exportJSON(summary, tracker, days)
	}
	if showCSV {
		return exportCSV(tracker, days)
	}

	if historyN > 0 {
		return showHistory(tracker, historyN)
	}

	if showDaily {
		return showDailyReport(tracker, days, summary)
	}

	// Default: summary view
	printSummary(summary)
	return nil
}

func printSummary(s *tracking.Summary) {
	tty := IsTerminal()

	fmt.Println()
	if tty {
		fmt.Println(HeaderStyle.Render("  snip — Token Savings Report"))
		fmt.Println(DimStyle.Render("  " + FormatSeparator(30)))
	} else {
		fmt.Println("  snip — Token Savings Report")
		fmt.Println("  " + FormatSeparator(30))
	}
	fmt.Println()

	printKPI := func(label, value string) {
		if tty {
			fmt.Printf("  %s  %s\n", DimStyle.Render(fmt.Sprintf("%-20s", label)), StatStyle.Render(value))
		} else {
			fmt.Printf("  %-20s  %s\n", label, value)
		}
	}

	printKPI("Commands filtered", fmt.Sprintf("%d", s.TotalCommands))
	printKPI("Tokens saved", utils.FormatTokens(s.TotalSaved))
	printKPI("Avg savings", fmt.Sprintf("%.1f%%", s.AvgSavings))
	printKPI("Total time", fmt.Sprintf("%.1fs", float64(s.TotalTimeMs)/1000))

	// Efficiency bar
	pct := s.AvgSavings
	if pct < 0 {
		pct = 0
	} else if pct > 100 {
		pct = 100
	}
	barLen := 20
	filled := int(pct / 100 * float64(barLen))
	bar := strings.Repeat("█", filled) + strings.Repeat("░", barLen-filled)
	fmt.Println()
	if tty {
		fmt.Printf("  %s %s\n", SuccessStyle.Render(bar), DimStyle.Render(fmt.Sprintf("%.0f%%", s.AvgSavings)))
	} else {
		fmt.Printf("  %s %.0f%%\n", bar, s.AvgSavings)
	}
	fmt.Println()
}

func showDailyReport(tracker *tracking.Tracker, days int, summary *tracking.Summary) error {
	daily, err := tracker.GetDaily(days)
	if err != nil {
		return err
	}

	printSummary(summary)

	headers := []string{"Date", "Cmds", "Input", "Output", "Saved", "Savings"}
	var rows [][]string
	for _, d := range daily {
		rows = append(rows, []string{
			d.Day,
			fmt.Sprintf("%d", d.Commands),
			utils.FormatTokens(d.InputTokens),
			utils.FormatTokens(d.OutputTokens),
			utils.FormatTokens(d.SavedTokens),
			fmt.Sprintf("%.1f%%", d.AvgSavings),
		})
	}

	fmt.Print(FormatTable(headers, rows))
	return nil
}

func showHistory(tracker *tracking.Tracker, n int) error {
	records, err := tracker.GetRecent(n)
	if err != nil {
		return err
	}

	headers := []string{"Command", "Input", "Output", "Saved", "Time"}
	var rows [][]string
	for _, r := range records {
		cmd := r.OriginalCmd
		if len(cmd) > 30 {
			cmd = cmd[:27] + "..."
		}
		rows = append(rows, []string{
			cmd,
			utils.FormatTokens(r.InputTokens),
			utils.FormatTokens(r.OutputTokens),
			fmt.Sprintf("%.0f%%", r.SavingsPct),
			fmt.Sprintf("%dms", r.ExecTimeMs),
		})
	}

	fmt.Print(FormatTable(headers, rows))
	return nil
}

func exportJSON(summary *tracking.Summary, tracker *tracking.Tracker, days int) error {
	daily, _ := tracker.GetDaily(days)
	data := map[string]any{
		"summary": summary,
		"daily":   daily,
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(data)
}

func exportCSV(tracker *tracking.Tracker, days int) error {
	daily, err := tracker.GetDaily(days)
	if err != nil {
		return err
	}

	w := csv.NewWriter(os.Stdout)
	w.Write([]string{"date", "commands", "input_tokens", "output_tokens", "saved_tokens", "avg_savings"})
	for _, d := range daily {
		w.Write([]string{
			d.Day,
			fmt.Sprintf("%d", d.Commands),
			fmt.Sprintf("%d", d.InputTokens),
			fmt.Sprintf("%d", d.OutputTokens),
			fmt.Sprintf("%d", d.SavedTokens),
			fmt.Sprintf("%.1f", d.AvgSavings),
		})
	}
	w.Flush()
	return w.Error()
}
