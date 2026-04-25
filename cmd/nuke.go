package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path"
	"sort"
	"strings"
	"syscall"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"github.com/projectchimera/chimera/internal/nuke"
	"github.com/projectchimera/chimera/internal/proxy"
	"github.com/projectchimera/chimera/internal/tui"
	"github.com/spf13/cobra"
)

var (
	nukeProject string
	nukeForce   bool
)

// nukeCmd represents the nuke command
var nukeCmd = &cobra.Command{
	Use:   "nuke",
	Short: "Tear down and remove all containers, volumes, and hosts entries",
	Long: `Completely tear down the Chimera environment by stopping and removing
all containers, volumes, and /etc/hosts entries.

This is a destructive operation. You will be asked to confirm by typing the
project name unless --force is used.

Examples:
  chimera nuke
  chimera nuke --force
  chimera nuke --project my-app --force`,
	RunE: runNuke,
}

func init() {
	rootCmd.AddCommand(nukeCmd)
	nukeCmd.Flags().StringVar(&nukeProject, "project", "", "Project name (auto-detected from .chimera file)")
	nukeCmd.Flags().BoolVar(&nukeForce, "force", false, "Skip confirmation prompt")
}

// runNuke executes the nuke command logic
func runNuke(cmd *cobra.Command, args []string) error {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	// Detect project
	project := nukeProject
	if project == "" {
		project = detectProject()
	}
	if project == "" {
		tui.PrintError("No project found. Use --project flag or run from a chimera workspace.")
		return fmt.Errorf("nuke: no project detected")
	}

	// Initialize Docker client
	dockerCli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		tui.PrintError(fmt.Sprintf("Failed to connect to Docker: %v", err))
		return fmt.Errorf("nuke: docker connection failed: %w", err)
	}
	defer dockerCli.Close()

	// List what will be destroyed
	containers, _ := listContainerNames(ctx, dockerCli, project)
	volumes, _ := nuke.ListProjectVolumes(ctx, dockerCli, project)
	projectImages, _ := listProjectImageTags(ctx, dockerCli, project)

	if len(containers) == 0 && len(volumes) == 0 && len(projectImages) == 0 {
		tui.PrintWarning("No chimera resources found for project: " + project)
		return nil
	}

	// Confirm unless --force
	if !nukeForce {
		if !nuke.ConfirmDestruction(project, containers, volumes, os.Stdout, os.Stdin) {
			tui.PrintInfo("Operation cancelled")
			return nil
		}
	}

	// Stop and remove containers
	tui.PrintInfo("Stopping and removing containers...")
	removed, err := nuke.StopAndRemove(ctx, dockerCli, project)
	if err != nil {
		tui.PrintError(fmt.Sprintf("Container removal error: %v", err))
	}

	// Remove volumes
	tui.PrintInfo("Removing volumes...")
	if err := nuke.RemoveVolumes(ctx, dockerCli, volumes); err != nil {
		tui.PrintError(fmt.Sprintf("Volume removal error: %v", err))
	}

	// Remove project images
	tui.PrintInfo("Removing project images...")
	removedImages, err := removeProjectImages(ctx, dockerCli, project)
	if err != nil {
		tui.PrintError(fmt.Sprintf("Image removal error: %v", err))
	}
	if len(removedImages) > 0 {
		tui.PrintSuccess(fmt.Sprintf("Removed %d project image(s)", len(removedImages)))
	}

	// Clean /etc/hosts
	hostsRemoved := false
	if err := proxy.RemoveEntry(project); err != nil {
		tui.PrintWarning(fmt.Sprintf("Could not clean /etc/hosts: %v", err))
	} else {
		hostsRemoved = true
	}

	// Print summary
	nuke.PrintSummary(removed, volumes, hostsRemoved, os.Stdout)

	return nil
}

func listProjectImageTags(ctx context.Context, cli *client.Client, project string) ([]string, error) {
	images, err := cli.ImageList(ctx, types.ImageListOptions{All: true})
	if err != nil {
		return nil, err
	}

	var tags []string
	for _, img := range images {
		for _, tag := range img.RepoTags {
			if isProjectImageTag(tag, project) {
				tags = append(tags, tag)
			}
		}
	}

	sort.Strings(tags)
	return tags, nil
}

func removeProjectImages(ctx context.Context, cli *client.Client, project string) ([]string, error) {
	images, err := cli.ImageList(ctx, types.ImageListOptions{All: true})
	if err != nil {
		return nil, fmt.Errorf("nuke: failed to list images: %w", err)
	}

	// Remove by image ID to avoid duplicate removals when multiple tags point to one image.
	projectTagsByID := make(map[string][]string)
	for _, img := range images {
		for _, tag := range img.RepoTags {
			if isProjectImageTag(tag, project) {
				projectTagsByID[img.ID] = append(projectTagsByID[img.ID], tag)
			}
		}
	}

	var removedTags []string
	for imageID, tags := range projectTagsByID {
		_, rmErr := cli.ImageRemove(ctx, imageID, types.ImageRemoveOptions{Force: true, PruneChildren: true})
		if rmErr != nil {
			if strings.Contains(rmErr.Error(), "No such image") || strings.Contains(rmErr.Error(), "not found") {
				continue
			}
			return removedTags, fmt.Errorf("nuke: failed to remove image %s: %w", imageID, rmErr)
		}
		removedTags = append(removedTags, tags...)
	}

	sort.Strings(removedTags)
	return removedTags, nil
}

func isProjectImageTag(tag, project string) bool {
	if tag == "" || tag == "<none>:<none>" {
		return false
	}

	name := strings.Split(tag, ":")[0]
	base := path.Base(name)
	return base == project || strings.HasPrefix(base, project+"-") || strings.HasPrefix(base, project+"_")
}

func listContainerNames(ctx context.Context, cli *client.Client, project string) ([]string, error) {
	// Use the Docker SDK directly to avoid interface issues
	containers, err := cli.ContainerList(ctx, containerListOpts(project))
	if err != nil {
		return nil, err
	}
	var names []string
	for _, c := range containers {
		name := c.ID[:12]
		if len(c.Names) > 0 {
			name = strings.TrimPrefix(c.Names[0], "/")
		}
		names = append(names, name)
	}
	return names, nil
}
