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

// FormatTable formats data as a simple aligned table.
func FormatTable(headers []string, rows [][]string) string {
	if len(headers) == 0 {
		return ""
	}

	// Calculate column widths
	widths := make([]int, len(headers))
	for i, h := range headers {
		widths[i] = len(h)
	}
	for _, row := range rows {
		for i, cell := range row {
			if i < len(widths) && len(cell) > widths[i] {
				widths[i] = len(cell)
			}
		}
	}

	var b strings.Builder

	// Header
	for i, h := range headers {
		if i > 0 {
			b.WriteString("  ")
		}
		b.WriteString(fmt.Sprintf("%-*s", widths[i], h))
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
				b.WriteString(fmt.Sprintf("%-*s", widths[i], cell))
			}
		}
		b.WriteString("\n")
	}

	return b.String()
}
