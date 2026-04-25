package cmd

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/docker/docker/client"
	"github.com/projectchimera/chimera/internal/agent"
	"github.com/projectchimera/chimera/internal/compose"
	"github.com/projectchimera/chimera/internal/git"
	"github.com/projectchimera/chimera/internal/healer"
	"github.com/projectchimera/chimera/internal/ports"
	"github.com/projectchimera/chimera/internal/proxy"
	"github.com/projectchimera/chimera/internal/scanner"
	"github.com/projectchimera/chimera/internal/tui"
	"github.com/spf13/cobra"
)

var (
	// initCwd is the working directory for the init command
	initCwd string
	// initForce forces re-initialization even if workspace exists
	initForce bool
	// initCreateProxy enables Caddy proxy and /etc/hosts setup
	initCreateProxy bool
	// initDockerRun starts Docker containers after generation
	initDockerRun bool
	// initNoAgent disables AI agent for config generation
	initNoAgent bool
)

// initCmd represents the init command
var initCmd = &cobra.Command{
	Use:   "init <github-repo-url>",
	Short: "Initialize a new Chimera workspace from a GitHub repository",
	Long: `Initialize a new Chimera workspace by cloning a GitHub repository,
analyzing its infrastructure dependencies, and generating a fully containerized
local development environment configuration.

By default, this command will:
  1. Clone the repository (supports private repos via GITHUB_TOKEN or SSH)
  2. Use AI agent to analyze the codebase (use --no-agent to disable)
  3. Generate docker-compose.yml, Dockerfile, .env.example, and quick_start_guide.txt
  4. Create configuration files WITHOUT starting containers

Optional flags allow you to:
  - Start Docker containers immediately (--docker-run)
  - Set up reverse proxy and local domain (--create-proxy)

Examples:
  chimera init https://github.com/user/repo
  chimera init https://github.com/user/repo --docker-run
  chimera init https://github.com/user/repo --docker-run --create-proxy
  chimera init https://github.com/user/repo --no-agent
  chimera init git@github.com:user/private-repo.git
  chimera init https://github.com/user/repo --cwd ./my-workspace`,
	Args: cobra.ExactArgs(1),
	RunE: runInit,
}

func init() {
	rootCmd.AddCommand(initCmd)
	initCmd.Flags().StringVar(&initCwd, "cwd", "", "Working directory for the workspace (defaults to ./chimera-<repo-name>)")
	initCmd.Flags().BoolVar(&initForce, "force", false, "Force re-initialization even if workspace exists")
	initCmd.Flags().BoolVar(&initCreateProxy, "create-proxy", false, "Set up Caddy proxy and /etc/hosts entry")
	initCmd.Flags().BoolVar(&initDockerRun, "docker-run", false, "Start Docker containers after generating configs")
	initCmd.Flags().BoolVar(&initNoAgent, "no-agent", false, "Disable AI agent (use template-based generation)")
}

// runInit executes the full chimera init flow.
func runInit(cmd *cobra.Command, args []string) error {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	repoURL := args[0]
	projectName := proxy.ProjectName(repoURL)

	// Phase 1: Determine workspace directory
	workspaceDir, err := determineWorkspaceDir(repoURL, initCwd)
	if err != nil {
		tui.PrintError(fmt.Sprintf("Failed to determine workspace: %v", err))
		return err
	}

	if _, err := os.Stat(workspaceDir); err == nil && !initForce {
		tui.PrintError(fmt.Sprintf("Workspace already exists at %s. Use --force to re-initialize.", workspaceDir))
		return fmt.Errorf("workspace already exists")
	}

	if err := os.MkdirAll(workspaceDir, 0755); err != nil {
		return fmt.Errorf("init: failed to create workspace: %w", err)
	}

	// Phase 2: Clone
	tui.PrintInfo("Cloning repository...")
	gitClient := git.NewClient()
	cloneOpts := &git.CloneOptions{
		URL:       repoURL,
		TargetDir: workspaceDir,
		Depth:     1,
	}
	if token := os.Getenv("GITHUB_TOKEN"); token != "" {
		cloneOpts.Token = token
	}

	startTime := time.Now()
	if err := gitClient.Clone(ctx, cloneOpts); err != nil {
		os.RemoveAll(workspaceDir)
		tui.PrintError(fmt.Sprintf("Clone failed: %v", err))
		return fmt.Errorf("init: clone failed: %w", err)
	}
	tui.PrintSuccess(fmt.Sprintf("Repository cloned in %v", time.Since(startTime).Round(time.Millisecond)))

	// Phase 3-6: Generate config (agent mode by default, unless --no-agent)
	agentMode := !initNoAgent && os.Getenv("CHIMERA_AGENT") != "0"
	var manifest *compose.ComposeManifest
	var scanResult *scanner.ScanResult
	var quickStartGuide string

	if initNoAgent {
		tui.PrintWarning("⚠ Running without AI agent")
		tui.PrintWarning("  → Semantic understanding of the repo is not available")
		tui.PrintWarning("  → Outputs are not guaranteed to be consistent")
		tui.PrintWarning("  → Quick start guide will not be generated")
		fmt.Println()
	}

	// Phase 3: Scan (required for template mode, best effort for agent mode)
	tui.PrintInfo("Scanning codebase...")
	repoScanner := scanner.NewScanner(workspaceDir)
	var scanErr error
	scanResult, scanErr = repoScanner.Scan(ctx)
	if scanErr != nil {
		if agentMode {
			tui.PrintWarning(fmt.Sprintf("Scan warning: %v", scanErr))
		} else {
			tui.PrintError(fmt.Sprintf("Scan failed: %v", scanErr))
			return fmt.Errorf("init: scan failed: %w", scanErr)
		}
	} else {
		tui.PrintSuccess(fmt.Sprintf("Detected: %s", formatLanguages(scanResult)))
	}

	if agentMode {
		// Try agent-based generation
		prov, provErr := agent.NewProvider()
		if provErr == nil {
			config, agentErr := agent.Run(ctx, prov, workspaceDir, projectName)
			if agentErr == nil {
				if config.Explanation != "" {
					tui.PrintInfo(fmt.Sprintf("Agent: %s", config.Explanation))
				}
				tui.PrintSuccess("Agent-generated config created")

				manifest = &compose.ComposeManifest{
					ProjectName:   projectName,
					DockerCompose: config.DockerCompose,
					Dockerfile:    config.Dockerfile,
					EnvExample:    config.EnvExample,
					AppPort:       3000,
				}
				
				// Generate quick start guide from agent config
				quickStartGuide = generateQuickStartGuide(projectName, config, scanResult)
			} else {
				tui.PrintWarning(fmt.Sprintf("Agent failed: %v — falling back to templates", agentErr))
				agentMode = false
			}
		} else {
			tui.PrintWarning(fmt.Sprintf("Agent unavailable: %v — falling back to templates", provErr))
			agentMode = false
		}
	}

	if manifest == nil {
		if scanResult == nil {
			return fmt.Errorf("init: unable to generate environment without scan results")
		}

		// Phase 4: Generate compose manifest from templates
		tui.PrintInfo("Generating environment...")
		var genErr error
		manifest, genErr = compose.Generate(projectName, scanResult)
		if genErr != nil {
			tui.PrintError(fmt.Sprintf("Generation failed: %v", genErr))
			return fmt.Errorf("init: generate failed: %w", genErr)
		}
		tui.PrintSuccess("docker-compose.yml generated")
		
		// Generate basic quick start guide for template mode
		if !initNoAgent {
			quickStartGuide = generateBasicQuickStartGuide(projectName, manifest, scanResult)
		}
	}

	proxyEnabled := initCreateProxy
	proxyHostPort := 80
	appHostPort, appContainerPort := detectAppPorts(manifest.DockerCompose, manifest.AppPort)
	if appHostPort <= 0 {
		appHostPort = manifest.AppPort
	}
	if appContainerPort > 0 {
		manifest.AppPort = appContainerPort
	}

	if proxyEnabled {
		manifest.DockerCompose = ensureCaddyService(manifest.DockerCompose, projectName)
	}

	// Phase 5: Resolve port conflicts
	tui.PrintInfo("Checking for port conflicts...")
	desired := map[string]int{"app": appHostPort}
	if scanResult != nil {
		desired = compose.DefaultPorts(scanResult)
		desired["app"] = appHostPort
	}
	if proxyEnabled {
		desired["caddy-http"] = 80
		desired["caddy-https"] = 443
	}

	_, remaps, portErr := ports.ResolveAll(desired)
	if portErr != nil {
		tui.PrintWarning(fmt.Sprintf("Port resolution issue: %v", portErr))
	}
	if len(remaps) > 0 {
		ports.PrintRemaps(remaps, os.Stdout)
		manifest.DockerCompose, manifest.EnvExample = ports.ApplyRemaps(
			manifest.DockerCompose, manifest.EnvExample, remaps)
		for _, remap := range remaps {
			switch remap.Service {
			case "app":
				appHostPort = remap.To
			case "caddy-http":
				proxyHostPort = remap.To
			}
		}
	}
	tui.PrintSuccess("Ports resolved")

	// Phase 6: Write files
	if err := writeManifest(workspaceDir, projectName, manifest, quickStartGuide); err != nil {
		return fmt.Errorf("init: failed to write files: %w", err)
	}
	tui.PrintSuccess("Files written to workspace")

	// Phase 7: Add /etc/hosts entry (only if --create-proxy)
	if proxyEnabled {
		domain := proxy.DeriveLocalDomain(repoURL)
		// Write Caddyfile
		caddyContent := proxy.GenerateCaddyfile(domain, manifest.AppPort)
		if err := os.WriteFile(filepath.Join(workspaceDir, "Caddyfile"), []byte(caddyContent), 0644); err != nil {
			return fmt.Errorf("init: failed to write Caddyfile: %w", err)
		}

		if err := proxy.AddEntry(domain, projectName); err != nil {
			tui.PrintWarning(fmt.Sprintf("Could not update /etc/hosts: %v", err))
		} else {
			tui.PrintSuccess(fmt.Sprintf("Local domain: %s", domain))
		}
	}

	// Phase 8: Start containers via docker compose (only if --docker-run)
	if initDockerRun {
		tui.PrintInfo("Starting environment...")
		if err := dockerComposeUp(ctx, workspaceDir, projectName); err != nil {
			tui.PrintError(fmt.Sprintf("Failed to start environment: %v", err))
			return fmt.Errorf("init: docker compose up failed: %w", err)
		}
		tui.PrintSuccess("All containers started")

		// Phase 9: Watch for failures (background AI healer) - only if docker is running
		go func() {
			dockerCli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
			if err != nil {
				return
			}
			defer dockerCli.Close()

			healer.Watch(ctx, dockerCli, projectName, func(containerID string) {
				logs, _ := healer.CaptureLogs(ctx, dockerCli, containerID)
				var envVars []string
				if scanResult != nil {
					envVars = scanResult.EnvVars
				}
				safeVars, safeCompose := healer.ScrubSecrets(envVars, manifest.DockerCompose)
				provider, err := healer.NewProvider(ctx)
				if err != nil {
					return
				}
				diagnosis, err := provider.Diagnose(ctx, healer.DiagRequest{
					Logs:          logs,
					ComposeYAML:   safeCompose,
					EnvVarNames:   safeVars,
					ContainerName: containerID,
				})
				if err != nil {
					return
				}
				healer.RenderDiagnosis(diagnosis, containerID, os.Stderr)
			})
		}()
	}

	// Phase 10: Print ready summary
	fmt.Println()
	printReadySummary(projectName, manifest, workspaceDir, proxyEnabled, initDockerRun, proxyHostPort, appHostPort)

	if initDockerRun && !GetQuiet(cmd) {
		tui.PrintInfo("Press Ctrl+C to stop watching...")
		<-ctx.Done()
	}

	return nil
}

var appPortBindingRegex = regexp.MustCompile(`-\s*"(\d{2,5}):(\d{2,5})"`)

// detectAppPorts attempts to infer app host and container ports from compose YAML.
func detectAppPorts(composeYAML string, fallback int) (int, int) {
	hostPort := fallback
	containerPort := fallback

	lines := strings.Split(composeYAML, "\n")
	inApp := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		if strings.HasPrefix(line, "  app:") {
			inApp = true
			continue
		}

		if inApp && strings.HasPrefix(line, "  ") && !strings.HasPrefix(line, "    ") && strings.HasSuffix(trimmed, ":") {
			break
		}

		if !inApp {
			continue
		}

		match := appPortBindingRegex.FindStringSubmatch(trimmed)
		if len(match) == 3 {
			host, errHost := strconv.Atoi(match[1])
			container, errContainer := strconv.Atoi(match[2])
			if errHost == nil {
				hostPort = host
			}
			if errContainer == nil {
				containerPort = container
			}
			break
		}
	}

	return hostPort, containerPort
}

// detectAppNetwork returns the first app service network, if present.
func detectAppNetwork(composeYAML string) string {
	lines := strings.Split(composeYAML, "\n")
	inApp := false
	inNetworks := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		if strings.HasPrefix(line, "  app:") {
			inApp = true
			continue
		}

		if inApp && strings.HasPrefix(line, "  ") && !strings.HasPrefix(line, "    ") && strings.HasSuffix(trimmed, ":") {
			break
		}

		if !inApp {
			continue
		}

		if strings.HasPrefix(line, "    networks:") {
			inNetworks = true
			continue
		}

		if !inNetworks {
			continue
		}

		if strings.HasPrefix(line, "      - ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "      - "))
		}

		if strings.HasPrefix(line, "      ") && strings.HasSuffix(trimmed, ":") {
			return strings.TrimSuffix(strings.TrimSpace(line), ":")
		}

		if !strings.HasPrefix(line, "      ") {
			inNetworks = false
		}
	}

	return ""
}

// ensureCaddyService injects a Caddy service into compose YAML if not already present.
func ensureCaddyService(composeYAML, projectName string) string {
	if strings.Contains(composeYAML, "\n  caddy:\n") {
		return composeYAML
	}

	netName := detectAppNetwork(composeYAML)
	if netName == "" && strings.Contains(composeYAML, "\nnetworks:\n") {
		netName = projectName + "_net"
	}

	snippet := proxy.CaddyComposeSnippet(projectName, netName)
	idx := strings.Index(composeYAML, "\nnetworks:\n")
	if idx == -1 {
		return strings.TrimRight(composeYAML, "\n") + snippet + "\n"
	}

	return composeYAML[:idx] + snippet + composeYAML[idx:]
}

// determineWorkspaceDir determines the workspace directory.
func determineWorkspaceDir(repoURL, cwdFlag string) (string, error) {
	if cwdFlag != "" {
		return filepath.Abs(cwdFlag)
	}
	repoName := git.ExtractRepoName(repoURL)
	if repoName == "" {
		return "", fmt.Errorf("init: could not extract repo name from URL: %s", repoURL)
	}
	return filepath.Abs(filepath.Join(".", fmt.Sprintf("chimera-%s", repoName)))
}

// writeManifest writes all generated files to the workspace.
func writeManifest(dir, projectName string, m *compose.ComposeManifest, quickStartGuide string) error {
	files := map[string]string{
		"docker-compose.yml": m.DockerCompose,
		"Dockerfile":         m.Dockerfile,
		".env.example":       m.EnvExample,
		".env":               m.EnvExample,
	}
	
	if quickStartGuide != "" {
		files["quick_start_guide.txt"] = quickStartGuide
	}
	
	for name, content := range files {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			return fmt.Errorf("init: failed to write %s: %w", name, err)
		}
	}

	// Write .chimera project marker file
	chimeraFile := filepath.Join(dir, ".chimera")
	return os.WriteFile(chimeraFile, []byte(projectName+"\n"), 0644)
}

// dockerComposeUp runs docker compose up in the workspace directory.
func dockerComposeUp(ctx context.Context, workspaceDir, projectName string) error {
	composeCmd := exec.CommandContext(ctx,
		"docker", "compose",
		"-f", filepath.Join(workspaceDir, "docker-compose.yml"),
		"-p", projectName,
		"up", "-d", "--build", "--remove-orphans",
	)
	composeCmd.Dir = workspaceDir
	composeCmd.Stdout = os.Stdout
	composeCmd.Stderr = os.Stderr

	return composeCmd.Run()
}

// formatLanguages returns a human-readable string of detected languages.
func formatLanguages(result *scanner.ScanResult) string {
	names := make([]string, 0, len(result.Languages))
	for _, l := range result.Languages {
		names = append(names, fmt.Sprintf("%s %s", l.Name, l.Version))
	}
	if len(names) == 0 {
		return "no languages"
	}
	s := ""
	for i, n := range names {
		if i > 0 {
			s += ", "
		}
		s += n
	}
	return s
}

// printReadySummary prints the final ready message.
func printReadySummary(projectName string, manifest *compose.ComposeManifest, workspaceDir string, proxyEnabled bool, dockerRunning bool, proxyHostPort int, appHostPort int) {
	successStyle := tui.SuccessStyle
	infoStyle := tui.InfoStyle

	fmt.Println(successStyle.Render("  ✓ Configuration generated!"))
	fmt.Println()

	fmt.Println(infoStyle.Render(fmt.Sprintf("  Workspace:   %s", workspaceDir)))
	fmt.Println(infoStyle.Render("  Files created:"))
	fmt.Println(infoStyle.Render("    • docker-compose.yml"))
	fmt.Println(infoStyle.Render("    • Dockerfile"))
	fmt.Println(infoStyle.Render("    • .env.example"))
	fmt.Println(infoStyle.Render("    • .env"))
	fmt.Println(infoStyle.Render("    • quick_start_guide.txt"))
	if proxyEnabled {
		fmt.Println(infoStyle.Render("    • Caddyfile"))
	}
	fmt.Println()

	if dockerRunning {
		fmt.Println(successStyle.Render("  ✓ Docker containers running!"))
		fmt.Println()
		
		if proxyEnabled {
			domain := projectName + ".local"
			if proxyHostPort == 80 {
				fmt.Println(infoStyle.Render(fmt.Sprintf("  Local URL:   http://%s", domain)))
			} else {
				fmt.Println(infoStyle.Render(fmt.Sprintf("  Local URL:   http://%s:%d", domain, proxyHostPort)))
			}
		} else {
			fmt.Println(infoStyle.Render(fmt.Sprintf("  Local URL:   http://localhost:%d", appHostPort)))
		}

		if len(manifest.InfraServices) > 0 {
			svc := ""
			for i, s := range manifest.InfraServices {
				if i > 0 {
					svc += ", "
				}
				svc += s
			}
			fmt.Println(infoStyle.Render(fmt.Sprintf("  Services:    %s", svc)))
		}
		fmt.Println()
		fmt.Println(infoStyle.Render("  Dashboard:   chimera stats"))
		fmt.Println(infoStyle.Render("  Cleanup:     chimera nuke"))
	} else {
		fmt.Println(infoStyle.Render("  📖 Next steps:"))
		fmt.Println(infoStyle.Render(fmt.Sprintf("    1. Review %s/quick_start_guide.txt", workspaceDir)))
		fmt.Println(infoStyle.Render(fmt.Sprintf("    2. Edit %s/.env with your secrets", workspaceDir)))
		fmt.Println(infoStyle.Render(fmt.Sprintf("    3. cd %s", workspaceDir)))
		fmt.Println(infoStyle.Render("    4. Choose your startup method:"))
		fmt.Println()
		fmt.Println(infoStyle.Render("       With Docker:"))
		fmt.Println(infoStyle.Render("         docker compose up -d"))
		fmt.Println()
		fmt.Println(infoStyle.Render("       Or re-run with Docker:"))
		fmt.Println(infoStyle.Render(fmt.Sprintf("         chimera init <repo-url> --docker-run --force")))
		fmt.Println()
		fmt.Println(infoStyle.Render("       Without Docker:"))
		fmt.Println(infoStyle.Render("         See quick_start_guide.txt for manual setup"))
	}
	fmt.Println()
}

// generateQuickStartGuide creates a comprehensive quick start guide from agent config
func generateQuickStartGuide(projectName string, config *agent.Config, scanResult *scanner.ScanResult) string {
	var b strings.Builder
	
	b.WriteString("═══════════════════════════════════════════════════════════════\n")
	b.WriteString(fmt.Sprintf("  QUICK START GUIDE - %s\n", strings.ToUpper(projectName)))
	b.WriteString("═══════════════════════════════════════════════════════════════\n\n")
	
	if config.Explanation != "" {
		b.WriteString("PROJECT OVERVIEW:\n")
		b.WriteString(fmt.Sprintf("  %s\n\n", config.Explanation))
	}
	
	// Detected technologies
	b.WriteString("DETECTED TECHNOLOGIES:\n")
	if scanResult != nil && len(scanResult.Languages) > 0 {
		for _, lang := range scanResult.Languages {
			b.WriteString(fmt.Sprintf("  • %s %s\n", lang.Name, lang.Version))
		}
	}
	if len(config.Services) > 0 {
		b.WriteString("\nSERVICES:\n")
		for _, svc := range config.Services {
			b.WriteString(fmt.Sprintf("  • %s (%s) - Port %d\n", svc.Name, svc.Type, svc.Port))
		}
	}
	b.WriteString("\n")
	
	// Prerequisites
	b.WriteString("═══════════════════════════════════════════════════════════════\n")
	b.WriteString("PREREQUISITES\n")
	b.WriteString("═══════════════════════════════════════════════════════════════\n\n")
	b.WriteString("For Docker setup:\n")
	b.WriteString("  • Docker 20.10+ installed\n")
	b.WriteString("  • Docker Compose V2\n\n")
	b.WriteString("For manual setup:\n")
	if scanResult != nil {
		for _, lang := range scanResult.Languages {
			switch lang.Name {
			case "Node.js":
				b.WriteString(fmt.Sprintf("  • Node.js %s or higher\n", lang.Version))
				b.WriteString("  • npm or yarn package manager\n")
			case "Python":
				b.WriteString(fmt.Sprintf("  • Python %s or higher\n", lang.Version))
				b.WriteString("  • pip package manager\n")
			case "Go":
				b.WriteString(fmt.Sprintf("  • Go %s or higher\n", lang.Version))
			}
		}
	}
	b.WriteString("\n")
	
	// Option 1: Docker
	b.WriteString("═══════════════════════════════════════════════════════════════\n")
	b.WriteString("OPTION 1: RUN WITH DOCKER (RECOMMENDED)\n")
	b.WriteString("═══════════════════════════════════════════════════════════════\n\n")
	b.WriteString("1. Configure environment variables:\n")
	b.WriteString("   cp .env.example .env\n")
	b.WriteString("   nano .env  # Edit with your secrets\n\n")
	b.WriteString("2. Start all services:\n")
	b.WriteString("   docker compose up -d\n\n")
	b.WriteString("3. View logs:\n")
	b.WriteString("   docker compose logs -f\n\n")
	b.WriteString("4. Stop services:\n")
	b.WriteString("   docker compose down\n\n")
	b.WriteString("5. Stop and remove volumes (⚠ deletes data):\n")
	b.WriteString("   docker compose down -v\n\n")
	
	// Option 2: Manual
	b.WriteString("═══════════════════════════════════════════════════════════════\n")
	b.WriteString("OPTION 2: RUN WITHOUT DOCKER (MANUAL SETUP)\n")
	b.WriteString("═══════════════════════════════════════════════════════════════\n\n")
	
	if scanResult != nil {
		hasNode := false
		hasPython := false
		hasGo := false
		
		for _, lang := range scanResult.Languages {
			switch lang.Name {
			case "Node.js":
				hasNode = true
			case "Python":
				hasPython = true
			case "Go":
				hasGo = true
			}
		}
		
		// Infrastructure services
		if scanResult.Infrastructure != nil && len(scanResult.Infrastructure) > 0 {
			b.WriteString("1. Start infrastructure services:\n\n")
			for _, infra := range scanResult.Infrastructure {
				switch infra.Type {
				case "postgresql":
					b.WriteString("   PostgreSQL:\n")
					b.WriteString("     # Using Docker:\n")
					b.WriteString("     docker run -d --name postgres -p 5432:5432 \\\n")
					b.WriteString("       -e POSTGRES_PASSWORD=postgres postgres:15-alpine\n\n")
					b.WriteString("     # Or install locally:\n")
					b.WriteString("     # macOS: brew install postgresql@15\n")
					b.WriteString("     # Ubuntu: sudo apt install postgresql-15\n\n")
				case "redis":
					b.WriteString("   Redis:\n")
					b.WriteString("     # Using Docker:\n")
					b.WriteString("     docker run -d --name redis -p 6379:6379 redis:7-alpine\n\n")
					b.WriteString("     # Or install locally:\n")
					b.WriteString("     # macOS: brew install redis\n")
					b.WriteString("     # Ubuntu: sudo apt install redis-server\n\n")
				case "mongodb":
					b.WriteString("   MongoDB:\n")
					b.WriteString("     # Using Docker:\n")
					b.WriteString("     docker run -d --name mongodb -p 27017:27017 mongo:7\n\n")
					b.WriteString("     # Or install locally:\n")
					b.WriteString("     # macOS: brew install mongodb-community\n")
					b.WriteString("     # Ubuntu: sudo apt install mongodb\n\n")
				case "mysql":
					b.WriteString("   MySQL:\n")
					b.WriteString("     # Using Docker:\n")
					b.WriteString("     docker run -d --name mysql -p 3306:3306 \\\n")
					b.WriteString("       -e MYSQL_ROOT_PASSWORD=root mysql:8-alpine\n\n")
				}
			}
		}
		
		stepNum := 2
		if len(scanResult.Infrastructure) == 0 {
			stepNum = 1
		}
		
		// Application setup
		b.WriteString(fmt.Sprintf("%d. Configure environment:\n", stepNum))
		b.WriteString("   cp .env.example .env\n")
		b.WriteString("   nano .env  # Edit with your configuration\n\n")
		stepNum++
		
		if hasNode {
			b.WriteString(fmt.Sprintf("%d. Install Node.js dependencies:\n", stepNum))
			nodeFiles := []string{}
			for _, lang := range scanResult.Languages {
				if lang.Name == "Node.js" {
					nodeFiles = lang.Files
					break
				}
			}
			if len(nodeFiles) > 1 {
				b.WriteString("   # Multiple Node.js projects detected:\n")
				for _, file := range nodeFiles {
					dir := filepath.Dir(file)
					if dir == "." {
						b.WriteString("   npm install\n")
					} else {
						b.WriteString(fmt.Sprintf("   cd %s && npm install && cd ..\n", dir))
					}
				}
			} else {
				b.WriteString("   npm install\n")
			}
			b.WriteString("\n")
			stepNum++
		}
		
		if hasPython {
			b.WriteString(fmt.Sprintf("%d. Install Python dependencies:\n", stepNum))
			b.WriteString("   python -m venv venv\n")
			b.WriteString("   source venv/bin/activate  # On Windows: venv\\Scripts\\activate\n")
			pythonFiles := []string{}
			for _, lang := range scanResult.Languages {
				if lang.Name == "Python" {
					pythonFiles = lang.Files
					break
				}
			}
			if len(pythonFiles) > 0 {
				reqFile := pythonFiles[0]
				if strings.Contains(reqFile, "/") {
					dir := filepath.Dir(reqFile)
					b.WriteString(fmt.Sprintf("   pip install -r %s/%s\n", dir, filepath.Base(reqFile)))
				} else {
					b.WriteString(fmt.Sprintf("   pip install -r %s\n", reqFile))
				}
			}
			b.WriteString("\n")
			stepNum++
		}
		
		if hasGo {
			b.WriteString(fmt.Sprintf("%d. Install Go dependencies:\n", stepNum))
			b.WriteString("   go mod download\n\n")
			stepNum++
		}
		
		// Start commands
		b.WriteString(fmt.Sprintf("%d. Start the application:\n\n", stepNum))
		
		if hasNode {
			b.WriteString("   Node.js:\n")
			nodeFiles := []string{}
			for _, lang := range scanResult.Languages {
				if lang.Name == "Node.js" {
					nodeFiles = lang.Files
					break
				}
			}
			if len(nodeFiles) > 1 {
				for _, file := range nodeFiles {
					dir := filepath.Dir(file)
					if dir == "." {
						b.WriteString("     npm start\n")
					} else {
						b.WriteString(fmt.Sprintf("     # In %s/:\n", dir))
						b.WriteString(fmt.Sprintf("     cd %s && npm start\n", dir))
					}
				}
			} else {
				b.WriteString("     npm start\n")
				b.WriteString("     # Or for development:\n")
				b.WriteString("     npm run dev\n")
			}
			b.WriteString("\n")
		}
		
		if hasPython {
			b.WriteString("   Python:\n")
			pythonFiles := []string{}
			for _, lang := range scanResult.Languages {
				if lang.Name == "Python" {
					pythonFiles = lang.Files
					break
				}
			}
			if len(pythonFiles) > 0 && strings.Contains(pythonFiles[0], "/") {
				dir := filepath.Dir(pythonFiles[0])
				b.WriteString(fmt.Sprintf("     cd %s\n", dir))
			}
			b.WriteString("     # FastAPI/Uvicorn:\n")
			b.WriteString("     uvicorn main:app --reload --host 0.0.0.0 --port 8000\n\n")
			b.WriteString("     # Flask:\n")
			b.WriteString("     flask run --host 0.0.0.0 --port 8000\n\n")
			b.WriteString("     # Django:\n")
			b.WriteString("     python manage.py runserver 0.0.0.0:8000\n\n")
		}
		
		if hasGo {
			b.WriteString("   Go:\n")
			b.WriteString("     go run .\n")
			b.WriteString("     # Or build and run:\n")
			b.WriteString("     go build -o app && ./app\n\n")
		}
	}
	
	// Troubleshooting
	b.WriteString("═══════════════════════════════════════════════════════════════\n")
	b.WriteString("TROUBLESHOOTING\n")
	b.WriteString("═══════════════════════════════════════════════════════════════\n\n")
	b.WriteString("Port already in use:\n")
	b.WriteString("  • Check: lsof -i :<port> (macOS/Linux) or netstat -ano | findstr :<port> (Windows)\n")
	b.WriteString("  • Kill process or change port in .env\n\n")
	b.WriteString("Database connection failed:\n")
	b.WriteString("  • Verify database is running\n")
	b.WriteString("  • Check DATABASE_URL in .env\n")
	b.WriteString("  • Ensure database exists: CREATE DATABASE <dbname>;\n\n")
	b.WriteString("Permission denied:\n")
	b.WriteString("  • Docker: Add user to docker group or use sudo\n")
	b.WriteString("  • Files: Check file permissions with ls -la\n\n")
	
	// Useful commands
	b.WriteString("═══════════════════════════════════════════════════════════════\n")
	b.WriteString("USEFUL COMMANDS\n")
	b.WriteString("═══════════════════════════════════════════════════════════════\n\n")
	b.WriteString("Chimera commands:\n")
	b.WriteString("  chimera stats              # View live container stats\n")
	b.WriteString("  chimera diagnose           # AI-powered diagnostics\n")
	b.WriteString("  chimera nuke               # Complete cleanup\n\n")
	b.WriteString("Docker commands:\n")
	b.WriteString("  docker compose ps          # List running containers\n")
	b.WriteString("  docker compose logs -f     # Follow logs\n")
	b.WriteString("  docker compose restart     # Restart services\n")
	b.WriteString("  docker compose exec app sh # Shell into app container\n\n")
	
	b.WriteString("═══════════════════════════════════════════════════════════════\n")
	b.WriteString("Generated by Chimera - Autonomous Environment Orchestration\n")
	b.WriteString("═══════════════════════════════════════════════════════════════\n")
	
	return b.String()
}

// generateBasicQuickStartGuide creates a basic guide for template-based generation
func generateBasicQuickStartGuide(projectName string, manifest *compose.ComposeManifest, scanResult *scanner.ScanResult) string {
	var b strings.Builder
	
	b.WriteString("═══════════════════════════════════════════════════════════════\n")
	b.WriteString(fmt.Sprintf("  QUICK START GUIDE - %s\n", strings.ToUpper(projectName)))
	b.WriteString("═══════════════════════════════════════════════════════════════\n\n")
	
	b.WriteString("⚠ NOTE: This guide was generated using template-based detection.\n")
	b.WriteString("   For more accurate instructions, re-run with AI agent enabled.\n\n")
	
	// Basic Docker instructions
	b.WriteString("═══════════════════════════════════════════════════════════════\n")
	b.WriteString("RUN WITH DOCKER\n")
	b.WriteString("═══════════════════════════════════════════════════════════════\n\n")
	b.WriteString("1. Configure environment:\n")
	b.WriteString("   cp .env.example .env\n")
	b.WriteString("   nano .env\n\n")
	b.WriteString("2. Start services:\n")
	b.WriteString("   docker compose up -d\n\n")
	b.WriteString("3. View logs:\n")
	b.WriteString("   docker compose logs -f\n\n")
	b.WriteString("4. Stop services:\n")
	b.WriteString("   docker compose down\n\n")
	
	b.WriteString("═══════════════════════════════════════════════════════════════\n")
	b.WriteString("For detailed manual setup instructions, please refer to your\n")
	b.WriteString("project's README or re-run chimera with AI agent enabled.\n")
	b.WriteString("═══════════════════════════════════════════════════════════════\n")
	
	return b.String()
}
