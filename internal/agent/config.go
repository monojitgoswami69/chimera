package agent

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

// LoadConfig loads environment variables from .chimera.env files.
// Priority: local (./.chimera.env) > global (~/.chimera.env) > existing env vars.
// Existing env vars are NOT overwritten — file values are only used as defaults.
func LoadConfig() {
	home, _ := os.UserHomeDir()

	// Load global first (lower priority)
	if home != "" {
		loadEnvFile(filepath.Join(home, ".chimera.env"))
	}

	// Load local (higher priority — overwrites global file values)
	loadEnvFile(".chimera.env")
}

// loadEnvFile reads a .env-style file and sets env vars that are not already set.
func loadEnvFile(path string) {
	f, err := os.Open(path)
	if err != nil {
		return // file doesn't exist, that's fine
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Skip comments and empty lines
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Parse KEY=VALUE
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		// Remove surrounding quotes
		value = strings.Trim(value, `"'`)

		// Only set if not already in environment (env vars take precedence)
		if os.Getenv(key) == "" && value != "" {
			os.Setenv(key, value)
		}
	}
}
