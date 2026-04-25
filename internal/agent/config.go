package agent

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

// LoadConfig loads environment variables from .chimera.env files.
// Priority: existing env vars (highest) > local (./.chimera.env or .env) > global (~/.chimera.env)
// Existing env vars are NOT overwritten — file values are only used as defaults.
func LoadConfig() {
	home, _ := os.UserHomeDir()

	// Collect values from both files
	globalVars := make(map[string]string)
	localVars := make(map[string]string)

	// Load global first
	globalPath := ""
	if home != "" {
		globalPath = filepath.Join(home, ".chimera.env")
		globalVars = readEnvFile(globalPath)
	}

	// Load local (overwrites global)
	// Try .chimera.env first, then fall back to .env
	localPath := ".chimera.env"
	localVars = readEnvFile(localPath)
	if len(localVars) == 0 {
		localPath = ".env"
		localVars = readEnvFile(localPath)
	}

	// Merge: local overwrites global
	merged := make(map[string]string)
	for k, v := range globalVars {
		merged[k] = v
	}
	for k, v := range localVars {
		merged[k] = v
	}

	// Set environment variables (only if not already set)
	for key, value := range merged {
		if os.Getenv(key) == "" && value != "" {
			os.Setenv(key, value)
		}
	}
	
	// Debug output if CHIMERA_DEBUG is set
	if os.Getenv("CHIMERA_DEBUG") == "1" {
		if len(globalVars) > 0 {
			println("[DEBUG] Loaded", len(globalVars), "vars from", globalPath)
		}
		if len(localVars) > 0 {
			println("[DEBUG] Loaded", len(localVars), "vars from", localPath)
		}
		if len(merged) > 0 {
			println("[DEBUG] Config loaded:")
			for k := range merged {
				// Don't print API keys
				if strings.Contains(strings.ToLower(k), "key") || strings.Contains(strings.ToLower(k), "token") {
					println("[DEBUG]  ", k, "= ***")
				} else {
					println("[DEBUG]  ", k, "=", merged[k])
				}
			}
		} else {
			println("[DEBUG] No config files found")
		}
	}
}

// readEnvFile reads a .env-style file and returns key-value pairs
func readEnvFile(path string) map[string]string {
	vars := make(map[string]string)
	
	f, err := os.Open(path)
	if err != nil {
		return vars // file doesn't exist, return empty map
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

		if key != "" && value != "" {
			vars[key] = value
		}
	}
	
	return vars
}

// loadEnvFile reads a .env-style file and sets env vars that are not already set.
// Deprecated: Use LoadConfig() instead which properly handles priority.
func loadEnvFile(path string) {
	vars := readEnvFile(path)
	for key, value := range vars {
		if os.Getenv(key) == "" && value != "" {
			os.Setenv(key, value)
		}
	}
}
