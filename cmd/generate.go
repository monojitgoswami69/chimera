package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/projectchimera/chimera/internal/agent"
	"github.com/projectchimera/chimera/internal/compose"
	"github.com/projectchimera/chimera/internal/git"
	"github.com/projectchimera/chimera/internal/ports"
	"github.com/projectchimera/chimera/internal/proxy"
	"github.com/projectchimera/chimera/internal/scanner"
	"github.com/projectchimera/chimera/internal/tui"
	"github.com/spf13/cobra"
)

var (
	generateOutput string
	useAgent       bool
)

// generateCmd represents the generate command
var generateCmd = &cobra.Command{
	Use:   "generate <github-repo-url>",
	Short: "Generate Docker Compose configuration without starting containers",
	Long: `Generate the Docker Compose configuration, Dockerfile, and .env files
for a GitHub repository without actually starting containers.

This is a dry-run mode useful for reviewing generated files before running chimera init.

Use --agent to enable the AI-powered agentic pipeline for smarter, project-aware
configuration generation (requires OPENAI_API_KEY, GEMINI_API_KEY, or GROQ_API_KEY).

Examples:
  chimera generate https://github.com/user/repo
  chimera generate https://github.com/user/repo --agent
  chimera generate https://github.com/user/repo --output ./my-output`,
	Args: cobra.ExactArgs(1),
	RunE: runGenerate,
}

func init() {
	rootCmd.AddCommand(generateCmd)
	generateCmd.Flags().StringVar(&generateOutput, "output", "", "Output directory (defaults to /tmp/chimera-generate-<repo>)")
	generateCmd.Flags().BoolVar(&useAgent, "agent", false, "Use AI agent for smarter config generation (requires LLM API key)")
}

// runGenerate executes the generate command logic (dry run — no containers)
func runGenerate(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	repoURL := args[0]
	projectName := proxy.ProjectName(repoURL)

	// Determine output directory
	outputDir := generateOutput
	if outputDir == "" {
		outputDir = filepath.Join(os.TempDir(), fmt.Sprintf("chimera-generate-%s", projectName))
	}

	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("generate: failed to create output directory: %w", err)
	}

	// Clone to temp dir
	tui.PrintInfo("Cloning repository...")
	tmpDir, err := os.MkdirTemp("", "chimera-clone-*")
	if err != nil {
		return fmt.Errorf("generate: failed to create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	gitClient := git.NewClient()
	cloneOpts := &git.CloneOptions{URL: repoURL, TargetDir: tmpDir, Depth: 1}
	if token := os.Getenv("GITHUB_TOKEN"); token != "" {
		cloneOpts.Token = token
	}
	if err := gitClient.Clone(ctx, cloneOpts); err != nil {
		return fmt.Errorf("generate: clone failed: %w", err)
	}
	tui.PrintSuccess("Repository cloned")

	// Check if agent mode should be used
	agentMode := useAgent || os.Getenv("CHIMERA_AGENT") == "1"

	if agentMode {
		return runAgentGenerate(ctx, tmpDir, projectName, outputDir)
	}

	return runTemplateGenerate(ctx, tmpDir, projectName, outputDir)
}

// runAgentGenerate uses the multi-turn LLM pipeline for config generation.
func runAgentGenerate(ctx context.Context, projectDir, projectName, outputDir string) error {
	provider, err := agent.NewProvider()
	if err != nil {
		tui.PrintWarning(fmt.Sprintf("Agent unavailable: %v", err))
		tui.PrintInfo("Falling back to template-based generation...")
		return runTemplateGenerate(ctx, projectDir, projectName, outputDir)
	}

	config, err := agent.Run(ctx, provider, projectDir, projectName)
	if err != nil {
		tui.PrintWarning(fmt.Sprintf("Agent failed: %v", err))
		tui.PrintInfo("Falling back to template-based generation...")
		return runTemplateGenerate(ctx, projectDir, projectName, outputDir)
	}

	// Write agent-generated files
	files := map[string]string{
		"docker-compose.yml": config.DockerCompose,
		"Dockerfile":         config.Dockerfile,
		".env.example":       config.EnvExample,
	}
	for name, content := range files {
		path := filepath.Join(outputDir, name)
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			return fmt.Errorf("generate: failed to write %s: %w", name, err)
		}
	}

	if config.Explanation != "" {
		tui.PrintInfo(fmt.Sprintf("Agent: %s", config.Explanation))
	}
	tui.PrintSuccess(fmt.Sprintf("Files generated at: %s (agent mode)", outputDir))
	return nil
}

// runTemplateGenerate uses the scanner + template pipeline.
func runTemplateGenerate(ctx context.Context, projectDir, projectName, outputDir string) error {
	// Scan
	tui.PrintInfo("Scanning codebase...")
	repoScanner := scanner.NewScanner(projectDir)
	scanResult, err := repoScanner.Scan(ctx)
	if err != nil {
		return fmt.Errorf("generate: scan failed: %w", err)
	}
	tui.PrintSuccess(fmt.Sprintf("Detected: %s", formatLanguages(scanResult)))

	// Generate
	tui.PrintInfo("Generating environment files...")
	manifest, err := compose.Generate(projectName, scanResult)
	if err != nil {
		return fmt.Errorf("generate: generation failed: %w", err)
	}

	// Resolve ports
	desired := compose.DefaultPorts(scanResult)
	_, remaps, _ := ports.ResolveAll(desired)
	if len(remaps) > 0 {
		ports.PrintRemaps(remaps, os.Stdout)
		manifest.DockerCompose, manifest.EnvExample = ports.ApplyRemaps(
			manifest.DockerCompose, manifest.EnvExample, remaps)
	}

	// Write to output directory
	files := map[string]string{
		"docker-compose.yml": manifest.DockerCompose,
		"Dockerfile":         manifest.Dockerfile,
		".env.example":       manifest.EnvExample,
	}
	for name, content := range files {
		path := filepath.Join(outputDir, name)
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			return fmt.Errorf("generate: failed to write %s: %w", name, err)
		}
	}

	tui.PrintSuccess(fmt.Sprintf("Files generated at: %s", outputDir))
	tui.PrintInfo("Review the files, then run 'chimera init' to start the environment.")
	return nil
}

// containerListOpts is a helper for creating container list options filtered by project.
func containerListOpts(project string) types.ContainerListOptions {
	return types.ContainerListOptions{
		All: true,
		Filters: filters.NewArgs(
			filters.Arg("label", "com.chimera.project="+project),
		),
	}
}
