package tui

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
)

// Color definitions following the UX requirements:
// - Green for success
// - Yellow for warnings
// - Red for errors
// - Blue for info
var (
	// SuccessStyle is used for success messages (green)
	SuccessStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("10")).
			Bold(true)

	// ErrorStyle is used for error messages (red)
	ErrorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("9")).
			Bold(true)

	// WarningStyle is used for warning messages (yellow)
	WarningStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("11")).
			Bold(true)

	// InfoStyle is used for informational messages (blue)
	InfoStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("12"))

	// HeaderStyle is used for section headers
	HeaderStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("13")).
			Bold(true).
			Underline(true)

	// SubheaderStyle is used for subsection headers
	SubheaderStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("14")).
			Bold(true)

	// ListItemStyle is used for list items
	ListItemStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("7"))

	// TableHeaderStyle is used for table headers
	TableHeaderStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("15")).
				Bold(true)
)

// PrintSuccess prints a success message in green
func PrintSuccess(message string) {
	fmt.Println(SuccessStyle.Render("✓ " + message))
}

// PrintError prints an error message in red
func PrintError(message string) {
	fmt.Println(ErrorStyle.Render("✗ " + message))
}

// PrintWarning prints a warning message in yellow
func PrintWarning(message string) {
	fmt.Println(WarningStyle.Render("⚠ " + message))
}

// PrintInfo prints an informational message in blue
func PrintInfo(message string) {
	fmt.Println(InfoStyle.Render("ℹ " + message))
}

// PrintHeader prints a section header
func PrintHeader(text string) {
	fmt.Println(HeaderStyle.Render(text))
}

// PrintSubheader prints a subsection header
func PrintSubheader(text string) {
	fmt.Println(SubheaderStyle.Render(text))
}

// PrintListItem prints a list item with a bullet point
func PrintListItem(text string) {
	fmt.Println(ListItemStyle.Render("  • " + text))
}

// PrintTableHeader prints a table header row
func PrintTableHeader(columns []string) {
	row := ""
	for i, col := range columns {
		if i > 0 {
			row += "  "
		}
		row += fmt.Sprintf("%-20s", col)
	}
	fmt.Println(TableHeaderStyle.Render(row))
	fmt.Println(TableHeaderStyle.Render("────────────────────────────────────────────────────────────────────────────────"))
}

// PrintTableRow prints a table data row
func PrintTableRow(columns []string) {
	row := ""
	for i, col := range columns {
		if i > 0 {
			row += "  "
		}
		// Truncate long columns
		if len(col) > 20 {
			col = col[:17] + "..."
		}
		row += fmt.Sprintf("%-20s", col)
	}
	fmt.Println(ListItemStyle.Render(row))
}
