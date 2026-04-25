package compose

import (
	"fmt"
	"strings"

	"github.com/projectchimera/chimera/internal/scanner"
)

// ComposeManifest holds all generated files for a chimera workspace.
type ComposeManifest struct {
	DockerCompose string
	Dockerfile    string
	EnvExample    string
	ProjectName   string
	AppPort       int
	InfraServices []string
}

// infraImage maps infrastructure type to its Docker image.
var infraImage = map[string]string{
	"postgresql":    "postgres:15-alpine",
	"mysql":         "mysql:8.0",
	"mongodb":       "mongo:6.0",
	"redis":         "redis:7-alpine",
	"rabbitmq":      "rabbitmq:3.12-management-alpine",
	"elasticsearch": "elasticsearch:8.11.0",
}

// infraPort maps infrastructure type to its default port.
var infraPort = map[string]int{
	"postgresql":    5432,
	"mysql":         3306,
	"mongodb":       27017,
	"redis":         6379,
	"rabbitmq":      5672,
	"elasticsearch": 9200,
}

// infraVolume maps infrastructure type to its data volume mount path.
var infraVolume = map[string]string{
	"postgresql":    "/var/lib/postgresql/data",
	"mysql":         "/var/lib/mysql",
	"mongodb":       "/data/db",
	"redis":         "/data",
	"rabbitmq":      "/var/lib/rabbitmq",
	"elasticsearch": "/usr/share/elasticsearch/data",
}

// infraEnv maps infrastructure type to required environment variables.
var infraEnv = map[string]map[string]string{
	"postgresql": {"POSTGRES_USER": "postgres", "POSTGRES_PASSWORD": "postgres", "POSTGRES_DB": "chimera"},
	"mysql":      {"MYSQL_ROOT_PASSWORD": "mysql", "MYSQL_DATABASE": "chimera"},
	"mongodb":    {},
	"redis":      {},
	"rabbitmq":   {"RABBITMQ_DEFAULT_USER": "guest", "RABBITMQ_DEFAULT_PASS": "guest"},
	"elasticsearch": {
		"discovery.type":         "single-node",
		"xpack.security.enabled": "false",
		"ES_JAVA_OPTS":           "-Xms256m -Xmx256m",
	},
}

// DefaultPorts returns desired host ports for all detected services.
func DefaultPorts(result *scanner.ScanResult) map[string]int {
	ports := make(map[string]int)
	for _, infra := range result.Infrastructure {
		if p, ok := infraPort[infra.Type]; ok {
			ports[infra.Type] = p
		}
	}
	// App port
	if len(result.Ports) > 0 {
		ports["app"] = result.Ports[0]
	} else {
		ports["app"] = 3000
	}
	return ports
}

// Generate produces a full ComposeManifest from scan results.
func Generate(projectName string, result *scanner.ScanResult) (*ComposeManifest, error) {
	if len(result.Languages) == 0 {
		return nil, fmt.Errorf("compose: no languages detected, cannot generate environment")
	}

	appPort := 3000
	if len(result.Ports) > 0 {
		appPort = result.Ports[0]
	}

	netName := projectName + "_net"
	manifest := &ComposeManifest{
		ProjectName:   projectName,
		AppPort:       appPort,
		InfraServices: make([]string, 0),
	}

	// Build docker-compose.yml
	var b strings.Builder
	b.WriteString("version: '3.8'\n\nservices:\n")

	// App service
	b.WriteString("  app:\n")
	b.WriteString("    build:\n")
	b.WriteString("      context: .\n")
	b.WriteString("      dockerfile: Dockerfile\n")
	b.WriteString(fmt.Sprintf("    ports:\n      - \"%d:%d\"\n", appPort, appPort))
	b.WriteString("    restart: unless-stopped\n")
	b.WriteString(fmt.Sprintf("    networks:\n      - %s\n", netName))
	b.WriteString(fmt.Sprintf("    labels:\n      com.chimera.project: \"%s\"\n", projectName))

	// env_file
	b.WriteString("    env_file:\n      - .env\n")

	// depends_on with healthchecks
	if len(result.Infrastructure) > 0 {
		b.WriteString("    depends_on:\n")
		for _, infra := range result.Infrastructure {
			svcName := infraServiceName(infra.Type)
			b.WriteString(fmt.Sprintf("      %s:\n        condition: service_healthy\n", svcName))
		}
	}

	// Infrastructure services
	for _, infra := range result.Infrastructure {
		svcName := infraServiceName(infra.Type)
		manifest.InfraServices = append(manifest.InfraServices, svcName)

		image, ok := infraImage[infra.Type]
		if !ok {
			continue
		}

		b.WriteString(fmt.Sprintf("\n  %s:\n", svcName))
		b.WriteString(fmt.Sprintf("    image: %s\n", image))

		port := infraPort[infra.Type]
		b.WriteString(fmt.Sprintf("    ports:\n      - \"%d:%d\"\n", port, port))

		b.WriteString("    restart: unless-stopped\n")
		b.WriteString(fmt.Sprintf("    networks:\n      - %s\n", netName))
		b.WriteString(fmt.Sprintf("    labels:\n      com.chimera.project: \"%s\"\n", projectName))

		// Volume
		if volPath, ok := infraVolume[infra.Type]; ok {
			volName := svcName + "_data"
			b.WriteString(fmt.Sprintf("    volumes:\n      - %s:%s\n", volName, volPath))
		}

		// Environment
		if envMap, ok := infraEnv[infra.Type]; ok && len(envMap) > 0 {
			b.WriteString("    environment:\n")
			for k, v := range envMap {
				b.WriteString(fmt.Sprintf("      %s: \"%s\"\n", k, v))
			}
		}

		// Healthcheck
		hc := Healthcheck(infra.Type)
		if hc != "" {
			b.WriteString(hc)
		}
	}

	// Networks
	b.WriteString(fmt.Sprintf("\nnetworks:\n  %s:\n    driver: bridge\n", netName))

	// Volumes
	if len(result.Infrastructure) > 0 {
		b.WriteString("\nvolumes:\n")
		for _, infra := range result.Infrastructure {
			svcName := infraServiceName(infra.Type)
			b.WriteString(fmt.Sprintf("  %s_data:\n", svcName))
		}
	}

	manifest.DockerCompose = b.String()

	// Generate Dockerfile
	manifest.Dockerfile = GenerateDockerfile(result)

	// Generate .env.example
	manifest.EnvExample = GenerateEnvFile(projectName, result)

	return manifest, nil
}

// infraServiceName returns the docker-compose service name for an infra type.
func infraServiceName(infraType string) string {
	switch infraType {
	case "postgresql":
		return "postgres"
	case "mysql":
		return "mysql"
	case "mongodb":
		return "mongodb"
	case "redis":
		return "redis"
	case "rabbitmq":
		return "rabbitmq"
	case "elasticsearch":
		return "elasticsearch"
	default:
		return infraType
	}
}
