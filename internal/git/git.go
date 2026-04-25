package git

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

// Clone clones a repository using git command
func Clone(ctx context.Context, repoURL, targetDir, pat string) error {
	// Validate URL
	if !IsValidGitHubURL(repoURL) {
		return fmt.Errorf("invalid GitHub URL: must match https://github.com/<owner>/<repo>")
	}

	// If PAT is provided, inject it into the URL
	cloneURL := repoURL
	if pat != "" {
		cloneURL = injectPAT(repoURL, pat)
	}

	// Execute git clone
	cmd := exec.CommandContext(ctx, "git", "clone", "--depth=1", cloneURL, targetDir)
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git clone failed: %w", err)
	}

	return nil
}

// ExtractRepoName extracts repository name from URL
func ExtractRepoName(repoURL string) string {
	// Handle SSH URLs (git@github.com:user/repo.git)
	if strings.HasPrefix(repoURL, "git@") {
		parts := strings.Split(repoURL, ":")
		if len(parts) == 2 {
			path := parts[1]
			path = strings.TrimSuffix(path, ".git")
			pathParts := strings.Split(path, "/")
			if len(pathParts) > 0 {
				return pathParts[len(pathParts)-1]
			}
		}
		return ""
	}

	// Handle HTTPS URLs
	parsedURL, err := url.Parse(repoURL)
	if err != nil {
		return ""
	}

	path := strings.TrimSuffix(parsedURL.Path, ".git")
	path = strings.TrimPrefix(path, "/")

	parts := strings.Split(path, "/")
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}

	return ""
}

// IsValidGitHubURL validates that the URL matches GitHub pattern
func IsValidGitHubURL(repoURL string) bool {
	pattern := regexp.MustCompile(`^https://github\.com/[a-zA-Z0-9_-]+/[a-zA-Z0-9_.-]+(?:\.git)?$`)
	return pattern.MatchString(repoURL)
}

// injectPAT injects a Personal Access Token into the GitHub URL
func injectPAT(repoURL, pat string) string {
	// Convert https://github.com/owner/repo to https://<PAT>@github.com/owner/repo
	return strings.Replace(repoURL, "https://", fmt.Sprintf("https://%s@", pat), 1)
}

// CountFiles counts files in a directory
func CountFiles(dir string) (int, int64, error) {
	var count int
	var size int64

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if !info.IsDir() {
			count++
			size += info.Size()
		}
		return nil
	})

	return count, size, err
}
