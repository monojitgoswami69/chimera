package proxy

import (
	"os"
	"strings"
	"testing"
)

func TestDeriveLocalDomain(t *testing.T) {
	tests := []struct {
		name   string
		url    string
		expect string
	}{
		{"https basic", "https://github.com/acme/my-app", "my-app.local"},
		{"https with .git", "https://github.com/acme/my-app.git", "my-app.local"},
		{"underscore replacement", "https://github.com/acme/My_Service", "my-service.local"},
		{"uppercase normalization", "https://github.com/acme/MyService", "myservice.local"},
		{"ssh url", "git@github.com:acme/private-repo.git", "private-repo.local"},
		{"org prefix stripped", "https://github.com/org/project-name", "project-name.local"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DeriveLocalDomain(tt.url)
			if got != tt.expect {
				t.Errorf("DeriveLocalDomain(%q) = %q, want %q", tt.url, got, tt.expect)
			}
		})
	}
}

func TestProjectName(t *testing.T) {
	tests := []struct {
		url    string
		expect string
	}{
		{"https://github.com/user/awesome-project", "awesome-project"},
		{"https://github.com/user/My_App.git", "my-app"},
		{"git@github.com:org/repo.git", "repo"},
		{"", "chimera"},
	}

	for _, tt := range tests {
		t.Run(tt.url, func(t *testing.T) {
			got := ProjectName(tt.url)
			if got != tt.expect {
				t.Errorf("ProjectName(%q) = %q, want %q", tt.url, got, tt.expect)
			}
		})
	}
}

func TestGenerateCaddyfile(t *testing.T) {
	cf := GenerateCaddyfile("myapp.local", 3000)
	if !strings.Contains(cf, "myapp.local") {
		t.Error("Caddyfile missing domain")
	}
	if !strings.Contains(cf, "reverse_proxy app:3000") {
		t.Error("Caddyfile missing reverse_proxy directive")
	}
}

func TestCaddyComposeSnippet_WithNetwork(t *testing.T) {
	snippet := CaddyComposeSnippet("myapp", "myapp_net")
	if !strings.Contains(snippet, "caddy:") {
		t.Error("snippet missing caddy service")
	}
	if !strings.Contains(snippet, "- myapp_net") {
		t.Error("snippet should include provided network")
	}
}

func TestCaddyComposeSnippet_WithoutNetwork(t *testing.T) {
	snippet := CaddyComposeSnippet("myapp", "")
	if strings.Contains(snippet, "networks:") {
		t.Error("snippet should omit networks when no network is provided")
	}
}

func TestAddRemoveEntry(t *testing.T) {
	// Create temp hosts file
	f, err := os.CreateTemp("", "hosts-test-*")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(f.Name())

	// Write initial content
	f.WriteString("127.0.0.1 localhost\n::1 localhost\n")
	f.Close()

	// Add entry
	err = addEntryToFile(f.Name(), "myapp.local", "myapp")
	if err != nil {
		t.Fatalf("addEntryToFile() error: %v", err)
	}

	// Verify entry exists
	exists, err := entryExistsInFile(f.Name(), "myapp.local")
	if err != nil {
		t.Fatalf("entryExistsInFile() error: %v", err)
	}
	if !exists {
		t.Error("entry should exist after add")
	}

	// Add again — should be idempotent
	err = addEntryToFile(f.Name(), "myapp.local", "myapp")
	if err != nil {
		t.Fatalf("idempotent addEntryToFile() error: %v", err)
	}

	// Count lines containing myapp.local (should be exactly 1)
	content, _ := os.ReadFile(f.Name())
	count := strings.Count(string(content), "myapp.local")
	if count != 1 {
		t.Errorf("expected 1 entry, got %d", count)
	}

	// Original entries should still be there
	if !strings.Contains(string(content), "127.0.0.1 localhost") {
		t.Error("original localhost entry should be preserved")
	}

	// Remove entry
	err = removeEntryFromFile(f.Name(), "myapp")
	if err != nil {
		t.Fatalf("removeEntryFromFile() error: %v", err)
	}

	// Verify entry is gone
	exists, err = entryExistsInFile(f.Name(), "myapp.local")
	if err != nil {
		t.Fatalf("entryExistsInFile() error: %v", err)
	}
	if exists {
		t.Error("entry should be removed")
	}

	// Verify other entries preserved
	content, _ = os.ReadFile(f.Name())
	if !strings.Contains(string(content), "127.0.0.1 localhost") {
		t.Error("original localhost entry should be preserved after remove")
	}

	// Remove again — should be idempotent
	err = removeEntryFromFile(f.Name(), "myapp")
	if err != nil {
		t.Fatalf("idempotent removeEntryFromFile() error: %v", err)
	}
}

func TestHostsFilePath(t *testing.T) {
	path := HostsFilePath()
	if path == "" {
		t.Error("HostsFilePath() should not be empty")
	}
}
