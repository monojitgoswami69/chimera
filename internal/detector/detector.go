package detector

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Result holds detection results
type Result struct {
	Repo     string    `json:"repo"`
	Services []Service `json:"services"`
}

// Service represents a detected service
type Service struct {
	ID              string   `json:"id"`
	Type            string   `json:"type"` // frontend, backend
	Directory       string   `json:"directory"`
	Language        string   `json:"language"` // javascript, typescript, python
	Framework       string   `json:"framework"`
	IdentifierFiles []string `json:"identifier_files"`
	Confidence      string   `json:"confidence"` // high, medium, low
}

// Detect performs static analysis
func Detect(rootDir string) (*Result, error) {
	result := &Result{
		Repo:     filepath.Base(rootDir),
		Services: []Service{},
	}

	serviceID := 1

	// Find JavaScript/TypeScript services
	err := filepath.Walk(rootDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			if info != nil && info.IsDir() && shouldSkipDir(info.Name()) {
				return filepath.SkipDir
			}
			return nil
		}

		if info.Name() == "package.json" {
			relDir, _ := filepath.Rel(rootDir, filepath.Dir(path))
			if relDir == "." {
				relDir = ""
			}

			service := Service{
				ID:              fmt.Sprintf("service_%d", serviceID),
				Directory:       relDir,
				Language:        "javascript",
				IdentifierFiles: []string{filepath.Join(relDir, "package.json")},
			}

			// Detect framework
			framework, serviceType, confidence := detectJSFramework(path, relDir)
			service.Framework = framework
			service.Type = serviceType
			service.Confidence = confidence

			// Check for TypeScript
			tsPath := filepath.Join(filepath.Dir(path), "tsconfig.json")
			if _, err := os.Stat(tsPath); err == nil {
				service.Language = "typescript"
			}

			result.Services = append(result.Services, service)
			serviceID++
		}

		if info.Name() == "requirements.txt" || info.Name() == "pyproject.toml" {
			relDir, _ := filepath.Rel(rootDir, filepath.Dir(path))
			if relDir == "." {
				relDir = ""
			}

			service := Service{
				ID:              fmt.Sprintf("service_%d", serviceID),
				Directory:       relDir,
				Language:        "python",
				IdentifierFiles: []string{filepath.Join(relDir, info.Name())},
			}

			framework, serviceType, confidence := detectPythonFramework(path, filepath.Dir(path))
			service.Framework = framework
			service.Type = serviceType
			service.Confidence = confidence

			result.Services = append(result.Services, service)
			serviceID++
		}

		return nil
	})

	return result, err
}

func detectJSFramework(packagePath, relDir string) (framework, serviceType, confidence string) {
	data, err := os.ReadFile(packagePath)
	if err != nil {
		return "unknown-node", "backend", "low"
	}

	var pkg struct {
		Dependencies    map[string]string `json:"dependencies"`
		DevDependencies map[string]string `json:"devDependencies"`
	}

	if err := json.Unmarshal(data, &pkg); err != nil {
		return "unknown-node", "backend", "low"
	}

	allDeps := make(map[string]bool)
	for k := range pkg.Dependencies {
		allDeps[k] = true
	}
	for k := range pkg.DevDependencies {
		allDeps[k] = true
	}

	// Check frameworks
	if allDeps["next"] {
		if strings.Contains(strings.ToLower(relDir), "api") || strings.Contains(strings.ToLower(relDir), "server") {
			return "next", "backend", "high"
		}
		return "next", "frontend", "high"
	}

	if allDeps["express"] {
		return "express", "backend", "high"
	}

	if allDeps["react"] && !allDeps["next"] {
		return "react", "frontend", "medium"
	}

	return "unknown-node", "backend", "low"
}

func detectPythonFramework(reqPath, dir string) (framework, serviceType, confidence string) {
	data, err := os.ReadFile(reqPath)
	if err != nil {
		return "unknown-python", "backend", "low"
	}

	content := strings.ToLower(string(data))

	if strings.Contains(content, "fastapi") {
		return "fastapi", "backend", "high"
	}

	if strings.Contains(content, "flask") {
		return "flask", "backend", "high"
	}

	if strings.Contains(content, "django") {
		managePath := filepath.Join(dir, "manage.py")
		if _, err := os.Stat(managePath); err == nil {
			return "django", "backend", "high"
		}
		return "django", "backend", "medium"
	}

	return "unknown-python", "backend", "low"
}

func shouldSkipDir(name string) bool {
	skipDirs := map[string]bool{
		"node_modules": true,
		".git":         true,
		".next":        true,
		"dist":         true,
		"build":        true,
		"__pycache__":  true,
		".venv":        true,
		"venv":         true,
		".cache":       true,
		"vendor":       true,
	}
	return skipDirs[name]
}

// ToJSON converts to JSON
func (r *Result) ToJSON() (string, error) {
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}
