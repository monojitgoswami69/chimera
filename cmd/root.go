package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	// Version is set during build via ldflags
	Version = "v0.1.0"
	// BuildTime is set during build via ldflags
	BuildTime = "unknown"
)

var rootCmd = &cobra.Command{
	Use:   "chimera",
	Short: "Autonomous environment orchestration for any GitHub repository",
	Long:  `Chimera autonomously clones, analyzes, and generates Docker configurations for GitHub repositories.`,
	Version: Version,
	Run: func(cmd *cobra.Command, args []string) {
		showHelp()
	},
}

func Execute() {
	// Disable default completion command
	rootCmd.CompletionOptions.DisableDefaultCmd = true
	
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	// Disable auto-generated help command (we have our own)
	rootCmd.SetHelpCommand(&cobra.Command{Hidden: true})
	
	// Global flags
	rootCmd.PersistentFlags().BoolP("verbose", "v", false, "Enable verbose output (show LLM responses, detailed logs)")
	rootCmd.PersistentFlags().BoolP("quiet", "q", false, "Suppress non-error output")
	
	// Set custom version template
	rootCmd.SetVersionTemplate(fmt.Sprintf("Chimera %s (built %s)\n", Version, BuildTime))
	
	rootCmd.AddCommand(helpCmd)
	rootCmd.AddCommand(setupCmd)
	rootCmd.AddCommand(initCmd)
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
