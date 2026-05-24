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
	fmt.Println(ui.Header(ui.HeaderArgs{Command: "help", Version: Version}))

	fmt.Println(ui.BoldStyle.Render("  COMMANDS"))
	fmt.Println()

	commands := []struct {
		name string
		args string
		desc string
	}{
		{"setup", "", "Configure your LLM provider, API key, and GitHub access"},
		{"init", "<github-url>", "Clone a repo and generate Docker configuration for it"},
		{"help", "", "Show this help message"},
	}
	for _, c := range commands {
		name := ui.PrimaryStyle.Render(fmt.Sprintf("%-6s", c.name))
		args := ui.HighlightStyle.Render(fmt.Sprintf("%-15s", c.args))
		fmt.Printf("    %s %s  %s\n", name, args, c.desc)
	}

	fmt.Println()
	fmt.Println(ui.BoldStyle.Render("  GLOBAL FLAGS"))
	fmt.Println()
	flags := []struct{ name, desc string }{
		{"--verbose, -v", "Show detailed logs (file reads, LLM responses)"},
		{"--quiet, -q", "Suppress non-error output (good for scripting)"},
		{"--no-color", "Disable ANSI colors (also honours NO_COLOR env var)"},
		{"--version", "Print version and exit"},
	}
	for _, f := range flags {
		fmt.Printf("    %s  %s\n", ui.MutedStyle.Render(fmt.Sprintf("%-16s", f.name)), f.desc)
	}

	fmt.Println()
	fmt.Println(ui.BoldStyle.Render("  EXAMPLES"))
	fmt.Println()
	examples := []string{
		"chimera setup",
		"chimera init https://github.com/tiangolo/fastapi",
		"chimera init --no-agent https://github.com/user/repo",
		"chimera init --force https://github.com/user/repo/tree/main/packages/web",
	}
	for _, e := range examples {
		fmt.Println("    " + ui.HighlightStyle.Render(e))
	}

	fmt.Println()
	fmt.Println(ui.DimStyle.Render("  Each command also supports --help, e.g. `chimera init --help`."))
	fmt.Println()
}
