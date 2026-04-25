package tui

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/projectchimera/chimera/internal/docker"

	tea "github.com/charmbracelet/bubbletea"
)

// ContainerStat holds computed stats for a single container.
type ContainerStat struct {
	ID      string
	Name    string
	Status  string
	CPU     float64
	MemMiB  float64
	NetRxMB float64
	NetTxMB float64
}

// StatsModel is the bubbletea model for the live TUI dashboard.
type StatsModel struct {
	containers []ContainerStat
	width      int
	height     int
	quitting   bool
	project    string
	cli        docker.DockerAPI
	ctx        context.Context
	cancel     context.CancelFunc
	once       bool // if true, render once and quit
	err        error
}

type statsTickMsg time.Time
type statsDataMsg []ContainerStat

// NewStatsModel creates a new stats dashboard model.
func NewStatsModel(ctx context.Context, cli docker.DockerAPI, project string, once bool) StatsModel {
	childCtx, cancel := context.WithCancel(ctx)
	return StatsModel{
		containers: []ContainerStat{},
		project:    project,
		cli:        cli,
		ctx:        childCtx,
		cancel:     cancel,
		once:       once,
		width:      80,
		height:     24,
	}
}

func (m StatsModel) Init() tea.Cmd {
	return tea.Batch(fetchStats(m.ctx, m.cli, m.project), tickStats())
}

func (m StatsModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			m.quitting = true
			m.cancel()
			return m, tea.Quit
		}
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
	case statsDataMsg:
		m.containers = []ContainerStat(msg)
		if m.once {
			m.quitting = true
			return m, tea.Quit
		}
		return m, nil
	case statsTickMsg:
		if m.quitting {
			return m, nil
		}
		return m, fetchStats(m.ctx, m.cli, m.project)
	}
	return m, nil
}

func (m StatsModel) View() string {
	if m.quitting && m.once {
		return renderStatsTable(m.containers, m.width) + "\n"
	}
	if m.quitting {
		return ""
	}

	var b strings.Builder
	b.WriteString("\n")
	title := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("13")).Render(
		fmt.Sprintf("  Chimera Dashboard — %s", m.project))
	b.WriteString(title + "\n\n")

	if len(m.containers) == 0 {
		b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Render("  No containers found. Waiting..."))
		b.WriteString("\n")
	} else {
		b.WriteString(renderStatsTable(m.containers, m.width))
	}

	footer := lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Italic(true).Render(
		fmt.Sprintf("  Press q to quit · updates every 2s · %d containers", len(m.containers)))
	b.WriteString("\n" + footer + "\n")
	return b.String()
}

func tickStats() tea.Cmd {
	return tea.Tick(2*time.Second, func(t time.Time) tea.Msg {
		return statsTickMsg(t)
	})
}

func fetchStats(ctx context.Context, cli docker.DockerAPI, project string) tea.Cmd {
	return func() tea.Msg {
		containers, err := cli.ContainerList(ctx, types.ContainerListOptions{
			All: true,
			Filters: filters.NewArgs(
				filters.Arg("label", "com.chimera.project="+project),
			),
		})
		if err != nil {
			return statsDataMsg{}
		}

		var stats []ContainerStat
		for _, c := range containers {
			name := c.ID[:12]
			if len(c.Names) > 0 {
				name = strings.TrimPrefix(c.Names[0], "/")
			}

			stat := ContainerStat{
				ID:     c.ID[:12],
				Name:   name,
				Status: c.State,
			}

			// Get live stats for running containers
			if c.State == "running" {
				if cs, err := cli.ContainerStats(ctx, c.ID, false); err == nil {
					var statsJSON struct {
						CPUStats struct {
							CPUUsage struct {
								TotalUsage  uint64   `json:"total_usage"`
								PercpuUsage []uint64 `json:"percpu_usage"`
							} `json:"cpu_usage"`
							SystemCPUUsage uint64 `json:"system_cpu_usage"`
							OnlineCPUs     uint32 `json:"online_cpus"`
						} `json:"cpu_stats"`
						PreCPUStats struct {
							CPUUsage struct {
								TotalUsage uint64 `json:"total_usage"`
							} `json:"cpu_usage"`
							SystemCPUUsage uint64 `json:"system_cpu_usage"`
						} `json:"precpu_stats"`
						MemoryStats struct {
							Usage uint64            `json:"usage"`
							Limit uint64            `json:"limit"`
							Stats map[string]uint64  `json:"stats"`
						} `json:"memory_stats"`
						Networks map[string]struct {
							RxBytes uint64 `json:"rx_bytes"`
							TxBytes uint64 `json:"tx_bytes"`
						} `json:"networks"`
					}

					if err := json.NewDecoder(cs.Body).Decode(&statsJSON); err == nil {
						// CPU %
						cpuDelta := float64(statsJSON.CPUStats.CPUUsage.TotalUsage - statsJSON.PreCPUStats.CPUUsage.TotalUsage)
						sysDelta := float64(statsJSON.CPUStats.SystemCPUUsage - statsJSON.PreCPUStats.SystemCPUUsage)
						numCPU := float64(statsJSON.CPUStats.OnlineCPUs)
						if numCPU == 0 {
							numCPU = float64(len(statsJSON.CPUStats.CPUUsage.PercpuUsage))
						}
						if numCPU == 0 {
							numCPU = 1
						}
						if sysDelta > 0 {
							stat.CPU = (cpuDelta / sysDelta) * numCPU * 100.0
						}

						// Memory
						cache := uint64(0)
						if c, ok := statsJSON.MemoryStats.Stats["cache"]; ok {
							cache = c
						}
						stat.MemMiB = float64(statsJSON.MemoryStats.Usage-cache) / 1024 / 1024

						// Network
						for _, n := range statsJSON.Networks {
							stat.NetRxMB += float64(n.RxBytes) / 1024 / 1024
							stat.NetTxMB += float64(n.TxBytes) / 1024 / 1024
						}
					}
					cs.Body.Close()
				}
			}
			stats = append(stats, stat)
		}
		return statsDataMsg(stats)
	}
}

func renderStatsTable(containers []ContainerStat, width int) string {
	headerStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("15"))
	runningStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("10"))
	stoppedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("9"))
	dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	warnStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("11"))
	redStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("9"))

	var b strings.Builder
	header := fmt.Sprintf("  %-20s %-12s %8s %10s %10s %10s",
		"NAME", "STATUS", "CPU%", "MEMORY", "NET RX", "NET TX")
	b.WriteString(headerStyle.Render(header) + "\n")
	b.WriteString(headerStyle.Render("  "+strings.Repeat("─", 74)) + "\n")

	for _, c := range containers {
		status := c.Status
		var styledStatus string
		if c.Status == "running" {
			styledStatus = runningStyle.Render(fmt.Sprintf("%-12s", status))
		} else {
			styledStatus = stoppedStyle.Render(fmt.Sprintf("%-12s", status))
		}

		cpuStr := fmt.Sprintf("%7.1f%%", c.CPU)
		if c.CPU > 70 {
			cpuStr = redStyle.Render(cpuStr)
		} else if c.CPU > 30 {
			cpuStr = warnStyle.Render(cpuStr)
		} else {
			cpuStr = dimStyle.Render(cpuStr)
		}

		name := c.Name
		if len(name) > 20 {
			name = name[:17] + "..."
		}

		row := fmt.Sprintf("  %-20s %s %s %9.1f MiB %8.1f MB %8.1f MB",
			name, styledStatus, cpuStr, c.MemMiB, c.NetRxMB, c.NetTxMB)
		b.WriteString(row + "\n")
	}
	return b.String()
}
