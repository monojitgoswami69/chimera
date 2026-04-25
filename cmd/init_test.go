package cmd

import (
	"strings"
	"testing"
)

func TestDetectAppPorts(t *testing.T) {
	composeYAML := `version: '3.8'

services:
  app:
    ports:
      - "3100:8080"
`

	host, container := detectAppPorts(composeYAML, 3000)
	if host != 3100 {
		t.Fatalf("host port = %d, want 3100", host)
	}
	if container != 8080 {
		t.Fatalf("container port = %d, want 8080", container)
	}
}

func TestEnsureCaddyService(t *testing.T) {
	composeYAML := `version: '3.8'

services:
  app:
    ports:
      - "3000:3000"
    networks:
      - myapp_net

networks:
  myapp_net:
    driver: bridge
`

	withCaddy := ensureCaddyService(composeYAML, "myapp")
	if !strings.Contains(withCaddy, "  caddy:\n") {
		t.Fatalf("expected caddy service to be injected")
	}

	caddyIdx := strings.Index(withCaddy, "  caddy:\n")
	networkIdx := strings.Index(withCaddy, "\nnetworks:\n")
	if caddyIdx == -1 || networkIdx == -1 || caddyIdx > networkIdx {
		t.Fatalf("expected caddy service to be inserted before top-level networks section")
	}

	// Should be idempotent and not duplicate caddy service.
	again := ensureCaddyService(withCaddy, "myapp")
	if strings.Count(again, "  caddy:\n") != 1 {
		t.Fatalf("expected caddy service injection to be idempotent")
	}
}

func TestDetectAppNetwork(t *testing.T) {
	composeYAML := `version: '3.8'

services:
  app:
    networks:
      - myapp_net
`

	netName := detectAppNetwork(composeYAML)
	if netName != "myapp_net" {
		t.Fatalf("detected app network = %q, want %q", netName, "myapp_net")
	}
}
