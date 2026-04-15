package economics

import (
	"fmt"

	"github.com/edouard-claude/snip/internal/display"
	"github.com/edouard-claude/snip/internal/tracking"
	"github.com/edouard-claude/snip/internal/utils"
)

// Tier holds a model tier name and its input price per 1M tokens.
type Tier struct {
	Name     string
	PriceM   float64 // price per 1M input tokens
}

// Tiers lists all supported pricing tiers in display order.
var Tiers = []Tier{
	{Name: "Haiku", PriceM: 0.25},
	{Name: "Sonnet", PriceM: 3.00},
	{Name: "Opus", PriceM: 15.00},
}

// TierByName returns the tier matching name (case-insensitive), or nil.
func TierByName(name string) *Tier {
	for i := range Tiers {
		if eqFold(Tiers[i].Name, name) {
			return &Tiers[i]
		}
	}
	return nil
}

// CostForTokens returns the dollar cost for the given token count at the tier price.
func CostForTokens(tokens int, pricePerM float64) float64 {
	return float64(tokens) / 1_000_000 * pricePerM
}

// FormatCost formats a dollar amount for display.
func FormatCost(amount float64) string {
	if amount >= 100 {
		return fmt.Sprintf("$%.0f", amount)
	}
	if amount >= 10 {
		return fmt.Sprintf("$%.1f", amount)
	}
	return fmt.Sprintf("$%.2f", amount)
}

// Run executes the cc-economics subcommand.
func Run(tracker *tracking.Tracker, args []string) error {
	if tracker == nil {
		display.PrintError("no tracking data (run some commands first)")
		return nil
	}

	// Parse --tier flag
	var filterTier string
	for i := 0; i < len(args); i++ {
		if args[i] == "--tier" && i+1 < len(args) {
			filterTier = args[i+1]
			i++
		}
	}

	if filterTier != "" && TierByName(filterTier) == nil {
		return fmt.Errorf("unknown tier %q (valid: haiku, sonnet, opus)", filterTier)
	}

	summary, err := tracker.GetSummary()
	if err != nil {
		return fmt.Errorf("get summary: %w", err)
	}

	daily, err := tracker.GetDaily(90)
	if err != nil {
		return fmt.Errorf("get daily stats: %w", err)
	}

	totalDays := len(daily)
	if totalDays == 0 {
		totalDays = 1
	}

	tty := display.IsTerminal()

	fmt.Println()
	if tty {
		fmt.Println(display.HeaderStyle.Render("  snip cc-economics - token savings cost analysis"))
	} else {
		fmt.Println("  snip cc-economics - token savings cost analysis")
	}
	fmt.Println()

	printKPI := func(label, value string) {
		if tty {
			fmt.Printf("  %s  %s\n", display.DimStyle.Render(fmt.Sprintf("%-24s", label)), display.StatStyle.Render(value))
		} else {
			fmt.Printf("  %-24s  %s\n", label, value)
		}
	}

	printKPI("Total tokens saved", utils.FormatTokens(summary.TotalSaved))
	fmt.Println()

	if tty {
		fmt.Println(display.DimStyle.Render("  Estimated savings by model tier:"))
	} else {
		fmt.Println("  Estimated savings by model tier:")
	}

	tiers := Tiers
	if filterTier != "" {
		t := TierByName(filterTier)
		tiers = []Tier{*t}
	}

	for _, t := range tiers {
		cost := CostForTokens(summary.TotalSaved, t.PriceM)
		label := fmt.Sprintf("    %-8s", t.Name)
		value := FormatCost(cost)
		if tty {
			fmt.Printf("  %s %s\n", display.DimStyle.Render(label), display.GreenStyle.Render(value))
		} else {
			fmt.Printf("  %s %s\n", label, value)
		}
	}

	fmt.Println()
	if tty {
		fmt.Println(display.DimStyle.Render(fmt.Sprintf("  Based on %d filtered commands over %d days.", summary.TotalCommands, totalDays)))
		fmt.Println(display.DimStyle.Render("  Assumes saved tokens are input tokens (filtered output fed back to LLM context)."))
	} else {
		fmt.Printf("  Based on %d filtered commands over %d days.\n", summary.TotalCommands, totalDays)
		fmt.Println("  Assumes saved tokens are input tokens (filtered output fed back to LLM context).")
	}
	fmt.Println()

	return nil
}

// eqFold is a simple case-insensitive string comparison.
func eqFold(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		ca, cb := a[i], b[i]
		if ca >= 'A' && ca <= 'Z' {
			ca += 'a' - 'A'
		}
		if cb >= 'A' && cb <= 'Z' {
			cb += 'a' - 'A'
		}
		if ca != cb {
			return false
		}
	}
	return true
}
