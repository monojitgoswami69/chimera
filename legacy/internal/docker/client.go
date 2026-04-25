package docker

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/volume"
	"github.com/docker/docker/client"
)

// Client wraps the Docker SDK client with Chimera-specific operations
// Interface-first design for testability (can be mocked)
type Client struct {
	cli *client.Client
}

// ContainerStats represents statistics for containers
type ContainerStats struct {
	Containers []ContainerInfo
}

// ContainerInfo represents information about a single container
type ContainerInfo struct {
	ID            string
	Name          string
	Status        string
	Health        string
	CPUPercent    float64
	MemoryUsage   uint64
	MemoryLimit   uint64
	Ports         []PortMapping
}

// PortMapping represents a port mapping
type PortMapping struct {
	HostPort      uint16
	ContainerPort uint16
	Protocol      string
}

// StartOptions contains options for starting containers
type StartOptions struct {
	WorkspaceDir string
	Detach       bool
}

// NukeOptions contains options for the nuke operation
type NukeOptions struct {
	WorkspaceDir   string
	RemoveVolumes  bool
	RemoveNetworks bool
}

// SystemInfo represents Docker system information
type SystemInfo struct {
	Containers        int
	ContainersRunning int
	Images            int
	NCPU              int
	MemTotal          int64
	OperatingSystem   string
	Architecture      string
}

// NetworkInfo represents Docker network information
type NetworkInfo struct {
	Name   string
	Driver string
	ID     string
}

// DiskUsage represents Docker disk usage information
type DiskUsage struct {
	ImagesSize      uint64
	ContainersSize  uint64
	VolumesSize     uint64
	BuildCacheSize  uint64
}

// NewClient creates a new Docker client
// Uses the default Docker host from environment variables
func NewClient() (*Client, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("failed to create Docker client: %w", err)
	}

	return &Client{cli: cli}, nil
}

// Close closes the Docker client connection
func (c *Client) Close() error {
	if c.cli != nil {
		return c.cli.Close()
	}
	return nil
}

// GetVersion returns the Docker daemon version
func (c *Client) GetVersion(ctx context.Context) (string, error) {
	version, err := c.cli.ServerVersion(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get Docker version: %w", err)
	}
	return version.Version, nil
}

// GetSystemInfo returns Docker system information
func (c *Client) GetSystemInfo(ctx context.Context) (*SystemInfo, error) {
	info, err := c.cli.Info(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get system info: %w", err)
	}

	return &SystemInfo{
		Containers:        info.Containers,
		ContainersRunning: info.ContainersRunning,
		Images:            info.Images,
		NCPU:              info.NCPU,
		MemTotal:          info.MemTotal,
		OperatingSystem:   info.OperatingSystem,
		Architecture:      info.Architecture,
	}, nil
}

// ListNetworks returns a list of Docker networks
func (c *Client) ListNetworks(ctx context.Context) ([]NetworkInfo, error) {
	networks, err := c.cli.NetworkList(ctx, types.NetworkListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list networks: %w", err)
	}

	result := make([]NetworkInfo, len(networks))
	for i, network := range networks {
		result[i] = NetworkInfo{
			Name:   network.Name,
			Driver: network.Driver,
			ID:     network.ID,
		}
	}

	return result, nil
}

// GetDiskUsage returns Docker disk usage information
func (c *Client) GetDiskUsage(ctx context.Context) (*DiskUsage, error) {
	usage, err := c.cli.DiskUsage(ctx, types.DiskUsageOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get disk usage: %w", err)
	}

	result := &DiskUsage{}

	// Calculate images size
	for _, image := range usage.Images {
		result.ImagesSize += uint64(image.Size)
	}

	// Calculate containers size
	for _, container := range usage.Containers {
		result.ContainersSize += uint64(container.SizeRw)
	}

	// Calculate volumes size
	for _, volume := range usage.Volumes {
		if volume.UsageData != nil {
			result.VolumesSize += uint64(volume.UsageData.Size)
		}
	}

	// Calculate build cache size
	for _, cache := range usage.BuildCache {
		result.BuildCacheSize += uint64(cache.Size)
	}

	return result, nil
}

// BuildImages builds Docker images for the workspace
func (c *Client) BuildImages(ctx context.Context, workspaceDir string) error {
	// This is a stub - full implementation in Phase 2
	// Will use docker-compose to build images
	return nil
}

// StartContainers starts all containers in the workspace
func (c *Client) StartContainers(ctx context.Context, opts *StartOptions) error {
	// This is a stub - full implementation in Phase 2
	// Will use docker-compose to start containers
	return nil
}

// GetStats returns statistics for containers in the workspace
func (c *Client) GetStats(ctx context.Context, workspaceDir string) (*ContainerStats, error) {
	// List containers with Chimera label
	containers, err := c.cli.ContainerList(ctx, types.ContainerListOptions{
		All: true,
		Filters: filters.NewArgs(
			filters.Arg("label", "com.chimera.workspace="+workspaceDir),
		),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list containers: %w", err)
	}

	stats := &ContainerStats{
		Containers: make([]ContainerInfo, 0, len(containers)),
	}

	for _, cont := range containers {
		name := cont.ID[:12]
		if len(cont.Names) > 0 {
			name = cont.Names[0]
		}
		info := ContainerInfo{
			ID:     cont.ID[:12],
			Name:   name,
			Status: cont.Status,
		}

		// Extract health status if available
		if cont.State == "running" {
			// Get container stats
			// This is simplified - full implementation would stream stats
		}

		stats.Containers = append(stats.Containers, info)
	}

	return stats, nil
}

// WatchStats continuously monitors container statistics
func (c *Client) WatchStats(ctx context.Context, workspaceDir string) error {
	// This is a stub - full implementation in Phase 2
	// Will continuously update stats display
	return nil
}

// StopContainers stops all containers in the workspace
func (c *Client) StopContainers(ctx context.Context, opts *NukeOptions) (int, error) {
	containers, err := c.cli.ContainerList(ctx, types.ContainerListOptions{
		All: true,
		Filters: filters.NewArgs(
			filters.Arg("label", "com.chimera.workspace="+opts.WorkspaceDir),
		),
	})
	if err != nil {
		return 0, fmt.Errorf("failed to list containers: %w", err)
	}

	count := 0
	for _, cont := range containers {
		if cont.State == "running" {
			if err := c.cli.ContainerStop(ctx, cont.ID, container.StopOptions{}); err != nil {
				return count, fmt.Errorf("failed to stop container %s: %w", cont.ID, err)
			}
			count++
		}
	}

	return count, nil
}

// RemoveContainers removes all containers in the workspace
func (c *Client) RemoveContainers(ctx context.Context, opts *NukeOptions) (int, error) {
	containers, err := c.cli.ContainerList(ctx, types.ContainerListOptions{
		All: true,
		Filters: filters.NewArgs(
			filters.Arg("label", "com.chimera.workspace="+opts.WorkspaceDir),
		),
	})
	if err != nil {
		return 0, fmt.Errorf("failed to list containers: %w", err)
	}

	count := 0
	for _, cont := range containers {
		if err := c.cli.ContainerRemove(ctx, cont.ID, types.ContainerRemoveOptions{
			Force: true,
		}); err != nil {
			return count, fmt.Errorf("failed to remove container %s: %w", cont.ID, err)
		}
		count++
	}

	return count, nil
}

// RemoveNetworks removes all networks in the workspace
func (c *Client) RemoveNetworks(ctx context.Context, opts *NukeOptions) (int, error) {
	if !opts.RemoveNetworks {
		return 0, nil
	}

	networks, err := c.cli.NetworkList(ctx, types.NetworkListOptions{
		Filters: filters.NewArgs(
			filters.Arg("label", "com.chimera.workspace="+opts.WorkspaceDir),
		),
	})
	if err != nil {
		return 0, fmt.Errorf("failed to list networks: %w", err)
	}

	count := 0
	for _, network := range networks {
		if err := c.cli.NetworkRemove(ctx, network.ID); err != nil {
			// Ignore errors for built-in networks
			continue
		}
		count++
	}

	return count, nil
}

// RemoveVolumes removes all volumes in the workspace
func (c *Client) RemoveVolumes(ctx context.Context, opts *NukeOptions) (int, error) {
	if !opts.RemoveVolumes {
		return 0, nil
	}

	volumeList, err := c.cli.VolumeList(ctx, volume.ListOptions{
		Filters: filters.NewArgs(
			filters.Arg("label", "com.chimera.workspace="+opts.WorkspaceDir),
		),
	})
	if err != nil {
		return 0, fmt.Errorf("failed to list volumes: %w", err)
	}

	count := 0
	for _, vol := range volumeList.Volumes {
		if err := c.cli.VolumeRemove(ctx, vol.Name, true); err != nil {
			return count, fmt.Errorf("failed to remove volume %s: %w", vol.Name, err)
		}
		count++
	}

	return count, nil
}

// PullImage pulls a Docker image
func (c *Client) PullImage(ctx context.Context, image string) error {
	reader, err := c.cli.ImagePull(ctx, image, types.ImagePullOptions{})
	if err != nil {
		return fmt.Errorf("failed to pull image %s: %w", image, err)
	}
	defer reader.Close()

	// Copy output to stdout to show progress
	_, err = io.Copy(os.Stdout, reader)
	return err
}
