package cmd

import (
	"fmt"

	"github.com/projectchimera/chimera/internal/agent"
	"github.com/spf13/cobra"
)

var (
	// Version is set during build via ldflags
	Version = "dev"
	// BuildTime is set during build via ldflags
	BuildTime = "unknown"
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "chimera",
	Short: "Chimera - Autonomous local development environment provisioner",
	Long: `Chimera is a developer CLI tool that autonomously clones a GitHub repository,
analyzes its infrastructure dependencies, and provisions a fully containerized
local development environment — all with zero manual configuration.

Run 'chimera init <github-url>' to get a running local environment in under 2 minutes.`,
	Version: Version,
	// Uncomment the following line if your bare application has an action associated with it:
	// Run: func(cmd *cobra.Command, args []string) { },
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() error {
	return rootCmd.Execute()
}

func init() {
	// Load .chimera.env config files (~/.chimera.env and ./.chimera.env)
	agent.LoadConfig()

	// Global flags can be added here
	rootCmd.PersistentFlags().BoolP("verbose", "v", false, "Enable verbose output")
	rootCmd.PersistentFlags().BoolP("quiet", "q", false, "Suppress non-error output")

	// Set custom version template
	rootCmd.SetVersionTemplate(fmt.Sprintf("Chimera %s (built %s)\n", Version, BuildTime))
}

// GetVerbose returns whether verbose mode is enabled
func GetVerbose(cmd *cobra.Command) bool {
	verbose, _ := cmd.Flags().GetBool("verbose")
	return verbose
}

// GetQuiet returns whether quiet mode is enabled
func GetQuiet(cmd *cobra.Command) bool {
	quiet, _ := cmd.Flags().GetBool("quiet")
	return quiet
}
