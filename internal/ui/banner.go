package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Banner displays the CHIMERA ASCII art with gradient effect.
func Banner() string {
	banner := ` ██████╗██╗  ██╗██╗███╗   ███╗███████╗██████╗  █████╗
██╔════╝██║  ██║██║████╗ ████║██╔════╝██╔══██╗██╔══██╗
██║     ███████║██║██╔████╔██║█████╗  ██████╔╝███████║
██║     ██╔══██║██║██║╚██╔╝██║██╔══╝  ██╔══██╗██╔══██║
╚██████╗██║  ██║██║██║ ╚═╝ ██║███████╗██║  ██║██║  ██║
 ╚═════╝╚═╝  ╚═╝╚═╝╚═╝     ╚═╝╚══════╝╚═╝  ╚═╝╚═╝  ╚═╝`

	lines := strings.Split(banner, "\n")
	colors := []lipgloss.Color{
		lipgloss.Color("#06B6D4"),
		lipgloss.Color("#3B82F6"),
		lipgloss.Color("#6366F1"),
		lipgloss.Color("#7C3AED"),
		lipgloss.Color("#A855F7"),
		lipgloss.Color("#7C3AED"),
	}

	var b strings.Builder
	b.WriteString("\n")
	for i, line := range lines {
		style := lipgloss.NewStyle().Foreground(colors[i%len(colors)])
		b.WriteString(style.Render(line) + "\n")
	}
	return b.String()
}

// HeaderArgs lets the caller pass in version info so the banner stays in sync.
type HeaderArgs struct {
	Command string
	Version string
}

// Header displays banner with tagline and version.
func Header(args HeaderArgs) string {
	var b strings.Builder
	b.WriteString(Banner())
	b.WriteString("\n")
	b.WriteString(DimStyle.Render("  Autonomous environment orchestration for any GitHub repository."))
	b.WriteString("\n")
	b.WriteString(MutedStyle.Render(fmt.Sprintf("  %s · %s", args.Version, args.Command)))
	b.WriteString("\n\n")
	return b.String()
}

// Step prints a numbered, bold step header.
func Step(n int, total int, title string) string {
	prefix := PrimaryStyle.Render(fmt.Sprintf("[%d/%d]", n, total))
	return fmt.Sprintf("\n  %s %s\n", prefix, BoldStyle.Render(title))
}
