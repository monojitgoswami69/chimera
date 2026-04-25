package scanner

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestExtractEnvVarNames(t *testing.T) {
	// Create a temporary file with test content
	tmpDir, err := os.MkdirTemp("", "scanner-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	envContent := `DATABASE_URL=postgres://localhost:5432/mydb
DATABASE_PASSWORD=secret123
# Comment line
REDIS_URL=redis://localhost:6379/0
EMPTY_VAR=
API_KEY=abc123
NODE_ENV=production`

	envFile := filepath.Join(tmpDir, ".env.example")
	if err := os.WriteFile(envFile, []byte(envContent), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	s := NewScanner(tmpDir)
	varNames, err := s.extractEnvVarNames(envFile)
	if err != nil {
		t.Fatalf("extractEnvVarNames() error: %v", err)
	}

	expected := map[string]bool{
		"DATABASE_URL":      true,
		"DATABASE_PASSWORD": true,
		"REDIS_URL":         true,
		"EMPTY_VAR":         true,
		"API_KEY":           true,
		"NODE_ENV":          true,
	}

	for _, name := range varNames {
		if !expected[name] {
			t.Errorf("Unexpected var name: %s", name)
		}
		delete(expected, name)
	}

	for name := range expected {
		t.Errorf("Missing expected var name: %s", name)
	}

	// Security check
	for _, v := range varNames {
		if v == "secret123" || v == "abc123" {
			t.Errorf("Security violation: actual value %q was extracted", v)
		}
	}
}

func TestScanLanguages(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "scanner-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create package.json
	os.WriteFile(filepath.Join(tmpDir, "package.json"), []byte(`{"name":"test","version":"1.0.0"}`), 0644)

	scanner := NewScanner(tmpDir)
	result := &ScanResult{
		Languages:      make([]Language, 0),
		Infrastructure: make([]Infrastructure, 0),
		EnvVars:        make([]string, 0),
		Ports:          make([]int, 0),
	}

	err = scanner.scanLanguages(context.Background(), result)
	if err != nil {
		t.Fatalf("scanLanguages() error: %v", err)
	}

	found := false
	for _, lang := range result.Languages {
		if lang.Name == "Node.js" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected Node.js to be detected")
	}
}

func TestScanLanguages_Python(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "scanner-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create requirements.txt
	os.WriteFile(filepath.Join(tmpDir, "requirements.txt"), []byte("flask==2.0.0\nrequests==2.28.0\n"), 0644)

	scanner := NewScanner(tmpDir)
	result := &ScanResult{
		Languages:      make([]Language, 0),
		Infrastructure: make([]Infrastructure, 0),
		EnvVars:        make([]string, 0),
		Ports:          make([]int, 0),
	}

	err = scanner.scanLanguages(context.Background(), result)
	if err != nil {
		t.Fatalf("scanLanguages() error: %v", err)
	}

	found := false
	for _, lang := range result.Languages {
		if lang.Name == "Python" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected Python to be detected")
	}
}

func TestScanLanguages_Go(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "scanner-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create go.mod
	os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte("module example.com/test\ngo 1.21\n"), 0644)

	scanner := NewScanner(tmpDir)
	result := &ScanResult{
		Languages:      make([]Language, 0),
		Infrastructure: make([]Infrastructure, 0),
		EnvVars:        make([]string, 0),
		Ports:          make([]int, 0),
	}

	err = scanner.scanLanguages(context.Background(), result)
	if err != nil {
		t.Fatalf("scanLanguages() error: %v", err)
	}

	found := false
	for _, lang := range result.Languages {
		if lang.Name == "Go" {
			found = true
			if lang.Version != "1.21" {
				t.Errorf("Expected Go version 1.21, got %s", lang.Version)
			}
			break
		}
	}
	if !found {
		t.Error("Expected Go to be detected")
	}
}

func TestScanFull(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "scanner-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create package.json + .env.example
	os.WriteFile(filepath.Join(tmpDir, "package.json"), []byte(`{"name":"test"}`), 0644)
	os.WriteFile(filepath.Join(tmpDir, ".env.example"), []byte("PORT=3000\nDB_URL=test\n"), 0644)

	scanner := NewScanner(tmpDir)
	result, err := scanner.Scan(context.Background())
	if err != nil {
		t.Fatalf("Scan() error: %v", err)
	}

	if len(result.Languages) == 0 {
		t.Error("Expected at least one language")
	}
	if len(result.EnvVars) == 0 {
		t.Error("Expected env vars to be detected")
	}
}

func TestScanLanguages_Empty(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "scanner-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	scanner := NewScanner(tmpDir)
	result := &ScanResult{
		Languages:      make([]Language, 0),
		Infrastructure: make([]Infrastructure, 0),
		EnvVars:        make([]string, 0),
		Ports:          make([]int, 0),
	}

	err = scanner.scanLanguages(context.Background(), result)
	if err != nil {
		t.Fatalf("scanLanguages() error: %v", err)
	}

	if len(result.Languages) != 0 {
		t.Error("Expected no languages in empty dir")
	}
}

func TestScanEnvVars_FromSourcePatterns(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "scanner-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	jsContent := `const db = process.env.DATABASE_URL
const key = process.env["API_KEY"]`
	pyContent := `import os
redis = os.environ.get("REDIS_URL")
secret = os.environ["SECRET_KEY"]`
	goContent := `package main
import "os"
func main() {
	_ = os.Getenv("PORT")
	_, _ = os.LookupEnv("LOG_LEVEL")
}`

	if err := os.WriteFile(filepath.Join(tmpDir, "app.js"), []byte(jsContent), 0644); err != nil {
		t.Fatalf("Failed to write JS file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "settings.py"), []byte(pyContent), 0644); err != nil {
		t.Fatalf("Failed to write Python file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte(goContent), 0644); err != nil {
		t.Fatalf("Failed to write Go file: %v", err)
	}

	s := NewScanner(tmpDir)
	result := &ScanResult{
		Languages:      make([]Language, 0),
		Infrastructure: make([]Infrastructure, 0),
		EnvVars:        make([]string, 0),
		Ports:          make([]int, 0),
	}

	if err := s.scanEnvVars(context.Background(), result); err != nil {
		t.Fatalf("scanEnvVars() error: %v", err)
	}

	expected := map[string]bool{
		"DATABASE_URL": true,
		"API_KEY":      true,
		"REDIS_URL":    true,
		"SECRET_KEY":   true,
		"PORT":         true,
		"LOG_LEVEL":    true,
	}

	for _, name := range result.EnvVars {
		if expected[name] {
			delete(expected, name)
		}
	}

	for name := range expected {
		t.Errorf("Missing expected env var from source scan: %s", name)
	}
}

func TestFileExists(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "scanner-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	os.WriteFile(filepath.Join(tmpDir, "exists.txt"), []byte("hi"), 0644)

	s := NewScanner(tmpDir)
	if !s.fileExists("exists.txt") {
		t.Error("fileExists should return true for existing file")
	}
	if s.fileExists("missing.txt") {
		t.Error("fileExists should return false for missing file")
	}
}

func TestDetectNodeVersion(t *testing.T) {
	s := &Scanner{}
	v := s.detectNodeVersion()
	if v == "" {
		t.Error("detectNodeVersion should return a default version")
	}
}

func TestDetectGoVersion(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "scanner-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte("module test\ngo 1.22\n"), 0644)

	s := NewScanner(tmpDir)
	v := s.detectGoVersion()
	if v != "1.22" {
		t.Errorf("detectGoVersion() = %q, want \"1.22\"", v)
	}
}
