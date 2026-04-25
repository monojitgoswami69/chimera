package scanner

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var sourceEnvVarPatterns = []*regexp.Regexp{
	regexp.MustCompile(`process\.env\.([A-Za-z_][A-Za-z0-9_]*)`),
	regexp.MustCompile(`process\.env\[\s*["']([A-Za-z_][A-Za-z0-9_]*)["']\s*\]`),
	regexp.MustCompile(`os\.(?:Getenv|LookupEnv)\(\s*["']([A-Za-z_][A-Za-z0-9_]*)["']\s*\)`),
	regexp.MustCompile(`os\.environ\.get\(\s*["']([A-Za-z_][A-Za-z0-9_]*)["']\s*\)`),
	regexp.MustCompile(`os\.environ\[\s*["']([A-Za-z_][A-Za-z0-9_]*)["']\s*\]`),
	regexp.MustCompile(`\bgetenv\(\s*["']([A-Za-z_][A-Za-z0-9_]*)["']\s*\)`),
	regexp.MustCompile(`ENV\[\s*["']([A-Za-z_][A-Za-z0-9_]*)["']\s*\]`),
}

var sourceEnvVarExtensions = map[string]bool{
	".go":   true,
	".js":   true,
	".jsx":  true,
	".ts":   true,
	".tsx":  true,
	".mjs":  true,
	".cjs":  true,
	".py":   true,
	".rb":   true,
	".java": true,
	".php":  true,
	".cs":   true,
	".sh":   true,
}

var sourceEnvVarIgnoredDirs = map[string]bool{
	".git":         true,
	".idea":        true,
	".vscode":      true,
	"node_modules": true,
	"vendor":       true,
	"build":        true,
	"dist":         true,
	"tmp":          true,
	"temp":         true,
	".venv":        true,
	"venv":         true,
	".gomod":       true,
	".gopath":      true,
}

// Scanner scans a repository for language runtimes and infrastructure dependencies
type Scanner struct {
	workspaceDir string
}

// ScanResult contains the results of a repository scan
type ScanResult struct {
	Languages      []Language
	Infrastructure []Infrastructure
	EnvVars        []string // Only variable names, never values (security requirement)
	Ports          []int
}

// Language represents a detected programming language
type Language struct {
	Name    string
	Version string
	Files   []string
}

// Infrastructure represents a detected infrastructure dependency
type Infrastructure struct {
	Name    string
	Type    string // postgres, mysql, mongodb, redis, rabbitmq, elasticsearch
	Version string
}

// NewScanner creates a new repository scanner
func NewScanner(workspaceDir string) *Scanner {
	return &Scanner{
		workspaceDir: workspaceDir,
	}
}

// Scan performs a comprehensive scan of the repository
// Performance requirement: Must complete in < 5 seconds
func (s *Scanner) Scan(ctx context.Context) (*ScanResult, error) {
	result := &ScanResult{
		Languages:      make([]Language, 0),
		Infrastructure: make([]Infrastructure, 0),
		EnvVars:        make([]string, 0),
		Ports:          make([]int, 0),
	}

	// Check for cancellation
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	// Scan for languages
	if err := s.scanLanguages(ctx, result); err != nil {
		return nil, fmt.Errorf("failed to scan languages: %w", err)
	}

	// Scan for infrastructure dependencies
	if err := s.scanInfrastructure(ctx, result); err != nil {
		return nil, fmt.Errorf("failed to scan infrastructure: %w", err)
	}

	// Scan for environment variables
	if err := s.scanEnvVars(ctx, result); err != nil {
		return nil, fmt.Errorf("failed to scan environment variables: %w", err)
	}

	// Scan for ports
	if err := s.scanPorts(ctx, result); err != nil {
		return nil, fmt.Errorf("failed to scan ports: %w", err)
	}

	return result, nil
}

// scanLanguages detects programming languages in the repository
// Supports both root-level and monorepo structures (subdirectories)
func (s *Scanner) scanLanguages(ctx context.Context, result *ScanResult) error {
	nodeFiles := []string{}
	pythonFiles := []string{}
	goFiles := []string{}

	// Check root directory first
	if s.fileExists("package.json") {
		nodeFiles = append(nodeFiles, "package.json")
	}

	if s.fileExists("requirements.txt") {
		pythonFiles = append(pythonFiles, "requirements.txt")
	}
	if s.fileExists("Pipfile") {
		pythonFiles = append(pythonFiles, "Pipfile")
	}
	if s.fileExists("pyproject.toml") {
		pythonFiles = append(pythonFiles, "pyproject.toml")
	}

	if s.fileExists("go.mod") {
		goFiles = append(goFiles, "go.mod")
	}

	// Scan subdirectories for monorepo structures (max depth 1)
	entries, err := os.ReadDir(s.workspaceDir)
	if err == nil {
		skipDirs := map[string]bool{
			"node_modules": true, ".git": true, ".svn": true, "__pycache__": true,
			".tox": true, ".venv": true, "venv": true, "vendor": true,
			"dist": true, "build": true, ".next": true, ".cache": true,
			".gomod": true, ".gopath": true, ".chimera": true,
		}

		for _, entry := range entries {
			if !entry.IsDir() || skipDirs[entry.Name()] {
				continue
			}

			subdir := filepath.Join(s.workspaceDir, entry.Name())

			// Check for Node.js in subdirectory
			if _, err := os.Stat(filepath.Join(subdir, "package.json")); err == nil {
				nodeFiles = append(nodeFiles, filepath.Join(entry.Name(), "package.json"))
			}

			// Check for Python in subdirectory
			if _, err := os.Stat(filepath.Join(subdir, "requirements.txt")); err == nil {
				pythonFiles = append(pythonFiles, filepath.Join(entry.Name(), "requirements.txt"))
			}
			if _, err := os.Stat(filepath.Join(subdir, "Pipfile")); err == nil {
				pythonFiles = append(pythonFiles, filepath.Join(entry.Name(), "Pipfile"))
			}
			if _, err := os.Stat(filepath.Join(subdir, "pyproject.toml")); err == nil {
				pythonFiles = append(pythonFiles, filepath.Join(entry.Name(), "pyproject.toml"))
			}

			// Check for Go in subdirectory
			if _, err := os.Stat(filepath.Join(subdir, "go.mod")); err == nil {
				goFiles = append(goFiles, filepath.Join(entry.Name(), "go.mod"))
			}
		}
	}

	// Add detected languages to result
	if len(nodeFiles) > 0 {
		version := s.detectNodeVersion()
		result.Languages = append(result.Languages, Language{
			Name:    "Node.js",
			Version: version,
			Files:   nodeFiles,
		})
	}

	if len(pythonFiles) > 0 {
		version := s.detectPythonVersion()
		result.Languages = append(result.Languages, Language{
			Name:    "Python",
			Version: version,
			Files:   pythonFiles,
		})
	}

	if len(goFiles) > 0 {
		version := s.detectGoVersion()
		result.Languages = append(result.Languages, Language{
			Name:    "Go",
			Version: version,
			Files:   goFiles,
		})
	}

	return nil
}

// scanInfrastructure detects infrastructure dependencies from package manifests,
// docker-compose files, and configuration files.
func (s *Scanner) scanInfrastructure(ctx context.Context, result *ScanResult) error {
	found := make(map[string]bool) // dedup by type

	// 1. Scan package manifests for database/cache driver dependencies
	infraPatterns := map[string][]string{
		"postgresql": {
			"pg", "postgres", "postgresql", "knex", "prisma", "typeorm",
			"psycopg2", "asyncpg", "sqlalchemy", "pgx", "lib/pq",
			"gorm.io/driver/postgres", "jackc/pgx",
		},
		"mysql": {
			"mysql", "mysql2", "pymysql", "mysqlclient",
			"go-sql-driver/mysql", "gorm.io/driver/mysql",
		},
		"mongodb": {
			"mongoose", "mongodb", "pymongo", "motor",
			"go.mongodb.org/mongo-driver",
		},
		"redis": {
			"redis", "ioredis", "bull", "bullmq", "celery",
			"go-redis", "github.com/redis/go-redis",
		},
		"rabbitmq": {
			"amqplib", "amqp", "pika", "celery",
			"github.com/rabbitmq/amqp091-go", "github.com/streadway/amqp",
		},
		"elasticsearch": {
			"elasticsearch", "@elastic/elasticsearch",
			"elasticsearch-py", "olivere/elastic",
		},
	}

	// Read all manifest files into a combined string for matching
	manifestFiles := []string{
		"package.json", "package-lock.json",
		"requirements.txt", "Pipfile", "pyproject.toml",
		"go.mod", "go.sum",
		"Gemfile", "composer.json",
	}
	var manifestContent strings.Builder
	for _, file := range manifestFiles {
		path := filepath.Join(s.workspaceDir, file)
		if data, err := os.ReadFile(path); err == nil {
			manifestContent.WriteString(string(data))
			manifestContent.WriteString("\n")
		}
	}
	content := strings.ToLower(manifestContent.String())

	for infraType, patterns := range infraPatterns {
		for _, pattern := range patterns {
			if strings.Contains(content, strings.ToLower(pattern)) {
				if !found[infraType] {
					found[infraType] = true
					result.Infrastructure = append(result.Infrastructure, Infrastructure{
						Name:    infraType,
						Type:    infraType,
						Version: "",
					})
				}
				break
			}
		}
	}

	// 2. Parse existing docker-compose.yml for image-based detection
	composeFiles := []string{"docker-compose.yml", "docker-compose.yaml", "compose.yml", "compose.yaml"}
	for _, file := range composeFiles {
		path := filepath.Join(s.workspaceDir, file)
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		composeContent := strings.ToLower(string(data))

		imageMap := map[string]string{
			"postgres:":      "postgresql",
			"mysql:":         "mysql",
			"mariadb:":       "mysql",
			"mongo:":         "mongodb",
			"redis:":         "redis",
			"rabbitmq:":      "rabbitmq",
			"elasticsearch:": "elasticsearch",
		}
		for image, infraType := range imageMap {
			if strings.Contains(composeContent, image) && !found[infraType] {
				found[infraType] = true
				result.Infrastructure = append(result.Infrastructure, Infrastructure{
					Name: infraType,
					Type: infraType,
				})
			}
		}
	}

	// 3. Detect from env files (connection string patterns)
	envFiles := []string{".env.example", ".env.sample", ".env.template"}
	for _, file := range envFiles {
		path := filepath.Join(s.workspaceDir, file)
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		envContent := strings.ToLower(string(data))

		envPatterns := map[string][]string{
			"postgresql": {"postgres://", "postgresql://", "pghost", "pgdatabase"},
			"mysql":      {"mysql://", "mysql_host"},
			"mongodb":    {"mongodb://", "mongodb+srv://", "mongo_url"},
			"redis":      {"redis://", "rediss://", "redis_url", "redis_host"},
			"rabbitmq":   {"amqp://", "amqps://", "rabbitmq_url"},
		}
		for infraType, patterns := range envPatterns {
			for _, pattern := range patterns {
				if strings.Contains(envContent, pattern) && !found[infraType] {
					found[infraType] = true
					result.Infrastructure = append(result.Infrastructure, Infrastructure{
						Name: infraType,
						Type: infraType,
					})
					break
				}
			}
		}
	}

	return nil
}

// scanEnvVars scans for environment variable names (never values)
// Security requirement: Only extract variable names, never actual values
func (s *Scanner) scanEnvVars(ctx context.Context, result *ScanResult) error {
	envFiles := []string{
		".env.example",
		".env.sample",
		".env.template",
	}

	envVarMap := make(map[string]bool)

	for _, file := range envFiles {
		filePath := filepath.Join(s.workspaceDir, file)
		if _, err := os.Stat(filePath); err == nil {
			vars, err := s.extractEnvVarNames(filePath)
			if err != nil {
				continue
			}
			for _, v := range vars {
				envVarMap[v] = true
			}
		}
	}

	// Also extract env var names referenced in source code patterns.
	if err := s.scanSourceEnvVars(ctx, envVarMap); err != nil {
		return err
	}

	// Convert map to slice
	for varName := range envVarMap {
		result.EnvVars = append(result.EnvVars, varName)
	}

	return nil
}

// scanSourceEnvVars extracts environment variable names referenced in source code.
func (s *Scanner) scanSourceEnvVars(ctx context.Context, envVarMap map[string]bool) error {
	return filepath.WalkDir(s.workspaceDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if d.IsDir() {
			if sourceEnvVarIgnoredDirs[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}

		ext := strings.ToLower(filepath.Ext(path))
		if !sourceEnvVarExtensions[ext] {
			return nil
		}

		info, statErr := d.Info()
		if statErr != nil {
			return nil
		}
		if info.Size() > 1024*1024 {
			return nil
		}

		content, readErr := os.ReadFile(path)
		if readErr != nil {
			return nil
		}

		text := string(content)
		for _, pattern := range sourceEnvVarPatterns {
			matches := pattern.FindAllStringSubmatch(text, -1)
			for _, match := range matches {
				if len(match) >= 2 {
					name := strings.TrimSpace(match[1])
					if name != "" {
						envVarMap[name] = true
					}
				}
			}
		}

		return nil
	})
}

// extractEnvVarNames extracts only the variable names from an env file
func (s *Scanner) extractEnvVarNames(filePath string) ([]string, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	lines := strings.Split(string(content), "\n")
	varNames := make([]string, 0)

	for _, line := range lines {
		line = strings.TrimSpace(line)

		// Skip comments and empty lines
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Extract variable name (before =)
		parts := strings.SplitN(line, "=", 2)
		if len(parts) > 0 {
			varName := strings.TrimSpace(parts[0])
			if varName != "" {
				varNames = append(varNames, varName)
			}
		}
	}

	return varNames, nil
}

// scanPorts scans for port numbers in configuration files and source code.
func (s *Scanner) scanPorts(ctx context.Context, result *ScanResult) error {
	portFiles := []string{
		"package.json",
		".env.example", ".env.sample", ".env.template",
		"Procfile",
		"config/settings.py", "config/application.rb",
	}

	portRegex := regexp.MustCompile(`(?i)(?:PORT|port)[=:]\s*["']?(\d{4,5})["']?`)

	for _, file := range portFiles {
		path := filepath.Join(s.workspaceDir, file)
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}

		matches := portRegex.FindAllStringSubmatch(string(data), -1)
		for _, match := range matches {
			if len(match) >= 2 {
				port := 0
				fmt.Sscanf(match[1], "%d", &port)
				if port >= 1024 && port <= 65535 {
					result.Ports = append(result.Ports, port)
					return nil // take the first valid port
				}
			}
		}
	}

	// Fallback: detect from language conventions
	for _, lang := range result.Languages {
		switch lang.Name {
		case "Node.js":
			result.Ports = append(result.Ports, 3000)
			return nil
		case "Python":
			result.Ports = append(result.Ports, 8000)
			return nil
		case "Go":
			result.Ports = append(result.Ports, 8080)
			return nil
		}
	}

	return nil
}

// fileExists checks if a file exists in the workspace
func (s *Scanner) fileExists(filename string) bool {
	path := filepath.Join(s.workspaceDir, filename)
	_, err := os.Stat(path)
	return err == nil
}

// detectNodeVersion attempts to detect the Node.js version requirement
func (s *Scanner) detectNodeVersion() string {
	return s.detectNodeVersionInDir(s.workspaceDir)
}

// detectNodeVersionInDir detects Node.js version in a specific directory
func (s *Scanner) detectNodeVersionInDir(dir string) string {
	// Check .nvmrc
	nvmrcPath := filepath.Join(dir, ".nvmrc")
	if data, err := os.ReadFile(nvmrcPath); err == nil {
		v := strings.TrimSpace(string(data))
		v = strings.TrimPrefix(v, "v")
		if v != "" {
			// Extract major version (e.g., "18.17.0" -> "18")
			parts := strings.Split(v, ".")
			if len(parts) > 0 && parts[0] != "" {
				return parts[0]
			}
		}
	}

	// Check package.json for engines.node
	pkgPath := filepath.Join(dir, "package.json")
	if data, err := os.ReadFile(pkgPath); err == nil {
		content := string(data)
		// Simple regex to extract engines.node version
		enginesRegex := regexp.MustCompile(`"engines"\s*:\s*\{[^}]*"node"\s*:\s*"[>=~^]*(\d+)`)
		if m := enginesRegex.FindStringSubmatch(content); len(m) >= 2 {
			return m[1]
		}
	}

	return "20"
}

// detectPythonVersion attempts to detect the Python version requirement
func (s *Scanner) detectPythonVersion() string {
	return s.detectPythonVersionInDir(s.workspaceDir)
}

// detectPythonVersionInDir detects Python version in a specific directory
func (s *Scanner) detectPythonVersionInDir(dir string) string {
	// Check .python-version (pyenv)
	pyVerPath := filepath.Join(dir, ".python-version")
	if data, err := os.ReadFile(pyVerPath); err == nil {
		v := strings.TrimSpace(string(data))
		if v != "" {
			return v
		}
	}

	// Check runtime.txt (Heroku style)
	runtimePath := filepath.Join(dir, "runtime.txt")
	if data, err := os.ReadFile(runtimePath); err == nil {
		content := strings.TrimSpace(string(data))
		// e.g., "python-3.11.5"
		content = strings.TrimPrefix(content, "python-")
		parts := strings.Split(content, ".")
		if len(parts) >= 2 {
			return parts[0] + "." + parts[1]
		}
	}

	// Check pyproject.toml for requires-python
	pyprojectPath := filepath.Join(dir, "pyproject.toml")
	if data, err := os.ReadFile(pyprojectPath); err == nil {
		pyRegex := regexp.MustCompile(`requires-python\s*=\s*"[>=]*(\d+\.\d+)`)
		if m := pyRegex.FindStringSubmatch(string(data)); len(m) >= 2 {
			return m[1]
		}
	}

	return "3.11"
}

// detectGoVersion attempts to detect the Go version requirement
func (s *Scanner) detectGoVersion() string {
	return s.detectGoVersionInDir(s.workspaceDir)
}

// detectGoVersionInDir detects Go version in a specific directory
func (s *Scanner) detectGoVersionInDir(dir string) string {
	// Parse go.mod for Go version
	goModPath := filepath.Join(dir, "go.mod")
	content, err := os.ReadFile(goModPath)
	if err != nil {
		return "1.21"
	}

	lines := strings.Split(string(content), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "go ") {
			version := strings.TrimPrefix(line, "go ")
			return strings.TrimSpace(version)
		}
	}

	return "1.21"
}
