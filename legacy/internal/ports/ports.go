// Package ports handles port conflict detection and resolution.
package ports

import (
	"fmt"
	"io"
	"net"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Remap records a port reassignment.
type Remap struct {
	Service string
	From    int
	To      int
}

// PortMap maps service name to resolved host port.
type PortMap map[string]int

// IsPortInUse checks if a TCP port is currently in use.
// Uses net.Listen which works cross-platform (macOS, Linux, WSL2).
func IsPortInUse(port int) bool {
	ln, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return true
	}
	ln.Close()
	return false
}

// FindFreePort finds the next free port starting from startPort.
// Scans up to 100 ports above startPort.
func FindFreePort(startPort int) (int, error) {
	for p := startPort; p < startPort+100; p++ {
		if !IsPortInUse(p) {
			return p, nil
		}
	}
	return 0, fmt.Errorf("ports: no free port found in range %d-%d", startPort, startPort+100)
}

// ResolveAll checks each desired port and remaps conflicts.
func ResolveAll(desired map[string]int) (PortMap, []Remap, error) {
	resolved := make(PortMap)
	var remaps []Remap

	for svc, port := range desired {
		if IsPortInUse(port) {
			free, err := FindFreePort(port + 1)
			if err != nil {
				return nil, nil, fmt.Errorf("ports: cannot resolve conflict for %s (port %d): %w", svc, port, err)
			}
			remaps = append(remaps, Remap{Service: svc, From: port, To: free})
			resolved[svc] = free
		} else {
			resolved[svc] = port
		}
	}
	return resolved, remaps, nil
}

// ApplyRemaps performs string replacement on compose YAML and env file
// to update host port bindings and connection URLs.
func ApplyRemaps(composeYAML string, envFile string, remaps []Remap) (string, string) {
	for _, r := range remaps {
		oldBind := fmt.Sprintf("\"%d:", r.From)
		newBind := fmt.Sprintf("\"%d:", r.To)
		composeYAML = strings.ReplaceAll(composeYAML, oldBind, newBind)

		// Update port references in env file (e.g., :5432/ → :5433/)
		oldPort := fmt.Sprintf(":%d", r.From)
		newPort := fmt.Sprintf(":%d", r.To)
		envFile = strings.ReplaceAll(envFile, oldPort, newPort)
	}
	return composeYAML, envFile
}

// PrintRemaps prints port remap notifications using lipgloss styling.
func PrintRemaps(remaps []Remap, w io.Writer) {
	style := lipgloss.NewStyle().Foreground(lipgloss.Color("11")).Bold(true)
	for _, r := range remaps {
		msg := fmt.Sprintf("  ⚑  Port %d in use → reassigned to %d  (%s)", r.From, r.To, r.Service)
		fmt.Fprintln(w, style.Render(msg))
	}
}
