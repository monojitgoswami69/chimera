package generator

import (
	"fmt"
	"strings"

	"chimera/internal/detector"
)

// GenerateCompose generates docker-compose.yml
func GenerateCompose(services []detector.Service, repoName string) string {
	var b strings.Builder

	b.WriteString("version: '3.8'\n\n")
	b.WriteString("services:\n")

	for _, svc := range services {
		serviceName := sanitizeServiceName(svc.ID)
		port := GetDefaultPort(svc.Framework)
		contextPath := "."
		if svc.Directory != "" {
			contextPath = "./" + svc.Directory
		}

		b.WriteString(fmt.Sprintf("  %s:\n", serviceName))
		b.WriteString("    build:\n")
		b.WriteString(fmt.Sprintf("      context: %s\n", contextPath))
		b.WriteString("      dockerfile: ../chimera-outputs/Dockerfile\n")
		b.WriteString("    ports:\n")
		b.WriteString(fmt.Sprintf("      - \"%d:%d\"\n", port, port))
		b.WriteString("    env_file:\n")
		b.WriteString(fmt.Sprintf("      - ./chimera-outputs/env-vars/%s/.env.example\n", serviceName))
		b.WriteString("    networks:\n")
		b.WriteString(fmt.Sprintf("      - %s_net\n", repoName))
		b.WriteString("    restart: unless-stopped\n")
		b.WriteString("\n")
	}

	b.WriteString("networks:\n")
	b.WriteString(fmt.Sprintf("  %s_net:\n", repoName))
	b.WriteString("    driver: bridge\n")

	return b.String()
}

func sanitizeServiceName(id string) string {
	name := strings.ReplaceAll(id, "_", "-")
	name = strings.ToLower(name)
	return name
}
