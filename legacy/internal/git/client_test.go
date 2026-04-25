package git

import (
	"testing"
)

func TestExtractRepoName(t *testing.T) {
	tests := []struct {
		name     string
		repoURL  string
		expected string
	}{
		{
			name:     "HTTPS URL with .git",
			repoURL:  "https://github.com/user/my-repo.git",
			expected: "my-repo",
		},
		{
			name:     "HTTPS URL without .git",
			repoURL:  "https://github.com/user/my-repo",
			expected: "my-repo",
		},
		{
			name:     "SSH URL with .git",
			repoURL:  "git@github.com:user/my-repo.git",
			expected: "my-repo",
		},
		{
			name:     "SSH URL without .git",
			repoURL:  "git@github.com:user/my-repo",
			expected: "my-repo",
		},
		{
			name:     "Complex repo name",
			repoURL:  "https://github.com/organization/project-chimera.git",
			expected: "project-chimera",
		},
		{
			name:     "Invalid URL",
			repoURL:  "not-a-valid-url",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ExtractRepoName(tt.repoURL)
			if result != tt.expected {
				t.Errorf("ExtractRepoName(%q) = %q, want %q", tt.repoURL, result, tt.expected)
			}
		})
	}
}
