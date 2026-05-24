package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"chimera/internal/config"
	"chimera/internal/detector"
	"chimera/internal/envvar"
	"chimera/internal/generator"
	"chimera/internal/git"
	"chimera/internal/llm"
	"chimera/internal/safefs"
	"chimera/internal/tree"
	"chimera/internal/ui"

	"github.com/spf13/cobra"
)

const outputSubdir = "chimera-outputs"

var initCmd = &cobra.Command{
	Use:   "init <github-url>",
	Short: "Clone and orchestrate a GitHub repository",
	Long: `Clone a GitHub repository, analyse its stack, and generate runnable Docker configurations.

URL forms accepted:
  https://github.com/<owner>/<repo>
  https://github.com/<owner>/<repo>.git
  https://github.com/<owner>/<repo>/tree/<branch>[/<subdir>]
  git@github.com:<owner>/<repo>[.git]

Flags:
  --force       Force re-initialisation even if the cloned directory exists
  --no-agent    Disable LLM validation (use static analysis only — works without setup)
  --output      Override the output directory (default: chimera-outputs)
`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		if err := runInit(cmd, args[0]); err != nil {
			fmt.Println()
			fmt.Println(ui.ErrorBox.Render(fmt.Sprintf("Error: %s", err.Error())))
			os.Exit(1)
		}
	},
}

var (
	initForce       bool
	initNoAgent     bool
	initOutputDir   string
	initKeepRepoDir bool
	initMaxServices int
)

func init() {
	initCmd.Flags().BoolVar(&initForce, "force", false, "Force re-initialisation even if directory exists")
	initCmd.Flags().BoolVar(&initNoAgent, "no-agent", false, "Disable LLM validation (use static analysis only)")
	initCmd.Flags().StringVar(&initOutputDir, "output", outputSubdir, "Output sub-directory (relative to the repo root)")
	initCmd.Flags().IntVar(&initMaxServices, "max-services", detector.MaxServices, "Cap on the number of detected services")
}

func runInit(cmd *cobra.Command, repoURL string) error {
	ctx := context.Background()
	start := time.Now()

	verbose := GetVerbose(cmd)
	quiet := GetQuiet(cmd)

	if !quiet {
		fmt.Print(ui.Header(ui.HeaderArgs{Command: "init", Version: Version}))
	}

	// Pre-flight
	parsed, err := git.ParseURL(repoURL)
	if err != nil {
		return err
	}

	var cfg *config.Config
	if initNoAgent {
		cfg = config.LoadOptional() // returns empty struct when not configured
		if !quiet {
			fmt.Println(ui.InfoLine("Static-only mode (--no-agent) — LLM configuration not required."))
		}
	} else {
		cfg, err = config.Load()
		if err != nil {
			return fmt.Errorf("%s\n\nrun  chimera setup  to configure a provider, or pass --no-agent to skip the LLM", err)
		}
		if !quiet {
			fmt.Println(ui.SuccessLine("Configuration loaded"))
		}
	}

	reposDir, err := config.ReposDir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(reposDir, 0755); err != nil {
		return fmt.Errorf("failed to create repos directory: %w", err)
	}
	targetDir := filepath.Join(reposDir, parsed.Repo)

	if _, err := os.Stat(targetDir); err == nil {
		if !initForce {
			return fmt.Errorf("%s already exists — use --force to re-initialise", targetDir)
		}
		if !quiet {
			fmt.Println(ui.WarningLine("Removing existing directory (--force)"))
		}
		if err := os.RemoveAll(targetDir); err != nil {
			return fmt.Errorf("failed to remove existing directory: %w", err)
		}
	}

	// Step 1: clone
	if !quiet {
		fmt.Println(ui.Step(1, 6, "Clone repository"))
	}
	var sp *ui.Spinner
	if !quiet {
		sp = ui.StartSpinner(fmt.Sprintf("Cloning %s...", parsed.Owner+"/"+parsed.Repo))
	}
	cloneErr := git.Clone(ctx, parsed.Clone, targetDir, cfg.GitHubPAT)
	if sp != nil {
		sp.Stop()
	}
	if cloneErr != nil {
		return cloneErr
	}
	fileCount, size, _ := git.CountFiles(targetDir)
	if !quiet {
		fmt.Println(ui.SuccessLine(fmt.Sprintf("%s cloned (%d files · %.1f MB)", parsed.Repo, fileCount, float64(size)/1024/1024)))
	}

	// Step 2: tree
	if !quiet {
		fmt.Println(ui.Step(2, 6, "Generate repository tree"))
	}
	treeStr, totalLines, err := tree.Generate(targetDir, 10000)
	if err != nil {
		return fmt.Errorf("tree generation failed: %w", err)
	}
	if !quiet {
		fmt.Println(ui.SuccessLine(fmt.Sprintf("Tree generated (%d lines)", totalLines)))
		if verbose {
			for _, line := range strings.Split(treeStr, "\n") {
				if line != "" {
					fmt.Println(ui.DimStyle.Render("  " + line))
				}
			}
		}
	}

	// Step 3: detection
	if !quiet {
		fmt.Println(ui.Step(3, 6, "Detect services and frameworks"))
	}
	detection, err := detector.DetectWithCap(targetDir, initMaxServices)
	if err != nil {
		return fmt.Errorf("detection failed: %w", err)
	}
	if len(detection.Services) == 0 {
		fmt.Println(ui.WarningBox.Render(
			"No supported project types detected.\n\n" +
				"Chimera currently supports Node-based (Next, React, Vite, Express, NestJS, etc.) and Python (FastAPI, Flask, Django).\n\n" +
				"If this repo uses a supported technology, please open an issue.",
		))
		return nil
	}
	if !quiet {
		printDetectionTable(detection)
		if detection.TruncatedFrom > 0 {
			fmt.Println(ui.WarningLine(fmt.Sprintf(
				"Found %d candidate services — kept the top %d by confidence. Re-run with --max-services to adjust.",
				detection.TruncatedFrom, len(detection.Services),
			)))
		}
	}

	// LLM validation of detection
	if !initNoAgent {
		var sp *ui.Spinner
		if !quiet {
			sp = ui.StartSpinner("Validating detection with LLM...")
		}
		err := llmValidateDetection(ctx, cfg, detection, treeStr, targetDir, quiet, verbose)
		if sp != nil {
			sp.Stop()
		}
		if err != nil && !quiet {
			fmt.Println(ui.WarningLine(fmt.Sprintf("LLM validation skipped: %v", err)))
		}
	}

	// Step 4: generation
	if !quiet {
		fmt.Println(ui.Step(4, 6, "Generate Dockerfile(s) and docker-compose.yml"))
	}
	out := generator.Build(detection.Services, parsed.Repo, initOutputDir)
	if !quiet {
		for _, name := range generator.SortedFilenames(out.Files) {
			fmt.Println(ui.SuccessLine("  " + initOutputDir + "/" + name))
		}
	}

	// LLM validation of Docker configs (optional)
	if !initNoAgent {
		var sp *ui.Spinner
		if !quiet {
			sp = ui.StartSpinner("Validating Dockerfile and compose with LLM...")
		}
		err := llmValidateDocker(ctx, cfg, out, treeStr, targetDir, quiet, verbose)
		if sp != nil {
			sp.Stop()
		}
		if err != nil && !quiet {
			fmt.Println(ui.WarningLine(fmt.Sprintf("LLM Docker validation skipped: %v", err)))
		}
	}

	// Step 5: env vars
	if !quiet {
		fmt.Println(ui.Step(5, 6, "Extract environment variables"))
	}
	envResults := envvar.Detect(targetDir, detection.Services)
	totalVars := 0
	for _, r := range envResults {
		totalVars += len(r.Vars)
	}
	if !quiet {
		fmt.Println(ui.SuccessLine(fmt.Sprintf("Detected %d variable reference(s) across %d service(s)", totalVars, len(envResults))))
	}
	if !initNoAgent {
		var sp *ui.Spinner
		if !quiet {
			sp = ui.StartSpinner("Enhancing env files with LLM...")
		}
		err := llmValidateEnvVars(ctx, cfg, &envResults, treeStr, targetDir, quiet, verbose)
		if sp != nil {
			sp.Stop()
		}
		if err != nil && !quiet {
			fmt.Println(ui.WarningLine(fmt.Sprintf("LLM env enhancement skipped: %v", err)))
		}
	}

	// Reconcile: if the env content (static or LLM-enhanced) declares a PORT,
	// propagate it to the service so the Dockerfile/compose stay consistent.
	if reconcilePortsFromEnv(detection.Services, envResults) {
		if !quiet {
			fmt.Println(ui.InfoLine("Updated service ports from .env content"))
		}
		out = generator.Build(detection.Services, parsed.Repo, initOutputDir)
	}

	// Step 6: write outputs
	if !quiet {
		fmt.Println(ui.Step(6, 6, "Write outputs"))
	}
	outputDir := filepath.Join(targetDir, initOutputDir)
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	for _, name := range generator.SortedFilenames(out.Files) {
		dest := filepath.Join(outputDir, name)
		if err := os.WriteFile(dest, []byte(out.Files[name]), 0644); err != nil {
			return fmt.Errorf("failed to write %s: %w", name, err)
		}
	}

	// .gitignore so the user doesn't accidentally commit the generated tree
	if err := os.WriteFile(filepath.Join(outputDir, ".gitignore"), []byte("# managed by chimera\n*\n!.gitignore\n!README.md\n!Dockerfile.*\n!docker-compose.yml\n!quick_start.md\n!env-vars/\n"), 0644); err != nil {
		return fmt.Errorf("failed to write .gitignore: %w", err)
	}

	// env files
	for _, envResult := range envResults {
		envDir := filepath.Join(outputDir, "env-vars", envResult.ServiceID)
		if err := os.MkdirAll(envDir, 0755); err != nil {
			return fmt.Errorf("failed to create env-vars directory: %w", err)
		}
		var content string
		if strings.TrimSpace(envResult.EnvContent) != "" {
			content = envResult.EnvContent
		} else {
			content = envvar.GenerateEnvExample(envResult.Vars, envResult.Technology)
		}
		envPath := filepath.Join(envDir, ".env.example")
		if err := os.WriteFile(envPath, []byte(content), 0644); err != nil {
			return fmt.Errorf("failed to write .env.example: %w", err)
		}
		if !quiet {
			fmt.Println(ui.SuccessLine(fmt.Sprintf("  %s/env-vars/%s/.env.example", initOutputDir, envResult.ServiceID)))
		}
	}

	// quick start guide
	quickPath := filepath.Join(outputDir, "quick_start.md")
	if err := os.WriteFile(quickPath, []byte(quickStart(parsed.Repo, detection.Services, initOutputDir)), 0644); err != nil {
		return fmt.Errorf("failed to write quick_start.md: %w", err)
	}
	if !quiet {
		fmt.Println(ui.SuccessLine("  " + initOutputDir + "/quick_start.md"))
	}

	if !quiet {
		duration := time.Since(start)
		var stepsBuilder strings.Builder
		stepsBuilder.WriteString(fmt.Sprintf("Init complete for %s in %.1fs\n\n", ui.HighlightStyle.Render(parsed.Repo), duration.Seconds()))
		stepsBuilder.WriteString("Files written to:\n  ")
		stepsBuilder.WriteString(ui.HighlightStyle.Render(outputDir))
		stepsBuilder.WriteString("\n\nNext steps:\n")
		stepsBuilder.WriteString("  1. Edit  " + initOutputDir + "/env-vars/<service>/.env.example\n")
		stepsBuilder.WriteString("  2. cd " + targetDir + "\n")
		stepsBuilder.WriteString("  3. docker compose -f " + initOutputDir + "/docker-compose.yml up --build\n")
		fmt.Println()
		fmt.Println(ui.SuccessBox.Render(stepsBuilder.String()))
		fmt.Println()
	}
	return nil
}

func printDetectionTable(d *detector.Result) {
	t := &ui.Table{
		Headers: []string{"ID", "Type", "Directory", "Framework", "Port", "Confidence"},
	}
	for _, s := range d.Services {
		dir := s.Directory
		if dir == "" {
			dir = "."
		}
		typeCell := ui.PrimaryStyle.Render(s.Type)
		if s.Type == "frontend" {
			typeCell = ui.HighlightStyle.Render(s.Type)
		}
		t.Rows = append(t.Rows, []string{
			s.ID,
			typeCell,
			dir,
			s.Framework,
			fmt.Sprintf("%d", s.Port),
			ui.ConfidenceBar(s.Confidence),
		})
	}
	fmt.Println(t.Render())
}

func llmValidateDetection(ctx context.Context, cfg *config.Config, detection *detector.Result, treeStr, targetDir string, quiet, verbose bool) error {
	client, err := llm.NewClient(cfg.LLMProvider, cfg.LLMModel, cfg.LLMAPIKey)
	if err != nil {
		return err
	}
	reader, err := safefs.New(targetDir)
	if err != nil {
		return err
	}
	fileReader := makeFileReader(reader, verbose)
	detJSON, _ := detection.ToJSON()
	resp, err := client.ValidateDetection(ctx, treeStr, detJSON, fileReader)
	if err != nil {
		return err
	}
	if resp.Valid {
		if !quiet {
			fmt.Println(ui.SuccessLine("LLM confirmed detection"))
		}
		return nil
	}
	if len(resp.CorrectedServices) > 0 {
		corrected := make([]detector.Service, 0, len(resp.CorrectedServices))
		for i, c := range resp.CorrectedServices {
			lang := "javascript"
			switch c.Framework {
			case "fastapi", "flask", "django", "python":
				lang = "python"
			}
			conf := "medium"
			if c.Confidence >= 0.9 {
				conf = "high"
			} else if c.Confidence < 0.6 {
				conf = "low"
			}
			corrected = append(corrected, detector.Service{
				ID:         fmt.Sprintf("service-%d", i+1),
				Type:       c.Type,
				Directory:  c.Directory,
				Language:   lang,
				Framework:  c.Framework,
				Confidence: conf,
				Port:       defaultPort(c.Framework),
			})
		}
		detection.Services = corrected
		if !quiet {
			fmt.Println(ui.SuccessLine("LLM provided corrections to detection"))
			printDetectionTable(detection)
		}
	}
	return nil
}

func defaultPort(framework string) int {
	switch framework {
	case "next", "react", "express", "node", "fastify", "nest", "cra":
		return 3000
	case "vite":
		return 5173
	case "fastapi", "django":
		return 8000
	case "flask":
		return 5000
	}
	return 8080
}

func llmValidateDocker(ctx context.Context, cfg *config.Config, out *generator.Output, treeStr, targetDir string, quiet, verbose bool) error {
	client, err := llm.NewClient(cfg.LLMProvider, cfg.LLMModel, cfg.LLMAPIKey)
	if err != nil {
		return err
	}
	reader, err := safefs.New(targetDir)
	if err != nil {
		return err
	}
	fileReader := makeFileReader(reader, verbose)

	// Build a combined dockerfile blob for the validator.
	var dockerBlob strings.Builder
	names := generator.SortedFilenames(out.Files)
	for _, n := range names {
		if strings.HasPrefix(n, "Dockerfile.") {
			fmt.Fprintf(&dockerBlob, "=== %s ===\n%s\n\n", n, out.Files[n])
		}
	}
	composeBlob := out.Files["docker-compose.yml"]

	resp, err := client.ValidateDockerfiles(ctx, treeStr, dockerBlob.String(), composeBlob, fileReader)
	if err != nil {
		return err
	}
	if resp.Valid {
		if !quiet {
			fmt.Println(ui.SuccessLine("LLM confirmed Docker configurations"))
		}
		return nil
	}
	if resp.CorrectedCompose != "" {
		out.Files["docker-compose.yml"] = resp.CorrectedCompose
		if !quiet {
			fmt.Println(ui.SuccessLine("LLM corrected docker-compose.yml"))
		}
	}
	// LLM may have returned a multi-dockerfile blob. We accept it as a single override.
	if resp.CorrectedDockerfile != "" {
		out.Files["Dockerfile"] = resp.CorrectedDockerfile
		if !quiet {
			fmt.Println(ui.WarningLine("LLM returned a corrected Dockerfile blob — saved as Dockerfile (review before use)"))
		}
	}
	return nil
}

func llmValidateEnvVars(ctx context.Context, cfg *config.Config, results *[]envvar.Result, treeStr, targetDir string, quiet, verbose bool) error {
	client, err := llm.NewClient(cfg.LLMProvider, cfg.LLMModel, cfg.LLMAPIKey)
	if err != nil {
		return err
	}
	reader, err := safefs.New(targetDir)
	if err != nil {
		return err
	}
	fileReader := makeFileReader(reader, verbose)

	existingEnv := readExistingEnvFiles(reader, targetDir)
	envJSON, _ := json.Marshal(*results)

	resp, err := client.ValidateEnvVars(ctx, treeStr, string(envJSON), existingEnv, fileReader)
	if err != nil {
		return err
	}
	if len(resp.CorrectedEnvVars) == 0 {
		return nil
	}
	// Map LLM corrections onto existing services by directory; fall back to ServiceID.
	byDir := map[string]int{}
	byID := map[string]int{}
	for i, r := range *results {
		byDir[r.Directory] = i
		byID[r.ServiceID] = i
	}
	for _, c := range resp.CorrectedEnvVars {
		if idx, ok := byDir[c.Directory]; ok {
			(*results)[idx].EnvContent = c.EnvContent
			(*results)[idx].Technology = c.Technology
			continue
		}
	}
	if !quiet {
		fmt.Println(ui.SuccessLine("LLM enhanced environment variables"))
	}
	return nil
}

func makeFileReader(reader *safefs.Reader, verbose bool) func(string) (string, error) {
	return func(p string) (string, error) {
		if verbose {
			fmt.Println(ui.DimStyle.Render("  → LLM reading " + p))
		}
		return reader.Read(p)
	}
}

func readExistingEnvFiles(reader *safefs.Reader, targetDir string) string {
	patterns := []string{".env.example", ".env.sample", ".env.template", ".env.local.example"}
	var found []string
	filepath.WalkDir(targetDir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		for _, p := range patterns {
			if d.Name() == p {
				rel, _ := filepath.Rel(targetDir, path)
				found = append(found, rel)
				return nil
			}
		}
		return nil
	})
	sort.Strings(found)
	var b strings.Builder
	for _, f := range found {
		content, err := reader.Read(f)
		if err != nil {
			continue
		}
		fmt.Fprintf(&b, "\n=== %s ===\n%s\n", f, content)
	}
	return b.String()
}

// reconcilePortsFromEnv scans each env file for PORT/HTTP_PORT/SERVER_PORT
// and updates the corresponding service's runtime port. Returns true when at
// least one port changed (so the caller knows to re-render generation).
func reconcilePortsFromEnv(services []detector.Service, envResults []envvar.Result) bool {
	if len(services) == 0 || len(envResults) == 0 {
		return false
	}
	byID := map[string]*detector.Service{}
	for i := range services {
		byID[services[i].ID] = &services[i]
	}
	changed := false
	portKeyRe := regexp.MustCompile(`(?m)^\s*(PORT|HTTP_PORT|SERVER_PORT|APP_PORT)\s*=\s*['"]?(\d{2,5})['"]?\s*$`)
	for _, r := range envResults {
		if r.EnvContent == "" {
			continue
		}
		svc, ok := byID[r.ServiceID]
		if !ok {
			continue
		}
		// Static-served SPAs are always served on port 80 (nginx); ignore PORT for them.
		switch svc.Framework {
		case "react", "vite", "cra":
			continue
		}
		m := portKeyRe.FindStringSubmatch(r.EnvContent)
		if m == nil {
			continue
		}
		p, err := strconv.Atoi(m[2])
		if err != nil || p < 80 || p > 65535 {
			continue
		}
		if svc.Port != p {
			svc.Port = p
			changed = true
		}
	}
	return changed
}

func quickStart(repo string, services []detector.Service, outDir string) string {
	var b strings.Builder
	b.WriteString("# Quick Start — " + repo + "\n\n")
	b.WriteString("Generated by Chimera " + Version + "\n\n")
	b.WriteString("## Detected services\n\n")
	for _, s := range services {
		dir := s.Directory
		if dir == "" {
			dir = "."
		}
		fmt.Fprintf(&b, "- **%s** — %s (%s) in `%s/` · port %d\n", s.ID, s.Framework, s.Type, dir, s.Port)
	}
	b.WriteString("\n## Run\n\n")
	b.WriteString("```sh\n")
	b.WriteString("# 1. Edit env files\n")
	for _, s := range services {
		fmt.Fprintf(&b, "$EDITOR %s/env-vars/%s/.env.example\n", outDir, s.ID)
	}
	b.WriteString("\n# 2. Build and start\n")
	fmt.Fprintf(&b, "docker compose -f %s/docker-compose.yml up --build\n", outDir)
	b.WriteString("\n# 3. View logs\n")
	fmt.Fprintf(&b, "docker compose -f %s/docker-compose.yml logs -f\n", outDir)
	b.WriteString("\n# 4. Stop\n")
	fmt.Fprintf(&b, "docker compose -f %s/docker-compose.yml down\n", outDir)
	b.WriteString("```\n\n")
	b.WriteString("## Endpoints\n\n")
	for _, s := range services {
		fmt.Fprintf(&b, "- %s → http://localhost:%d\n", s.ID, s.Port)
	}
	return b.String()
}
