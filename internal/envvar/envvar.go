// Package envvar finds environment variables referenced in source code.
package envvar

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"chimera/internal/detector"
)

// JS/TS patterns. Each pattern captures group #1 as the variable name.
// IMPORTANT: NEXT_PUBLIC_ matches must capture the whole name including the prefix.
var jsPatterns = []*regexp.Regexp{
	regexp.MustCompile(`process\.env\.([A-Z][A-Z0-9_]*)`),
	regexp.MustCompile(`process\.env\[['"]([A-Z][A-Z0-9_]*)['"]\]`),
	regexp.MustCompile(`import\.meta\.env\.([A-Z][A-Z0-9_]*)`),
	regexp.MustCompile(`import\.meta\.env\[['"]([A-Z][A-Z0-9_]*)['"]\]`),
	regexp.MustCompile(`\b(NEXT_PUBLIC_[A-Z0-9_]+)\b`),
	regexp.MustCompile(`\b(VITE_[A-Z0-9_]+)\b`),
	regexp.MustCompile(`\b(REACT_APP_[A-Z0-9_]+)\b`),
}

// Python patterns.
var pythonPatterns = []*regexp.Regexp{
	regexp.MustCompile(`os\.environ\.get\(['"]([A-Z][A-Z0-9_]*)['"]\)`),
	regexp.MustCompile(`os\.environ\[['"]([A-Z][A-Z0-9_]*)['"]\]`),
	regexp.MustCompile(`os\.getenv\(['"]([A-Z][A-Z0-9_]*)['"]\)`),
	regexp.MustCompile(`getenv\(['"]([A-Z][A-Z0-9_]*)['"]\)`),
	regexp.MustCompile(`env\(['"]([A-Z][A-Z0-9_]*)['"]\)`),
	regexp.MustCompile(`config\(['"]([A-Z][A-Z0-9_]*)['"]\)`),
}

var excludedDirs = map[string]bool{
	"node_modules":    true,
	".git":            true,
	".next":           true,
	".nuxt":           true,
	"dist":            true,
	"build":           true,
	"__pycache__":     true,
	".venv":           true,
	"venv":            true,
	"env":             true,
	".cache":          true,
	"vendor":          true,
	".pytest_cache":   true,
	"chimera-outputs": true,
	".idea":           true,
	".vscode":         true,
}

// Result holds env vars detected for a single service.
type Result struct {
	ServiceID   string   `json:"service_id"`
	Directory   string   `json:"directory"`
	ServiceType string   `json:"service_type"`
	Technology  string   `json:"technology"`
	Vars        []EnvVar `json:"vars"`
	EnvContent  string   `json:"env_content,omitempty"` // populated when LLM rewrites the .env.example
}

// EnvVar is a single discovered variable.
type EnvVar struct {
	Name        string   `json:"name"`
	Occurrences int      `json:"occurrences"`
	Files       []string `json:"files"`
}

// Detect scans each service's directory for env-var references.
func Detect(rootDir string, services []detector.Service) []Result {
	results := make([]Result, 0, len(services))
	for _, svc := range services {
		serviceDir := filepath.Join(rootDir, svc.Directory)
		vars := scanDirectory(serviceDir, svc.Language, rootDir)
		results = append(results, Result{
			ServiceID:   svc.ID,
			Directory:   svc.Directory,
			ServiceType: svc.Type,
			Technology:  svc.Framework,
			Vars:        vars,
		})
	}
	return results
}

func scanDirectory(dir, language, rootDir string) []EnvVar {
	varMap := make(map[string]*EnvVar)

	scanEnvExampleFiles(dir, varMap)

	patterns := jsPatterns
	validExts := map[string]bool{
		".js": true, ".jsx": true, ".ts": true, ".tsx": true,
		".mjs": true, ".cjs": true,
	}
	if language == "python" {
		patterns = pythonPatterns
		validExts = map[string]bool{".py": true}
	}

	filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if excludedDirs[d.Name()] {
				return fs.SkipDir
			}
			return nil
		}
		if !validExts[filepath.Ext(path)] {
			return nil
		}
		info, err := d.Info()
		if err != nil || info.Size() > 1<<20 {
			return nil
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		rel, _ := filepath.Rel(rootDir, path)
		text := string(content)
		for _, pattern := range patterns {
			for _, match := range pattern.FindAllStringSubmatch(text, -1) {
				if len(match) < 2 {
					continue
				}
				name := strings.TrimSpace(match[1])
				if name == "" {
					continue
				}
				if existing, ok := varMap[name]; ok {
					existing.Occurrences++
					if !containsString(existing.Files, rel) {
						existing.Files = append(existing.Files, rel)
					}
				} else {
					varMap[name] = &EnvVar{Name: name, Occurrences: 1, Files: []string{rel}}
				}
			}
		}
		return nil
	})

	out := make([]EnvVar, 0, len(varMap))
	for _, v := range varMap {
		sort.Strings(v.Files)
		out = append(out, *v)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

func scanEnvExampleFiles(dir string, varMap map[string]*EnvVar) {
	names := []string{".env.example", ".env.sample", ".env.template", ".env.local.example"}
	for _, name := range names {
		path := filepath.Join(dir, name)
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		for _, line := range strings.Split(string(data), "\n") {
			line = strings.TrimSpace(line)
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			parts := strings.SplitN(line, "=", 2)
			key := strings.TrimSpace(parts[0])
			if key == "" {
				continue
			}
			if existing, ok := varMap[key]; ok {
				if !containsString(existing.Files, name) {
					existing.Files = append(existing.Files, name)
				}
			} else {
				varMap[key] = &EnvVar{Name: key, Occurrences: 1, Files: []string{name}}
			}
		}
	}
}

func containsString(haystack []string, needle string) bool {
	for _, s := range haystack {
		if s == needle {
			return true
		}
	}
	return false
}

// GenerateEnvExample renders a .env.example file for the given technology.
// Common variables for the technology are added before discovered variables,
// with no duplicates.
func GenerateEnvExample(vars []EnvVar, technology string) string {
	seen := map[string]bool{}

	var b strings.Builder
	fmt.Fprintf(&b, "# .env.example for %s service\n", technology)
	b.WriteString("# Copy to .env and fill in real values before running docker compose up.\n\n")

	common := getCommonVars(technology)
	if len(common) > 0 {
		fmt.Fprintf(&b, "# Common defaults for %s\n", technology)
		for _, v := range common {
			if seen[v.name] {
				continue
			}
			seen[v.name] = true
			if v.comment != "" {
				fmt.Fprintf(&b, "# %s\n", v.comment)
			}
			fmt.Fprintf(&b, "%s=%s\n", v.name, v.example)
		}
		b.WriteString("\n")
	}

	if len(vars) > 0 {
		b.WriteString("# Detected from source code\n")
		for _, v := range vars {
			if seen[v.Name] {
				continue
			}
			seen[v.Name] = true
			fmt.Fprintf(&b, "%s=\n", v.Name)
		}
	}
	return b.String()
}

type commonVar struct {
	name    string
	example string
	comment string
}

func getCommonVars(technology string) []commonVar {
	switch technology {
	case "fastapi":
		return []commonVar{
			{"DATABASE_URL", "postgresql://user:pass@db:5432/app", "App database connection string"},
			{"SECRET_KEY", "change-me", "Application secret"},
		}
	case "flask":
		return []commonVar{
			{"FLASK_ENV", "production", ""},
			{"DATABASE_URL", "postgresql://user:pass@db:5432/app", ""},
			{"SECRET_KEY", "change-me", ""},
		}
	case "django":
		return []commonVar{
			{"DJANGO_SETTINGS_MODULE", "project.settings", ""},
			{"DEBUG", "0", ""},
			{"SECRET_KEY", "change-me", ""},
			{"DATABASE_URL", "postgresql://user:pass@db:5432/app", ""},
			{"ALLOWED_HOSTS", "*", ""},
		}
	case "next":
		return []commonVar{
			{"NODE_ENV", "production", ""},
			{"NEXT_PUBLIC_API_URL", "http://localhost:3000/api", "URL exposed to the browser"},
		}
	case "express", "fastify", "nest", "node":
		return []commonVar{
			{"NODE_ENV", "production", ""},
			{"PORT", "3000", ""},
			{"DATABASE_URL", "postgresql://user:pass@db:5432/app", ""},
		}
	case "react", "vite", "cra":
		return []commonVar{
			{"VITE_API_URL", "http://localhost:3000/api", "(Vite) URL exposed to the browser"},
		}
	}
	return nil
}
