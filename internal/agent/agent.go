// Package agent provides a multi-turn LLM-powered agentic pipeline for
// generating production-quality Docker configurations.
//
// The pipeline combines heuristic scanning (regex-based) with LLM intelligence:
//  1. Run regex scanner → detect languages, infra, ports, env vars
//  2. Send directory tree + scan results to LLM → it picks which files to read
//  3. Send requested files + scan results → LLM generates structured config
//  4. Validate JSON schema → fall back to templates on failure
package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/projectchimera/chimera/internal/scanner"
	"github.com/projectchimera/chimera/internal/tui"
)

// maxFileSize limits individual files sent to the LLM (32KB).
const maxFileSize = 32 * 1024

// maxFiles limits how many files the LLM can request.
const maxFiles = 20

// maxTreeDepth limits directory tree depth.
const maxTreeDepth = 5

// ──────────────────────────────────────────────────────────────
// Structured output schema
// ──────────────────────────────────────────────────────────────

// Config holds the generated configuration from the agent.
type Config struct {
	Dockerfile    string        `json:"dockerfile"`
	DockerCompose string        `json:"docker_compose"`
	EnvExample    string        `json:"env_example"`
	Explanation   string        `json:"explanation"`
	Services      []ServiceInfo `json:"services"`
}

// ServiceInfo describes a service detected and configured by the agent.
type ServiceInfo struct {
	Name        string `json:"name"`
	Type        string `json:"type"` // "app", "database", "cache", "queue", "search"
	Image       string `json:"image"`
	Port        int    `json:"port"`
	Healthcheck string `json:"healthcheck"`
}

// PlanningResponse is the structured output from the planning call.
type PlanningResponse struct {
	Files    []string `json:"files"`
	Reasoning string  `json:"reasoning"`
}

// Provider configures which LLM backend to use.
type Provider struct {
	Name     string // "openai", "gemini", "groq"
	APIKey   string
	Model    string
	Endpoint string
}

// Run executes the full agentic pipeline.
// Returns a Config or an error if the LLM is unavailable/fails.
func Run(ctx context.Context, provider *Provider, projectDir string, projectName string) (*Config, error) {
	// Step 0: Run regex-based scanner for heuristic data
	tui.PrintInfo("Agent: Running heuristic scanner...")
	repoScanner := scanner.NewScanner(projectDir)
	scanResult, err := repoScanner.Scan(ctx)
	if err != nil {
		tui.PrintWarning(fmt.Sprintf("Scanner failed: %v (continuing with LLM only)", err))
		scanResult = &scanner.ScanResult{}
	}
	scanSummary := formatScanResults(scanResult)
	tui.PrintInfo(fmt.Sprintf("Agent: Scanner found %d languages, %d infra deps, %d env vars, %d ports",
		len(scanResult.Languages), len(scanResult.Infrastructure),
		len(scanResult.EnvVars), len(scanResult.Ports)))

	// Step 1: Build directory tree
	tui.PrintInfo("Agent: Analyzing project structure...")
	tree, err := buildTree(projectDir, maxTreeDepth)
	if err != nil {
		return nil, fmt.Errorf("agent: failed to build tree: %w", err)
	}

	// Step 2: Ask LLM which files it needs (with scan context)
	tui.PrintInfo("Agent: Asking LLM which files to inspect...")
	planResp, err := planningCall(ctx, provider, tree, scanSummary, projectName)
	if err != nil {
		return nil, fmt.Errorf("agent: planning call failed: %w", err)
	}

	if planResp.Reasoning != "" {
		tui.PrintInfo(fmt.Sprintf("Agent reasoning: %s", planResp.Reasoning))
	}
	tui.PrintInfo(fmt.Sprintf("Agent: LLM requested %d files", len(planResp.Files)))

	// Step 3: Read the requested files
	fileContents, err := readRequestedFiles(projectDir, planResp.Files)
	if err != nil {
		return nil, fmt.Errorf("agent: failed to read files: %w", err)
	}

	// Step 4: Send everything to LLM for generation
	tui.PrintInfo("Agent: Generating Docker configuration...")
	config, err := generationCall(ctx, provider, tree, scanSummary, fileContents, projectName)
	if err != nil {
		return nil, fmt.Errorf("agent: generation call failed: %w", err)
	}

	// Step 5: Validate
	if err := validateConfig(config); err != nil {
		return nil, fmt.Errorf("agent: validation failed: %w", err)
	}

	tui.PrintSuccess("Agent: Configuration generated successfully")
	if len(config.Services) > 0 {
		for _, svc := range config.Services {
			tui.PrintListItem(fmt.Sprintf("%s (%s) → port %d", svc.Name, svc.Type, svc.Port))
		}
	}
	return config, nil
}

// ──────────────────────────────────────────────────────────────
// Format scanner results for LLM context
// ──────────────────────────────────────────────────────────────

func formatScanResults(result *scanner.ScanResult) string {
	var b strings.Builder
	b.WriteString("=== HEURISTIC SCAN RESULTS (regex-based) ===\n\n")

	// Languages
	b.WriteString("LANGUAGES DETECTED:\n")
	if len(result.Languages) == 0 {
		b.WriteString("  (none detected)\n")
	}
	for _, lang := range result.Languages {
		b.WriteString(fmt.Sprintf("  - %s %s (files: %s)\n", lang.Name, lang.Version, strings.Join(lang.Files, ", ")))
	}

	// Infrastructure
	b.WriteString("\nINFRASTRUCTURE DEPENDENCIES:\n")
	if len(result.Infrastructure) == 0 {
		b.WriteString("  (none detected)\n")
	}
	for _, infra := range result.Infrastructure {
		b.WriteString(fmt.Sprintf("  - %s (type: %s)\n", infra.Name, infra.Type))
	}

	// Ports
	b.WriteString("\nPORTS DETECTED:\n")
	if len(result.Ports) == 0 {
		b.WriteString("  (none detected — use language defaults)\n")
	}
	for _, port := range result.Ports {
		b.WriteString(fmt.Sprintf("  - %d\n", port))
	}

	// Environment variables
	b.WriteString("\nENVIRONMENT VARIABLES (names only, no values):\n")
	if len(result.EnvVars) == 0 {
		b.WriteString("  (none detected)\n")
	}
	for _, v := range result.EnvVars {
		b.WriteString(fmt.Sprintf("  - %s\n", v))
	}

	return b.String()
}

// ──────────────────────────────────────────────────────────────
// Provider management
// ──────────────────────────────────────────────────────────────

// NewProvider creates a Provider from environment variables.
// Call LoadConfig() before this to load from .chimera.env files.
func NewProvider() (*Provider, error) {
	name := os.Getenv("CHIMERA_LLM_PROVIDER")
	if name == "" {
		name = "openai"
	}

	switch strings.ToLower(name) {
	case "openai":
		key := os.Getenv("OPENAI_API_KEY")
		if key == "" {
			return nil, fmt.Errorf("agent: OPENAI_API_KEY not set — add it to ~/.chimera.env or export it")
		}
		return &Provider{
			Name:     "openai",
			APIKey:   key,
			Model:    envOrDefault("CHIMERA_MODEL", "gpt-4o-mini"),
			Endpoint: "https://api.openai.com/v1/chat/completions",
		}, nil
	case "gemini":
		key := os.Getenv("GEMINI_API_KEY")
		if key == "" {
			return nil, fmt.Errorf("agent: GEMINI_API_KEY not set — add it to ~/.chimera.env or export it")
		}
		model := envOrDefault("CHIMERA_MODEL", "gemini-2.0-flash")
		return &Provider{
			Name:     "gemini",
			APIKey:   key,
			Model:    model,
			Endpoint: fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent", model),
		}, nil
	case "groq":
		key := os.Getenv("GROQ_API_KEY")
		if key == "" {
			return nil, fmt.Errorf("agent: GROQ_API_KEY not set — add it to ~/.chimera.env or export it")
		}
		return &Provider{
			Name:     "groq",
			APIKey:   key,
			Model:    envOrDefault("CHIMERA_MODEL", "llama-3.1-70b-versatile"),
			Endpoint: "https://api.groq.com/openai/v1/chat/completions",
		}, nil
	default:
		return nil, fmt.Errorf("agent: unsupported provider %q (use openai, gemini, or groq)", name)
	}
}

func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// ──────────────────────────────────────────────────────────────
// Step 1: Build directory tree
// ──────────────────────────────────────────────────────────────

func buildTree(root string, maxDepth int) (string, error) {
	var b strings.Builder
	err := walkTree(&b, root, root, 0, maxDepth)
	return b.String(), err
}

func walkTree(b *strings.Builder, root, dir string, depth, maxDepth int) error {
	if depth > maxDepth {
		return nil
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}

	indent := strings.Repeat("  ", depth)
	for _, e := range entries {
		name := e.Name()

		if shouldSkip(name) {
			continue
		}

		rel, _ := filepath.Rel(root, filepath.Join(dir, name))

		if e.IsDir() {
			b.WriteString(fmt.Sprintf("%s📁 %s/\n", indent, rel))
			walkTree(b, root, filepath.Join(dir, name), depth+1, maxDepth)
		} else {
			info, _ := e.Info()
			size := ""
			if info != nil {
				size = fmt.Sprintf(" (%s)", humanSize(info.Size()))
			}
			b.WriteString(fmt.Sprintf("%s📄 %s%s\n", indent, rel, size))
		}
	}
	return nil
}

func shouldSkip(name string) bool {
	skip := []string{
		"node_modules", ".git", ".svn", "__pycache__", ".tox",
		".venv", "venv", "vendor", "dist", "build", ".next",
		".cache", ".DS_Store", "coverage", ".nyc_output",
		".gomod", ".chimera", ".chimera.env",
	}
	for _, s := range skip {
		if name == s {
			return true
		}
	}
	return false
}

func humanSize(bytes int64) string {
	if bytes < 1024 {
		return fmt.Sprintf("%dB", bytes)
	}
	if bytes < 1024*1024 {
		return fmt.Sprintf("%.1fKB", float64(bytes)/1024)
	}
	return fmt.Sprintf("%.1fMB", float64(bytes)/1024/1024)
}

// ──────────────────────────────────────────────────────────────
// Step 2: Planning call — LLM picks which files to read
// ──────────────────────────────────────────────────────────────

const planningPrompt = `You are a DevOps expert analyzing a project to generate Docker configuration.

PROJECT NAME: %s

DIRECTORY TREE:
%s

%s

Based on the directory tree and scan results above, decide which files you need to read to generate a perfect, working Dockerfile, docker-compose.yml, and .env file.

Return a JSON object with this exact schema:
{
  "files": ["path/to/file1", "path/to/file2"],
  "reasoning": "Brief explanation of why you need these files"
}

Guidelines:
- Always request package manifests (package.json, requirements.txt, go.mod, Gemfile, Cargo.toml, etc.)
- Always request entry points (main.*, index.*, app.*, server.*, manage.py, etc.)
- Request existing Docker files if present (Dockerfile, docker-compose.yml, .dockerignore)
- Request config files (.env.example, Procfile, Makefile, tsconfig.json, etc.)
- Request build/start scripts or config (webpack.config.*, vite.config.*, next.config.*, etc.)
- DO NOT request files from node_modules, vendor, dist, or build directories
- Max %d files. Prioritize the most important ones.

Return ONLY the JSON object. No markdown fences, no extra text.`

func planningCall(ctx context.Context, provider *Provider, tree, scanSummary, projectName string) (*PlanningResponse, error) {
	prompt := fmt.Sprintf(planningPrompt, projectName, tree, scanSummary, maxFiles)

	response, err := llmCall(ctx, provider, "You are a DevOps file selector. Return only valid JSON matching the specified schema.", prompt)
	if err != nil {
		return nil, err
	}

	response = extractJSON(response)

	var planResp PlanningResponse
	if err := json.Unmarshal([]byte(response), &planResp); err != nil {
		// Fallback: try parsing as a plain array
		var files []string
		if arrErr := json.Unmarshal([]byte(response), &files); arrErr == nil {
			return &PlanningResponse{Files: files}, nil
		}
		return nil, fmt.Errorf("agent: failed to parse planning response: %w (raw: %s)", err, truncate(response, 200))
	}

	if len(planResp.Files) > maxFiles {
		planResp.Files = planResp.Files[:maxFiles]
	}

	return &planResp, nil
}

// ──────────────────────────────────────────────────────────────
// Step 3: Read requested files
// ──────────────────────────────────────────────────────────────

func readRequestedFiles(projectDir string, files []string) (map[string]string, error) {
	contents := make(map[string]string, len(files))

	for _, file := range files {
		clean := filepath.Clean(file)
		if strings.HasPrefix(clean, "..") || filepath.IsAbs(clean) {
			continue
		}

		fullPath := filepath.Join(projectDir, clean)
		info, err := os.Stat(fullPath)
		if err != nil {
			contents[file] = "(file not found)"
			continue
		}

		if info.Size() > maxFileSize {
			contents[file] = fmt.Sprintf("(file too large: %s — truncated to first %d bytes)", humanSize(info.Size()), maxFileSize)
			data, _ := os.ReadFile(fullPath)
			if len(data) > maxFileSize {
				data = data[:maxFileSize]
			}
			contents[file] = string(data) + "\n... (truncated)"
			continue
		}

		if isBinary(fullPath) {
			contents[file] = "(binary file, skipped)"
			continue
		}

		data, err := os.ReadFile(fullPath)
		if err != nil {
			contents[file] = fmt.Sprintf("(read error: %v)", err)
			continue
		}

		contents[file] = string(data)
	}

	return contents, nil
}

func isBinary(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()

	buf := make([]byte, 512)
	n, err := f.Read(buf)
	if err != nil {
		return false
	}

	for i := 0; i < n; i++ {
		if buf[i] == 0 {
			return true
		}
	}
	return false
}

// ──────────────────────────────────────────────────────────────
// Step 4: Generation call — LLM creates structured config
// ──────────────────────────────────────────────────────────────

const generationPrompt = `You are an expert DevOps engineer. Generate production-quality Docker configuration for the project "%s".

%s

PROJECT TREE:
%s

FILE CONTENTS:
%s

Generate a complete, immediately-runnable Docker setup. Return a JSON object matching this EXACT schema:

{
  "dockerfile": "Full Dockerfile content as a single string",
  "docker_compose": "Full docker-compose.yml content as a single string",
  "env_example": "Full .env.example content as a single string",
  "explanation": "2-3 sentence explanation of what was detected and key decisions made",
  "services": [
    {
      "name": "service-name",
      "type": "app|database|cache|queue|search",
      "image": "image:tag used",
      "port": 3000,
      "healthcheck": "healthcheck command if any"
    }
  ]
}

STRICT RULES:
1. Dockerfile:
   - Use multi-stage builds for compiled languages (Go, Java, Rust)
   - Pin base image versions (node:20-alpine, NOT node:latest)
   - COPY dependency files first for layer caching
   - Use alpine variants where possible
   - Set correct WORKDIR, EXPOSE, and CMD
   - Match the actual start command from package.json/Procfile/main file

2. docker-compose.yml:
   - Use version '3.8'
   - Include ALL infrastructure from the scan results (databases, caches, queues)
   - Add healthchecks for every infrastructure service
   - Use named volumes for data persistence
   - Use a shared network named "%s_net"
   - Add restart: unless-stopped to all services
   - Add label: com.chimera.project: "%s" to every service
   - Use depends_on with condition: service_healthy
   - Map correct ports based on scan results

3. .env.example:
   - Pre-populate connection strings pointing to compose service names
   - Include ALL env vars found in scan results
   - Leave sensitive values empty with a descriptive comment
   - Include DATABASE_URL, REDIS_URL etc. with correct service hostnames

4. services array:
   - List EVERY service in the docker-compose
   - Include accurate port numbers and healthcheck commands

Return ONLY the JSON object. No markdown fences. No extra text outside the JSON.`

func generationCall(ctx context.Context, provider *Provider, tree, scanSummary string, files map[string]string, projectName string) (*Config, error) {
	var fileSection strings.Builder
	for name, content := range files {
		fileSection.WriteString(fmt.Sprintf("\n━━━ %s ━━━\n%s\n", name, content))
	}

	prompt := fmt.Sprintf(generationPrompt, projectName, scanSummary, tree, fileSection.String(), projectName, projectName)

	systemPrompt := `You are a Docker configuration generator. You MUST return valid JSON matching the specified schema exactly. 
Do not include markdown code fences. Do not include any text outside the JSON object.
Ensure all string values in JSON use proper escaping (newlines as \n, quotes as \").`

	response, err := llmCall(ctx, provider, systemPrompt, prompt)
	if err != nil {
		return nil, err
	}

	response = extractJSON(response)

	var config Config
	if err := json.Unmarshal([]byte(response), &config); err != nil {
		return nil, fmt.Errorf("agent: JSON parse failed: %w\nRaw (first 500 chars):\n%s", err, truncate(response, 500))
	}

	return &config, nil
}

// ──────────────────────────────────────────────────────────────
// Validation
// ──────────────────────────────────────────────────────────────

func validateConfig(config *Config) error {
	if config.Dockerfile == "" {
		return fmt.Errorf("Dockerfile is empty")
	}
	if config.DockerCompose == "" {
		return fmt.Errorf("docker-compose.yml is empty")
	}

	// Validate Dockerfile has required instructions
	df := strings.ToUpper(config.Dockerfile)
	if !strings.Contains(df, "FROM") {
		return fmt.Errorf("Dockerfile missing FROM instruction")
	}

	// Validate docker-compose has services
	if !strings.Contains(config.DockerCompose, "services:") {
		return fmt.Errorf("docker-compose.yml missing services section")
	}

	return nil
}

// ──────────────────────────────────────────────────────────────
// LLM call abstraction
// ──────────────────────────────────────────────────────────────

func llmCall(ctx context.Context, provider *Provider, systemPrompt, userPrompt string) (string, error) {
	switch provider.Name {
	case "gemini":
		return geminiCall(ctx, provider, systemPrompt, userPrompt)
	default:
		return openAICompatibleCall(ctx, provider, systemPrompt, userPrompt)
	}
}

func openAICompatibleCall(ctx context.Context, provider *Provider, systemPrompt, userPrompt string) (string, error) {
	body := map[string]interface{}{
		"model": provider.Model,
		"messages": []map[string]string{
			{"role": "system", "content": systemPrompt},
			{"role": "user", "content": userPrompt},
		},
		"max_tokens":  8192,
		"temperature": 0.1,
		"response_format": map[string]string{
			"type": "json_object",
		},
	}

	data, _ := json.Marshal(body)
	req, err := http.NewRequestWithContext(ctx, "POST", provider.Endpoint, bytes.NewReader(data))
	if err != nil {
		return "", fmt.Errorf("agent: request creation failed: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+provider.APIKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("agent: LLM request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("agent: LLM returned %d: %s", resp.StatusCode, truncate(string(respBody), 300))
	}

	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("agent: failed to decode response: %w", err)
	}
	if len(result.Choices) == 0 {
		return "", fmt.Errorf("agent: LLM returned no choices")
	}
	return result.Choices[0].Message.Content, nil
}

func geminiCall(ctx context.Context, provider *Provider, systemPrompt, userPrompt string) (string, error) {
	body := map[string]interface{}{
		"system_instruction": map[string]interface{}{
			"parts": []map[string]string{{"text": systemPrompt}},
		},
		"contents": []map[string]interface{}{
			{
				"role":  "user",
				"parts": []map[string]string{{"text": userPrompt}},
			},
		},
		"generationConfig": map[string]interface{}{
			"temperature":     0.1,
			"maxOutputTokens": 8192,
			"responseMimeType": "application/json",
		},
	}

	data, _ := json.Marshal(body)
	endpoint := provider.Endpoint + "?key=" + provider.APIKey
	req, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewReader(data))
	if err != nil {
		return "", fmt.Errorf("agent: request creation failed: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("agent: Gemini request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("agent: Gemini returned %d: %s", resp.StatusCode, truncate(string(respBody), 300))
	}

	var result struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("agent: failed to decode Gemini response: %w", err)
	}
	if len(result.Candidates) > 0 && len(result.Candidates[0].Content.Parts) > 0 {
		return result.Candidates[0].Content.Parts[0].Text, nil
	}
	return "", fmt.Errorf("agent: Gemini returned empty response")
}

// ──────────────────────────────────────────────────────────────
// Helpers
// ──────────────────────────────────────────────────────────────

// extractJSON tries to extract a JSON object or array from LLM output
// that might be wrapped in markdown fences or extra text.
func extractJSON(s string) string {
	s = strings.TrimSpace(s)

	// Strip markdown code fences
	if strings.HasPrefix(s, "```") {
		lines := strings.Split(s, "\n")
		start, end := 0, len(lines)
		for i, line := range lines {
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "```") {
				if start == 0 {
					start = i + 1
				} else {
					end = i
					break
				}
			}
		}
		if start < end {
			s = strings.Join(lines[start:end], "\n")
			s = strings.TrimSpace(s)
		}
	}

	// Try to find JSON object boundaries
	if idx := strings.Index(s, "{"); idx >= 0 {
		depth := 0
		inString := false
		escaped := false
		for i := idx; i < len(s); i++ {
			ch := s[i]
			if escaped {
				escaped = false
				continue
			}
			if ch == '\\' && inString {
				escaped = true
				continue
			}
			if ch == '"' {
				inString = !inString
				continue
			}
			if inString {
				continue
			}
			if ch == '{' {
				depth++
			} else if ch == '}' {
				depth--
				if depth == 0 {
					return s[idx : i+1]
				}
			}
		}
	}

	// Try array
	if idx := strings.Index(s, "["); idx >= 0 {
		depth := 0
		inString := false
		escaped := false
		for i := idx; i < len(s); i++ {
			ch := s[i]
			if escaped {
				escaped = false
				continue
			}
			if ch == '\\' && inString {
				escaped = true
				continue
			}
			if ch == '"' {
				inString = !inString
				continue
			}
			if inString {
				continue
			}
			if ch == '[' {
				depth++
			} else if ch == ']' {
				depth--
				if depth == 0 {
					return s[idx : i+1]
				}
			}
		}
	}

	return s
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}
