package envvar

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"chimera/internal/detector"
)

// JavaScript/TypeScript patterns
var jsPatterns = []*regexp.Regexp{
	regexp.MustCompile(`process\.env\.([A-Z_][A-Z0-9_]*)`),
	regexp.MustCompile(`process\.env\[['"]([A-Z_][A-Z0-9_]*)['"\]]`),
	regexp.MustCompile(`import\.meta\.env\.([A-Z_][A-Z0-9_]*)`),
	regexp.MustCompile(`import\.meta\.env\[['"]([A-Z_][A-Z0-9_]*)['"\]]`),
	regexp.MustCompile(`NEXT_PUBLIC_([A-Z_][A-Z0-9_]*)`),
	regexp.MustCompile(`env\.([A-Z_][A-Z0-9_]*)`),
}

// Python patterns
var pythonPatterns = []*regexp.Regexp{
	regexp.MustCompile(`os\.environ\.get\(['"]([A-Z_][A-Z0-9_]*)['"\]]`),
	regexp.MustCompile(`os\.environ\[['"]([A-Z_][A-Z0-9_]*)['"\]]`),
	regexp.MustCompile(`os\.getenv\(['"]([A-Z_][A-Z0-9_]*)['"\]]`),
	regexp.MustCompile(`getenv\(['"]([A-Z_][A-Z0-9_]*)['"\]]`),
	regexp.MustCompile(`config\(['"]([A-Z_][A-Z0-9_]*)['"\]]`),
	regexp.MustCompile(`env\(['"]([A-Z_][A-Z0-9_]*)['"\]]`),
	regexp.MustCompile(`settings\.([A-Z_][A-Z0-9_]*)`),
}

var excludedDirs = map[string]bool{
	"node_modules":  true,
	".git":          true,
	".next":         true,
	"dist":          true,
	"build":         true,
	"__pycache__":   true,
	".venv":         true,
	"venv":          true,
	".cache":        true,
	"vendor":        true,
	".pytest_cache": true,
}

// Result holds env var detection results
type Result struct {
	Directory   string    `json:"directory"`
	ServiceType string    `json:"service_type"`
	Technology  string    `json:"technology"`
	Vars        []EnvVar  `json:"vars"`
	EnvContent  string    `json:"env_content,omitempty"` // LLM-generated content
}

// EnvVar holds info about a single env var
type EnvVar struct {
	Name        string   `json:"name"`
	Occurrences int      `json:"occurrences"`
	Files       []string `json:"files"`
}

// Detect scans for environment variables
func Detect(rootDir string, services []detector.Service) []Result {
	results := []Result{}

	// Process frontend first, then backend
	var frontendServices, backendServices []detector.Service
	for _, svc := range services {
		if svc.Type == "frontend" {
			frontendServices = append(frontendServices, svc)
		} else {
			backendServices = append(backendServices, svc)
		}
	}

	allServices := append(frontendServices, backendServices...)

	for _, service := range allServices {
		serviceDir := filepath.Join(rootDir, service.Directory)
		vars := scanDirectory(serviceDir, service.Language, rootDir)

		result := Result{
			Directory:   service.Directory,
			ServiceType: service.Type,
			Technology:  service.Framework,
			Vars:        vars,
		}

		results = append(results, result)
	}

	return results
}

func scanDirectory(dir, language, rootDir string) []EnvVar {
	varMap := make(map[string]*EnvVar)

	// Scan .env.example files first
	scanEnvExampleFiles(dir, varMap)

	// Scan source files
	patterns := jsPatterns
	if language == "python" {
		patterns = pythonPatterns
	}

	filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			if info != nil && info.IsDir() && excludedDirs[info.Name()] {
				return filepath.SkipDir
			}
			return nil
		}

		ext := filepath.Ext(path)
		validExts := map[string]bool{
			".js": true, ".jsx": true, ".ts": true, ".tsx": true,
			".mjs": true, ".cjs": true, ".py": true,
		}

		if !validExts[ext] {
			return nil
		}

		if info.Size() > 1024*1024 {
			return nil
		}

		content, err := os.ReadFile(path)
		if err != nil {
			return nil
		}

		text := string(content)
		relPath, _ := filepath.Rel(rootDir, path)

		for _, pattern := range patterns {
			matches := pattern.FindAllStringSubmatch(text, -1)
			for _, match := range matches {
				if len(match) >= 2 {
					varName := strings.TrimSpace(match[1])
					if varName == "" {
						continue
					}

					if existing, ok := varMap[varName]; ok {
						existing.Occurrences++
						existing.Files = append(existing.Files, relPath)
					} else {
						varMap[varName] = &EnvVar{
							Name:        varName,
							Occurrences: 1,
							Files:       []string{relPath},
						}
					}
				}
			}
		}

		return nil
	})

	// Convert to slice
	vars := []EnvVar{}
	for _, v := range varMap {
		vars = append(vars, *v)
	}

	return vars
}

func scanEnvExampleFiles(dir string, varMap map[string]*EnvVar) {
	exampleFiles := []string{".env.example", ".env.sample", ".env.template", ".env.local.example"}

	for _, filename := range exampleFiles {
		path := filepath.Join(dir, filename)
		if _, err := os.Stat(path); err != nil {
			continue
		}

		content, err := os.ReadFile(path)
		if err != nil {
			continue
		}

		lines := strings.Split(string(content), "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}

			parts := strings.SplitN(line, "=", 2)
			if len(parts) > 0 {
				varName := strings.TrimSpace(parts[0])
				if varName != "" {
					if existing, ok := varMap[varName]; ok {
						existing.Occurrences++
						existing.Files = append(existing.Files, filename)
					} else {
						varMap[varName] = &EnvVar{
							Name:        varName,
							Occurrences: 1,
							Files:       []string{filename},
						}
					}
				}
			}
		}
	}
}

// GenerateEnvExample generates .env.example content
func GenerateEnvExample(vars []EnvVar, technology string) string {
	var b strings.Builder

	b.WriteString(fmt.Sprintf("# Environment variables for %s\n", technology))
	b.WriteString("# Copy this file to .env and fill in the values\n\n")

	// Add common vars
	commonVars := getCommonVars(technology)
	for _, v := range commonVars {
		b.WriteString(fmt.Sprintf("%s=\n", v))
	}

	if len(commonVars) > 0 && len(vars) > 0 {
		b.WriteString("\n# Detected variables\n")
	}

	for _, v := range vars {
		b.WriteString(fmt.Sprintf("%s=\n", v.Name))
	}

	return b.String()
}

func getCommonVars(technology string) []string {
	commonVars := map[string][]string{
		"fastapi": {"DATABASE_URL", "SECRET_KEY", "API_KEY"},
		"flask":   {"DATABASE_URL", "SECRET_KEY", "FLASK_ENV"},
		"django":  {"DATABASE_URL", "SECRET_KEY", "DEBUG", "ALLOWED_HOSTS"},
		"next":    {"NEXT_PUBLIC_API_URL", "DATABASE_URL"},
		"express": {"DATABASE_URL", "PORT", "NODE_ENV"},
	}

	if vars, ok := commonVars[technology]; ok {
		return vars
	}
	return []string{}
}
