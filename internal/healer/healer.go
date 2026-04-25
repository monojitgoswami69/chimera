// Package healer provides AI-powered self-healing diagnostics for failed containers.
package healer

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/projectchimera/chimera/internal/docker"
)

// DiagRequest holds sanitized data sent to the LLM.
type DiagRequest struct {
	Logs          string
	ComposeYAML   string
	EnvVarNames   []string
	ContainerName string
}

// LLMProvider is the interface for AI diagnosis backends.
type LLMProvider interface {
	Diagnose(ctx context.Context, req DiagRequest) (string, error)
}

// Watch subscribes to Docker events and calls onFailure for non-zero exit containers.
func Watch(ctx context.Context, cli docker.DockerAPI, projectName string, onFailure func(containerID string)) {
	filterArgs := filters.NewArgs(
		filters.Arg("type", "container"),
		filters.Arg("event", "die"),
		filters.Arg("label", "com.chimera.project="+projectName),
	)

	msgCh, errCh := cli.Events(ctx, types.EventsOptions{Filters: filterArgs})

	for {
		select {
		case <-ctx.Done():
			return
		case err := <-errCh:
			if err != nil {
				return
			}
		case msg := <-msgCh:
			if msg.Action == "die" {
				exitCode, ok := msg.Actor.Attributes["exitCode"]
				if ok && exitCode != "0" {
					go onFailure(msg.Actor.ID)
				}
			}
		}
	}
}

// CaptureLogs retrieves the last 100 lines of a container's logs.
func CaptureLogs(ctx context.Context, cli docker.DockerAPI, containerID string) (string, error) {
	reader, err := cli.ContainerLogs(ctx, containerID, types.ContainerLogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Tail:       "100",
	})
	if err != nil {
		return "", fmt.Errorf("healer: failed to capture logs for %s: %w", containerID, err)
	}
	defer reader.Close()

	// Read all data first
	rawData, err := io.ReadAll(reader)
	if err != nil {
		return "", fmt.Errorf("healer: failed to read logs: %w", err)
	}

	// Try stdcopy demux for multiplexed Docker logs
	var stdout, stderr bytes.Buffer
	_, demuxErr := stdcopy.StdCopy(&stdout, &stderr, bytes.NewReader(rawData))
	if demuxErr != nil || (stdout.Len() == 0 && stderr.Len() == 0) {
		// Fallback: return raw data if demux fails or yields nothing
		return string(rawData), nil
	}

	combined := stdout.String() + stderr.String()
	return combined, nil
}

// scrubRegex matches secret-like values in compose YAML and env content.
var scrubRegex = regexp.MustCompile(`(?i)(password|secret|key|token|auth|credential|api_key|private)[=:]\s*["']?([^\s"']+)["']?`)

// ScrubSecrets removes sensitive values from env var names and compose YAML.
// Returns only variable names (never values) and a sanitized compose string.
func ScrubSecrets(envVarNames []string, composeYAML string) ([]string, string) {
	// env var names are already safe (no values) — return as-is
	safeVars := make([]string, len(envVarNames))
	copy(safeVars, envVarNames)

	// Scrub compose YAML
	safeCompose := scrubRegex.ReplaceAllString(composeYAML, "${1}=***REDACTED***")

	return safeVars, safeCompose
}

const systemPrompt = `You are an expert DevOps engineer diagnosing a failed Docker container. Analyze the logs and configuration provided. Identify the root cause in one sentence. Then provide 1-3 concrete, actionable fix steps the developer can take. Be specific about file names, env variables, and commands. Never suggest vague steps like 'check the configuration'.`

func buildUserPrompt(req DiagRequest) string {
	return fmt.Sprintf(`Container '%s' exited unexpectedly.

Last 100 log lines:
%s

Docker Compose config (sanitized):
%s

Detected env var names (no values):
%s`, req.ContainerName, req.Logs, req.ComposeYAML, strings.Join(req.EnvVarNames, ", "))
}

// NewProvider creates an LLMProvider based on CHIMERA_LLM_PROVIDER env var.
func NewProvider(ctx context.Context) (LLMProvider, error) {
	provider := os.Getenv("CHIMERA_LLM_PROVIDER")
	if provider == "" {
		provider = "openai"
	}

	switch strings.ToLower(provider) {
	case "openai":
		key := os.Getenv("OPENAI_API_KEY")
		if key == "" {
			return nil, fmt.Errorf("healer: OPENAI_API_KEY not set (required for AI diagnostics)")
		}
		return &openaiProvider{apiKey: key}, nil
	case "gemini":
		key := os.Getenv("GEMINI_API_KEY")
		if key == "" {
			return nil, fmt.Errorf("healer: GEMINI_API_KEY not set")
		}
		return &geminiProvider{apiKey: key}, nil
	case "groq":
		key := os.Getenv("GROQ_API_KEY")
		if key == "" {
			return nil, fmt.Errorf("healer: GROQ_API_KEY not set")
		}
		return &groqProvider{apiKey: key}, nil
	default:
		return nil, fmt.Errorf("healer: unsupported LLM provider %q (use openai, gemini, or groq)", provider)
	}
}

// --- OpenAI Provider ---

type openaiProvider struct{ apiKey string }

func (p *openaiProvider) Diagnose(ctx context.Context, req DiagRequest) (string, error) {
	return chatComplete(ctx, "https://api.openai.com/v1/chat/completions", p.apiKey, "gpt-4o-mini", req)
}

// --- Gemini Provider ---

type geminiProvider struct{ apiKey string }

func (p *geminiProvider) Diagnose(ctx context.Context, req DiagRequest) (string, error) {
	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/gemini-2.0-flash:generateContent?key=%s", p.apiKey)
	body := map[string]interface{}{
		"contents": []map[string]interface{}{
			{"role": "user", "parts": []map[string]string{{"text": systemPrompt + "\n\n" + buildUserPrompt(req)}}},
		},
	}
	data, _ := json.Marshal(body)
	httpReq, _ := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(data))
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("healer: gemini request failed: %w", err)
	}
	defer resp.Body.Close()

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
		return "", fmt.Errorf("healer: failed to parse gemini response: %w", err)
	}
	if len(result.Candidates) > 0 && len(result.Candidates[0].Content.Parts) > 0 {
		return result.Candidates[0].Content.Parts[0].Text, nil
	}
	return "No diagnosis available from Gemini.", nil
}

// --- Groq Provider ---

type groqProvider struct{ apiKey string }

func (p *groqProvider) Diagnose(ctx context.Context, req DiagRequest) (string, error) {
	return chatComplete(ctx, "https://api.groq.com/openai/v1/chat/completions", p.apiKey, "llama-3.1-70b-versatile", req)
}

// chatComplete handles OpenAI-compatible chat completion APIs (OpenAI + Groq).
func chatComplete(ctx context.Context, endpoint, apiKey, model string, req DiagRequest) (string, error) {
	body := map[string]interface{}{
		"model": model,
		"messages": []map[string]string{
			{"role": "system", "content": systemPrompt},
			{"role": "user", "content": buildUserPrompt(req)},
		},
		"max_tokens":  1024,
		"temperature": 0.3,
	}

	data, _ := json.Marshal(body)
	httpReq, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewReader(data))
	if err != nil {
		return "", fmt.Errorf("healer: failed to create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("healer: LLM request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("healer: LLM returned status %d: %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("healer: failed to parse LLM response: %w", err)
	}

	if len(result.Choices) > 0 {
		return result.Choices[0].Message.Content, nil
	}
	return "No diagnosis available.", nil
}

// RenderDiagnosis prints a styled diagnosis box to the writer.
func RenderDiagnosis(diagnosis string, containerName string, w io.Writer) {
	titleStyle := lipgloss.NewStyle().
		Background(lipgloss.Color("196")).
		Foreground(lipgloss.Color("15")).
		Bold(true).
		Padding(0, 1)

	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("196")).
		Padding(1, 2).
		Width(80)

	footerStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("8")).
		Italic(true)

	title := titleStyle.Render(fmt.Sprintf(" Chimera Diagnosis — %s ", containerName))
	body := boxStyle.Render(diagnosis)
	footer := footerStyle.Render("Run chimera diagnose to re-trigger · chimera nuke to restart")

	fmt.Fprintln(w)
	fmt.Fprintln(w, title)
	fmt.Fprintln(w, body)
	fmt.Fprintln(w, footer)
	fmt.Fprintln(w)
}
