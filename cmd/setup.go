package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"chimera/internal/config"
	"chimera/internal/llm"
	"chimera/internal/ui"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
)

var setupCmd = &cobra.Command{
	Use:   "setup",
	Short: "Configure your LLM provider and API key",
	Long: `Interactive wizard to configure Chimera with your LLM provider, API key, and optional GitHub PAT.

This creates ~/.chimera/.chimera.env with your configuration.

Flags:
  --force    Force reconfiguration even if config already exists

Examples:
  chimera setup
  chimera setup --force`,
	Run: func(cmd *cobra.Command, args []string) {
		runSetup()
	},
}

var (
	setupForce bool
)

func init() {
	setupCmd.Flags().BoolVar(&setupForce, "force", false, "Force reconfiguration even if config exists")
}



func runSetup() {
	// Check if config already exists
	if !setupForce {
		configPath, _ := config.ConfigPath()
		if _, err := os.Stat(configPath); err == nil {
			// Config exists
			fmt.Println(ui.Header("setup"))
			fmt.Println(ui.WarningLine("Configuration already exists"))
			fmt.Println()
			fmt.Println(ui.InfoLine(fmt.Sprintf("Config file: %s", configPath)))
			fmt.Println()
			fmt.Println(ui.DimStyle.Render("To reconfigure, run:"))
			fmt.Println(ui.HighlightStyle.Render("  chimera setup --force"))
			fmt.Println()
			return
		}
	}

	// Print banner once
	fmt.Print(ui.Header("setup"))

	// Step 1: Provider selection
	fmt.Println(ui.BoldStyle.Render("Select LLM Provider:"))
	fmt.Println()
	
	providers := []string{"OpenAI", "Anthropic (Claude)", "Groq", "Google Gemini", "Ollama (local)"}
	providerKeys := []string{"openai", "anthropic", "groq", "gemini", "ollama"}
	
	selectedProvider := selectFromList(providers)
	if selectedProvider == -1 {
		fmt.Println(ui.ErrorLine("Setup cancelled"))
		return
	}
	provider := providerKeys[selectedProvider]
	
	fmt.Println(ui.SuccessLine(fmt.Sprintf("Selected: %s", providers[selectedProvider])))
	fmt.Println()

	// Step 2: API key entry
	fmt.Println(ui.BoldStyle.Render(fmt.Sprintf("Enter your %s API key:", strings.Title(provider))))
	fmt.Print("  ")
	apiKey := readMaskedInput()
	if apiKey == "" {
		fmt.Println(ui.ErrorLine("API key cannot be empty"))
		return
	}
	fmt.Println(ui.SuccessLine("API key entered"))
	fmt.Println()

	// Step 3: Fetch models
	fmt.Println(ui.SpinnerLine("⠹", fmt.Sprintf("Fetching available models from %s...", strings.Title(provider))))
	
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	
	models, err := fetchModelsFromProvider(ctx, provider, apiKey)
	var model string
	
	if err != nil {
		fmt.Println(ui.WarningLine(fmt.Sprintf("Failed to fetch models: %v", err)))
		fmt.Println()
		fmt.Println(ui.BoldStyle.Render("Enter model name manually:"))
		fmt.Print("  ")
		model = readInput()
		if model == "" {
			fmt.Println(ui.ErrorLine("Model name cannot be empty"))
			return
		}
	} else {
		fmt.Println(ui.SuccessLine(fmt.Sprintf("Fetched %d models", len(models))))
		fmt.Println()
		
		// Step 4: Model selection
		fmt.Println(ui.BoldStyle.Render("Select Model:"))
		fmt.Println()
		
		// Mark recommended models
		displayModels := make([]string, len(models))
		for i, m := range models {
			if isRecommendedModel(provider, m) {
				displayModels[i] = "★ " + m
			} else {
				displayModels[i] = m
			}
		}
		
		selectedModel := selectFromList(displayModels)
		if selectedModel == -1 {
			fmt.Println(ui.ErrorLine("Setup cancelled"))
			return
		}
		model = models[selectedModel]
		
		fmt.Println(ui.SuccessLine(fmt.Sprintf("Selected: %s", model)))
		fmt.Println()
	}

	// Step 5: Verify connection prompt
	fmt.Println(ui.BoldStyle.Render("Verify connection with a test message?"))
	fmt.Println()
	
	verifyChoice := selectFromList([]string{"Yes", "No"})
	if verifyChoice == -1 {
		fmt.Println(ui.ErrorLine("Setup cancelled"))
		return
	}
	
	fmt.Println(ui.SuccessLine([]string{"Yes", "No"}[verifyChoice]))
	fmt.Println()

	if verifyChoice == 0 {
		// Step 6: Verify connection
		fmt.Println(ui.SpinnerLine("⠸", fmt.Sprintf("Sending ping to %s/%s...", strings.Title(provider), model)))
		
		llmClient, err := llm.NewClient(provider, model, apiKey)
		if err != nil {
			fmt.Println(ui.ErrorLine(fmt.Sprintf("Failed to create client: %v", err)))
			fmt.Println()
			
			retryChoice := selectFromList([]string{"Change model", "Re-enter API key", "Skip verification"})
			if retryChoice == 0 {
				fmt.Println(ui.InfoLine("Please restart setup to change model"))
				return
			} else if retryChoice == 1 {
				fmt.Println(ui.InfoLine("Please restart setup to re-enter API key"))
				return
			}
			// Skip verification
		} else {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			
			_, err = llmClient.Call(ctx, "You are a helpful assistant.", "ping")
			if err != nil {
				fmt.Println(ui.ErrorLine(fmt.Sprintf("Connection failed: %v", err)))
				fmt.Println()
				
				retryChoice := selectFromList([]string{"Change model", "Re-enter API key", "Skip verification"})
				if retryChoice == 0 || retryChoice == 1 {
					fmt.Println(ui.InfoLine("Please restart setup"))
					return
				}
				// Skip verification
			} else {
				fmt.Println(ui.SuccessLine("Connection verified"))
				fmt.Println()
			}
		}
	}

	// Step 7: GitHub PAT prompt
	fmt.Println(ui.BoldStyle.Render("Add GitHub Personal Access Token for private repos?"))
	fmt.Println()
	
	patChoice := selectFromList([]string{"Yes", "No"})
	if patChoice == -1 {
		fmt.Println(ui.ErrorLine("Setup cancelled"))
		return
	}
	
	fmt.Println(ui.SuccessLine([]string{"Yes", "No"}[patChoice]))
	fmt.Println()

	var githubPAT string
	if patChoice == 0 {
		fmt.Println(ui.DimStyle.Render("  Your PAT needs 'repo' scope. Generate one at:"))
		fmt.Println("  " + ui.HighlightStyle.Render("https://github.com/settings/tokens"))
		fmt.Println()
		fmt.Println(ui.BoldStyle.Render("Enter GitHub PAT:"))
		fmt.Print("  ")
		githubPAT = readMaskedInput()
		
		if githubPAT != "" && !strings.HasPrefix(githubPAT, "ghp_") && !strings.HasPrefix(githubPAT, "github_pat_") {
			fmt.Println(ui.WarningLine("Warning: PAT should start with ghp_ or github_pat_"))
			fmt.Println()
		} else if githubPAT != "" {
			fmt.Println(ui.SuccessLine("GitHub PAT entered"))
			fmt.Println()
		}
	}

	// Step 8: Save configuration
	fmt.Println(ui.SpinnerLine("⠹", "Saving configuration..."))
	
	cfg := &config.Config{
		LLMProvider: provider,
		LLMModel:    model,
		LLMAPIKey:   apiKey,
		GitHubPAT:   githubPAT,
	}

	if err := config.Save(cfg); err != nil {
		fmt.Println(ui.ErrorLine(fmt.Sprintf("Failed to save config: %v", err)))
		return
	}

	configPath, _ := config.ConfigPath()
	fmt.Println(ui.SuccessLine(fmt.Sprintf("Config written to %s", configPath)))
	fmt.Println()

	// Step 9: Completion
	completionMsg := fmt.Sprintf(
		"Chimera is ready.\n\n"+
			"Provider: %s · Model: %s\n\n"+
			"Run  chimera init <github-url>  to get started.",
		ui.HighlightStyle.Render(strings.Title(provider)),
		ui.HighlightStyle.Render(model),
	)

	box := ui.SuccessBox.Render(completionMsg)
	fmt.Println(box)
	fmt.Println()
}

// selectorModel is a simple Bubble Tea model for interactive list selection
type selectorModel struct {
	cursor int
	items  []string
	done   bool
	choice int
}

// selectFromList shows an interactive list and returns the selected index (-1 if cancelled)
func selectFromList(items []string) int {
	m := selectorModel{items: items, choice: -1}

	p := tea.NewProgram(m)
	result, err := p.Run()
	if err != nil {
		return -1
	}

	return result.(selectorModel).choice
}

// Init for the selector
func (m selectorModel) Init() tea.Cmd {
	return nil
}

// Update for the selector
func (m selectorModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			m.done = true
			m.choice = -1
			return m, tea.Quit
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.items)-1 {
				m.cursor++
			}
		case "enter":
			m.done = true
			m.choice = m.cursor
			return m, tea.Quit
		}
	}
	return m, nil
}

// View for the selector
func (m selectorModel) View() string {
	if m.done {
		return ""
	}

	var b strings.Builder
	for i, item := range m.items {
		cursor := " "
		if m.cursor == i {
			cursor = ui.PrimaryStyle.Render("●")
			item = ui.PrimaryStyle.Render(item)
		} else {
			item = ui.MutedStyle.Render(item)
		}
		b.WriteString(fmt.Sprintf("  %s %s\n", cursor, item))
	}
	b.WriteString("\n")
	b.WriteString(ui.DimStyle.Render("Use ↑/↓ to navigate, Enter to select, q to quit"))
	return b.String()
}

// readInput reads a line of input
func readInput() string {
	var input string
	fmt.Scanln(&input)
	return strings.TrimSpace(input)
}

// readMaskedInput reads input without echoing (for passwords)
func readMaskedInput() string {
	// For simplicity, we'll use a basic approach
	// In production, you'd want to use golang.org/x/term for proper masking
	var input string
	fmt.Scanln(&input)
	return strings.TrimSpace(input)
}

func fetchModelsFromProvider(ctx context.Context, provider, apiKey string) ([]string, error) {
	switch provider {
	case "openai":
		return fetchOpenAIModels(ctx, apiKey)
	case "anthropic":
		return getAnthropicModels(), nil
	case "groq":
		return fetchGroqModels(ctx, apiKey)
	case "gemini":
		return fetchGeminiModels(ctx, apiKey)
	case "ollama":
		return fetchOllamaModels(ctx)
	default:
		return nil, fmt.Errorf("unsupported provider: %s", provider)
	}
}

func fetchOpenAIModels(ctx context.Context, apiKey string) ([]string, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", "https://api.openai.com/v1/models", nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error (%d): %s", resp.StatusCode, string(body))
	}

	var result struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	// Filter to chat models only
	var models []string
	for _, m := range result.Data {
		if strings.Contains(m.ID, "gpt") && !strings.Contains(m.ID, "instruct") {
			models = append(models, m.ID)
		}
	}

	// Sort with recommended first
	sortedModels := []string{}
	recommended := []string{"gpt-4o", "gpt-4-turbo", "gpt-4", "gpt-3.5-turbo"}
	for _, r := range recommended {
		for _, m := range models {
			if m == r {
				sortedModels = append(sortedModels, m)
			}
		}
	}

	// Add remaining models
	for _, m := range models {
		found := false
		for _, s := range sortedModels {
			if s == m {
				found = true
				break
			}
		}
		if !found {
			sortedModels = append(sortedModels, m)
		}
	}

	return sortedModels, nil
}

func getAnthropicModels() []string {
	// Anthropic doesn't expose a public models endpoint
	return []string{
		"claude-sonnet-4",
		"claude-3-5-sonnet-20241022",
		"claude-3-opus-20240229",
		"claude-3-sonnet-20240229",
		"claude-3-haiku-20240307",
	}
}

func fetchGroqModels(ctx context.Context, apiKey string) ([]string, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", "https://api.groq.com/openai/v1/models", nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error (%d): %s", resp.StatusCode, string(body))
	}

	var result struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	var models []string
	for _, m := range result.Data {
		models = append(models, m.ID)
	}

	return models, nil
}

func fetchGeminiModels(ctx context.Context, apiKey string) ([]string, error) {
	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models?key=%s", apiKey)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error (%d): %s", resp.StatusCode, string(body))
	}

	var result struct {
		Models []struct {
			Name string `json:"name"`
		} `json:"models"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	var models []string
	for _, m := range result.Models {
		// Extract model ID from name (e.g., "models/gemini-pro" -> "gemini-pro")
		parts := strings.Split(m.Name, "/")
		if len(parts) > 1 {
			modelID := parts[len(parts)-1]
			// Filter to generative models only
			if strings.Contains(modelID, "gemini") && !strings.Contains(modelID, "embedding") {
				models = append(models, modelID)
			}
		}
	}

	return models, nil
}

func fetchOllamaModels(ctx context.Context) ([]string, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", "http://localhost:11434/api/tags", nil)
	if err != nil {
		return nil, err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Ollama not running or not accessible at localhost:11434")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Ollama API error (%d)", resp.StatusCode)
	}

	var result struct {
		Models []struct {
			Name string `json:"name"`
		} `json:"models"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	var models []string
	for _, m := range result.Models {
		models = append(models, m.Name)
	}

	return models, nil
}

func isRecommendedModel(provider, model string) bool {
	recommended := map[string][]string{
		"openai":    {"gpt-4o", "gpt-4-turbo"},
		"anthropic": {"claude-sonnet-4", "claude-3-5-sonnet-20241022"},
		"groq":      {"llama3-70b-8192", "mixtral-8x7b-32768"},
		"gemini":    {"gemini-1.5-pro", "gemini-pro"},
		"ollama":    {"llama2", "mistral"},
	}

	for _, r := range recommended[provider] {
		if model == r {
			return true
		}
	}
	return false
}
