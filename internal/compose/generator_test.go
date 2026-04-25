package compose

import (
	"strings"
	"testing"

	"github.com/projectchimera/chimera/internal/scanner"
)

func TestGenerateNodePostgresRedis(t *testing.T) {
	result := &scanner.ScanResult{
		Languages: []scanner.Language{
			{Name: "Node.js", Version: "18", Files: []string{"package.json"}},
		},
		Infrastructure: []scanner.Infrastructure{
			{Name: "PostgreSQL", Type: "postgresql", Version: "15"},
			{Name: "Redis", Type: "redis", Version: "7"},
		},
		EnvVars: []string{"DATABASE_URL", "REDIS_URL", "PORT", "SECRET_KEY"},
		Ports:   []int{3000},
	}

	manifest, err := Generate("testproject", result)
	if err != nil {
		t.Fatalf("Generate() error: %v", err)
	}

	// Verify docker-compose.yml content
	if !strings.Contains(manifest.DockerCompose, "version: '3.8'") {
		t.Error("compose missing version 3.8")
	}
	if !strings.Contains(manifest.DockerCompose, "postgres:15-alpine") {
		t.Error("compose missing postgres:15-alpine image")
	}
	if !strings.Contains(manifest.DockerCompose, "redis:7-alpine") {
		t.Error("compose missing redis:7-alpine image")
	}
	if !strings.Contains(manifest.DockerCompose, "pg_isready") {
		t.Error("compose missing postgres healthcheck")
	}
	if !strings.Contains(manifest.DockerCompose, "redis-cli ping") {
		t.Error("compose missing redis healthcheck")
	}
	if !strings.Contains(manifest.DockerCompose, "condition: service_healthy") {
		t.Error("compose missing depends_on service_healthy condition")
	}
	if !strings.Contains(manifest.DockerCompose, "com.chimera.project") {
		t.Error("compose missing chimera label")
	}
	if !strings.Contains(manifest.DockerCompose, "testproject_net") {
		t.Error("compose missing network")
	}
	if !strings.Contains(manifest.DockerCompose, "postgres_data") {
		t.Error("compose missing postgres volume")
	}

	// Verify Dockerfile
	if !strings.Contains(manifest.Dockerfile, "node:18-alpine") {
		t.Error("Dockerfile missing node:18-alpine")
	}
	if !strings.Contains(manifest.Dockerfile, "npm ci") {
		t.Error("Dockerfile missing npm ci")
	}

	// Verify env file
	if !strings.Contains(manifest.EnvExample, "DATABASE_URL=postgresql://") {
		t.Error("env file missing pre-populated DATABASE_URL")
	}
	if !strings.Contains(manifest.EnvExample, "REDIS_URL=redis://") {
		t.Error("env file missing pre-populated REDIS_URL")
	}
	if !strings.Contains(manifest.EnvExample, "SECRET_KEY=") {
		t.Error("env file missing SECRET_KEY placeholder")
	}
}

func TestGeneratePythonMongo(t *testing.T) {
	result := &scanner.ScanResult{
		Languages: []scanner.Language{
			{Name: "Python", Version: "3.11", Files: []string{"requirements.txt"}},
		},
		Infrastructure: []scanner.Infrastructure{
			{Name: "MongoDB", Type: "mongodb", Version: "6.0"},
		},
		EnvVars: []string{"MONGODB_URL"},
		Ports:   []int{8000},
	}

	manifest, err := Generate("pyapp", result)
	if err != nil {
		t.Fatalf("Generate() error: %v", err)
	}

	if !strings.Contains(manifest.DockerCompose, "mongo:6.0") {
		t.Error("compose missing mongo:6.0 image")
	}
	if !strings.Contains(manifest.DockerCompose, "mongosh") {
		t.Error("compose missing mongodb healthcheck")
	}
	if !strings.Contains(manifest.Dockerfile, "python:3.11-slim") {
		t.Error("Dockerfile missing python:3.11-slim")
	}
	if !strings.Contains(manifest.EnvExample, "MONGODB_URL=mongodb://") {
		t.Error("env file missing MONGODB_URL")
	}
}

func TestGenerateGoRabbitMQ(t *testing.T) {
	result := &scanner.ScanResult{
		Languages: []scanner.Language{
			{Name: "Go", Version: "1.21", Files: []string{"go.mod"}},
		},
		Infrastructure: []scanner.Infrastructure{
			{Name: "RabbitMQ", Type: "rabbitmq", Version: "3.12"},
		},
		EnvVars: []string{"RABBITMQ_URL"},
		Ports:   []int{8080},
	}

	manifest, err := Generate("goapp", result)
	if err != nil {
		t.Fatalf("Generate() error: %v", err)
	}

	if !strings.Contains(manifest.DockerCompose, "rabbitmq:3.12-management-alpine") {
		t.Error("compose missing rabbitmq image")
	}
	if !strings.Contains(manifest.DockerCompose, "rabbitmq-diagnostics") {
		t.Error("compose missing rabbitmq healthcheck")
	}
	if !strings.Contains(manifest.Dockerfile, "golang:1.21-alpine") {
		t.Error("Dockerfile missing golang:1.21-alpine")
	}
	if !strings.Contains(manifest.Dockerfile, "CGO_ENABLED=0") {
		t.Error("Dockerfile missing CGO_ENABLED=0")
	}
}

func TestGenerateNoLanguage(t *testing.T) {
	result := &scanner.ScanResult{}
	_, err := Generate("empty", result)
	if err == nil {
		t.Error("Generate() should fail with no languages")
	}
}

func TestEnvFilePopulation(t *testing.T) {
	result := &scanner.ScanResult{
		Languages: []scanner.Language{{Name: "Node.js", Version: "18"}},
		Infrastructure: []scanner.Infrastructure{
			{Name: "PostgreSQL", Type: "postgresql"},
		},
		EnvVars: []string{"DATABASE_URL", "CUSTOM_VAR", "API_KEY"},
	}

	manifest, err := Generate("myproject", result)
	if err != nil {
		t.Fatalf("Generate() error: %v", err)
	}

	// DATABASE_URL should be pre-populated
	if !strings.Contains(manifest.EnvExample, "DATABASE_URL=postgresql://") {
		t.Error("DATABASE_URL should be pre-populated")
	}

	// CUSTOM_VAR and API_KEY should be empty placeholders
	if !strings.Contains(manifest.EnvExample, "CUSTOM_VAR=") {
		t.Error("CUSTOM_VAR should have empty placeholder")
	}
	if !strings.Contains(manifest.EnvExample, "API_KEY=") {
		t.Error("API_KEY should have empty placeholder")
	}

	// DATABASE_URL should NOT appear twice (once populated, once empty)
	count := strings.Count(manifest.EnvExample, "DATABASE_URL")
	if count != 1 {
		t.Errorf("DATABASE_URL appeared %d times, want 1", count)
	}
}

func TestHealthchecks(t *testing.T) {
	tests := []struct {
		infraType string
		expect    string
	}{
		{"postgresql", "pg_isready"},
		{"mysql", "mysqladmin ping"},
		{"mongodb", "mongosh"},
		{"redis", "redis-cli ping"},
		{"rabbitmq", "rabbitmq-diagnostics"},
		{"elasticsearch", "curl"},
		{"unknown", ""},
	}

	for _, tt := range tests {
		t.Run(tt.infraType, func(t *testing.T) {
			hc := Healthcheck(tt.infraType)
			if tt.expect == "" {
				if hc != "" {
					t.Errorf("expected empty healthcheck for %s", tt.infraType)
				}
			} else {
				if !strings.Contains(hc, tt.expect) {
					t.Errorf("healthcheck for %s should contain %q, got %q", tt.infraType, tt.expect, hc)
				}
			}
		})
	}
}

func TestDockerfileGeneration(t *testing.T) {
	tests := []struct {
		lang   scanner.Language
		expect string
	}{
		{scanner.Language{Name: "Node.js", Version: "20"}, "node:20-alpine"},
		{scanner.Language{Name: "Python", Version: "3.11"}, "python:3.11-slim"},
		{scanner.Language{Name: "Go", Version: "1.21"}, "golang:1.21-alpine"},
	}

	for _, tt := range tests {
		t.Run(tt.lang.Name, func(t *testing.T) {
			df := GenerateDockerfile(tt.lang)
			if !strings.Contains(df, tt.expect) {
				t.Errorf("Dockerfile for %s should contain %q", tt.lang.Name, tt.expect)
			}
		})
	}
}
