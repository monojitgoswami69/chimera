package git

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
)

// Client represents a Git client for repository operations
type Client struct{}

// CloneOptions contains options for cloning a repository
type CloneOptions struct {
	URL       string
	TargetDir string
	Depth     int
	Token     string // GitHub token for private repos (FR-1.2)
}

// NewClient creates a new Git client
func NewClient() *Client {
	return &Client{}
}

// Clone clones a Git repository to the target directory
// Implements FR-1.1: Repository cloning
// Implements FR-1.2: Private repository authentication via GITHUB_TOKEN
func (c *Client) Clone(ctx context.Context, opts *CloneOptions) error {
	if opts == nil {
		return fmt.Errorf("clone options cannot be nil")
	}

	if opts.URL == "" {
		return fmt.Errorf("repository URL is required")
	}

	if opts.TargetDir == "" {
		return fmt.Errorf("target directory is required")
	}

	// Prepare clone options
	cloneOpts := &git.CloneOptions{
		URL:      opts.URL,
		Progress: os.Stdout,
	}

	// Set depth for shallow clone (performance optimization)
	if opts.Depth > 0 {
		cloneOpts.Depth = opts.Depth
	}

	// FR-1.2: Configure authentication for private repositories
	// If a GitHub token is provided, use it for authentication
	if opts.Token != "" {
		cloneOpts.Auth = &http.BasicAuth{
			Username: "x-access-token", // GitHub uses this as username for token auth
			Password: opts.Token,
		}
	}

	// Perform the clone operation
	_, err := git.PlainCloneContext(ctx, opts.TargetDir, false, cloneOpts)
	if err != nil {
		// Provide helpful error messages for common issues
		if err == git.ErrRepositoryAlreadyExists {
			return fmt.Errorf("repository already exists at %s", opts.TargetDir)
		}
		return fmt.Errorf("failed to clone repository: %w", err)
	}

	return nil
}

// ExtractRepoName extracts the repository name from a Git URL
// Supports both HTTPS and SSH URLs
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

	// Validate URL has a proper scheme and host
	if parsedURL.Scheme == "" || parsedURL.Host == "" {
		return ""
	}

	// Extract the last part of the path
	path := strings.TrimSuffix(parsedURL.Path, ".git")
	path = strings.TrimPrefix(path, "/")
	
	parts := strings.Split(path, "/")
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}

	return ""
}

// IsGitRepository checks if a directory is a Git repository
func IsGitRepository(dir string) bool {
	gitDir := filepath.Join(dir, ".git")
	info, err := os.Stat(gitDir)
	if err != nil {
		return false
	}
	return info.IsDir()
}

// GetRemoteURL returns the remote URL of a Git repository
func GetRemoteURL(dir string) (string, error) {
	repo, err := git.PlainOpen(dir)
	if err != nil {
		return "", fmt.Errorf("failed to open repository: %w", err)
	}

	remote, err := repo.Remote("origin")
	if err != nil {
		return "", fmt.Errorf("failed to get remote: %w", err)
	}

	config := remote.Config()
	if len(config.URLs) == 0 {
		return "", fmt.Errorf("no remote URL found")
	}

	return config.URLs[0], nil
}
