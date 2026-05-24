package generator

import (
	"strings"
	"testing"

	"chimera/internal/detector"

	"gopkg.in/yaml.v3"
)

func TestBuildEmitsDockerfilePerServiceAndCompose(t *testing.T) {
	services := []detector.Service{
		{ID: "service-1", Type: "frontend", Directory: "apps/web", Language: "typescript", Framework: "next", PackageManager: "npm", Port: 3000, InstallCmd: []string{"npm", "ci"}, BuildCmd: []string{"npm", "run", "build"}, StartCmd: []string{"npm", "start"}},
		{ID: "service-2", Type: "backend", Directory: "services/api", Language: "python", Framework: "fastapi", PackageManager: "pip", Port: 8000, InstallCmd: []string{"pip", "install", "--no-cache-dir", "-r", "requirements.txt"}, StartCmd: []string{"uvicorn", "main:app", "--host", "0.0.0.0", "--port", "8000"}},
	}
	out := Build(services, "myrepo", "chimera-outputs")

	if _, ok := out.Files["Dockerfile.service-1"]; !ok {
		t.Errorf("missing Dockerfile.service-1; files=%v", keys(out.Files))
	}
	if _, ok := out.Files["Dockerfile.service-2"]; !ok {
		t.Errorf("missing Dockerfile.service-2")
	}
	compose, ok := out.Files["docker-compose.yml"]
	if !ok {
		t.Fatal("missing docker-compose.yml")
	}
	// Compose paths must be relative to the compose file location.
	if !strings.Contains(compose, "context: ..") {
		t.Errorf("compose context should be `..` (repo root); got:\n%s", compose)
	}
	if !strings.Contains(compose, "dockerfile: chimera-outputs/Dockerfile.service-1") {
		t.Errorf("compose dockerfile path wrong:\n%s", compose)
	}
	if !strings.Contains(compose, "env-vars/service-1/.env.example") {
		t.Errorf("compose env_file path wrong:\n%s", compose)
	}
	// Multi-service should not collide on host ports.
	if strings.Count(compose, "\"3000:3000\"") > 1 {
		t.Errorf("host port collision in compose:\n%s", compose)
	}
}

func TestDockerfileForNextUsesRepoRootContext(t *testing.T) {
	svc := detector.Service{ID: "s1", Directory: "apps/web", Framework: "next", PackageManager: "npm", Port: 3000, InstallCmd: []string{"npm", "ci"}, BuildCmd: []string{"npm", "run", "build"}, StartCmd: []string{"npm", "start"}}
	df := renderDockerfile(svc)
	if !strings.Contains(df, "COPY apps/web/package*.json ./") {
		t.Errorf("dockerfile should COPY from apps/web/:\n%s", df)
	}
	if !strings.Contains(df, "EXPOSE 3000") {
		t.Errorf("dockerfile should EXPOSE 3000:\n%s", df)
	}
	if !strings.Contains(df, `CMD ["npm", "start"]`) {
		t.Errorf("dockerfile should have CMD npm start; got:\n%s", df)
	}
}

func TestComposeHandlesPortCollisions(t *testing.T) {
	svcs := []detector.Service{
		{ID: "a", Framework: "express", Port: 3000},
		{ID: "b", Framework: "node", Port: 3000},
		{ID: "c", Framework: "fastify", Port: 3000},
	}
	out := Build(svcs, "repo", "chimera-outputs")
	c := out.Files["docker-compose.yml"]
	if !strings.Contains(c, "\"3000:3000\"") {
		t.Errorf("first service should bind 3000:3000")
	}
	if !strings.Contains(c, "\"3001:3000\"") {
		t.Errorf("second service should remap 3001:3000")
	}
	if !strings.Contains(c, "\"3002:3000\"") {
		t.Errorf("third service should remap 3002:3000")
	}
}

func TestBuildOverridesPortForStaticSPA(t *testing.T) {
	svcs := []detector.Service{
		{ID: "a", Framework: "vite", Port: 5173},
		{ID: "b", Framework: "cra", Port: 3000},
	}
	out := Build(svcs, "repo", "chimera-outputs")
	c := out.Files["docker-compose.yml"]
	if !strings.Contains(c, "\"80:80\"") {
		t.Errorf("first static-SPA should bind 80:80 (nginx). Got:\n%s", c)
	}
	if !strings.Contains(c, "\"81:80\"") {
		t.Errorf("second static-SPA should remap to 81:80. Got:\n%s", c)
	}
}

func keys(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

func TestComposeYAMLParses(t *testing.T) {
	svcs := []detector.Service{
		{ID: "service-1", Framework: "next", Directory: "apps/web", Port: 3000, Type: "frontend"},
		{ID: "service-2", Framework: "fastapi", Directory: "services/api", Port: 8000, Type: "backend"},
	}
	out := Build(svcs, "MyRepo Name!", "chimera-outputs")
	var parsed struct {
		Services map[string]struct {
			Build struct {
				Context    string `yaml:"context"`
				Dockerfile string `yaml:"dockerfile"`
			} `yaml:"build"`
			ContainerName string   `yaml:"container_name"`
			Ports         []string `yaml:"ports"`
			EnvFile       []string `yaml:"env_file"`
			Networks      []string `yaml:"networks"`
		} `yaml:"services"`
		Networks map[string]struct {
			Driver string `yaml:"driver"`
		} `yaml:"networks"`
	}
	if err := yaml.Unmarshal([]byte(out.Files["docker-compose.yml"]), &parsed); err != nil {
		t.Fatalf("compose file is not valid YAML: %v\n%s", err, out.Files["docker-compose.yml"])
	}
	if len(parsed.Services) != 2 {
		t.Errorf("expected 2 services in YAML, got %d", len(parsed.Services))
	}
	if len(parsed.Networks) != 1 {
		t.Errorf("expected 1 network, got %d", len(parsed.Networks))
	}
	// Container names must be sanitised — no spaces or `!`.
	for _, s := range parsed.Services {
		if strings.ContainsAny(s.ContainerName, " !") {
			t.Errorf("container_name contains illegal chars: %q", s.ContainerName)
		}
	}
}
