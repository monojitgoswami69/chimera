package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Client represents an LLM client
type Client struct {
	Provider string
	Model    string
	APIKey   string
	Endpoint string
}

// NewClient creates a new LLM client
func NewClient(provider, model, apiKey string) (*Client, error) {
	client := &Client{
		Provider: strings.ToLower(provider),
		Model:    model,
		APIKey:   apiKey,
	}

	switch client.Provider {
	case "openai":
		client.Endpoint = "https://api.openai.com/v1/chat/completions"
	case "anthropic":
		client.Endpoint = "https://api.anthropic.com/v1/messages"
	case "groq":
		client.Endpoint = "https://api.groq.com/openai/v1/chat/completions"
	case "gemini":
		client.Endpoint = fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent", model)
	case "ollama":
		client.Endpoint = "http://localhost:11434/api/generate"
	default:
		return nil, fmt.Errorf("unsupported provider: %s", provider)
	}

	return client, nil
}

// Call makes an LLM API call
func (c *Client) Call(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	switch c.Provider {
	case "openai", "groq":
		return c.callOpenAICompatible(ctx, systemPrompt, userPrompt)
	case "anthropic":
		return c.callAnthropic(ctx, systemPrompt, userPrompt)
	case "gemini":
		return c.callGemini(ctx, systemPrompt, userPrompt)
	case "ollama":
		return c.callOllama(ctx, systemPrompt, userPrompt)
	default:
		return "", fmt.Errorf("unsupported provider: %s", c.Provider)
	}
}

func (c *Client) callOllama(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	body := map[string]interface{}{
		"model":  c.Model,
		"prompt": systemPrompt + "\n\n" + userPrompt,
		"stream": false,
		"options": map[string]interface{}{
			"temperature": 0.1,
		},
	}
	data, _ := json.Marshal(body)
	req, err := http.NewRequestWithContext(ctx, "POST", c.Endpoint, bytes.NewReader(data))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("Ollama error (%d): %s", resp.StatusCode, string(body))
	}
	var result struct {
		Response string `json:"response"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}
	return result.Response, nil
}

func (c *Client) callOpenAICompatible(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	body := map[string]interface{}{
		"model": c.Model,
		"messages": []map[string]string{
			{"role": "system", "content": systemPrompt},
			{"role": "user", "content": userPrompt},
		},
		"temperature": 0.1,
	}

	data, _ := json.Marshal(body)
	req, err := http.NewRequestWithContext(ctx, "POST", c.Endpoint, bytes.NewReader(data))
	if err != nil {
		return "", err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.APIKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("API error (%d): %s", resp.StatusCode, string(body))
	}

	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	if len(result.Choices) == 0 {
		return "", fmt.Errorf("no response from LLM")
	}

	return result.Choices[0].Message.Content, nil
}

func (c *Client) callAnthropic(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	body := map[string]interface{}{
		"model": c.Model,
		"messages": []map[string]string{
			{"role": "user", "content": userPrompt},
		},
		"system":      systemPrompt,
		"max_tokens":  4096,
		"temperature": 0.1,
	}

	data, _ := json.Marshal(body)
	req, err := http.NewRequestWithContext(ctx, "POST", c.Endpoint, bytes.NewReader(data))
	if err != nil {
		return "", err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", c.APIKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("API error (%d): %s", resp.StatusCode, string(body))
	}

	var result struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	if len(result.Content) == 0 {
		return "", fmt.Errorf("no response from Anthropic")
	}

	return result.Content[0].Text, nil
}

func (c *Client) callGemini(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	body := map[string]interface{}{
		"contents": []map[string]interface{}{
			{
				"parts": []map[string]string{
					{"text": systemPrompt + "\n\n" + userPrompt},
				},
			},
		},
		"generationConfig": map[string]interface{}{
			"temperature": 0.1,
		},
	}

	data, _ := json.Marshal(body)
	endpoint := c.Endpoint + "?key=" + c.APIKey
	req, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewReader(data))
	if err != nil {
		return "", err
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("API error (%d): %s", resp.StatusCode, string(body))
	}

	var result struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	if len(result.Candidates) == 0 || len(result.Candidates[0].Content.Parts) == 0 {
		return "", fmt.Errorf("no response from Gemini")
	}

	return result.Candidates[0].Content.Parts[0].Text, nil
}

// ValidationResponse represents LLM validation response
type ValidationResponse struct {
	Valid            bool                     `json:"valid"`
	NeedsFiles       []string                 `json:"needs_files,omitempty"`
	CorrectedServices []CorrectedService      `json:"corrected_services,omitempty"`
	CorrectedDockerfile string                `json:"corrected_dockerfile,omitempty"`
	CorrectedCompose    string                `json:"corrected_compose,omitempty"`
	CorrectedEnvVars    []CorrectedEnvResult  `json:"corrected_env_vars,omitempty"`
}

// CorrectedService represents a corrected service detection
type CorrectedService struct {
	Type       string  `json:"type"`        // "frontend" or "backend"
	Directory  string  `json:"directory"`   // relative path from root
	Framework  string  `json:"framework"`   // e.g., "next", "fastapi"
	Confidence float64 `json:"confidence"`  // 0.0 to 1.0
}

// CorrectedEnvResult represents corrected env vars for a service
type CorrectedEnvResult struct {
	Directory   string            `json:"directory"`
	ServiceType string            `json:"service_type"`
	Technology  string            `json:"technology"`
	EnvContent  string            `json:"env_content"` // Full .env.example content with comments
}

// FileChunkResponse represents LLM response to file chunk
type FileChunkResponse struct {
	Filename   string `json:"filename"`
	Sufficient bool   `json:"sufficient"`
}

// ValidateDetection validates service detection with file request loop
func (c *Client) ValidateDetection(ctx context.Context, tree, analysis string, fileReader func(string) (string, error)) (*ValidationResponse, error) {
	systemPrompt := `You are a senior DevOps engineer validating static analysis of a code repository.

Your response must be valid JSON in one of these formats:

Format A (need files to validate):
{
  "valid": false,
  "needs_files": ["path/to/file1", "path/to/file2"]
}

Format B (validation passed):
{
  "valid": true
}

Format C (corrections needed):
{
  "valid": false,
  "corrected_services": [
    {
      "type": "frontend|backend",
      "directory": "relative/path",
      "framework": "next|express|node|fastapi|flask|django",
      "confidence": 0.95
    }
  ]
}

RULES:
- Request files if you need to see actual code to validate
- Maximum 5 files per request
- Only request files that materially affect the outcome
- Respond ONLY with valid JSON, no markdown, no backticks`

	userPrompt := fmt.Sprintf(`Validate this static analysis:

TREE:
%s

ANALYSIS:
%s`, tree, analysis)

	return c.runValidationLoop(ctx, systemPrompt, userPrompt, fileReader, 3)
}

// ValidateDockerfiles validates generated Docker configurations
func (c *Client) ValidateDockerfiles(ctx context.Context, tree, dockerfile, compose string, fileReader func(string) (string, error)) (*ValidationResponse, error) {
	systemPrompt := `You are a senior DevOps engineer validating generated Dockerfile and docker-compose.yml.

Your response must be valid JSON in one of these formats:

Format A (need files):
{
  "valid": false,
  "needs_files": ["path/to/file"]
}

Format B (valid):
{
  "valid": true
}

Format C (corrections):
{
  "valid": false,
  "corrected_dockerfile": "...",
  "corrected_compose": "..."
}

Respond ONLY with valid JSON, no markdown, no backticks.`

	userPrompt := fmt.Sprintf(`Validate these Docker configurations:

TREE:
%s

DOCKERFILE:
%s

COMPOSE:
%s`, tree, dockerfile, compose)

	return c.runValidationLoop(ctx, systemPrompt, userPrompt, fileReader, 5)
}

// ValidateEnvVars validates and enhances environment variable detection
func (c *Client) ValidateEnvVars(ctx context.Context, tree, envAnalysis, existingEnvFiles string, fileReader func(string) (string, error)) (*ValidationResponse, error) {
	systemPrompt := `You are a senior DevOps engineer creating .env.example files.

Your response must be valid JSON in one of these formats:

Format A (need files):
{
  "valid": false,
  "needs_files": ["path/to/file"]
}

Format B (corrections with full .env.example content):
{
  "valid": false,
  "corrected_env_vars": [
    {
      "directory": "path",
      "service_type": "frontend|backend",
      "technology": "next|fastapi|...",
      "env_content": "# Database\nDATABASE_URL=postgresql://...\n\n# Auth\nSECRET_KEY=..."
    }
  ]
}

RULES:
- Create complete .env.example files with proper comments
- Group related variables with comment headers
- Provide example values that show the expected format
- Include all detected variables plus any common ones for the technology
- Respond ONLY with valid JSON, no markdown, no backticks`

	userPrompt := fmt.Sprintf(`Create .env.example files for these services:

TREE:
%s

DETECTED VARS:
%s

EXISTING ENV FILES:
%s

Generate complete, well-commented .env.example files.`, tree, envAnalysis, existingEnvFiles)

	return c.runValidationLoop(ctx, systemPrompt, userPrompt, fileReader, 3)
}

// runValidationLoop implements the file request/response loop
func (c *Client) runValidationLoop(ctx context.Context, systemPrompt, userPrompt string, fileReader func(string) (string, error), maxAttempts int) (*ValidationResponse, error) {
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		response, err := c.Call(ctx, systemPrompt, userPrompt)
		if err != nil {
			return nil, err
		}

		// Robust JSON extraction: many models (Groq Llama, Gemini, Anthropic)
		// wrap their response in prose or a Markdown fence. We pull out the
		// first balanced {...} block instead of trusting the exact format.
		jsonBlob := extractJSON(response)

		var validationResp ValidationResponse
		if err := json.Unmarshal([]byte(jsonBlob), &validationResp); err != nil {
			if attempt < maxAttempts {
				userPrompt += "\n\nYour previous response could not be parsed as JSON. Respond with a single JSON object only, no prose, no markdown, no backticks. Start with { and end with }."
				continue
			}
			return nil, fmt.Errorf("failed to parse LLM response as JSON after %d attempts: %w\nlast response: %s", maxAttempts, err, truncateForLog(response, 400))
		}

		// If valid or has corrections, return
		if validationResp.Valid || len(validationResp.CorrectedServices) > 0 ||
			validationResp.CorrectedDockerfile != "" || validationResp.CorrectedCompose != "" ||
			len(validationResp.CorrectedEnvVars) > 0 {
			return &validationResp, nil
		}

		// If needs files, send them
		if len(validationResp.NeedsFiles) > 0 && fileReader != nil {
			// Cap at 5 files per round
			needed := validationResp.NeedsFiles
			if len(needed) > 5 {
				needed = needed[:5]
			}
			var filesSection strings.Builder
			filesSection.WriteString("\n\nREQUESTED FILES:\n")
			for _, filePath := range needed {
				content, err := fileReader(filePath)
				if err != nil {
					filesSection.WriteString(fmt.Sprintf("\n=== %s ===\n[unavailable: %v]\n", filePath, err))
					continue
				}
				// Cap individual file size in the prompt.
				if len(content) > 16*1024 {
					content = content[:16*1024] + "\n[...truncated...]\n"
				}
				filesSection.WriteString(fmt.Sprintf("\n=== %s ===\n%s\n", filePath, content))
			}
			userPrompt += filesSection.String()
			continue
		}

		// No progress made — the LLM returned {"valid": false} with neither
		// corrections nor file requests. Treat this as "no opinion" rather
		// than an error.
		return &ValidationResponse{Valid: true}, nil
	}

	return nil, fmt.Errorf("LLM did not converge after %d rounds (keeping static output)", maxAttempts)
}

// extractJSON pulls the first balanced JSON object out of s. It tolerates
// Markdown fences, prose before/after, and partial code blocks.
func extractJSON(s string) string {
	s = strings.TrimSpace(s)
	// Strip a leading ```json or ``` fence if present.
	if strings.HasPrefix(s, "```") {
		if nl := strings.IndexByte(s, '\n'); nl >= 0 {
			s = s[nl+1:]
		}
		if idx := strings.LastIndex(s, "```"); idx >= 0 {
			s = s[:idx]
		}
	}
	// Find the first { and match braces, respecting strings.
	start := strings.IndexByte(s, '{')
	if start < 0 {
		return s
	}
	depth := 0
	inStr := false
	esc := false
	for i := start; i < len(s); i++ {
		ch := s[i]
		if esc {
			esc = false
			continue
		}
		if ch == '\\' && inStr {
			esc = true
			continue
		}
		if ch == '"' {
			inStr = !inStr
			continue
		}
		if inStr {
			continue
		}
		if ch == '{' {
			depth++
		} else if ch == '}' {
			depth--
			if depth == 0 {
				return s[start : i+1]
			}
		}
	}
	return s[start:]
}

func truncateForLog(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "...[truncated]"
}
