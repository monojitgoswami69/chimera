package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"

	"chimera/internal/config"
	"chimera/internal/llm"
	"chimera/internal/termio"
	"chimera/internal/ui"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
)

var setupCmd = &cobra.Command{
	Use:   "setup",
	Short: "Configure your LLM provider and API key",
	Long: `Interactive wizard to configure Chimera with your LLM provider, API key, and optional GitHub PAT.

This creates ~/.chimera/config.json with your configuration (chmod 0600).

Flags:
  --force    Force reconfiguration even if config already exists

Examples:
  chimera setup
  chimera setup --force`,
	Run: func(cmd *cobra.Command, args []string) {
		if err := runSetup(); err != nil {
			fmt.Println(ui.ErrorLine(err.Error()))
			os.Exit(1)
		}
	},
}

var setupForce bool

func init() {
	setupCmd.Flags().BoolVar(&setupForce, "force", false, "Force reconfiguration even if config exists")
}

func runSetup() error {
	if !setupForce && config.Exists() {
		configPath, _ := config.ConfigPath()
		fmt.Println(ui.Header(ui.HeaderArgs{Command: "setup", Version: Version}))
		fmt.Println(ui.WarningLine("Configuration already exists"))
		fmt.Println(ui.InfoLine(fmt.Sprintf("Config file: %s", configPath)))
		fmt.Println()
		fmt.Println(ui.DimStyle.Render("To reconfigure, run:"))
		fmt.Println(ui.HighlightStyle.Render("  chimera setup --force"))
		return nil
	}

	fmt.Print(ui.Header(ui.HeaderArgs{Command: "setup", Version: Version}))

	// Step 1: provider
	fmt.Println(ui.Step(1, 5, "Choose an LLM provider"))
	providers := []string{"OpenAI", "Anthropic (Claude)", "Groq", "Google Gemini", "Ollama (local, no API key)"}
	providerKeys := []string{"openai", "anthropic", "groq", "gemini", "ollama"}
	idx := selectFromList(providers)
	if idx == -1 {
		return fmt.Errorf("setup cancelled")
	}
	provider := providerKeys[idx]
	fmt.Println(ui.SuccessLine("Selected " + providers[idx]))

	// Step 2: API key (skipped for Ollama)
	var apiKey string
	if provider != "ollama" {
		fmt.Println(ui.Step(2, 5, fmt.Sprintf("Paste your %s API key", titleCase(provider))))
		fmt.Println(ui.DimStyle.Render("  (input is hidden)"))
		fmt.Print("  ")
		key, err := termio.ReadSecret()
		if err != nil {
			return fmt.Errorf("failed to read API key: %w", err)
		}
		if strings.TrimSpace(key) == "" {
			return fmt.Errorf("API key cannot be empty")
		}
		apiKey = strings.TrimSpace(key)
		fmt.Println(ui.SuccessLine("API key recorded"))
	} else {
		fmt.Println(ui.Step(2, 5, "Ollama detected — checking local daemon"))
	}

	// Step 3: model
	fmt.Println(ui.Step(3, 5, "Pick a model"))
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	models, err := fetchModelsFromProvider(ctx, provider, apiKey)
	var model string
	if err != nil || len(models) == 0 {
		if err != nil {
			fmt.Println(ui.WarningLine(fmt.Sprintf("Could not list models automatically (%v) — enter a name manually.", err)))
		} else {
			fmt.Println(ui.WarningLine("No models returned — enter a name manually."))
		}
		fmt.Print("  Model name: ")
		model = termio.ReadLine()
		if strings.TrimSpace(model) == "" {
			return fmt.Errorf("model name cannot be empty")
		}
	} else {
		display := make([]string, len(models))
		for i, m := range models {
			if isRecommendedModel(provider, m) {
				display[i] = "★ " + m
			} else {
				display[i] = m
			}
		}
		mIdx := selectFromList(display)
		if mIdx == -1 {
			return fmt.Errorf("setup cancelled")
		}
		model = models[mIdx]
	}
	fmt.Println(ui.SuccessLine("Selected " + model))

	// Step 4: verify
	fmt.Println(ui.Step(4, 5, "Verify connection?"))
	verifyIdx := selectFromList([]string{"Yes (recommended)", "Skip"})
	if verifyIdx == -1 {
		return fmt.Errorf("setup cancelled")
	}
	if verifyIdx == 0 {
		client, cerr := llm.NewClient(provider, model, apiKey)
		if cerr != nil {
			return fmt.Errorf("could not create LLM client: %w", cerr)
		}
		vctx, vcancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer vcancel()
		_, perr := client.Call(vctx, "You are a helpful assistant. Reply with only the word pong.", "ping")
		if perr != nil {
			return fmt.Errorf("verification failed: %w", perr)
		}
		fmt.Println(ui.SuccessLine("Connection verified"))
	} else {
		fmt.Println(ui.WarningLine("Skipped verification"))
	}

	// Step 5: PAT
	fmt.Println(ui.Step(5, 5, "Add a GitHub PAT for private repos? (optional)"))
	patIdx := selectFromList([]string{"No", "Yes"})
	if patIdx == -1 {
		return fmt.Errorf("setup cancelled")
	}
	var pat string
	if patIdx == 1 {
		fmt.Println(ui.DimStyle.Render("  Needs `repo` scope. Create one at https://github.com/settings/tokens"))
		fmt.Print("  PAT: ")
		key, err := termio.ReadSecret()
		if err != nil {
			return fmt.Errorf("failed to read PAT: %w", err)
		}
		pat = strings.TrimSpace(key)
		if pat != "" && !strings.HasPrefix(pat, "ghp_") && !strings.HasPrefix(pat, "github_pat_") {
			fmt.Println(ui.WarningLine("PAT does not start with ghp_ or github_pat_ — saving anyway"))
		}
	}

	cfg := &config.Config{
		LLMProvider: provider,
		LLMModel:    model,
		LLMAPIKey:   apiKey,
		GitHubPAT:   pat,
	}
	if err := config.Save(cfg); err != nil {
		return fmt.Errorf("could not save config: %w", err)
	}
	cfgPath, _ := config.ConfigPath()

	msg := fmt.Sprintf(
		"Chimera is ready.\n\n  Provider · %s\n  Model    · %s\n  Config   · %s\n\nTry:\n  %s",
		ui.HighlightStyle.Render(titleCase(provider)),
		ui.HighlightStyle.Render(model),
		ui.DimStyle.Render(cfgPath),
		ui.HighlightStyle.Render("chimera init <github-url>"),
	)
	fmt.Println()
	fmt.Println(ui.SuccessBox.Render(msg))
	fmt.Println()
	return nil
}

func titleCase(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

// selectorModel is a simple Bubble Tea model for interactive list selection.
type selectorModel struct {
	cursor int
	items  []string
	done   bool
	choice int
}

func selectFromList(items []string) int {
	m := selectorModel{items: items, choice: -1}
	p := tea.NewProgram(m)
	result, err := p.Run()
	if err != nil {
		return -1
	}
	return result.(selectorModel).choice
}

func (m selectorModel) Init() tea.Cmd { return nil }

func (m selectorModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q", "esc":
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
	}
	return nil, fmt.Errorf("unsupported provider: %s", provider)
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
	var chat []string
	for _, m := range result.Data {
		if strings.Contains(m.ID, "gpt") && !strings.Contains(m.ID, "instruct") {
			chat = append(chat, m.ID)
		}
	}
	sort.Strings(chat)
	preferred := []string{"gpt-4o", "gpt-4o-mini", "gpt-4-turbo", "gpt-4", "gpt-3.5-turbo"}
	return sortRecommendedFirst(chat, preferred), nil
}

func getAnthropicModels() []string {
	return []string{
		"claude-opus-4-7",
		"claude-sonnet-4-6",
		"claude-haiku-4-5",
		"claude-3-5-sonnet-20241022",
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
	sort.Strings(models)
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
		parts := strings.Split(m.Name, "/")
		if len(parts) > 1 {
			id := parts[len(parts)-1]
			if strings.Contains(id, "gemini") && !strings.Contains(id, "embedding") {
				models = append(models, id)
			}
		}
	}
	sort.Strings(models)
	return models, nil
}

func fetchOllamaModels(ctx context.Context) ([]string, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", "http://localhost:11434/api/tags", nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ollama not running on localhost:11434")
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ollama API error (%d)", resp.StatusCode)
	}
	var result struct {
		Models []struct {
			Name string `json:"name"`
		} `json:"models"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	var names []string
	for _, m := range result.Models {
		names = append(names, m.Name)
	}
	sort.Strings(names)
	return names, nil
}

func sortRecommendedFirst(models, preferred []string) []string {
	pref := map[string]int{}
	for i, p := range preferred {
		pref[p] = i + 1
	}
	sort.SliceStable(models, func(i, j int) bool {
		pi, oi := pref[models[i]], pref[models[j]]
		if pi != 0 && oi != 0 {
			return pi < oi
		}
		if pi != 0 {
			return true
		}
		if oi != 0 {
			return false
		}
		return models[i] < models[j]
	})
	return models
}

func isRecommendedModel(provider, model string) bool {
	rec := map[string][]string{
		"openai":    {"gpt-4o", "gpt-4o-mini"},
		"anthropic": {"claude-opus-4-7", "claude-sonnet-4-6"},
		"groq":      {"llama3-70b-8192", "mixtral-8x7b-32768"},
		"gemini":    {"gemini-1.5-pro", "gemini-1.5-flash"},
		"ollama":    {"llama3", "mistral"},
	}
	for _, r := range rec[provider] {
		if model == r {
			return true
		}
	}
	return false
}
