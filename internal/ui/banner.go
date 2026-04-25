package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Banner displays the CHIMERA ASCII art with gradient effect
func Banner() string {
	banner := `
 ██████╗██╗  ██╗██╗███╗   ███╗███████╗██████╗  █████╗ 
██╔════╝██║  ██║██║████╗ ████║██╔════╝██╔══██╗██╔══██╗
██║     ███████║██║██╔████╔██║█████╗  ██████╔╝███████║
██║     ██╔══██║██║██║╚██╔╝██║██╔══╝  ██╔══██╗██╔══██║
╚██████╗██║  ██║██║██║ ╚═╝ ██║███████╗██║  ██║██║  ██║
 ╚═════╝╚═╝  ╚═╝╚═╝╚═╝     ╚═╝╚══════╝╚═╝  ╚═╝╚═╝  ╚═╝`

	// Apply gradient from cyan to purple
	lines := strings.Split(banner, "\n")
	var result strings.Builder

	colors := []lipgloss.Color{
		lipgloss.Color("#06B6D4"), // cyan
		lipgloss.Color("#3B82F6"), // blue
		lipgloss.Color("#6366F1"), // indigo
		lipgloss.Color("#7C3AED"), // purple
		lipgloss.Color("#A855F7"), // purple-light
		lipgloss.Color("#7C3AED"), // purple
	}

	for i, line := range lines {
		if i == 0 {
			result.WriteString(line + "\n")
			continue
		}
		colorIdx := (i - 1) % len(colors)
		style := lipgloss.NewStyle().Foreground(colors[colorIdx])
		result.WriteString(style.Render(line) + "\n")
	}

	return result.String()
}

// Header displays banner with tagline and version
func Header(command string) string {
	var b strings.Builder
	b.WriteString(Banner())
	b.WriteString("\n")
	b.WriteString(DimStyle.Render("  Autonomous environment orchestration for any GitHub repository."))
	b.WriteString("\n")
	b.WriteString(MutedStyle.Render(fmt.Sprintf("  v0.1.0 · %s", command)))
	b.WriteString("\n\n")
	return b.String()
}
