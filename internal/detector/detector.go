// Package detector identifies the services that make up a project.
package detector

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// Result is the output of Detect.
type Result struct {
	Repo     string    `json:"repo"`
	Services []Service `json:"services"`
	// TruncatedFrom is non-zero when Detect found more services than MaxServices
	// and dropped the surplus. Callers should surface this to the user.
	TruncatedFrom int `json:"truncated_from,omitempty"`
}

// Service is the canonical description of a single deployable unit.
// All downstream subsystems (generator, env, output writer, quick-start, LLM
// validator) read from this struct, so changes here ripple consistently.
type Service struct {
	ID              string   `json:"id"`               // stable id, e.g. service-1, also used as compose service name
	Name            string   `json:"name"`             // friendly name, e.g. "frontend (next)"
	Type            string   `json:"type"`             // frontend | backend | worker
	Directory       string   `json:"directory"`        // path relative to repo root ("" == root)
	Language        string   `json:"language"`         // javascript | typescript | python
	Framework       string   `json:"framework"`        // next | express | react | vite | fastapi | flask | django | node
	PackageManager  string   `json:"package_manager"`  // npm | yarn | pnpm | pip | poetry
	InstallCmd      []string `json:"install_cmd"`      // e.g. ["npm","ci"]
	BuildCmd        []string `json:"build_cmd,omitempty"`
	StartCmd        []string `json:"start_cmd"`        // e.g. ["npm","start"]
	Port            int      `json:"port"`
	IdentifierFiles []string `json:"identifier_files"`
	Confidence      string   `json:"confidence"` // high | medium | low
}

// MaxServices is the default cap applied to Detect results.
const MaxServices = 12

// Detect performs static analysis over rootDir, returning at most MaxServices
// services. Use DetectWithCap to override the cap.
func Detect(rootDir string) (*Result, error) {
	return DetectWithCap(rootDir, MaxServices)
}

// DetectWithCap performs static analysis with an explicit cap on services.
// A cap of 0 or negative means no cap.
func DetectWithCap(rootDir string, cap int) (*Result, error) {
	result := &Result{
		Repo:     filepath.Base(rootDir),
		Services: []Service{},
	}

	type candidate struct {
		dir    string // absolute
		relDir string
		kind   string // js | python
		ident  string // file name that triggered detection
	}

	candidates := map[string]*candidate{}
	add := func(c *candidate) {
		// Per-directory candidates: prefer package.json over python markers when
		// both exist (e.g. JS app with a small python tool); prefer requirements
		// over pyproject when both python markers exist.
		key := c.relDir + "|" + c.kind
		existing, ok := candidates[key]
		if !ok {
			candidates[key] = c
			return
		}
		if existing.ident == "pyproject.toml" && c.ident == "requirements.txt" {
			candidates[key] = c
		}
	}

	err := filepath.WalkDir(rootDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if shouldSkipDir(d.Name()) && path != rootDir {
				return fs.SkipDir
			}
			return nil
		}
		name := d.Name()
		dir := filepath.Dir(path)
		rel, _ := filepath.Rel(rootDir, dir)
		if rel == "." {
			rel = ""
		}
		switch name {
		case "package.json":
			add(&candidate{dir: dir, relDir: rel, kind: "js", ident: name})
		case "requirements.txt", "pyproject.toml", "Pipfile":
			add(&candidate{dir: dir, relDir: rel, kind: "python", ident: name})
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walk failed: %w", err)
	}

	// Sort candidates by directory for deterministic IDs.
	keys := make([]string, 0, len(candidates))
	for k := range candidates {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	id := 0
	all := make([]Service, 0)
	for _, k := range keys {
		c := candidates[k]
		svc := Service{Directory: c.relDir}
		keep := true
		switch c.kind {
		case "js":
			keep = detectJS(c.dir, &svc)
		case "python":
			detectPython(c.dir, c.ident, &svc)
		}
		if !keep {
			continue
		}
		id++
		svc.ID = fmt.Sprintf("service-%d", id)
		svc.IdentifierFiles = []string{filepath.Join(c.relDir, c.ident)}
		if svc.Name == "" {
			svc.Name = fmt.Sprintf("%s (%s)", svc.Type, svc.Framework)
		}
		all = append(all, svc)
	}

	// Apply cap, preferring high-confidence services if we're over the limit.
	if cap > 0 && len(all) > cap {
		result.TruncatedFrom = len(all)
		ranked := make([]Service, len(all))
		copy(ranked, all)
		sort.SliceStable(ranked, func(i, j int) bool {
			return confidenceRank(ranked[i].Confidence) > confidenceRank(ranked[j].Confidence)
		})
		all = ranked[:cap]
		// Re-assign IDs in original directory order for stability.
		sort.SliceStable(all, func(i, j int) bool { return all[i].Directory < all[j].Directory })
		for i := range all {
			all[i].ID = fmt.Sprintf("service-%d", i+1)
		}
	}
	result.Services = all

	return result, nil
}

func detectJS(dir string, svc *Service) bool {
	svc.Language = "javascript"
	if _, err := os.Stat(filepath.Join(dir, "tsconfig.json")); err == nil {
		svc.Language = "typescript"
	}

	svc.PackageManager = detectJSPM(dir)

	data, _ := os.ReadFile(filepath.Join(dir, "package.json"))
	var pkg struct {
		Name            string            `json:"name"`
		Private         bool              `json:"private"`
		Workspaces      json.RawMessage   `json:"workspaces"`
		Scripts         map[string]string `json:"scripts"`
		Dependencies    map[string]string `json:"dependencies"`
		DevDependencies map[string]string `json:"devDependencies"`
		Main            string            `json:"main"`
		Bin             json.RawMessage   `json:"bin"`
	}
	if err := json.Unmarshal(data, &pkg); err != nil {
		return false
	}

	// Workspace root package.json (no real service of its own) — skip.
	if len(pkg.Workspaces) > 0 {
		return false
	}

	deps := map[string]bool{}
	for k := range pkg.Dependencies {
		deps[strings.ToLower(k)] = true
	}
	for k := range pkg.DevDependencies {
		deps[strings.ToLower(k)] = true
	}

	framework := ""
	switch {
	case deps["next"]:
		framework = "next"
	case deps["@nestjs/core"]:
		framework = "nest"
	case deps["fastify"]:
		framework = "fastify"
	case deps["express"], deps["koa"], deps["hapi"]:
		framework = "express"
	case deps["vite"]:
		framework = "vite"
	case deps["react-scripts"]:
		framework = "cra"
	case deps["react"]:
		framework = "react"
	}

	hasStart := pkg.Scripts["start"] != "" || pkg.Scripts["dev"] != ""

	// Skip libraries / build tooling: no recognised framework and no start
	// script means this is almost certainly not a deployable service.
	if framework == "" && !hasStart {
		return false
	}

	switch framework {
	case "next":
		svc.Framework = "next"
		svc.Type = "frontend"
		svc.Confidence = "high"
		svc.Port = 3000
		svc.BuildCmd = pmRun(svc.PackageManager, "build")
		svc.StartCmd = pmRun(svc.PackageManager, "start")
	case "nest":
		svc.Framework = "nest"
		svc.Type = "backend"
		svc.Confidence = "high"
		svc.Port = 3000
		svc.BuildCmd = pmRun(svc.PackageManager, "build")
		svc.StartCmd = pmRun(svc.PackageManager, "start:prod")
	case "fastify":
		svc.Framework = "fastify"
		svc.Type = "backend"
		svc.Confidence = "high"
		svc.Port = 3000
		svc.StartCmd = nodeStart(pkg.Main, pkg.Scripts)
	case "express":
		svc.Framework = "express"
		svc.Type = "backend"
		svc.Confidence = "high"
		svc.Port = 3000
		svc.StartCmd = nodeStart(pkg.Main, pkg.Scripts)
	case "vite":
		svc.Framework = "vite"
		svc.Type = "frontend"
		svc.Confidence = "high"
		svc.Port = 5173
		svc.BuildCmd = pmRun(svc.PackageManager, "build")
		svc.StartCmd = pmRun(svc.PackageManager, "preview")
	case "cra":
		svc.Framework = "cra"
		svc.Type = "frontend"
		svc.Confidence = "high"
		svc.Port = 3000
		svc.BuildCmd = pmRun(svc.PackageManager, "build")
		svc.StartCmd = pmRun(svc.PackageManager, "start")
	case "react":
		svc.Framework = "react"
		svc.Type = "frontend"
		svc.Confidence = "medium"
		svc.Port = 3000
		svc.BuildCmd = pmRun(svc.PackageManager, "build")
		svc.StartCmd = pmRun(svc.PackageManager, "start")
	default:
		svc.Framework = "node"
		svc.Type = "backend"
		svc.Confidence = "low"
		svc.Port = 3000
		svc.StartCmd = nodeStart(pkg.Main, pkg.Scripts)
	}

	svc.InstallCmd = pmInstall(svc.PackageManager)

	// Refine the port from the entrypoint if we can. Patterns like
	//   process.env.PORT || 4000
	//   listen(parseInt(process.env.PORT || '4000'))
	// are very common in plain Node services.
	if port, ok := portFromJSEntry(dir, pkg.Main, pkg.Scripts); ok {
		svc.Port = port
	}

	// Heuristic: if directory hints API/server but framework is a frontend default, flip.
	low := strings.ToLower(svc.Directory)
	if (svc.Type == "frontend") && (strings.Contains(low, "/api") || strings.HasSuffix(low, "api") || strings.Contains(low, "server")) {
		svc.Type = "backend"
	}
	return true
}

func nodeStart(main string, scripts map[string]string) []string {
	if _, ok := scripts["start"]; ok {
		return []string{"npm", "start"}
	}
	if main != "" {
		return []string{"node", main}
	}
	return []string{"node", "index.js"}
}

var jsPortRe = regexp.MustCompile(`(?:process\.env\.PORT|PORT)[\s)|]*\|\|\s*['"]?(\d{2,5})['"]?`)
var jsListenRe = regexp.MustCompile(`\.listen\(\s*['"]?(\d{2,5})['"]?`)

// portFromJSEntry scans the candidate entry file(s) for a port literal
// associated with the runtime listener. Returns (port, true) on a hit.
func portFromJSEntry(dir, main string, scripts map[string]string) (int, bool) {
	candidates := []string{}
	if main != "" {
		candidates = append(candidates, main)
	}
	for _, n := range []string{"server.js", "index.js", "app.js", "src/server.ts", "src/index.ts", "server.ts", "index.ts"} {
		candidates = append(candidates, n)
	}
	// Also try the file named by `node <foo.js>` in scripts.start.
	if s, ok := scripts["start"]; ok {
		fields := strings.Fields(s)
		for _, f := range fields {
			if strings.HasSuffix(f, ".js") || strings.HasSuffix(f, ".ts") || strings.HasSuffix(f, ".mjs") {
				candidates = append(candidates, f)
			}
		}
	}
	seen := map[string]bool{}
	for _, c := range candidates {
		if seen[c] {
			continue
		}
		seen[c] = true
		p := filepath.Join(dir, c)
		data, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		if m := jsPortRe.FindStringSubmatch(string(data)); m != nil {
			if n, err := strconv.Atoi(m[1]); err == nil && n >= 80 && n <= 65535 {
				return n, true
			}
		}
		if m := jsListenRe.FindStringSubmatch(string(data)); m != nil {
			if n, err := strconv.Atoi(m[1]); err == nil && n >= 80 && n <= 65535 {
				return n, true
			}
		}
	}
	return 0, false
}

func detectJSPM(dir string) string {
	if exists(filepath.Join(dir, "pnpm-lock.yaml")) {
		return "pnpm"
	}
	if exists(filepath.Join(dir, "yarn.lock")) {
		return "yarn"
	}
	return "npm"
}

func pmInstall(pm string) []string {
	switch pm {
	case "pnpm":
		return []string{"pnpm", "install", "--frozen-lockfile"}
	case "yarn":
		return []string{"yarn", "install", "--frozen-lockfile"}
	default:
		return []string{"npm", "ci"}
	}
}

func pmRun(pm, script string) []string {
	switch pm {
	case "pnpm":
		return []string{"pnpm", "run", script}
	case "yarn":
		return []string{"yarn", script}
	default:
		return []string{"npm", "run", script}
	}
}

func detectPython(dir, ident string, svc *Service) {
	svc.Language = "python"

	var content string
	if data, err := os.ReadFile(filepath.Join(dir, ident)); err == nil {
		content = strings.ToLower(string(data))
	}

	if exists(filepath.Join(dir, "pyproject.toml")) && exists(filepath.Join(dir, "poetry.lock")) {
		svc.PackageManager = "poetry"
		svc.InstallCmd = []string{"poetry", "install", "--no-root", "--no-interaction"}
	} else {
		svc.PackageManager = "pip"
		svc.InstallCmd = []string{"pip", "install", "--no-cache-dir", "-r", "requirements.txt"}
	}

	switch {
	case strings.Contains(content, "fastapi"):
		svc.Framework = "fastapi"
		svc.Type = "backend"
		svc.Confidence = "high"
		svc.Port = 8000
		entry := findPythonEntry(dir, []string{"main.py", "app.py", "server.py"})
		mod := strings.TrimSuffix(entry, ".py")
		if mod == "" {
			mod = "main"
		}
		svc.StartCmd = []string{"uvicorn", mod + ":app", "--host", "0.0.0.0", "--port", "8000"}
	case strings.Contains(content, "django"):
		svc.Framework = "django"
		svc.Type = "backend"
		svc.Port = 8000
		if exists(filepath.Join(dir, "manage.py")) {
			svc.Confidence = "high"
		} else {
			svc.Confidence = "medium"
		}
		svc.StartCmd = []string{"python", "manage.py", "runserver", "0.0.0.0:8000"}
	case strings.Contains(content, "flask"):
		svc.Framework = "flask"
		svc.Type = "backend"
		svc.Confidence = "high"
		svc.Port = 5000
		entry := findPythonEntry(dir, []string{"app.py", "main.py", "wsgi.py"})
		if entry == "" {
			entry = "app.py"
		}
		svc.StartCmd = []string{"flask", "--app", strings.TrimSuffix(entry, ".py"), "run", "--host", "0.0.0.0", "--port", "5000"}
	default:
		svc.Framework = "python"
		svc.Type = "backend"
		svc.Confidence = "low"
		svc.Port = 8000
		entry := findPythonEntry(dir, []string{"main.py", "app.py"})
		if entry == "" {
			entry = "main.py"
		}
		svc.StartCmd = []string{"python", entry}
	}
}

func findPythonEntry(dir string, candidates []string) string {
	for _, name := range candidates {
		if exists(filepath.Join(dir, name)) {
			return name
		}
	}
	return ""
}

func exists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

func confidenceRank(c string) int {
	switch c {
	case "high":
		return 3
	case "medium":
		return 2
	case "low":
		return 1
	}
	return 0
}

func shouldSkipDir(name string) bool {
	skip := map[string]bool{
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
		// Common monorepo noise — these dirs contain package.json files that
		// describe demos/tests/docs/tooling, not deployable services.
		"examples":     true,
		"example":      true,
		"e2e":          true,
		"test":         true,
		"tests":        true,
		"__tests__":    true,
		"__fixtures__": true,
		"__testfixtures__": true,
		"fixtures":     true,
		"docs":         true,
		"website":      true,
		"benchmarks":   true,
		"bench":        true,
		"playground":   true,
		"scripts":      true,
		"tools":        true,
		"evals":        true,
	}
	return skip[name]
}

// ToJSON renders the result as pretty JSON.
func (r *Result) ToJSON() (string, error) {
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}
