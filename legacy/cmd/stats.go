package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/docker/docker/client"
	"github.com/projectchimera/chimera/internal/tui"
	"github.com/spf13/cobra"

	tea "github.com/charmbracelet/bubbletea"
)

var (
	statsProject string
	statsOnce    bool
)

// statsCmd represents the stats command
var statsCmd = &cobra.Command{
	Use:   "stats",
	Short: "Display live container statistics dashboard",
	Long: `Launch a live TUI dashboard showing real-time CPU, memory, and network
statistics for all containers in the Chimera workspace.

The dashboard updates every 2 seconds and supports terminal resize.

Examples:
  chimera stats
  chimera stats --project my-app
  chimera stats --once`,
	RunE: runStats,
}

func init() {
	rootCmd.AddCommand(statsCmd)
	statsCmd.Flags().StringVar(&statsProject, "project", "", "Project name (auto-detected from .chimera file)")
	statsCmd.Flags().BoolVar(&statsOnce, "once", false, "Print one snapshot and exit (for CI use)")
}

// runStats executes the stats command logic
func runStats(cmd *cobra.Command, args []string) error {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	// Auto-detect project name
	project := statsProject
	if project == "" {
		project = detectProject()
	}
	if project == "" {
		tui.PrintError("No project found. Use --project flag or run from a chimera workspace.")
		return fmt.Errorf("stats: no project detected")
	}

	// Initialize Docker client
	dockerCli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		tui.PrintError(fmt.Sprintf("Failed to connect to Docker: %v", err))
		return fmt.Errorf("stats: docker connection failed: %w", err)
	}
	defer dockerCli.Close()

	model := tui.NewStatsModel(ctx, dockerCli, project, statsOnce)

	var p *tea.Program
	if statsOnce {
		p = tea.NewProgram(model)
	} else {
		p = tea.NewProgram(model, tea.WithAltScreen())
	}

	if _, err := p.Run(); err != nil {
		return fmt.Errorf("stats: TUI error: %w", err)
	}

	return nil
}

// detectProject reads the .chimera file in the current directory to detect the project name.
func detectProject() string {
	data, err := os.ReadFile(".chimera")
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}
