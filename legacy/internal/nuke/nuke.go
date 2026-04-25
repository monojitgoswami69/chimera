// Package nuke handles teardown of chimera environments.
package nuke

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/volume"
	"github.com/projectchimera/chimera/internal/docker"
)

// StopAndRemove stops and removes all containers for a project. Idempotent.
func StopAndRemove(ctx context.Context, cli docker.DockerAPI, projectName string) ([]string, error) {
	containers, err := cli.ContainerList(ctx, types.ContainerListOptions{
		All: true,
		Filters: filters.NewArgs(
			filters.Arg("label", "com.chimera.project="+projectName),
		),
	})
	if err != nil {
		return nil, fmt.Errorf("nuke: failed to list containers: %w", err)
	}

	var removed []string
	timeout := 10
	for _, c := range containers {
		name := c.ID[:12]
		if len(c.Names) > 0 {
			name = strings.TrimPrefix(c.Names[0], "/")
		}

		if c.State == "running" {
			_ = cli.ContainerStop(ctx, c.ID, container.StopOptions{Timeout: &timeout})
		}

		err := cli.ContainerRemove(ctx, c.ID, types.ContainerRemoveOptions{Force: true})
		if err != nil {
			// Idempotent: skip if already removed
			if strings.Contains(err.Error(), "No such container") || strings.Contains(err.Error(), "not found") {
				continue
			}
			return removed, fmt.Errorf("nuke: failed to remove container %s: %w", name, err)
		}
		removed = append(removed, name)
	}

	return removed, nil
}

// ListProjectVolumes returns volume names for a project.
func ListProjectVolumes(ctx context.Context, cli docker.DockerAPI, projectName string) ([]string, error) {
	resp, err := cli.VolumeList(ctx, volume.ListOptions{
		Filters: filters.NewArgs(
			filters.Arg("label", "com.chimera.project="+projectName),
		),
	})
	if err != nil {
		return nil, fmt.Errorf("nuke: failed to list volumes: %w", err)
	}

	var names []string
	for _, v := range resp.Volumes {
		names = append(names, v.Name)
	}
	return names, nil
}

// RemoveVolumes deletes volumes by name. Idempotent.
func RemoveVolumes(ctx context.Context, cli docker.DockerAPI, names []string) error {
	for _, name := range names {
		err := cli.VolumeRemove(ctx, name, true)
		if err != nil {
			if strings.Contains(err.Error(), "no such volume") || strings.Contains(err.Error(), "not found") {
				continue // idempotent
			}
			return fmt.Errorf("nuke: failed to remove volume %s: %w", name, err)
		}
	}
	return nil
}

// ConfirmDestruction prompts the user to type the project name to confirm.
func ConfirmDestruction(projectName string, containers []string, volumes []string, w io.Writer, r io.Reader) bool {
	warnStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("196")).
		Bold(true)

	fmt.Fprintln(w, warnStyle.Render("⚠  DESTRUCTIVE OPERATION"))
	fmt.Fprintln(w)

	if len(containers) > 0 {
		fmt.Fprintf(w, "  Containers to remove: %s\n", strings.Join(containers, ", "))
	}
	if len(volumes) > 0 {
		fmt.Fprintf(w, "  Volumes to delete:    %s\n", strings.Join(volumes, ", "))
	}

	fmt.Fprintln(w)
	fmt.Fprintf(w, "Type \"%s\" to confirm: ", projectName)

	scanner := bufio.NewScanner(r)
	if !scanner.Scan() {
		return false
	}

	return strings.TrimSpace(scanner.Text()) == projectName
}

// PrintSummary prints the nuke results.
func PrintSummary(containers []string, volumes []string, hostsRemoved bool, w io.Writer) {
	successStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("10")).Bold(true)
	infoStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("12"))

	fmt.Fprintln(w)
	if len(containers) > 0 {
		fmt.Fprintln(w, infoStyle.Render(fmt.Sprintf("  ✓ %d container(s) removed", len(containers))))
	}
	if len(volumes) > 0 {
		fmt.Fprintln(w, infoStyle.Render(fmt.Sprintf("  ✓ %d volume(s) purged", len(volumes))))
	}
	if hostsRemoved {
		fmt.Fprintln(w, infoStyle.Render("  ✓ /etc/hosts entry cleaned"))
	}
	fmt.Fprintln(w)
	fmt.Fprintln(w, successStyle.Render("  Environment fully purged. Nothing left behind."))
	fmt.Fprintln(w)
}
