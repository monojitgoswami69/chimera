package config

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Config holds the Chimera configuration.
type Config struct {
	LLMProvider string `json:"llm_provider"`
	LLMModel    string `json:"llm_model"`
	LLMAPIKey   string `json:"llm_api_key"`
	GitHubPAT   string `json:"github_pat,omitempty"`
}

// ChimeraHome returns the platform-specific chimera home directory.
func ChimeraHome() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get user home directory: %w", err)
	}
	return filepath.Join(home, ".chimera"), nil
}

// ReposDir returns the directory where cloned repos live.
func ReposDir() (string, error) {
	home, err := ChimeraHome()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "repos"), nil
}

// ConfigPath returns the path to the JSON config file.
func ConfigPath() (string, error) {
	home, err := ChimeraHome()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "config.json"), nil
}

// legacyConfigPath is the dotenv path used by older versions.
func legacyConfigPath() (string, error) {
	home, err := ChimeraHome()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".chimera.env"), nil
}

// Exists reports whether a configuration file is present.
func Exists() bool {
	p, err := ConfigPath()
	if err == nil {
		if _, err := os.Stat(p); err == nil {
			return true
		}
	}
	if lp, err := legacyConfigPath(); err == nil {
		if _, err := os.Stat(lp); err == nil {
			return true
		}
	}
	return false
}

// LoadOptional returns the config if present, or an empty config and no error.
// Use this when downstream code can run without an LLM configured (e.g. --no-agent).
func LoadOptional() *Config {
	cfg, err := load(false)
	if err != nil || cfg == nil {
		return &Config{}
	}
	return cfg
}

// Load reads the configuration and validates required fields.
func Load() (*Config, error) {
	return load(true)
}

func load(strict bool) (*Config, error) {
	path, err := ConfigPath()
	if err != nil {
		return nil, err
	}

	if data, err := os.ReadFile(path); err == nil {
		cfg := &Config{}
		if err := json.Unmarshal(data, cfg); err != nil {
			return nil, fmt.Errorf("config file is not valid JSON: %w", err)
		}
		if strict {
			if err := cfg.Validate(); err != nil {
				return nil, err
			}
		}
		return cfg, nil
	}

	// Fallback: legacy dotenv file.
	if lp, err := legacyConfigPath(); err == nil {
		if _, err := os.Stat(lp); err == nil {
			vars := readEnvFile(lp)
			cfg := &Config{
				LLMProvider: vars["CHIMERA_LLM_PROVIDER"],
				LLMModel:    vars["CHIMERA_LLM_MODEL"],
				LLMAPIKey:   vars["CHIMERA_LLM_API_KEY"],
				GitHubPAT:   vars["CHIMERA_GITHUB_PAT"],
			}
			if strict {
				if err := cfg.Validate(); err != nil {
					return nil, err
				}
			}
			return cfg, nil
		}
	}

	if strict {
		return nil, fmt.Errorf("config not found at %s — run `chimera setup`", path)
	}
	return nil, nil
}

// Validate checks that required fields are present (Ollama exempts API key).
func (c *Config) Validate() error {
	if c.LLMProvider == "" {
		return fmt.Errorf("config is missing llm_provider")
	}
	if c.LLMModel == "" {
		return fmt.Errorf("config is missing llm_model")
	}
	if c.LLMProvider != "ollama" && c.LLMAPIKey == "" {
		return fmt.Errorf("config is missing llm_api_key")
	}
	return nil
}

// Save writes the configuration to ~/.chimera/config.json with 0600 perms.
func Save(cfg *Config) error {
	home, err := ChimeraHome()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(home, 0700); err != nil {
		return fmt.Errorf("failed to create chimera home: %w", err)
	}

	path := filepath.Join(home, "config.json")

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to encode config: %w", err)
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}

	// Clean up legacy file if present so we don't have two sources of truth.
	if lp, err := legacyConfigPath(); err == nil {
		_ = os.Remove(lp)
	}
	return nil
}

// readEnvFile reads a .env-style file and returns key-value pairs.
func readEnvFile(path string) map[string]string {
	vars := make(map[string]string)

	f, err := os.Open(path)
	if err != nil {
		return vars
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		value = strings.Trim(value, `"'`)
		if key != "" {
			vars[key] = value
		}
	}
	return vars
}
