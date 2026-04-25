package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"chimera/internal/config"
	"chimera/internal/detector"
	"chimera/internal/envvar"
	"chimera/internal/generator"
	"chimera/internal/git"
	"chimera/internal/llm"
	"chimera/internal/tree"
	"chimera/internal/ui"

	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init <github-url>",
	Short: "Clone and orchestrate a GitHub repository",
	Long: `Clone a GitHub repository, analyze its stack, and generate Docker configurations.

By default, this command will:
  1. Clone the repository to ~/.chimera/repos/<repo-name>/
  2. Generate a smart file tree (excluding node_modules, venv, etc.)
  3. Detect technologies and frameworks (JS/TS, Python)
  4. Validate findings with LLM (unless --no-agent is used)
  5. Generate Dockerfile and docker-compose.yml
  6. Extract environment variables
  7. Write everything to chimera-outputs/

Flags:
  --force       Force re-initialization even if directory exists
  --no-agent    Disable LLM validation (use static analysis only)

Examples:
  chimera init https://github.com/tiangolo/fastapi
  chimera init https://github.com/user/repo --force
  chimera init https://github.com/user/repo --no-agent
  chimera init https://github.com/user/repo --force --no-agent`,
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		if err := runInit(cmd, args[0]); err != nil {
			fmt.Println()
			errorBox := ui.ErrorBox.Render(fmt.Sprintf("Error: %s", err.Error()))
			fmt.Println(errorBox)
			os.Exit(1)
		}
	},
}

var (
	initForce    bool
	initNoAgent  bool
)

func init() {
	initCmd.Flags().BoolVar(&initForce, "force", false, "Force re-initialization even if directory exists")
	initCmd.Flags().BoolVar(&initNoAgent, "no-agent", false, "Disable LLM validation (use static analysis only)")
}

func runInit(cmd *cobra.Command, repoURL string) error {
	ctx := context.Background()
	start := time.Now()

	// Get flags
	verbose := GetVerbose(cmd)
	quiet := GetQuiet(cmd)

	// Print banner (unless quiet)
	if !quiet {
		fmt.Print(ui.Header("init"))
	}

	// PRE-FLIGHT CHECK
	if !quiet {
		fmt.Println(ui.InfoLine("Pre-flight checks..."))
	}

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("Chimera is not configured.\nRun  chimera setup  first to choose a provider and set your API key")
	}

	if !git.IsValidGitHubURL(repoURL) {
		return fmt.Errorf("invalid GitHub URL: must match https://github.com/<owner>/<repo>")
	}

	repoName := git.ExtractRepoName(repoURL)
	if repoName == "" {
		return fmt.Errorf("could not extract repository name from URL")
	}

	if !quiet {
		fmt.Println(ui.SuccessLine("Configuration loaded"))
		fmt.Println()
	}

	// STEP 1: CLONE
	if !quiet {
		fmt.Println(ui.InfoLine(fmt.Sprintf("Cloning %s...", ui.HighlightStyle.Render(repoName))))
	}

	reposDir, err := config.ReposDir()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(reposDir, 0755); err != nil {
		return fmt.Errorf("failed to create repos directory: %w", err)
	}

	targetDir := filepath.Join(reposDir, repoName)

	// Check if directory exists
	if _, err := os.Stat(targetDir); err == nil {
		if !initForce {
			return fmt.Errorf("directory %s already exists. Use --force to re-initialize", targetDir)
		}
		if !quiet {
			fmt.Println(ui.WarningLine("Removing existing directory (--force)"))
		}
		if err := os.RemoveAll(targetDir); err != nil {
			return fmt.Errorf("failed to remove existing directory: %w", err)
		}
	}

	// Clone the repository
	if err := git.Clone(ctx, repoURL, targetDir, cfg.GitHubPAT); err != nil {
		return fmt.Errorf("git clone failed: %w", err)
	}

	fileCount, size, _ := git.CountFiles(targetDir)
	sizeMB := float64(size) / 1024 / 1024
	if !quiet {
		fmt.Println(ui.SuccessLine(fmt.Sprintf("Cloned %s (%d files, %.1f MB)", repoName, fileCount, sizeMB)))
		fmt.Println()
	}

	// STEP 2: TREE GENERATION
	if !quiet {
		fmt.Println(ui.InfoLine("Generating file tree..."))
	}

	treeStr, totalLines, err := tree.Generate(targetDir, 10000)
	if err != nil {
		return fmt.Errorf("tree generation failed: %w", err)
	}

	if !quiet {
		fmt.Println(ui.SuccessLine(fmt.Sprintf("Generated tree (%d lines)", totalLines)))
		fmt.Println()
		
		// Display tree
		fmt.Println(ui.BoldStyle.Render("Repository Structure:"))
		fmt.Println()
		treeLines := strings.Split(treeStr, "\n")
		displayLines := treeLines
		if len(treeLines) > 60 && !verbose {
			displayLines = treeLines[:60]
		}
		for _, line := range displayLines {
			if line != "" {
				fmt.Println(ui.DimStyle.Render("  " + line))
			}
		}
		if len(treeLines) > 60 && !verbose {
			fmt.Println(ui.DimStyle.Render(fmt.Sprintf("  ... (%d more lines, use --verbose to see all)", len(treeLines)-60)))
		}
		fmt.Println()
	}

	// STEP 3: STATIC ANALYSIS
	if !quiet {
		fmt.Println(ui.InfoLine("Analyzing technology stack..."))
	}

	detectionResult, err := detector.Detect(targetDir)
	if err != nil {
		return fmt.Errorf("detection failed: %w", err)
	}

	if len(detectionResult.Services) == 0 {
		warningBox := ui.WarningBox.Render(
			"No supported project types detected.\n\n" +
				"Chimera currently supports: Node.js/Express/Next.js, Python/FastAPI/Flask/Django.\n\n" +
				"If this repo uses a supported technology, please open an issue.",
		)
		fmt.Println(warningBox)
		return nil
	}

	if !quiet {
		fmt.Println(ui.SuccessLine(fmt.Sprintf("Detected %d service(s) via static analysis", len(detectionResult.Services))))
		fmt.Println()
		
		// Display detection summary
		fmt.Println(ui.BoldStyle.Render("Detected Services:"))
		fmt.Println()
		table := &ui.Table{
			Headers: []string{"Type", "Directory", "Technology", "Confidence"},
			Rows:    [][]string{},
		}

		for _, svc := range detectionResult.Services {
			typeStr := svc.Type
			if svc.Type == "frontend" {
				typeStr = ui.HighlightStyle.Render("Frontend")
			} else {
				typeStr = ui.PrimaryStyle.Render("Backend")
			}

			dir := svc.Directory
			if dir == "" {
				dir = "."
			}

			table.Rows = append(table.Rows, []string{
				typeStr,
				dir,
				svc.Framework,
				ui.ConfidenceBar(svc.Confidence),
			})
		}

		fmt.Println(table.Render())
		fmt.Println()
	}

	// STEP 4: LLM VALIDATION OF DETECTION (unless --no-agent)
	if !initNoAgent {
		if !quiet {
			fmt.Println(ui.InfoLine("Validating detection with LLM..."))
		}

		llmClient, err := llm.NewClient(cfg.LLMProvider, cfg.LLMModel, cfg.LLMAPIKey)
		if err != nil {
			if !quiet {
				fmt.Println(ui.WarningLine(fmt.Sprintf("LLM validation skipped: %v", err)))
			}
		} else {
			detectionJSON, _ := detectionResult.ToJSON()
			
			// File reader function
			fileReader := func(path string) (string, error) {
				if verbose {
					fmt.Println(ui.DimStyle.Render(fmt.Sprintf("  → Reading %s", path)))
				}
				fullPath := filepath.Join(targetDir, path)
				content, err := os.ReadFile(fullPath)
				if err != nil {
					return "", err
				}
				return string(content), nil
			}

			validationResp, err := llmClient.ValidateDetection(ctx, treeStr, detectionJSON, fileReader)
			if err != nil {
				if !quiet {
					fmt.Println(ui.WarningLine(fmt.Sprintf("LLM validation failed: %v", err)))
				}
			} else if validationResp.Valid {
				if !quiet {
					fmt.Println(ui.SuccessLine("LLM confirmed detection"))
				}
			} else if len(validationResp.CorrectedServices) > 0 {
				if !quiet {
					fmt.Println(ui.SuccessLine("LLM provided corrections"))
				}
				
				// Convert LLM corrections to detector.Service format
				correctedServices := make([]detector.Service, 0, len(validationResp.CorrectedServices))
				for i, corrected := range validationResp.CorrectedServices {
					confidenceStr := "medium"
					if corrected.Confidence >= 0.9 {
						confidenceStr = "high"
					} else if corrected.Confidence < 0.6 {
						confidenceStr = "low"
					}
					
					language := "unknown"
					if corrected.Framework == "next" || corrected.Framework == "express" || corrected.Framework == "node" {
						language = "javascript"
					} else if corrected.Framework == "fastapi" || corrected.Framework == "flask" || corrected.Framework == "django" {
						language = "python"
					}
					
					correctedServices = append(correctedServices, detector.Service{
						ID:         fmt.Sprintf("service-%d", i+1),
						Type:       corrected.Type,
						Directory:  corrected.Directory,
						Language:   language,
						Framework:  corrected.Framework,
						Confidence: confidenceStr,
						IdentifierFiles: []string{},
					})
				}
				
				detectionResult.Services = correctedServices
			}
		}
		if !quiet {
			fmt.Println()
		}
	} else {
		if !quiet {
			fmt.Println(ui.InfoLine("LLM validation skipped (--no-agent mode)"))
			fmt.Println()
		}
	}

	// STEP 5: DOCKERFILE & COMPOSE GENERATION
	if !quiet {
		fmt.Println(ui.InfoLine("Generating Docker configurations..."))
	}

	dockerfile := generator.GenerateMultiStage(detectionResult.Services)
	composeFile := generator.GenerateCompose(detectionResult.Services, repoName)

	if !quiet {
		fmt.Println(ui.SuccessLine("Generated Dockerfile and docker-compose.yml"))
		fmt.Println()
	}

	// STEP 6: LLM VALIDATION OF DOCKERFILES (unless --no-agent)
	if !initNoAgent {
		if !quiet {
			fmt.Println(ui.InfoLine("Validating Docker configurations with LLM..."))
		}

		llmClient, err := llm.NewClient(cfg.LLMProvider, cfg.LLMModel, cfg.LLMAPIKey)
		if err != nil {
			if !quiet {
				fmt.Println(ui.WarningLine(fmt.Sprintf("LLM validation skipped: %v", err)))
			}
		} else {
			fileReader := func(path string) (string, error) {
				if verbose {
					fmt.Println(ui.DimStyle.Render(fmt.Sprintf("  → Reading %s", path)))
				}
				fullPath := filepath.Join(targetDir, path)
				content, err := os.ReadFile(fullPath)
				if err != nil {
					return "", err
				}
				return string(content), nil
			}

			validationResp, err := llmClient.ValidateDockerfiles(ctx, treeStr, dockerfile, composeFile, fileReader)
			if err != nil {
				if !quiet {
					fmt.Println(ui.WarningLine(fmt.Sprintf("LLM validation failed: %v", err)))
				}
			} else if validationResp.Valid {
				if !quiet {
					fmt.Println(ui.SuccessLine("LLM confirmed Docker configurations"))
				}
			} else {
				if validationResp.CorrectedDockerfile != "" {
					if !quiet {
						fmt.Println(ui.SuccessLine("LLM corrected Dockerfile"))
					}
					dockerfile = validationResp.CorrectedDockerfile
				}
				if validationResp.CorrectedCompose != "" {
					if !quiet {
						fmt.Println(ui.SuccessLine("LLM corrected docker-compose.yml"))
					}
					composeFile = validationResp.CorrectedCompose
				}
			}
		}
		if !quiet {
			fmt.Println()
		}
	}

	// STEP 7: ENV VAR DETECTION
	if !quiet {
		fmt.Println(ui.InfoLine("Detecting environment variables..."))
	}

	envResults := envvar.Detect(targetDir, detectionResult.Services)

	totalVars := 0
	for _, result := range envResults {
		totalVars += len(result.Vars)
	}

	if !quiet {
		fmt.Println(ui.SuccessLine(fmt.Sprintf("Detected %d environment variable(s)", totalVars)))
		fmt.Println()
	}

	// STEP 8: LLM VALIDATION OF ENV VARS (unless --no-agent)
	if !initNoAgent {
		if !quiet {
			fmt.Println(ui.InfoLine("Enhancing environment variables with LLM..."))
		}

		llmClient, err := llm.NewClient(cfg.LLMProvider, cfg.LLMModel, cfg.LLMAPIKey)
		if err != nil {
			if !quiet {
				fmt.Println(ui.WarningLine(fmt.Sprintf("LLM enhancement skipped: %v", err)))
			}
		} else {
			// Check for existing .env files
			existingEnvFiles := findExistingEnvFiles(targetDir)
			existingEnvContent := ""
			if len(existingEnvFiles) > 0 {
				if !quiet {
					fmt.Println(ui.InfoLine(fmt.Sprintf("Found %d existing .env file(s)", len(existingEnvFiles))))
				}
				var envBuilder strings.Builder
				for _, envFile := range existingEnvFiles {
					content, err := os.ReadFile(filepath.Join(targetDir, envFile))
					if err == nil {
						envBuilder.WriteString(fmt.Sprintf("\n=== %s ===\n%s\n", envFile, string(content)))
					}
				}
				existingEnvContent = envBuilder.String()
			}

			// Convert env results to JSON
			envJSON, _ := json.Marshal(envResults)

			fileReader := func(path string) (string, error) {
				if verbose {
					fmt.Println(ui.DimStyle.Render(fmt.Sprintf("  → Reading %s", path)))
				}
				fullPath := filepath.Join(targetDir, path)
				content, err := os.ReadFile(fullPath)
				if err != nil {
					return "", err
				}
				return string(content), nil
			}

			validationResp, err := llmClient.ValidateEnvVars(ctx, treeStr, string(envJSON), existingEnvContent, fileReader)
			if err != nil {
				if !quiet {
					fmt.Println(ui.WarningLine(fmt.Sprintf("LLM enhancement failed: %v", err)))
				}
			} else if len(validationResp.CorrectedEnvVars) > 0 {
				if !quiet {
					fmt.Println(ui.SuccessLine("LLM enhanced environment variables"))
				}
				
				// Replace env results with LLM-enhanced versions
				envResults = make([]envvar.Result, 0, len(validationResp.CorrectedEnvVars))
				for _, corrected := range validationResp.CorrectedEnvVars {
					envResults = append(envResults, envvar.Result{
						Directory:   corrected.Directory,
						ServiceType: corrected.ServiceType,
						Technology:  corrected.Technology,
						Vars:        []envvar.EnvVar{},
						EnvContent:  corrected.EnvContent,
					})
				}
			}
		}
		if !quiet {
			fmt.Println()
		}
	}

	// STEP 9: WRITE chimera-outputs/
	if !quiet {
		fmt.Println(ui.InfoLine("Writing output files..."))
	}

	outputDir := filepath.Join(targetDir, "chimera-outputs")
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Write Dockerfile
	dockerfilePath := filepath.Join(outputDir, "Dockerfile")
	if err := os.WriteFile(dockerfilePath, []byte(dockerfile), 0644); err != nil {
		return fmt.Errorf("failed to write Dockerfile: %w", err)
	}
	if !quiet {
		fmt.Println(ui.SuccessLine("  ✓ Dockerfile"))
	}

	// Write docker-compose.yml
	composePath := filepath.Join(outputDir, "docker-compose.yml")
	if err := os.WriteFile(composePath, []byte(composeFile), 0644); err != nil {
		return fmt.Errorf("failed to write docker-compose.yml: %w", err)
	}
	if !quiet {
		fmt.Println(ui.SuccessLine("  ✓ docker-compose.yml"))
	}

	// Write .gitignore
	gitignorePath := filepath.Join(outputDir, ".gitignore")
	if err := os.WriteFile(gitignorePath, []byte("*\n"), 0644); err != nil {
		return fmt.Errorf("failed to write .gitignore: %w", err)
	}
	if !quiet {
		fmt.Println(ui.SuccessLine("  ✓ .gitignore"))
	}

	// Write env vars
	for _, envResult := range envResults {
		serviceName := strings.ReplaceAll(envResult.Directory, "/", "-")
		if serviceName == "" {
			serviceName = "root"
		}
		serviceName = strings.ToLower(serviceName)

		envDir := filepath.Join(outputDir, "env-vars", serviceName)
		if err := os.MkdirAll(envDir, 0755); err != nil {
			return fmt.Errorf("failed to create env-vars directory: %w", err)
		}

		var envContent string
		if envResult.EnvContent != "" {
			// Use LLM-generated content
			envContent = envResult.EnvContent
		} else {
			// Use static generation
			envContent = envvar.GenerateEnvExample(envResult.Vars, envResult.Technology)
		}

		envPath := filepath.Join(envDir, ".env.example")
		if err := os.WriteFile(envPath, []byte(envContent), 0644); err != nil {
			return fmt.Errorf("failed to write .env.example: %w", err)
		}
		if !quiet {
			fmt.Println(ui.SuccessLine(fmt.Sprintf("  ✓ env-vars/%s/.env.example", serviceName)))
		}
	}

	// Generate and write quick start guide
	quickStartGuide := generateQuickStartGuide(repoName, detectionResult, envResults)
	quickStartPath := filepath.Join(outputDir, "quick_start_guide.txt")
	if err := os.WriteFile(quickStartPath, []byte(quickStartGuide), 0644); err != nil {
		return fmt.Errorf("failed to write quick_start_guide.txt: %w", err)
	}
	if !quiet {
		fmt.Println(ui.SuccessLine("  ✓ quick_start_guide.txt"))
	}

	if !quiet {
		fmt.Println()
	}

	// COMPLETION
	if !quiet {
		duration := time.Since(start)
		completionBox := ui.SuccessBox.Render(fmt.Sprintf(
			"Init complete for %s\n\n"+
				"Files written to: %s\n\n"+
				"Next steps:\n"+
				"  1. Review chimera-outputs/env-vars/<service>/.env.example\n"+
				"  2. Copy to .env and fill in your secrets\n"+
				"  3. Run: docker compose -f chimera-outputs/docker-compose.yml up\n\n"+
				"Completed in %.1fs",
			ui.HighlightStyle.Render(repoName),
			ui.HighlightStyle.Render(outputDir),
			duration.Seconds(),
		))

		fmt.Println(completionBox)
		fmt.Println()
	}

	return nil
}

// findExistingEnvFiles searches for .env.example, .env.sample, etc.
func findExistingEnvFiles(rootDir string) []string {
	var envFiles []string
	patterns := []string{".env.example", ".env.sample", ".env.template", ".env.local.example"}
	
	filepath.Walk(rootDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		
		relPath, _ := filepath.Rel(rootDir, path)
		for _, pattern := range patterns {
			if strings.HasSuffix(info.Name(), pattern) {
				envFiles = append(envFiles, relPath)
				break
			}
		}
		return nil
	})
	
	return envFiles
}

func generateQuickStartGuide(projectName string, detection *detector.Result, envResults []envvar.Result) string {
var b strings.Builder
b.WriteString("═══════════════════════════════════════════════════════════════\n")
b.WriteString(fmt.Sprintf("  QUICK START GUIDE - %s\n", strings.ToUpper(projectName)))
b.WriteString("═══════════════════════════════════════════════════════════════\n\n")
b.WriteString("DETECTED SERVICES:\n")
for _, svc := range detection.Services {
dir := svc.Directory
if dir == "" {
dir = "root"
}
b.WriteString(fmt.Sprintf("  • %s (%s) in %s/\n", svc.Framework, svc.Type, dir))
}
b.WriteString("\n═══════════════════════════════════════════════════════════════\n")
b.WriteString("DOCKER SETUP (RECOMMENDED)\n")
b.WriteString("═══════════════════════════════════════════════════════════════\n\n")
b.WriteString("1. Configure environment:\n")
b.WriteString("   cp chimera-outputs/env-vars/*/.env.example .env\n")
b.WriteString("   nano .env  # Edit with your secrets\n\n")
b.WriteString("2. Start services:\n")
b.WriteString("   docker compose -f chimera-outputs/docker-compose.yml up -d\n\n")
b.WriteString("3. View logs:\n")
b.WriteString("   docker compose -f chimera-outputs/docker-compose.yml logs -f\n\n")
b.WriteString("4. Stop services:\n")
b.WriteString("   docker compose -f chimera-outputs/docker-compose.yml down\n\n")
b.WriteString("═══════════════════════════════════════════════════════════════\n")
b.WriteString("Generated by Chimera v0.1.0\n")
b.WriteString("═══════════════════════════════════════════════════════════════\n")
return b.String()
}
