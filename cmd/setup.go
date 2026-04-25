package cmd

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/projectchimera/chimera/internal/tui"
	"github.com/spf13/cobra"
)

// setupCmd represents the setup command
var setupCmd = &cobra.Command{
	Use:   "setup",
	Short: "Interactive setup wizard for Chimera configuration",
	Long: `Launch an interactive setup wizard to configure Chimera's AI agent.

This wizard will guide you through:
  1. Selecting an LLM provider (OpenAI, Gemini, Groq)
  2. Entering your API key
  3. Validating the API key
  4. Selecting a model from available options
  5. Saving configuration to ./.chimera.env (current directory)

After setup, Chimera will automatically use AI agent mode for smarter
code analysis and configuration generation when run from this directory.

Configuration priority:
  1. Environment variables (highest)
  2. ./.chimera.env (current directory)
  3. ~/.chimera.env (home directory)

Example:
  chimera setup`,
	RunE: runSetup,
}

func init() {
	rootCmd.AddCommand(setupCmd)
}

// runSetup executes the interactive setup wizard
func runSetup(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	
	tui.PrintHeader("Chimera Setup Wizard")
	fmt.Println()
	
	fmt.Println("This wizard will help you configure Chimera's AI agent for smarter")
	fmt.Println("code analysis and environment generation.")
	fmt.Println()
	
	// Step 1: Select provider
	provider := selectProvider()
	if provider == "" {
		return fmt.Errorf("setup cancelled")
	}
	
	fmt.Println()
	
	// Step 2: Enter API key
	apiKey := enterAPIKey(provider)
	if apiKey == "" {
		return fmt.Errorf("setup cancelled")
	}
	
	fmt.Println()
	tui.PrintInfo("Validating API key...")
	
	// Step 3: Validate API key and fetch models
	models, err := fetchModels(ctx, provider, apiKey)
	if err != nil {
		tui.PrintError(fmt.Sprintf("Failed to validate API key: %v", err))
		return fmt.Errorf("setup: API key validation failed")
	}
	
	tui.PrintSuccess("API key validated successfully!")
	fmt.Println()
	
	// Step 4: Select model
	selectedModel := selectModel(provider, models)
	if selectedModel == "" {
		return fmt.Errorf("setup cancelled")
	}
	
	fmt.Println()
	
	// Step 5: Save configuration
	if err := saveConfig(provider, apiKey, selectedModel); err != nil {
		tui.PrintError(fmt.Sprintf("Failed to save configuration: %v", err))
		return fmt.Errorf("setup: failed to save config")
	}
	
	fmt.Println()
	tui.PrintSuccess("✓ Setup complete!")
	fmt.Println()
	
	cwd, _ := os.Getwd()
	configPath := filepath.Join(cwd, ".chimera.env")
	fmt.Println("Configuration saved to:", configPath)
	fmt.Println()
	fmt.Println("You can now use Chimera with AI agent mode:")
	fmt.Println("  chimera init <github-repo-url>")
	fmt.Println()
	fmt.Println("Note: The .chimera.env file is in your current directory.")
	fmt.Println("      Run chimera commands from this directory to use this config.")
	fmt.Println()
	
	return nil
}

// selectProvider prompts user to select an LLM provider
func selectProvider() string {
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println("  STEP 1: Select LLM Provider")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println()
	fmt.Println("Available providers:")
	fmt.Println()
	fmt.Println("  1. OpenAI")
	fmt.Println("     • Models: GPT-4o, GPT-4o-mini, GPT-4-turbo")
	fmt.Println("     • Best for: General purpose, high quality")
	fmt.Println("     • Get API key: https://platform.openai.com/api-keys")
	fmt.Println("     • Recommended: ⭐ GPT-4o-mini (fast, cost-effective)")
	fmt.Println()
	fmt.Println("  2. Google Gemini")
	fmt.Println("     • Models: Gemini 2.0 Flash, Gemini 1.5 Pro")
	fmt.Println("     • Best for: Fast responses, large context")
	fmt.Println("     • Get API key: https://aistudio.google.com/apikey")
	fmt.Println("     • Recommended: ⭐ Gemini 2.0 Flash (fastest)")
	fmt.Println()
	fmt.Println("  3. Groq")
	fmt.Println("     • Models: Llama 3.1 70B, Mixtral 8x7B")
	fmt.Println("     • Best for: Ultra-fast inference, open models")
	fmt.Println("     • Get API key: https://console.groq.com/keys")
	fmt.Println("     • Recommended: ⭐ Llama 3.1 70B (powerful, fast)")
	fmt.Println()
	
	reader := bufio.NewReader(os.Stdin)
	
	for {
		fmt.Print("Select provider (1-3) or 'q' to quit: ")
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(input)
		
		switch input {
		case "1":
			return "openai"
		case "2":
			return "gemini"
		case "3":
			return "groq"
		case "q", "Q":
			return ""
		default:
			tui.PrintWarning("Invalid selection. Please enter 1, 2, 3, or 'q'")
		}
	}
}

// enterAPIKey prompts user to enter their API key
func enterAPIKey(provider string) string {
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println("  STEP 2: Enter API Key")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println()
	
	var keyURL string
	switch provider {
	case "openai":
		keyURL = "https://platform.openai.com/api-keys"
		fmt.Println("Get your OpenAI API key from:", keyURL)
		fmt.Println("Format: sk-...")
	case "gemini":
		keyURL = "https://aistudio.google.com/apikey"
		fmt.Println("Get your Gemini API key from:", keyURL)
	case "groq":
		keyURL = "https://console.groq.com/keys"
		fmt.Println("Get your Groq API key from:", keyURL)
		fmt.Println("Format: gsk_...")
	}
	
	fmt.Println()
	
	reader := bufio.NewReader(os.Stdin)
	
	for {
		fmt.Print("Enter your API key (or 'q' to quit): ")
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(input)
		
		if input == "q" || input == "Q" {
			return ""
		}
		
		if len(input) < 10 {
			tui.PrintWarning("API key seems too short. Please check and try again.")
			continue
		}
		
		return input
	}
}

// fetchModels fetches available models from the provider
func fetchModels(ctx context.Context, provider, apiKey string) ([]ModelInfo, error) {
	switch provider {
	case "openai":
		return fetchOpenAIModels(ctx, apiKey)
	case "gemini":
		return getGeminiModels(), nil
	case "groq":
		return fetchGroqModels(ctx, apiKey)
	default:
		return nil, fmt.Errorf("unsupported provider: %s", provider)
	}
}

// ModelInfo represents a model with metadata
type ModelInfo struct {
	ID          string
	Name        string
	Description string
	Recommended bool
}

// fetchOpenAIModels fetches available models from OpenAI
func fetchOpenAIModels(ctx context.Context, apiKey string) ([]ModelInfo, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", "https://api.openai.com/v1/models", nil)
	if err != nil {
		return nil, err
	}
	
	req.Header.Set("Authorization", "Bearer "+apiKey)
	
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to OpenAI: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("OpenAI API error (status %d): %s", resp.StatusCode, string(body))
	}
	
	var result struct {
		Data []struct {
			ID      string `json:"id"`
			Created int64  `json:"created"`
		} `json:"data"`
	}
	
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	
	// Filter for chat/completion models and add metadata
	models := []ModelInfo{}
	modelPriority := map[string]int{
		"gpt-4o":          1,
		"gpt-4o-mini":     2,
		"gpt-4-turbo":     3,
		"gpt-4":           4,
		"gpt-3.5-turbo":   5,
	}
	
	modelDescriptions := map[string]string{
		"gpt-4o":          "Most capable model, multimodal, best quality",
		"gpt-4o-mini":     "Fast and cost-effective, great for most tasks",
		"gpt-4-turbo":     "Previous generation, still very capable",
		"gpt-4":           "Original GPT-4, reliable",
		"gpt-3.5-turbo":   "Fastest, lowest cost",
	}
	
	seenModels := make(map[string]bool)
	
	for _, model := range result.Data {
		// Only include chat/completion models
		if !strings.Contains(model.ID, "gpt") {
			continue
		}
		
		// Skip embeddings, whisper, tts, dall-e, etc.
		if strings.Contains(model.ID, "embed") || 
		   strings.Contains(model.ID, "whisper") ||
		   strings.Contains(model.ID, "tts") ||
		   strings.Contains(model.ID, "dall-e") ||
		   strings.Contains(model.ID, "babbage") ||
		   strings.Contains(model.ID, "davinci") ||
		   strings.Contains(model.ID, "instruct") {
			continue
		}
		
		// Get base model name (without date suffixes)
		baseModel := model.ID
		for prefix := range modelPriority {
			if strings.HasPrefix(model.ID, prefix) {
				baseModel = prefix
				break
			}
		}
		
		// Skip if we've already added this base model
		if seenModels[baseModel] {
			continue
		}
		seenModels[baseModel] = true
		
		description := modelDescriptions[baseModel]
		if description == "" {
			description = "OpenAI language model"
		}
		
		models = append(models, ModelInfo{
			ID:          model.ID,
			Name:        formatModelName(model.ID),
			Description: description,
			Recommended: baseModel == "gpt-4o-mini",
		})
	}
	
	// Sort by priority
	sortModelsByPriority(models, modelPriority)
	
	// If no models found, return error
	if len(models) == 0 {
		return nil, fmt.Errorf("no compatible models found in your OpenAI account")
	}
	
	return models, nil
}

// getGeminiModels returns available Gemini models by querying the API
func getGeminiModels() []ModelInfo {
	// Gemini doesn't have a models list endpoint that works without complex auth
	// Return known available models
	return []ModelInfo{
		{
			ID:          "gemini-2.0-flash-exp",
			Name:        "Gemini 2.0 Flash (Experimental)",
			Description: "Latest model, fastest responses, experimental features",
			Recommended: true,
		},
		{
			ID:          "gemini-1.5-flash",
			Name:        "Gemini 1.5 Flash",
			Description: "Stable, fast, good quality, 1M token context",
			Recommended: false,
		},
		{
			ID:          "gemini-1.5-flash-8b",
			Name:        "Gemini 1.5 Flash 8B",
			Description: "Smaller, faster, cost-effective",
			Recommended: false,
		},
		{
			ID:          "gemini-1.5-pro",
			Name:        "Gemini 1.5 Pro",
			Description: "Most capable, 2M token context, best quality",
			Recommended: false,
		},
	}
}

// fetchGroqModels fetches available models from Groq
func fetchGroqModels(ctx context.Context, apiKey string) ([]ModelInfo, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", "https://api.groq.com/openai/v1/models", nil)
	if err != nil {
		return nil, err
	}
	
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")
	
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to Groq: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("Groq API error (status %d): %s", resp.StatusCode, string(body))
	}
	
	var result struct {
		Data []struct {
			ID      string `json:"id"`
			OwnedBy string `json:"owned_by"`
			Active  bool   `json:"active"`
		} `json:"data"`
	}
	
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	
	// Build models list with metadata
	models := []ModelInfo{}
	modelPriority := map[string]int{
		"llama-3.3-70b-versatile":    1,
		"llama-3.1-70b-versatile":    2,
		"llama-3.2-90b-vision-preview": 3,
		"mixtral-8x7b-32768":         4,
		"llama-3.1-8b-instant":       5,
		"gemma2-9b-it":               6,
	}
	
	modelDescriptions := map[string]string{
		"llama-3.3-70b-versatile":    "Latest Llama 3.3, most capable, best quality",
		"llama-3.1-70b-versatile":    "Llama 3.1 70B, powerful and reliable",
		"llama-3.2-90b-vision-preview": "Llama 3.2 with vision, multimodal",
		"mixtral-8x7b-32768":         "Mixtral MoE, fast, 32K context",
		"llama-3.1-8b-instant":       "Llama 3.1 8B, ultra-fast, good for simple tasks",
		"gemma2-9b-it":               "Google Gemma 2, efficient",
		"llama-3.2-11b-vision-preview": "Llama 3.2 11B with vision",
		"llama-3.2-3b-preview":       "Llama 3.2 3B, very fast",
		"llama-3.2-1b-preview":       "Llama 3.2 1B, fastest",
	}
	
	for _, model := range result.Data {
		// Skip inactive models
		if !model.Active {
			continue
		}
		
		// Only include models we know about or that look like chat models
		description := modelDescriptions[model.ID]
		if description == "" {
			// Try to infer description
			if strings.Contains(model.ID, "llama") {
				description = "Llama language model"
			} else if strings.Contains(model.ID, "mixtral") {
				description = "Mixtral mixture-of-experts model"
			} else if strings.Contains(model.ID, "gemma") {
				description = "Google Gemma model"
			} else {
				description = "Language model"
			}
		}
		
		models = append(models, ModelInfo{
			ID:          model.ID,
			Name:        formatModelName(model.ID),
			Description: description,
			Recommended: model.ID == "llama-3.3-70b-versatile" || model.ID == "llama-3.1-70b-versatile",
		})
	}
	
	// Sort by priority
	sortModelsByPriority(models, modelPriority)
	
	// If no models found, return error
	if len(models) == 0 {
		return nil, fmt.Errorf("no compatible models found in your Groq account")
	}
	
	return models, nil
}

// formatModelName converts model ID to a human-readable name
func formatModelName(id string) string {
	// Remove common suffixes
	name := strings.TrimSuffix(id, "-preview")
	name = strings.TrimSuffix(name, "-latest")
	
	// Capitalize and format
	parts := strings.Split(name, "-")
	for i, part := range parts {
		if len(part) > 0 {
			// Keep version numbers and special terms lowercase
			if part == "it" || part == "instruct" || part == "chat" {
				continue
			}
			// Capitalize first letter
			parts[i] = strings.ToUpper(part[:1]) + part[1:]
		}
	}
	
	return strings.Join(parts, " ")
}

// sortModelsByPriority sorts models based on priority map
func sortModelsByPriority(models []ModelInfo, priority map[string]int) {
	// Simple bubble sort based on priority
	for i := 0; i < len(models); i++ {
		for j := i + 1; j < len(models); j++ {
			iPriority := priority[models[i].ID]
			jPriority := priority[models[j].ID]
			
			// If not in priority map, assign high number
			if iPriority == 0 {
				iPriority = 999
			}
			if jPriority == 0 {
				jPriority = 999
			}
			
			if iPriority > jPriority {
				models[i], models[j] = models[j], models[i]
			}
		}
	}
}

// selectModel prompts user to select a model
func selectModel(provider string, models []ModelInfo) string {
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println("  STEP 3: Select Model")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println()
	fmt.Println("Available models:")
	fmt.Println()
	
	for i, model := range models {
		prefix := fmt.Sprintf("  %d. %s", i+1, model.Name)
		if model.Recommended {
			prefix += " ⭐ RECOMMENDED"
		}
		fmt.Println(prefix)
		fmt.Printf("     %s\n", model.Description)
		fmt.Printf("     ID: %s\n", model.ID)
		fmt.Println()
	}
	
	reader := bufio.NewReader(os.Stdin)
	
	for {
		fmt.Printf("Select model (1-%d) or 'q' to quit: ", len(models))
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(input)
		
		if input == "q" || input == "Q" {
			return ""
		}
		
		var selection int
		if _, err := fmt.Sscanf(input, "%d", &selection); err == nil {
			if selection >= 1 && selection <= len(models) {
				return models[selection-1].ID
			}
		}
		
		tui.PrintWarning(fmt.Sprintf("Invalid selection. Please enter 1-%d or 'q'", len(models)))
	}
}

// saveConfig saves the configuration to ./.chimera.env (current directory)
func saveConfig(provider, apiKey, model string) error {
	// Get current working directory
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}
	
	configPath := filepath.Join(cwd, ".chimera.env")
	
	// Read existing config if it exists
	existingConfig := make(map[string]string)
	if data, err := os.ReadFile(configPath); err == nil {
		lines := strings.Split(string(data), "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				existingConfig[parts[0]] = parts[1]
			}
		}
	}
	
	// Update with new values
	existingConfig["CHIMERA_LLM_PROVIDER"] = provider
	existingConfig["CHIMERA_MODEL"] = model
	
	switch provider {
	case "openai":
		existingConfig["OPENAI_API_KEY"] = apiKey
	case "gemini":
		existingConfig["GEMINI_API_KEY"] = apiKey
	case "groq":
		existingConfig["GROQ_API_KEY"] = apiKey
	}
	
	// Write config file
	var content strings.Builder
	content.WriteString("# Chimera Configuration\n")
	content.WriteString("# Generated by: chimera setup\n")
	content.WriteString(fmt.Sprintf("# Date: %s\n\n", time.Now().Format("2006-01-02 15:04:05")))
	
	content.WriteString("# LLM Provider Configuration\n")
	content.WriteString(fmt.Sprintf("CHIMERA_LLM_PROVIDER=%s\n", existingConfig["CHIMERA_LLM_PROVIDER"]))
	content.WriteString(fmt.Sprintf("CHIMERA_MODEL=%s\n\n", existingConfig["CHIMERA_MODEL"]))
	
	content.WriteString("# API Keys\n")
	if val, ok := existingConfig["OPENAI_API_KEY"]; ok {
		content.WriteString(fmt.Sprintf("OPENAI_API_KEY=%s\n", val))
	}
	if val, ok := existingConfig["GEMINI_API_KEY"]; ok {
		content.WriteString(fmt.Sprintf("GEMINI_API_KEY=%s\n", val))
	}
	if val, ok := existingConfig["GROQ_API_KEY"]; ok {
		content.WriteString(fmt.Sprintf("GROQ_API_KEY=%s\n", val))
	}
	
	// Add GitHub token if it exists
	if val, ok := existingConfig["GITHUB_TOKEN"]; ok {
		content.WriteString(fmt.Sprintf("\n# GitHub Access\nGITHUB_TOKEN=%s\n", val))
	}
	
	return os.WriteFile(configPath, []byte(content.String()), 0600)
}
