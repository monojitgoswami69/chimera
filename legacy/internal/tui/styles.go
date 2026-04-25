package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// ──────────────────────────────────────────────────────────────
// Color palette — muted neon on dark terminals
// ──────────────────────────────────────────────────────────────

var (
	colorGreen   = lipgloss.Color("#7ec699")
	colorRed     = lipgloss.Color("#e06c75")
	colorYellow  = lipgloss.Color("#e5c07b")
	colorBlue    = lipgloss.Color("#61afef")
	colorCyan    = lipgloss.Color("#56b6c2")
	colorMagenta = lipgloss.Color("#c678dd")
	colorDim     = lipgloss.Color("#5c6370")
	colorWhite   = lipgloss.Color("#abb2bf")
	colorOrange  = lipgloss.Color("#d19a66")
)

// ──────────────────────────────────────────────────────────────
// Reusable styles
// ──────────────────────────────────────────────────────────────

var (
	// SuccessStyle is used for success messages (green)
	SuccessStyle = lipgloss.NewStyle().
			Foreground(colorGreen).
			Bold(true)

	// ErrorStyle is used for error messages (red)
	ErrorStyle = lipgloss.NewStyle().
			Foreground(colorRed).
			Bold(true)

	// WarningStyle is used for warning messages (yellow)
	WarningStyle = lipgloss.NewStyle().
			Foreground(colorYellow).
			Bold(true)

	// InfoStyle is used for informational messages (blue)
	InfoStyle = lipgloss.NewStyle().
			Foreground(colorBlue)

	// HeaderStyle is used for section headers
	HeaderStyle = lipgloss.NewStyle().
			Foreground(colorMagenta).
			Bold(true)

	// SubheaderStyle is used for subsection headers
	SubheaderStyle = lipgloss.NewStyle().
			Foreground(colorCyan).
			Bold(true)

	// ListItemStyle is used for list items
	ListItemStyle = lipgloss.NewStyle().
			Foreground(colorWhite)

	// DimStyle is used for secondary/muted text
	DimStyle = lipgloss.NewStyle().
			Foreground(colorDim)

	// HintStyle is used for helpful hint text
	HintStyle = lipgloss.NewStyle().
			Foreground(colorDim).
			Italic(true)

	// BoldStyle for emphasis
	BoldStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorWhite)

	// TableHeaderStyle is used for table headers
	TableHeaderStyle = lipgloss.NewStyle().
				Foreground(colorCyan).
				Bold(true)

	// FileStyle for file paths
	FileStyle = lipgloss.NewStyle().
			Foreground(colorOrange)

	// CmdStyle for commands
	CmdStyle = lipgloss.NewStyle().
			Foreground(colorCyan)
)

// ──────────────────────────────────────────────────────────────
// Banner / Branding
// ──────────────────────────────────────────────────────────────

const chimeraBanner = `
     ██████╗██╗  ██╗██╗███╗   ███╗███████╗██████╗  █████╗
    ██╔════╝██║  ██║██║████╗ ████║██╔════╝██╔══██╗██╔══██╗
    ██║     ███████║██║██╔████╔██║█████╗  ██████╔╝███████║
    ██║     ██╔══██║██║██║╚██╔╝██║██╔══╝  ██╔══██╗██╔══██║
    ╚██████╗██║  ██║██║██║ ╚═╝ ██║███████╗██║  ██║██║  ██║
     ╚═════╝╚═╝  ╚═╝╚═╝╚═╝     ╚═╝╚══════╝╚═╝  ╚═╝╚═╝  ╚═╝`

// PrintBanner prints the ASCII art banner
func PrintBanner() {
	gradient := lipgloss.NewStyle().Foreground(colorMagenta).Bold(true)
	fmt.Println(gradient.Render(chimeraBanner))
	fmt.Println(DimStyle.Render("    Autonomous Environment Orchestration"))
	fmt.Println()
}

// PrintBannerCompact prints a single-line banner for sub-commands
func PrintBannerCompact() {
	fmt.Print(lipgloss.NewStyle().Foreground(colorMagenta).Bold(true).Render("◆ chimera"))
	fmt.Println(DimStyle.Render(" · autonomous env orchestration"))
}

// ──────────────────────────────────────────────────────────────
// Message printing helpers
// ──────────────────────────────────────────────────────────────

// PrintSuccess prints a success message in green
func PrintSuccess(message string) {
	fmt.Println(SuccessStyle.Render("  ✓ " + message))
}

// PrintError prints an error message in red
func PrintError(message string) {
	fmt.Println(ErrorStyle.Render("  ✗ " + message))
}

// PrintWarning prints a warning message in yellow
func PrintWarning(message string) {
	fmt.Println(WarningStyle.Render("  ⚠ " + message))
}

// PrintInfo prints an informational message in blue
func PrintInfo(message string) {
	fmt.Println(InfoStyle.Render("  → " + message))
}

// PrintHint prints a dim italic hint
func PrintHint(message string) {
	fmt.Println(HintStyle.Render("    " + message))
}

// PrintHeader prints a section header
func PrintHeader(text string) {
	fmt.Println()
	fmt.Println(HeaderStyle.Render("  " + text))
	fmt.Println(DimStyle.Render("  " + strings.Repeat("─", len(text)+2)))
}

// PrintSubheader prints a subsection header
func PrintSubheader(text string) {
	fmt.Println(SubheaderStyle.Render("  " + text))
}

// PrintListItem prints a list item with a bullet point
func PrintListItem(text string) {
	fmt.Println(ListItemStyle.Render("    • " + text))
}

// PrintDivider prints a thin horizontal divider
func PrintDivider() {
	fmt.Println(DimStyle.Render("  " + strings.Repeat("─", 56)))
}

// PrintStep prints a numbered step
func PrintStep(n int, text string) {
	num := lipgloss.NewStyle().Foreground(colorCyan).Bold(true).Render(fmt.Sprintf("  %d.", n))
	fmt.Printf("%s %s\n", num, ListItemStyle.Render(text))
}

// PrintKeyValue prints a key-value pair with alignment
func PrintKeyValue(key, value string) {
	k := lipgloss.NewStyle().Foreground(colorDim).Render(fmt.Sprintf("    %-14s", key))
	v := lipgloss.NewStyle().Foreground(colorWhite).Render(value)
	fmt.Printf("%s %s\n", k, v)
}

// PrintFile prints a filename in orange
func PrintFile(text string) {
	fmt.Println(FileStyle.Render("    " + text))
}

// PrintCmd prints a command in cyan
func PrintCmd(text string) {
	fmt.Println(CmdStyle.Render("      " + text))
}

// ──────────────────────────────────────────────────────────────
// Table helpers
// ──────────────────────────────────────────────────────────────

// PrintTableHeader prints a table header row
func PrintTableHeader(columns []string) {
	row := ""
	for i, col := range columns {
		if i > 0 {
			row += "  "
		}
		row += fmt.Sprintf("%-20s", col)
	}
	fmt.Println(TableHeaderStyle.Render("  " + row))
	fmt.Println(DimStyle.Render("  " + strings.Repeat("─", 76)))
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
	fmt.Println(ListItemStyle.Render("  " + row))
}

// ──────────────────────────────────────────────────────────────
// Phase printing — shows progress through multi-step operations
// ──────────────────────────────────────────────────────────────

// PrintPhase prints a phase indicator for multi-step workflows
func PrintPhase(current, total int, text string) {
	phase := lipgloss.NewStyle().Foreground(colorDim).Render(fmt.Sprintf("[%d/%d]", current, total))
	label := lipgloss.NewStyle().Foreground(colorBlue).Bold(true).Render(text)
	fmt.Printf("\n%s %s\n", phase, label)
}
