package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Table renders a simple table
type Table struct {
	Headers []string
	Rows    [][]string
	Title   string
}

// Render renders the table with borders
func (t *Table) Render() string {
	if len(t.Rows) == 0 {
		return ""
	}

	// Calculate column widths
	colWidths := make([]int, len(t.Headers))
	for i, h := range t.Headers {
		colWidths[i] = len(h)
	}
	for _, row := range t.Rows {
		for i, cell := range row {
			if i < len(colWidths) && len(cell) > colWidths[i] {
				colWidths[i] = len(cell)
			}
		}
	}

	var b strings.Builder

	// Title
	if t.Title != "" {
		titleStyle := lipgloss.NewStyle().
			Bold(true).
			Foreground(ColourPrimary).
			MarginBottom(1)
		b.WriteString(titleStyle.Render(t.Title))
		b.WriteString("\n")
	}

	// Headers
	headerStyle := lipgloss.NewStyle().Bold(true).Foreground(ColourPrimary)
	for i, h := range t.Headers {
		b.WriteString(headerStyle.Render(padRight(h, colWidths[i])))
		if i < len(t.Headers)-1 {
			b.WriteString("  │  ")
		}
	}
	b.WriteString("\n")

	// Separator
	for i := range t.Headers {
		b.WriteString(strings.Repeat("─", colWidths[i]))
		if i < len(t.Headers)-1 {
			b.WriteString("──┼──")
		}
	}
	b.WriteString("\n")

	// Rows
	for _, row := range t.Rows {
		for i, cell := range row {
			if i < len(colWidths) {
				b.WriteString(padRight(cell, colWidths[i]))
				if i < len(row)-1 {
					b.WriteString("  │  ")
				}
			}
		}
		b.WriteString("\n")
	}

	// Wrap in box
	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColourPrimary).
		Padding(1, 2)

	return boxStyle.Render(b.String())
}

func padRight(s string, width int) string {
	if len(s) >= width {
		return s
	}
	return s + strings.Repeat(" ", width-len(s))
}

// ConfidenceBar renders a confidence bar
func ConfidenceBar(confidence string) string {
	var filled int
	switch confidence {
	case "high":
		filled = 5
	case "medium":
		filled = 3
	case "low":
		filled = 1
	default:
		filled = 0
	}

	bar := strings.Repeat("█", filled) + strings.Repeat("░", 5-filled)

	var style lipgloss.Style
	switch confidence {
	case "high":
		style = SuccessStyle
	case "medium":
		style = WarningStyle
	default:
		style = ErrorStyle
	}

	return fmt.Sprintf("%s %s", style.Render(bar), MutedStyle.Render(confidence))
}
