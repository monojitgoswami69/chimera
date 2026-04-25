package cmd

import (
	"fmt"

	"chimera/internal/ui"

	"github.com/spf13/cobra"
)

var helpCmd = &cobra.Command{
	Use:   "help",
	Short: "Show help message",
	Long:  `Display help information about Chimera and its commands.`,
	Run: func(cmd *cobra.Command, args []string) {
		showHelp()
	},
}

func showHelp() {
	fmt.Println(ui.Header("help"))

	fmt.Println(ui.BoldStyle.Render("COMMANDS"))
	fmt.Println()

	commands := []struct {
		name string
		args string
		desc string
	}{
		{"init", "<github-url>", "Clone a repo and generate its full local environment"},
		{"setup", "", "Configure your LLM provider, API key, and GitHub access"},
		{"help", "", "Show this help message"},
	}

	for _, c := range commands {
		name := ui.PrimaryStyle.Render(c.name)
		if c.args != "" {
			args := ui.HighlightStyle.Render(c.args)
			fmt.Printf("  %-6s %-15s %s\n", name, args, c.desc)
		} else {
			fmt.Printf("  %-22s %s\n", name, c.desc)
		}
	}

	fmt.Println()
	fmt.Println(ui.DimStyle.Render("Each command also supports its own help:"))
	fmt.Println(ui.DimStyle.Render("  chimera init --help"))
	fmt.Println(ui.DimStyle.Render("  chimera setup --help"))
	fmt.Println()
}
