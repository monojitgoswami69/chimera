package scanner

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// FrameworkInfo describes a detected framework with its runtime requirements.
type FrameworkInfo struct {
	Name       string // e.g. "Next.js", "Django", "FastAPI"
	Category   string // "frontend", "backend", "fullstack"
	StartCmd   string // e.g. "npm run dev", "uvicorn main:app"
	BuildCmd   string // e.g. "npm run build"
	InstallCmd string // e.g. "npm ci", "pip install -r requirements.txt"
	Port       int    // default port
	EntryFile  string // detected entry file
}

// detectNodeFrameworks inspects package.json to detect Node.js frameworks.
func (s *Scanner) detectNodeFrameworks(dir string) *FrameworkInfo {
	pkgPath := filepath.Join(dir, "package.json")
	data, err := os.ReadFile(pkgPath)
	if err != nil {
		// Search one level deep
		entries, _ := os.ReadDir(dir)
		for _, e := range entries {
			if e.IsDir() && e.Name() != ".git" {
				subPath := filepath.Join(dir, e.Name(), "package.json")
				if d, err := os.ReadFile(subPath); err == nil {
					data = d
					dir = filepath.Join(dir, e.Name()) // update dir to the subfolder
					break
				}
			}
		}
	}
	if len(data) == 0 {
		return nil
	}

	var pkg struct {
		Scripts      map[string]string `json:"scripts"`
		Dependencies map[string]string `json:"dependencies"`
		DevDeps      map[string]string `json:"devDependencies"`
		Main         string            `json:"main"`
	}
	if err := json.Unmarshal(data, &pkg); err != nil {
		return nil
	}

	allDeps := make(map[string]bool)
	for k := range pkg.Dependencies {
		allDeps[k] = true
	}
	for k := range pkg.DevDeps {
		allDeps[k] = true
	}

	info := &FrameworkInfo{Port: 3000}

	// Detect framework by dependencies
	switch {
	case allDeps["next"]:
		info.Name = "Next.js"
		info.Category = "fullstack"
		info.Port = 3000
		info.BuildCmd = "npm run build"
		info.StartCmd = "npm start"
		info.InstallCmd = "npm ci"
	case allDeps["nuxt"] || allDeps["nuxt3"]:
		info.Name = "Nuxt.js"
		info.Category = "fullstack"
		info.Port = 3000
		info.BuildCmd = "npm run build"
		info.StartCmd = "npm start"
		info.InstallCmd = "npm ci"
	case allDeps["vite"]:
		info.Name = "Vite"
		info.Category = "frontend"
		info.Port = 5173
		info.BuildCmd = "npm run build"
		info.StartCmd = "npm run dev"
		info.InstallCmd = "npm ci"
	case allDeps["@angular/core"]:
		info.Name = "Angular"
		info.Category = "frontend"
		info.Port = 4200
		info.BuildCmd = "npm run build"
		info.StartCmd = "npm start"
		info.InstallCmd = "npm ci"
	case allDeps["express"]:
		info.Name = "Express"
		info.Category = "backend"
		info.Port = 3000
		info.StartCmd = "node ."
		info.InstallCmd = "npm ci"
	case allDeps["fastify"]:
		info.Name = "Fastify"
		info.Category = "backend"
		info.Port = 3000
		info.StartCmd = "node ."
		info.InstallCmd = "npm ci"
	case allDeps["koa"]:
		info.Name = "Koa"
		info.Category = "backend"
		info.Port = 3000
		info.StartCmd = "node ."
		info.InstallCmd = "npm ci"
	case allDeps["react-scripts"]:
		info.Name = "Create React App"
		info.Category = "frontend"
		info.Port = 3000
		info.BuildCmd = "npm run build"
		info.StartCmd = "npm start"
		info.InstallCmd = "npm ci"
	default:
		info.Name = "Node.js"
		info.Category = "backend"
		info.StartCmd = "node ."
		info.InstallCmd = "npm ci"
	}

	// Override start command from scripts if available
	if cmd, ok := pkg.Scripts["start"]; ok {
		info.StartCmd = "npm start"
		// Try to detect actual entry from start script
		if strings.Contains(cmd, "node ") {
			re := regexp.MustCompile(`node\s+(\S+)`)
			if m := re.FindStringSubmatch(cmd); len(m) >= 2 {
				info.EntryFile = m[1]
			}
		}
	}
	if _, ok := pkg.Scripts["dev"]; ok && info.Category != "backend" {
		info.StartCmd = "npm run dev"
	}

	// Detect entry file
	if info.EntryFile == "" {
		if pkg.Main != "" {
			info.EntryFile = pkg.Main
		} else {
			for _, candidate := range []string{"index.js", "server.js", "app.js", "src/index.js", "src/server.js", "src/app.js", "src/main.js", "src/index.ts", "src/server.ts", "src/app.ts", "src/main.ts"} {
				if _, err := os.Stat(filepath.Join(dir, candidate)); err == nil {
					info.EntryFile = candidate
					break
				}
			}
		}
	}

	// Detect port from start script or source
	if cmd, ok := pkg.Scripts["start"]; ok {
		portRe := regexp.MustCompile(`(?:--port|PORT=|-p)\s*(\d{4,5})`)
		if m := portRe.FindStringSubmatch(cmd); len(m) >= 2 {
			var p int
			if _, err := fmt.Sscanf(m[1], "%d", &p); err == nil && p > 0 {
				info.Port = p
			}
		}
	}

	return info
}

// detectPythonFramework inspects requirements and source to detect Python frameworks.
func (s *Scanner) detectPythonFramework(dir string) *FrameworkInfo {
	// Read all requirement sources
	var allReqs strings.Builder
	
	// Check root and 1 level deep
	dirsToCheck := []string{dir}
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if e.IsDir() && e.Name() != ".git" && e.Name() != "venv" && e.Name() != "node_modules" {
			dirsToCheck = append(dirsToCheck, filepath.Join(dir, e.Name()))
		}
	}
	
	for _, d := range dirsToCheck {
		for _, f := range []string{"requirements.txt", "Pipfile", "pyproject.toml", "setup.py", "setup.cfg"} {
			if data, err := os.ReadFile(filepath.Join(d, f)); err == nil {
				allReqs.WriteString(strings.ToLower(string(data)))
				allReqs.WriteString("\n")
				
				// Set dir to the matched directory so entry points resolve relative to it
				dir = d
			}
		}
	}
	
	reqs := allReqs.String()

	info := &FrameworkInfo{Port: 8000, InstallCmd: "pip install -r requirements.txt"}

	// Check for Pipfile -> use pipenv
	if _, err := os.Stat(filepath.Join(dir, "Pipfile")); err == nil {
		info.InstallCmd = "pipenv install"
	}
	// Check for pyproject.toml with poetry
	if data, err := os.ReadFile(filepath.Join(dir, "pyproject.toml")); err == nil {
		if strings.Contains(string(data), "[tool.poetry]") {
			info.InstallCmd = "poetry install"
		}
	}

	// Detect framework
	switch {
	case strings.Contains(reqs, "django"):
		info.Name = "Django"
		info.Category = "backend"
		info.Port = 8000
		// Find manage.py
		manageDir := ""
		if _, err := os.Stat(filepath.Join(dir, "manage.py")); err == nil {
			info.EntryFile = "manage.py"
		} else {
			// Search one level deep
			entries, _ := os.ReadDir(dir)
			for _, e := range entries {
				if e.IsDir() {
					if _, err := os.Stat(filepath.Join(dir, e.Name(), "manage.py")); err == nil {
						manageDir = e.Name()
						info.EntryFile = filepath.Join(e.Name(), "manage.py")
						break
					}
				}
			}
		}
		if manageDir != "" {
			info.StartCmd = fmt.Sprintf("python %s/manage.py runserver 0.0.0.0:8000", manageDir)
		} else {
			info.StartCmd = "python manage.py runserver 0.0.0.0:8000"
		}
		// Check for gunicorn
		if strings.Contains(reqs, "gunicorn") {
			// Try to find wsgi module
			wsgiMod := findDjangoWSGI(dir)
			if wsgiMod != "" {
				info.StartCmd = fmt.Sprintf("gunicorn %s --bind 0.0.0.0:8000", wsgiMod)
			} else {
				info.StartCmd = "gunicorn app.wsgi --bind 0.0.0.0:8000"
			}
		}

	case strings.Contains(reqs, "fastapi"):
		info.Name = "FastAPI"
		info.Category = "backend"
		info.Port = 8000
		entry := findPythonEntry(dir, "fastapi")
		if entry != "" {
			info.EntryFile = entry
			module := strings.TrimSuffix(entry, ".py")
			module = strings.ReplaceAll(module, "/", ".")
			info.StartCmd = fmt.Sprintf("uvicorn %s:app --host 0.0.0.0 --port 8000", module)
		} else {
			info.StartCmd = "uvicorn main:app --host 0.0.0.0 --port 8000"
		}

	case strings.Contains(reqs, "flask"):
		info.Name = "Flask"
		info.Category = "backend"
		info.Port = 5000
		entry := findPythonEntry(dir, "flask")
		if entry != "" {
			info.EntryFile = entry
		}
		if strings.Contains(reqs, "gunicorn") {
			module := "app:app"
			if entry != "" {
				module = strings.TrimSuffix(filepath.Base(entry), ".py") + ":app"
			}
			info.StartCmd = fmt.Sprintf("gunicorn %s --bind 0.0.0.0:5000", module)
		} else {
			info.StartCmd = "flask run --host 0.0.0.0 --port 5000"
		}

	case strings.Contains(reqs, "streamlit"):
		info.Name = "Streamlit"
		info.Category = "frontend"
		info.Port = 8501
		entry := findPythonEntry(dir, "streamlit")
		if entry != "" {
			info.EntryFile = entry
			info.StartCmd = fmt.Sprintf("streamlit run %s --server.port 8501 --server.address 0.0.0.0", entry)
		} else {
			info.StartCmd = "streamlit run app.py --server.port 8501 --server.address 0.0.0.0"
		}

	case strings.Contains(reqs, "gradio"):
		info.Name = "Gradio"
		info.Category = "frontend"
		info.Port = 7860
		info.StartCmd = "python app.py"

	default:
		info.Name = "Python"
		info.Category = "backend"
		// Try to find a main entry
		for _, candidate := range []string{"main.py", "app.py", "run.py", "server.py", "manage.py", "src/main.py"} {
			if _, err := os.Stat(filepath.Join(dir, candidate)); err == nil {
				info.EntryFile = candidate
				info.StartCmd = fmt.Sprintf("python %s", candidate)
				break
			}
		}
		if info.StartCmd == "" {
			info.StartCmd = "python main.py"
		}
	}

	return info
}

// detectGoFramework inspects go.mod to detect Go frameworks.
func (s *Scanner) detectGoFramework(dir string) *FrameworkInfo {
	data, err := os.ReadFile(filepath.Join(dir, "go.mod"))
	if err != nil {
		return nil
	}
	mod := strings.ToLower(string(data))

	info := &FrameworkInfo{
		Name:       "Go",
		Category:   "backend",
		Port:       8080,
		InstallCmd: "go mod download",
		BuildCmd:   "go build -o app .",
		StartCmd:   "./app",
	}

	switch {
	case strings.Contains(mod, "github.com/gin-gonic/gin"):
		info.Name = "Gin"
	case strings.Contains(mod, "github.com/labstack/echo"):
		info.Name = "Echo"
	case strings.Contains(mod, "github.com/gofiber/fiber"):
		info.Name = "Fiber"
	case strings.Contains(mod, "github.com/gorilla/mux"):
		info.Name = "Gorilla Mux"
	}

	// Find main package
	for _, candidate := range []string{"main.go", "cmd/main.go", "cmd/server/main.go", "cmd/api/main.go"} {
		if _, err := os.Stat(filepath.Join(dir, candidate)); err == nil {
			info.EntryFile = candidate
			if candidate != "main.go" {
				info.BuildCmd = fmt.Sprintf("go build -o app ./%s", filepath.Dir(candidate))
			}
			break
		}
	}

	return info
}

// findPythonEntry searches for a Python entry file that imports the given framework.
func findPythonEntry(dir, framework string) string {
	candidates := []string{"main.py", "app.py", "run.py", "server.py", "api.py",
		"src/main.py", "src/app.py", "app/main.py", "app/app.py"}

	for _, c := range candidates {
		path := filepath.Join(dir, c)
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		content := string(data)
		if strings.Contains(content, "import "+framework) || strings.Contains(content, "from "+framework) {
			return c
		}
	}

	// Fallback: just check if file exists
	for _, c := range candidates[:4] {
		if _, err := os.Stat(filepath.Join(dir, c)); err == nil {
			return c
		}
	}
	return ""
}

// findDjangoWSGI searches for the WSGI module in a Django project.
func findDjangoWSGI(dir string) string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		wsgiPath := filepath.Join(dir, e.Name(), "wsgi.py")
		if _, err := os.Stat(wsgiPath); err == nil {
			return e.Name() + ".wsgi:application"
		}
	}
	return ""
}


