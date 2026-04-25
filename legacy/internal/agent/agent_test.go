package agent

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/projectchimera/chimera/internal/scanner"
)

func TestBuildTree(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "src"), 0755)
	os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{"name":"test"}`), 0644)
	os.WriteFile(filepath.Join(dir, "src", "index.js"), []byte("console.log('hi')"), 0644)
	os.MkdirAll(filepath.Join(dir, "node_modules", "foo"), 0755)

	tree, err := buildTree(dir, 3)
	if err != nil {
		t.Fatalf("buildTree error: %v", err)
	}

	if !strings.Contains(tree, "package.json") {
		t.Error("tree should contain package.json")
	}
	if !strings.Contains(tree, "index.js") {
		t.Error("tree should contain index.js")
	}
	if strings.Contains(tree, "node_modules") {
		t.Error("tree should skip node_modules")
	}
}

func TestShouldSkip(t *testing.T) {
	tests := map[string]bool{
		"node_modules": true,
		".git":         true,
		"__pycache__":  true,
		"src":          false,
		"main.go":      false,
	}
	for name, want := range tests {
		if got := shouldSkip(name); got != want {
			t.Errorf("shouldSkip(%q) = %v, want %v", name, got, want)
		}
	}
}

func TestReadRequestedFiles(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{"name":"test"}`), 0644)

	contents, err := readRequestedFiles(dir, []string{"package.json", "missing.txt", "../etc/passwd"})
	if err != nil {
		t.Fatalf("error: %v", err)
	}

	if contents["package.json"] != `{"name":"test"}` {
		t.Errorf("unexpected content: %s", contents["package.json"])
	}
	if !strings.Contains(contents["missing.txt"], "not found") {
		t.Error("missing file should have 'not found' message")
	}
	if _, ok := contents["../etc/passwd"]; ok {
		t.Error("path traversal should be skipped")
	}
}

func TestReadRequestedFiles_LargeFile(t *testing.T) {
	dir := t.TempDir()
	// Create a file larger than maxFileSize
	bigContent := strings.Repeat("x", maxFileSize+100)
	os.WriteFile(filepath.Join(dir, "big.txt"), []byte(bigContent), 0644)

	contents, _ := readRequestedFiles(dir, []string{"big.txt"})
	if !strings.Contains(contents["big.txt"], "truncated") {
		t.Error("large file should be truncated")
	}
}

func TestExtractJSON_Object(t *testing.T) {
	input := `Here is the config: {"key": "value"} done`
	result := extractJSON(input)
	if result != `{"key": "value"}` {
		t.Errorf("expected JSON object, got: %s", result)
	}
}

func TestExtractJSON_Array(t *testing.T) {
	input := `["file1.js", "file2.py"]`
	result := extractJSON(input)
	if result != `["file1.js", "file2.py"]` {
		t.Errorf("expected JSON array, got: %s", result)
	}
}

func TestExtractJSON_CodeFence(t *testing.T) {
	input := "```json\n[\"a.js\"]\n```"
	result := extractJSON(input)
	if result != `["a.js"]` {
		t.Errorf("expected extracted JSON, got: %s", result)
	}
}

func TestExtractJSON_NestedQuotes(t *testing.T) {
	input := `{"dockerfile": "FROM node:20\nCOPY . .", "name": "test"}`
	result := extractJSON(input)

	var parsed map[string]string
	if err := json.Unmarshal([]byte(result), &parsed); err != nil {
		t.Fatalf("extracted JSON should be parseable: %v", err)
	}
}

func TestPlanningCall_MockServer(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"choices": []map[string]interface{}{
				{"message": map[string]string{
					"content": `{"files": ["package.json", "src/index.js"], "reasoning": "Need manifest and entry point"}`,
				}},
			},
		})
	}))
	defer server.Close()

	provider := &Provider{Name: "openai", APIKey: "test-key", Model: "test-model", Endpoint: server.URL}

	planResp, err := planningCall(context.Background(), provider, "📄 package.json", "scan results", "test-project")
	if err != nil {
		t.Fatalf("planningCall error: %v", err)
	}
	if len(planResp.Files) != 2 {
		t.Errorf("expected 2 files, got %d", len(planResp.Files))
	}
	if planResp.Reasoning == "" {
		t.Error("reasoning should not be empty")
	}
}

func TestPlanningCall_FallbackToArray(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"choices": []map[string]interface{}{
				{"message": map[string]string{
					"content": `["package.json"]`,
				}},
			},
		})
	}))
	defer server.Close()

	provider := &Provider{Name: "openai", APIKey: "test-key", Model: "test-model", Endpoint: server.URL}
	planResp, err := planningCall(context.Background(), provider, "tree", "scan", "proj")
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if len(planResp.Files) != 1 {
		t.Errorf("expected 1 file, got %d", len(planResp.Files))
	}
}

func TestGenerationCall_MockServer(t *testing.T) {
	config := Config{
		Dockerfile:    "FROM node:20-alpine\nWORKDIR /app\nCOPY . .\nCMD [\"node\", \".\"]",
		DockerCompose: "version: '3.8'\nservices:\n  app:\n    build: .",
		EnvExample:    "PORT=3000",
		Explanation:   "Detected Node.js project with Redis",
		Services: []ServiceInfo{
			{Name: "app", Type: "app", Image: "node:20-alpine", Port: 3000},
			{Name: "redis", Type: "cache", Image: "redis:7-alpine", Port: 6379, Healthcheck: "redis-cli ping"},
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		configJSON, _ := json.Marshal(config)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"choices": []map[string]interface{}{
				{"message": map[string]string{"content": string(configJSON)}},
			},
		})
	}))
	defer server.Close()

	provider := &Provider{Name: "openai", APIKey: "test-key", Model: "test-model", Endpoint: server.URL}

	files := map[string]string{"package.json": `{"name": "test"}`}
	result, err := generationCall(context.Background(), provider, "📄 package.json", "scan results", files, "test-project")
	if err != nil {
		t.Fatalf("generationCall error: %v", err)
	}
	if result.Dockerfile == "" {
		t.Error("Dockerfile should not be empty")
	}
	if len(result.Services) != 2 {
		t.Errorf("expected 2 services, got %d", len(result.Services))
	}
	if result.Services[1].Healthcheck != "redis-cli ping" {
		t.Error("healthcheck should be preserved")
	}
}

func TestValidateConfig(t *testing.T) {
	// Valid config
	valid := &Config{
		Dockerfile:    "FROM node:20\nCOPY . .\nCMD [\"node\", \".\"]",
		DockerCompose: "version: '3.8'\nservices:\n  app:\n    build: .",
	}
	if err := validateConfig(valid); err != nil {
		t.Errorf("valid config should pass: %v", err)
	}

	// Empty Dockerfile
	if err := validateConfig(&Config{DockerCompose: "services:"}); err == nil {
		t.Error("empty Dockerfile should fail")
	}

	// Empty compose
	if err := validateConfig(&Config{Dockerfile: "FROM node:20"}); err == nil {
		t.Error("empty docker-compose should fail")
	}

	// Missing FROM
	if err := validateConfig(&Config{Dockerfile: "COPY . .", DockerCompose: "services:"}); err == nil {
		t.Error("Dockerfile without FROM should fail")
	}

	// Missing services
	if err := validateConfig(&Config{Dockerfile: "FROM node:20", DockerCompose: "version: 3.8"}); err == nil {
		t.Error("docker-compose without services should fail")
	}
}

func TestNewProvider_Providers(t *testing.T) {
	tests := []struct {
		provider string
		keyEnv   string
		keyVal   string
		wantName string
	}{
		{"openai", "OPENAI_API_KEY", "sk-test", "openai"},
		{"gemini", "GEMINI_API_KEY", "gem-test", "gemini"},
		{"groq", "GROQ_API_KEY", "groq-test", "groq"},
	}

	for _, tt := range tests {
		t.Run(tt.provider, func(t *testing.T) {
			t.Setenv("CHIMERA_LLM_PROVIDER", tt.provider)
			t.Setenv(tt.keyEnv, tt.keyVal)
			p, err := NewProvider()
			if err != nil {
				t.Fatalf("error: %v", err)
			}
			if p.Name != tt.wantName {
				t.Errorf("expected %s, got %s", tt.wantName, p.Name)
			}
		})
	}
}

func TestNewProvider_MissingKey(t *testing.T) {
	t.Setenv("CHIMERA_LLM_PROVIDER", "openai")
	t.Setenv("OPENAI_API_KEY", "")
	_, err := NewProvider()
	if err == nil {
		t.Error("should fail with missing key")
	}
	if !strings.Contains(err.Error(), ".chimera.env") {
		t.Error("error should mention config file")
	}
}

func TestNewProvider_CustomModel(t *testing.T) {
	t.Setenv("CHIMERA_LLM_PROVIDER", "openai")
	t.Setenv("OPENAI_API_KEY", "sk-test")
	t.Setenv("CHIMERA_MODEL", "gpt-4o")
	p, err := NewProvider()
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if p.Model != "gpt-4o" {
		t.Errorf("expected gpt-4o, got %s", p.Model)
	}
}

func TestHumanSize(t *testing.T) {
	tests := map[int64]string{
		500:            "500B",
		2048:           "2.0KB",
		2 * 1024 * 1024: "2.0MB",
	}
	for input, want := range tests {
		got := humanSize(input)
		if got != want {
			t.Errorf("humanSize(%d) = %s, want %s", input, got, want)
		}
	}
}

func TestTruncate(t *testing.T) {
	if truncate("hello", 10) != "hello" {
		t.Error("short string should not be truncated")
	}
	if truncate("hello world", 5) != "hello..." {
		t.Error("long string should be truncated")
	}
}

func TestLoadConfig(t *testing.T) {
	dir := t.TempDir()
	envFile := filepath.Join(dir, ".chimera.env")
	os.WriteFile(envFile, []byte("TEST_CHIMERA_VAR=hello\n# comment\nEMPTY_VAR=\n"), 0644)

	// Set env var that should NOT be overwritten
	t.Setenv("TEST_CHIMERA_VAR", "existing")

	loadEnvFile(envFile)

	// Should NOT overwrite existing env var
	if os.Getenv("TEST_CHIMERA_VAR") != "existing" {
		t.Error("should not overwrite existing env vars")
	}
}

func TestFormatScanResults(t *testing.T) {
	result := &scanner.ScanResult{
		Languages: []scanner.Language{
			{Name: "Node.js", Version: "20", Files: []string{"package.json"}},
		},
		Infrastructure: []scanner.Infrastructure{
			{Name: "postgresql", Type: "postgresql"},
		},
		Ports:   []int{3000},
		EnvVars: []string{"DATABASE_URL", "PORT"},
	}

	formatted := formatScanResults(result)
	if !strings.Contains(formatted, "Node.js") {
		t.Error("should contain language")
	}
	if !strings.Contains(formatted, "postgresql") {
		t.Error("should contain infra")
	}
	if !strings.Contains(formatted, "3000") {
		t.Error("should contain port")
	}
	if !strings.Contains(formatted, "DATABASE_URL") {
		t.Error("should contain env var")
	}
}

func TestIsBinary(t *testing.T) {
	dir := t.TempDir()

	// Text file
	textFile := filepath.Join(dir, "text.txt")
	os.WriteFile(textFile, []byte("hello world"), 0644)
	if isBinary(textFile) {
		t.Error("text file should not be detected as binary")
	}

	// Binary file (contains null bytes)
	binFile := filepath.Join(dir, "binary.bin")
	os.WriteFile(binFile, []byte{0x00, 0x01, 0x02}, 0644)
	if !isBinary(binFile) {
		t.Error("binary file should be detected as binary")
	}
}
