package nuke

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/events"
	"github.com/docker/docker/api/types/volume"
)

// mockDockerClient implements docker.DockerAPI for testing.
type mockDockerClient struct {
	containers   []types.Container
	volumes      []*volume.Volume
	removeErr    error
	stopped      []string
	removed      []string
	volsRemoved  []string
}

func (m *mockDockerClient) ContainerList(ctx context.Context, options types.ContainerListOptions) ([]types.Container, error) {
	return m.containers, nil
}
func (m *mockDockerClient) ContainerStop(ctx context.Context, containerID string, options container.StopOptions) error {
	m.stopped = append(m.stopped, containerID)
	return nil
}
func (m *mockDockerClient) ContainerRemove(ctx context.Context, containerID string, options types.ContainerRemoveOptions) error {
	if m.removeErr != nil {
		return m.removeErr
	}
	m.removed = append(m.removed, containerID)
	return nil
}
func (m *mockDockerClient) ContainerLogs(ctx context.Context, ctr string, options types.ContainerLogsOptions) (io.ReadCloser, error) {
	return io.NopCloser(strings.NewReader("")), nil
}
func (m *mockDockerClient) ContainerStats(ctx context.Context, containerID string, stream bool) (types.ContainerStats, error) {
	return types.ContainerStats{Body: io.NopCloser(strings.NewReader("{}"))}, nil
}
func (m *mockDockerClient) Events(ctx context.Context, options types.EventsOptions) (<-chan events.Message, <-chan error) {
	return make(chan events.Message), make(chan error)
}
func (m *mockDockerClient) VolumeList(ctx context.Context, options volume.ListOptions) (volume.ListResponse, error) {
	return volume.ListResponse{Volumes: m.volumes}, nil
}
func (m *mockDockerClient) VolumeRemove(ctx context.Context, volumeID string, force bool) error {
	m.volsRemoved = append(m.volsRemoved, volumeID)
	return nil
}
func (m *mockDockerClient) Close() error { return nil }

func TestStopAndRemove(t *testing.T) {
	mock := &mockDockerClient{
		containers: []types.Container{
			{ID: "abc123def456ghi7", Names: []string{"/app"}, State: "running"},
			{ID: "xyz789abc012def3", Names: []string{"/postgres"}, State: "exited"},
		},
	}

	removed, err := StopAndRemove(context.Background(), mock, "test-project")
	if err != nil {
		t.Fatalf("StopAndRemove() error: %v", err)
	}

	if len(removed) != 2 {
		t.Errorf("expected 2 removed, got %d", len(removed))
	}

	// Running container should be stopped
	if len(mock.stopped) != 1 || mock.stopped[0] != "abc123def456ghi7" {
		t.Error("running container should be stopped")
	}

	// Both should be removed
	if len(mock.removed) != 2 {
		t.Errorf("expected 2 removed calls, got %d", len(mock.removed))
	}
}

func TestStopAndRemove_Idempotent(t *testing.T) {
	mock := &mockDockerClient{
		containers: []types.Container{
			{ID: "abc123def456ghi7", Names: []string{"/gone"}, State: "exited"},
		},
		removeErr: fmt.Errorf("No such container: abc123"),
	}

	removed, err := StopAndRemove(context.Background(), mock, "test-project")
	if err != nil {
		t.Fatalf("StopAndRemove() should be idempotent, got error: %v", err)
	}

	// Container was "not found" — should be skipped, not reported as removed
	if len(removed) != 0 {
		t.Errorf("expected 0 removed (not found), got %d", len(removed))
	}
}

func TestStopAndRemove_Empty(t *testing.T) {
	mock := &mockDockerClient{containers: []types.Container{}}

	removed, err := StopAndRemove(context.Background(), mock, "empty-project")
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if len(removed) != 0 {
		t.Error("expected no containers removed")
	}
}

func TestListProjectVolumes(t *testing.T) {
	mock := &mockDockerClient{
		volumes: []*volume.Volume{
			{Name: "vol1"},
			{Name: "vol2"},
		},
	}

	names, err := ListProjectVolumes(context.Background(), mock, "test-project")
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if len(names) != 2 {
		t.Errorf("expected 2 volumes, got %d", len(names))
	}
}

func TestRemoveVolumes(t *testing.T) {
	mock := &mockDockerClient{}

	err := RemoveVolumes(context.Background(), mock, []string{"vol1", "vol2"})
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if len(mock.volsRemoved) != 2 {
		t.Errorf("expected 2 volumes removed, got %d", len(mock.volsRemoved))
	}
}

func TestRemoveVolumes_Idempotent(t *testing.T) {
	// Empty list should be fine
	mock := &mockDockerClient{}
	err := RemoveVolumes(context.Background(), mock, nil)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
}

func TestConfirmDestruction_Match(t *testing.T) {
	var output bytes.Buffer
	input := strings.NewReader("my-project\n")
	result := ConfirmDestruction("my-project", []string{"c1", "c2"}, []string{"v1"}, &output, input)
	if !result {
		t.Error("should return true for matching name")
	}
}

func TestConfirmDestruction_Mismatch(t *testing.T) {
	var output bytes.Buffer
	input := strings.NewReader("wrong-name\n")
	result := ConfirmDestruction("my-project", []string{"c1"}, []string{"v1"}, &output, input)
	if result {
		t.Error("should return false for wrong name")
	}
}

func TestConfirmDestruction_Empty(t *testing.T) {
	var output bytes.Buffer
	input := strings.NewReader("\n")
	result := ConfirmDestruction("my-project", nil, nil, &output, input)
	if result {
		t.Error("should return false for empty input")
	}
}

func TestConfirmDestruction_EOF(t *testing.T) {
	var output bytes.Buffer
	input := strings.NewReader("")
	result := ConfirmDestruction("my-project", nil, nil, &output, input)
	if result {
		t.Error("should return false on EOF")
	}
}

func TestPrintSummary(t *testing.T) {
	var buf bytes.Buffer
	PrintSummary([]string{"c1", "c2"}, []string{"v1"}, true, &buf)
	output := buf.String()
	if !strings.Contains(output, "2 container(s) removed") {
		t.Error("should mention 2 containers")
	}
	if !strings.Contains(output, "1 volume(s) purged") {
		t.Error("should mention 1 volume")
	}
	if !strings.Contains(output, "fully purged") {
		t.Error("should contain fully purged")
	}
}

func TestPrintSummary_Empty(t *testing.T) {
	var buf bytes.Buffer
	PrintSummary(nil, nil, false, &buf)
	output := buf.String()
	if !strings.Contains(output, "fully purged") {
		t.Error("should still show final message")
	}
}
