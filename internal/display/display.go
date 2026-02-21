package display

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-isatty"
)

var (
	HeaderStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("12"))
	SuccessStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("10"))
	ErrorStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("9"))
	DimStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	StatStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("14"))
	WarnStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("11"))
	GreenStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("10"))
	YellowStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("11"))
	RedStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("9"))
)

// IsTerminal returns true if stdout is a TTY.
func IsTerminal() bool {
	return isatty.IsTerminal(os.Stdout.Fd()) || isatty.IsCygwinTerminal(os.Stdout.Fd())
}

// PrintFiltered prints filtered output with optional verbosity header.
func PrintFiltered(output string, verbose int) {
	if verbose > 0 && IsTerminal() {
		fmt.Fprintln(os.Stderr, DimStyle.Render("--- snip filtered ---"))
	}
	fmt.Print(output)
}

// PrintError prints a styled error to stderr.
func PrintError(msg string) {
	if IsTerminal() {
		fmt.Fprintln(os.Stderr, ErrorStyle.Render("snip: "+msg))
	} else {
		fmt.Fprintln(os.Stderr, "snip: "+msg)
	}
}

// FormatSeparator returns a horizontal separator line.
func FormatSeparator(width int) string {
	return strings.Repeat("═", width)
}

// ColorSavings returns a savings percentage string colored by tier.
// ≥70% green, 30-70% yellow, <30% red.
func ColorSavings(pct float64) string {
	text := fmt.Sprintf("%.1f%%", pct)
	if !IsTerminal() {
		return text
	}
	switch {
	case pct >= 70:
		return GreenStyle.Render(text)
	case pct >= 30:
		return YellowStyle.Render(text)
	default:
		return RedStyle.Render(text)
	}
}

// TierLabel returns an efficiency tier label based on savings percentage.
func TierLabel(pct float64) string {
	switch {
	case pct >= 90:
		return "Elite"
	case pct >= 70:
		return "Great"
	case pct >= 50:
		return "Good"
	case pct >= 30:
		return "Fair"
	default:
		return "Low"
	}
}

// ColorTier returns a tier label colored by level.
func ColorTier(tier string) string {
	if !IsTerminal() {
		return tier
	}
	switch tier {
	case "Elite":
		return GreenStyle.Bold(true).Render(tier)
	case "Great":
		return GreenStyle.Render(tier)
	case "Good":
		return YellowStyle.Render(tier)
	case "Fair":
		return WarnStyle.Render(tier)
	default:
		return RedStyle.Render(tier)
	}
}

// ColorBar returns a colored impact bar (green filled, dim empty).
// Guarantees at least 1 filled block when value > 0.
func ColorBar(value, maxVal, width int) string {
	if maxVal <= 0 || width <= 0 {
		return strings.Repeat("░", width)
	}
	filled := min(max(value*width/maxVal, 0), width)
	if filled == 0 && value > 0 {
		filled = 1
	}
	filledStr := strings.Repeat("█", filled)
	emptyStr := strings.Repeat("░", width-filled)
	if !IsTerminal() {
		return filledStr + emptyStr
	}
	return GreenStyle.Render(filledStr) + DimStyle.Render(emptyStr)
}

// FormatBar renders a horizontal bar proportional to value/maxVal.
func FormatBar(value, maxVal, width int) string {
	if maxVal <= 0 || width <= 0 {
		return strings.Repeat("░", width)
	}
	filled := min(max(value*width/maxVal, 0), width)
	return strings.Repeat("█", filled) + strings.Repeat("░", width-filled)
}

// FormatSparkline renders a sparkline from a slice of values.
// Uses Unicode block characters ▁▂▃▄▅▆▇█.
func FormatSparkline(values []float64) string {
	blocks := []rune("▁▂▃▄▅▆▇█")

	if len(values) == 0 {
		return ""
	}

	max := values[0]
	for _, v := range values[1:] {
		if v > max {
			max = v
		}
	}

	var b strings.Builder
	for _, v := range values {
		idx := 0
		if max > 0 {
			idx = int(v / max * float64(len(blocks)-1))
		}
		if idx >= len(blocks) {
			idx = len(blocks) - 1
		}
		if idx < 0 {
			idx = 0
		}
		b.WriteRune(blocks[idx])
	}
	return b.String()
}

// visualWidth returns the visible width of a string, ignoring ANSI escape codes.
func visualWidth(s string) int {
	return lipgloss.Width(s)
}

// padRight pads a string to the target visual width with spaces,
// correctly handling ANSI escape codes.
func padRight(s string, targetWidth int) string {
	vw := visualWidth(s)
	if vw >= targetWidth {
		return s
	}
	return s + strings.Repeat(" ", targetWidth-vw)
}

// FormatTable formats data as a simple aligned table.
// Handles ANSI-colored cells correctly for alignment.
func FormatTable(headers []string, rows [][]string) string {
	if len(headers) == 0 {
		return ""
	}

	// Calculate column widths using visual width (ANSI-safe)
	widths := make([]int, len(headers))
	for i, h := range headers {
		widths[i] = visualWidth(h)
	}
	for _, row := range rows {
		for i, cell := range row {
			if i < len(widths) {
				if w := visualWidth(cell); w > widths[i] {
					widths[i] = w
				}
			}
		}
	}

	var b strings.Builder

	// Header
	for i, h := range headers {
		if i > 0 {
			b.WriteString("  ")
		}
		b.WriteString(padRight(h, widths[i]))
	}
	b.WriteString("\n")

	// Separator
	for i, w := range widths {
		if i > 0 {
			b.WriteString("  ")
		}
		b.WriteString(strings.Repeat("─", w))
	}
	b.WriteString("\n")

	// Rows
	for _, row := range rows {
		for i, cell := range row {
			if i > 0 {
				b.WriteString("  ")
			}
			if i < len(widths) {
				b.WriteString(padRight(cell, widths[i]))
			}
		}
		b.WriteString("\n")
	}

	return b.String()
}
