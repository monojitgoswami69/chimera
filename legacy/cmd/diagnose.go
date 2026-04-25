package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"runtime"
	"syscall"

	"github.com/docker/docker/client"
	"github.com/projectchimera/chimera/internal/healer"
	"github.com/projectchimera/chimera/internal/tui"
	"github.com/spf13/cobra"
)

var diagnoseProject string

// diagnoseCmd represents the diagnose command
var diagnoseCmd = &cobra.Command{
	Use:   "diagnose",
	Short: "Run AI-powered diagnostics on failing containers",
	Long: `Scan for containers with non-zero exit codes and trigger an AI-powered
diagnosis using the configured LLM provider.

Set CHIMERA_LLM_PROVIDER (openai|gemini|groq) and the corresponding API key
environment variable to enable AI diagnostics.

Examples:
  chimera diagnose
  chimera diagnose --project my-app`,
	RunE: runDiagnose,
}

func init() {
	rootCmd.AddCommand(diagnoseCmd)
	diagnoseCmd.Flags().StringVar(&diagnoseProject, "project", "", "Project name (auto-detected from .chimera file)")
}

// runDiagnose executes the diagnose command logic
func runDiagnose(cmd *cobra.Command, args []string) error {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	tui.PrintHeader("Chimera Diagnostics")
	fmt.Println()

	// System information
	tui.PrintSubheader("System Information")
	tui.PrintListItem(fmt.Sprintf("OS: %s", runtime.GOOS))
	tui.PrintListItem(fmt.Sprintf("Architecture: %s", runtime.GOARCH))
	tui.PrintListItem(fmt.Sprintf("Go Version: %s", runtime.Version()))
	tui.PrintListItem(fmt.Sprintf("Chimera Version: %s", Version))
	fmt.Println()

	// Docker connectivity
	tui.PrintSubheader("Docker Connectivity")
	dockerCli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		tui.PrintError(fmt.Sprintf("✗ Docker daemon not accessible: %v", err))
		tui.PrintInfo("  Make sure Docker is installed and running")
		return fmt.Errorf("diagnose: docker not available")
	}
	defer dockerCli.Close()

	version, err := dockerCli.ServerVersion(ctx)
	if err != nil {
		tui.PrintError(fmt.Sprintf("✗ Cannot reach Docker daemon: %v", err))
		return fmt.Errorf("diagnose: docker not reachable")
	}
	tui.PrintSuccess(fmt.Sprintf("✓ Docker %s", version.Version))
	fmt.Println()

	// Project detection
	project := diagnoseProject
	if project == "" {
		project = detectProject()
	}
	if project == "" {
		tui.PrintWarning("No project detected. Use --project flag to specify.")
		tui.PrintInfo("General system diagnostics completed.")
		return nil
	}

	// Check for failed containers
	tui.PrintSubheader(fmt.Sprintf("Project: %s", project))
	containers, err := dockerCli.ContainerList(ctx, containerListOpts(project))
	if err != nil {
		tui.PrintError(fmt.Sprintf("Failed to list containers: %v", err))
		return err
	}

	if len(containers) == 0 {
		tui.PrintWarning("No containers found for this project")
		return nil
	}

	tui.PrintListItem(fmt.Sprintf("Found %d container(s)", len(containers)))

	// Find failed containers and run AI diagnosis
	failedCount := 0
	for _, c := range containers {
		if c.State != "running" {
			failedCount++
			tui.PrintWarning(fmt.Sprintf("Container %s is %s", c.ID[:12], c.State))

			// Attempt AI diagnosis
			logs, err := healer.CaptureLogs(ctx, dockerCli, c.ID)
			if err != nil {
				tui.PrintError(fmt.Sprintf("  Cannot capture logs: %v", err))
				continue
			}

			provider, err := healer.NewProvider(ctx)
			if err != nil {
				tui.PrintWarning(fmt.Sprintf("  AI diagnosis unavailable: %v", err))
				tui.PrintInfo("  Set CHIMERA_LLM_PROVIDER and the corresponding API key env var")
				continue
			}

			safeVars, _ := healer.ScrubSecrets(nil, "")
			diagnosis, err := provider.Diagnose(ctx, healer.DiagRequest{
				Logs:          logs,
				EnvVarNames:   safeVars,
				ContainerName: c.ID[:12],
			})
			if err != nil {
				tui.PrintError(fmt.Sprintf("  Diagnosis failed: %v", err))
				continue
			}

			healer.RenderDiagnosis(diagnosis, c.ID[:12], os.Stdout)
		}
	}

	if failedCount == 0 {
		tui.PrintSuccess("✓ All containers are running healthy")
	}

	return nil
}
