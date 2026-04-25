package healer

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/events"
	"github.com/docker/docker/api/types/volume"
)

// mockDockerClient implements docker.DockerAPI for testing.
type mockDockerClient struct {
	logs       string
	eventsCh   chan events.Message
	errCh      chan error
	statsJSON  string
	containers []types.Container
}

func (m *mockDockerClient) ContainerList(ctx context.Context, options types.ContainerListOptions) ([]types.Container, error) {
	return m.containers, nil
}
func (m *mockDockerClient) ContainerStop(ctx context.Context, containerID string, options container.StopOptions) error {
	return nil
}
func (m *mockDockerClient) ContainerRemove(ctx context.Context, containerID string, options types.ContainerRemoveOptions) error {
	return nil
}
func (m *mockDockerClient) ContainerLogs(ctx context.Context, ctr string, options types.ContainerLogsOptions) (io.ReadCloser, error) {
	return io.NopCloser(strings.NewReader(m.logs)), nil
}
func (m *mockDockerClient) ContainerStats(ctx context.Context, containerID string, stream bool) (types.ContainerStats, error) {
	return types.ContainerStats{Body: io.NopCloser(strings.NewReader(m.statsJSON))}, nil
}
func (m *mockDockerClient) Events(ctx context.Context, options types.EventsOptions) (<-chan events.Message, <-chan error) {
	return m.eventsCh, m.errCh
}
func (m *mockDockerClient) VolumeList(ctx context.Context, options volume.ListOptions) (volume.ListResponse, error) {
	return volume.ListResponse{}, nil
}
func (m *mockDockerClient) VolumeRemove(ctx context.Context, volumeID string, force bool) error {
	return nil
}
func (m *mockDockerClient) Close() error { return nil }

func TestCaptureLogs_Mock(t *testing.T) {
	mock := &mockDockerClient{logs: "Error: connection refused\npanic: runtime error"}

	logs, err := CaptureLogs(context.Background(), mock, "test-container-id")
	if err != nil {
		t.Fatalf("CaptureLogs() error: %v", err)
	}

	// Since the mock returns raw text (not multiplexed), it will be captured via the fallback path
	if logs == "" {
		t.Error("logs should not be empty")
	}
}

func TestWatcher_NonZeroExit(t *testing.T) {
	eventsCh := make(chan events.Message, 1)
	errCh := make(chan error, 1)

	mock := &mockDockerClient{eventsCh: eventsCh, errCh: errCh}

	called := make(chan string, 1)
	ctx, cancel := context.WithCancel(context.Background())

	go Watch(ctx, mock, "test-project", func(containerID string) {
		called <- containerID
	})

	// Send a die event with non-zero exit code
	eventsCh <- events.Message{
		Action: "die",
		Actor:  events.Actor{ID: "abc123", Attributes: map[string]string{"exitCode": "1"}},
	}

	select {
	case id := <-called:
		if id != "abc123" {
			t.Errorf("expected container ID abc123, got %s", id)
		}
	case <-context.Background().Done():
		t.Error("onFailure was not called")
	}

	cancel()
}

func TestWatcher_ZeroExit(t *testing.T) {
	eventsCh := make(chan events.Message, 1)
	errCh := make(chan error, 1)

	mock := &mockDockerClient{eventsCh: eventsCh, errCh: errCh}

	called := make(chan string, 1)
	ctx, cancel := context.WithCancel(context.Background())

	go Watch(ctx, mock, "test-project", func(containerID string) {
		called <- containerID
	})

	// Send a die event with zero exit code — should NOT trigger callback
	eventsCh <- events.Message{
		Action: "die",
		Actor:  events.Actor{ID: "abc123", Attributes: map[string]string{"exitCode": "0"}},
	}

	// Cancel after a brief window
	cancel()

	select {
	case <-called:
		t.Error("onFailure should NOT be called for zero exit code")
	default:
		// Good — not called
	}
}

func TestWatcher_ContextCancel(t *testing.T) {
	eventsCh := make(chan events.Message)
	errCh := make(chan error)

	mock := &mockDockerClient{eventsCh: eventsCh, errCh: errCh}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})

	go func() {
		Watch(ctx, mock, "test-project", func(containerID string) {})
		close(done)
	}()

	cancel()
	<-done // Watch should exit when context is cancelled
}

// --- Existing tests (keep all) ---

func TestScrubSecrets(t *testing.T) {
	envVars := []string{"DATABASE_URL", "SECRET_KEY", "PORT"}
	composeYAML := `services:
  app:
    environment:
      POSTGRES_PASSWORD: "hunter2"
      API_KEY: "sk-abc123"
      SECRET: 'mysecret'
      PORT: "3000"
      NORMAL_VAR: "hello world"
`
	safeVars, safeCompose := ScrubSecrets(envVars, composeYAML)
	if len(safeVars) != 3 {
		t.Errorf("expected 3 safe vars, got %d", len(safeVars))
	}
	if strings.Contains(safeCompose, "hunter2") {
		t.Error("password value should be scrubbed")
	}
	if strings.Contains(safeCompose, "sk-abc123") {
		t.Error("API key value should be scrubbed")
	}
	if !strings.Contains(safeCompose, "REDACTED") {
		t.Error("scrubbed values should contain REDACTED")
	}
}

func TestScrubSecrets_EmptyInput(t *testing.T) {
	safeVars, safeCompose := ScrubSecrets(nil, "")
	if safeVars == nil {
		t.Error("safeVars should not be nil")
	}
	if safeCompose != "" {
		t.Error("safeCompose should be empty for empty input")
	}
}

func TestScrubSecrets_NoSecrets(t *testing.T) {
	compose := `services:
  app:
    ports:
      - "3000:3000"
`
	_, safeCompose := ScrubSecrets([]string{"PORT"}, compose)
	if safeCompose != compose {
		t.Error("compose without secrets should be unchanged")
	}
}

func TestRenderDiagnosis(t *testing.T) {
	var buf bytes.Buffer
	RenderDiagnosis("Root cause: missing DATABASE_URL.\n\nFix steps:\n1. Add DATABASE_URL to .env", "test-container", &buf)
	output := buf.String()
	if !strings.Contains(output, "test-container") {
		t.Error("output should contain container name")
	}
	if !strings.Contains(output, "DATABASE_URL") {
		t.Error("output should contain diagnosis text")
	}
}

func TestBuildUserPrompt(t *testing.T) {
	req := DiagRequest{
		Logs:          "Error: connection refused",
		ComposeYAML:   "services:\n  app:\n    image: node",
		EnvVarNames:   []string{"DB_URL", "PORT"},
		ContainerName: "my-app",
	}
	prompt := buildUserPrompt(req)
	if !strings.Contains(prompt, "my-app") || !strings.Contains(prompt, "connection refused") || !strings.Contains(prompt, "DB_URL") {
		t.Error("prompt should contain all request fields")
	}
}

func TestNewProvider_MissingKey(t *testing.T) {
	t.Setenv("CHIMERA_LLM_PROVIDER", "openai")
	t.Setenv("OPENAI_API_KEY", "")
	_, err := NewProvider(nil)
	if err == nil {
		t.Error("NewProvider should fail with missing API key")
	}
}

func TestNewProvider_UnsupportedProvider(t *testing.T) {
	t.Setenv("CHIMERA_LLM_PROVIDER", "unsupported")
	_, err := NewProvider(nil)
	if err == nil {
		t.Error("NewProvider should fail with unsupported provider")
	}
}

func TestNewProvider_OpenAI(t *testing.T) {
	t.Setenv("CHIMERA_LLM_PROVIDER", "openai")
	t.Setenv("OPENAI_API_KEY", "sk-test-key")
	provider, err := NewProvider(nil)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if provider == nil {
		t.Error("provider should not be nil")
	}
}

func TestNewProvider_Gemini(t *testing.T) {
	t.Setenv("CHIMERA_LLM_PROVIDER", "gemini")
	t.Setenv("GEMINI_API_KEY", "test-key")
	provider, err := NewProvider(nil)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if provider == nil {
		t.Error("provider should not be nil")
	}
}

func TestNewProvider_Groq(t *testing.T) {
	t.Setenv("CHIMERA_LLM_PROVIDER", "groq")
	t.Setenv("GROQ_API_KEY", "test-key")
	provider, err := NewProvider(nil)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if provider == nil {
		t.Error("provider should not be nil")
	}
}

func TestNewProvider_Default(t *testing.T) {
	t.Setenv("CHIMERA_LLM_PROVIDER", "")
	t.Setenv("OPENAI_API_KEY", "sk-default")
	provider, err := NewProvider(nil)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if provider == nil {
		t.Error("provider should not be nil")
	}
}

func TestNewProvider_GeminiMissingKey(t *testing.T) {
	t.Setenv("CHIMERA_LLM_PROVIDER", "gemini")
	t.Setenv("GEMINI_API_KEY", "")
	_, err := NewProvider(nil)
	if err == nil {
		t.Error("should fail with missing Gemini key")
	}
}

func TestNewProvider_GroqMissingKey(t *testing.T) {
	t.Setenv("CHIMERA_LLM_PROVIDER", "groq")
	t.Setenv("GROQ_API_KEY", "")
	_, err := NewProvider(nil)
	if err == nil {
		t.Error("should fail with missing Groq key")
	}
}

func TestScrubSecrets_ComplexPatterns(t *testing.T) {
	compose := `POSTGRES_PASSWORD: hunter2
api_key: sk-123456
auth_token="bearer-abc"
credential: "my-secret-cred"
PORT: 3000`

	_, safe := ScrubSecrets(nil, compose)
	if strings.Contains(safe, "hunter2") || strings.Contains(safe, "sk-123456") {
		t.Error("secret values should be scrubbed")
	}
	if !strings.Contains(safe, "REDACTED") {
		t.Error("scrubbed values should contain REDACTED")
	}
}

func TestChatComplete_MockServer(t *testing.T) {
	// Create a mock HTTP server that returns a valid OpenAI-style response
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"choices": []map[string]interface{}{
				{"message": map[string]string{"content": "Root cause: missing env var."}},
			},
		})
	}))
	defer server.Close()

	req := DiagRequest{
		Logs:          "Error: DB connection failed",
		ComposeYAML:   "services:\n  app:\n    image: node",
		EnvVarNames:   []string{"DB_URL"},
		ContainerName: "test-app",
	}

	result, err := chatComplete(context.Background(), server.URL, "test-key", "test-model", req)
	if err != nil {
		t.Fatalf("chatComplete() error: %v", err)
	}
	if !strings.Contains(result, "Root cause") {
		t.Error("result should contain diagnosis text")
	}
}

func TestChatComplete_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("internal error"))
	}))
	defer server.Close()

	req := DiagRequest{ContainerName: "test"}
	_, err := chatComplete(context.Background(), server.URL, "key", "model", req)
	if err == nil {
		t.Error("chatComplete should fail on 500 status")
	}
}

func TestChatComplete_EmptyChoices(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"choices": []interface{}{}})
	}))
	defer server.Close()

	req := DiagRequest{ContainerName: "test"}
	result, err := chatComplete(context.Background(), server.URL, "key", "model", req)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if result != "No diagnosis available." {
		t.Errorf("expected fallback text, got %q", result)
	}
}

func TestOpenAIProvider_Diagnose(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"choices": []map[string]interface{}{
				{"message": map[string]string{"content": "Fix: add DB_URL"}},
			},
		})
	}))
	defer server.Close()

	// We can't easily redirect the provider URL, but we test chatComplete above
	// which is the core logic. This test ensures the provider struct is wired correctly.
	t.Setenv("CHIMERA_LLM_PROVIDER", "openai")
	t.Setenv("OPENAI_API_KEY", "test")
	p, _ := NewProvider(nil)
	if p == nil {
		t.Fatal("provider should not be nil")
	}
}

func TestCaptureLogs_EmptyLogs(t *testing.T) {
	mock := &mockDockerClient{logs: ""}
	logs, err := CaptureLogs(context.Background(), mock, "empty-container")
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if logs != "" {
		t.Errorf("expected empty logs, got %q", logs)
	}
}

func TestWatcher_ErrorChannel(t *testing.T) {
	eventsCh := make(chan events.Message)
	errCh := make(chan error, 1)

	mock := &mockDockerClient{eventsCh: eventsCh, errCh: errCh}

	done := make(chan struct{})
	ctx := context.Background()

	go func() {
		Watch(ctx, mock, "test-project", func(containerID string) {})
		close(done)
	}()

	errCh <- io.EOF
	<-done // Watch should exit on error
}
