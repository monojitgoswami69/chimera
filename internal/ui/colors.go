package ui

import "github.com/charmbracelet/lipgloss"

// Color palette as per specification
var (
	ColourPrimary   = lipgloss.Color("#7C3AED") // purple — brand colour
	ColourSuccess   = lipgloss.Color("#10B981") // green
	ColourWarning   = lipgloss.Color("#F59E0B") // amber
	ColourError     = lipgloss.Color("#EF4444") // red
	ColourMuted     = lipgloss.Color("#6B7280") // grey
	ColourHighlight = lipgloss.Color("#60A5FA") // blue — for file paths and URLs
	ColourDim       = lipgloss.Color("#374151") // very dark — for secondary info
)

// Styles
var (
	PrimaryStyle   = lipgloss.NewStyle().Foreground(ColourPrimary)
	SuccessStyle   = lipgloss.NewStyle().Foreground(ColourSuccess)
	WarningStyle   = lipgloss.NewStyle().Foreground(ColourWarning)
	ErrorStyle     = lipgloss.NewStyle().Foreground(ColourError)
	MutedStyle     = lipgloss.NewStyle().Foreground(ColourMuted)
	HighlightStyle = lipgloss.NewStyle().Foreground(ColourHighlight)
	DimStyle       = lipgloss.NewStyle().Foreground(ColourDim)
	BoldStyle      = lipgloss.NewStyle().Bold(true)
)

// Box styles
var (
	SuccessBox = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColourSuccess).
			Padding(0, 1)

	ErrorBox = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColourError).
			Padding(0, 1)

	WarningBox = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColourWarning).
			Padding(0, 1)

	InfoBox = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColourPrimary).
		Padding(0, 1)
)
