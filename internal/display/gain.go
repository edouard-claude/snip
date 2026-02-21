package display

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"

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
	var (
		showDaily   bool
		showWeekly  bool
		showMonthly bool
		showJSON    bool
		showCSV     bool
		showTop     bool
		historyN    int
		topN        int
		days        = 7
	)

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--daily":
			showDaily = true
		case "--weekly":
			showWeekly = true
		case "--monthly":
			showMonthly = true
		case "--json":
			showJSON = true
		case "--csv":
			showCSV = true
		case "--top":
			showTop = true
			if i+1 < len(args) {
				_, _ = fmt.Sscanf(args[i+1], "%d", &topN)
				i++
			}
			if topN <= 0 {
				topN = 10
			}
		case "--history":
			if i+1 < len(args) {
				_, _ = fmt.Sscanf(args[i+1], "%d", &historyN)
				i++
			}
			if historyN <= 0 {
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

	if showTop {
		printSummary(summary)
		return showByCommand(tracker, topN)
	}

	if showWeekly {
		printSummary(summary)
		return showPeriodReport(tracker, "weekly")
	}

	if showMonthly {
		printSummary(summary)
		return showPeriodReport(tracker, "monthly")
	}

	if showDaily {
		return showDailyReport(tracker, days, summary)
	}

	// Default: full dashboard (summary + sparkline + top commands)
	printSummary(summary)
	showSparkline(tracker)
	_ = showByCommand(tracker, 10)
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

	tier := TierLabel(s.AvgSavings)

	// printKPI renders a label-value pair. If value is already styled
	// (contains ANSI codes), pass styled=true to avoid double-wrapping.
	printKPI := func(label, value string, styled bool) {
		if tty {
			styledValue := value
			if !styled {
				styledValue = StatStyle.Render(value)
			}
			fmt.Printf("  %s  %s\n", DimStyle.Render(fmt.Sprintf("%-20s", label)), styledValue)
		} else {
			fmt.Printf("  %-20s  %s\n", label, value)
		}
	}

	printKPI("Commands filtered", fmt.Sprintf("%d", s.TotalCommands), false)
	printKPI("Tokens saved", utils.FormatTokens(s.TotalSaved), false)
	printKPI("Avg savings", ColorSavings(s.AvgSavings), true)
	printKPI("Efficiency", ColorTier(tier), true)
	printKPI("Total time", fmt.Sprintf("%.1fs", float64(s.TotalTimeMs)/1000), false)

	// Efficiency bar
	pct := s.AvgSavings
	if pct < 0 {
		pct = 0
	} else if pct > 100 {
		pct = 100
	}
	bar := ColorBar(int(pct), 100, 20)
	fmt.Println()
	if tty {
		fmt.Printf("  %s %s\n", bar, DimStyle.Render(fmt.Sprintf("%.0f%%", s.AvgSavings)))
	} else {
		fmt.Printf("  %s %.0f%%\n", bar, s.AvgSavings)
	}
	fmt.Println()
}

func showByCommand(tracker *tracking.Tracker, limit int) error {
	stats, err := tracker.GetByCommand(limit)
	if err != nil {
		return err
	}
	if len(stats) == 0 {
		return nil
	}

	tty := IsTerminal()

	// Find max saved for bar scaling
	maxSaved := 0
	for _, s := range stats {
		if s.SavedTokens > maxSaved {
			maxSaved = s.SavedTokens
		}
	}

	if tty {
		fmt.Println(DimStyle.Render("  Top commands by tokens saved"))
		fmt.Println()
	} else {
		fmt.Println("  Top commands by tokens saved")
		fmt.Println()
	}

	headers := []string{"Command", "Runs", "Saved", "Savings", "Impact"}
	var rows [][]string
	for _, s := range stats {
		cmd := s.Command
		if len(cmd) > 25 {
			cmd = cmd[:22] + "..."
		}
		bar := ColorBar(s.SavedTokens, maxSaved, 12)
		rows = append(rows, []string{
			cmd,
			fmt.Sprintf("%d", s.Count),
			utils.FormatTokens(s.SavedTokens),
			ColorSavings(s.AvgSavings),
			bar,
		})
	}

	fmt.Print(FormatTable(headers, rows))
	fmt.Println()
	return nil
}

func showSparkline(tracker *tracking.Tracker) {
	daily, err := tracker.GetDaily(14)
	if err != nil || len(daily) < 2 {
		return
	}

	// Daily data is DESC, reverse for chronological sparkline
	values := make([]float64, len(daily))
	for i, d := range daily {
		values[len(daily)-1-i] = d.AvgSavings
	}

	spark := FormatSparkline(values)
	tty := IsTerminal()

	if tty {
		fmt.Printf("  %s  %s\n", DimStyle.Render("14-day trend"), SuccessStyle.Render(spark))
	} else {
		fmt.Printf("  14-day trend  %s\n", spark)
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
			ColorSavings(d.AvgSavings),
		})
	}

	fmt.Print(FormatTable(headers, rows))
	return nil
}

func showPeriodReport(tracker *tracking.Tracker, period string) error {
	var stats []tracking.PeriodStats
	var err error
	var label string

	switch period {
	case "weekly":
		stats, err = tracker.GetWeekly(8)
		label = "Weekly"
	case "monthly":
		stats, err = tracker.GetMonthly(6)
		label = "Monthly"
	default:
		return fmt.Errorf("unknown period: %s", period)
	}
	if err != nil {
		return err
	}

	tty := IsTerminal()
	if tty {
		fmt.Println(DimStyle.Render(fmt.Sprintf("  %s breakdown", label)))
	} else {
		fmt.Printf("  %s breakdown\n", label)
	}
	fmt.Println()

	headers := []string{"Period", "Cmds", "Input", "Output", "Saved", "Savings"}
	var rows [][]string
	for _, s := range stats {
		rows = append(rows, []string{
			s.Period,
			fmt.Sprintf("%d", s.Commands),
			utils.FormatTokens(s.InputTokens),
			utils.FormatTokens(s.OutputTokens),
			utils.FormatTokens(s.SavedTokens),
			ColorSavings(s.AvgSavings),
		})
	}

	fmt.Print(FormatTable(headers, rows))
	fmt.Println()
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
			ColorSavings(r.SavingsPct),
			fmt.Sprintf("%dms", r.ExecTimeMs),
		})
	}

	fmt.Print(FormatTable(headers, rows))
	return nil
}

func exportJSON(summary *tracking.Summary, tracker *tracking.Tracker, days int) error {
	daily, _ := tracker.GetDaily(days)
	byCmd, _ := tracker.GetByCommand(10)
	data := map[string]any{
		"summary":    summary,
		"daily":      daily,
		"by_command": byCmd,
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
	_ = w.Write([]string{"date", "commands", "input_tokens", "output_tokens", "saved_tokens", "avg_savings"})
	for _, d := range daily {
		_ = w.Write([]string{
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
