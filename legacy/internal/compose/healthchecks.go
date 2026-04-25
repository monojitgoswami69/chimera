package compose

import "fmt"

// Healthcheck returns the YAML healthcheck block for an infrastructure type.
func Healthcheck(infraType string) string {
	switch infraType {
	case "postgresql":
		return healthcheckYAML("pg_isready -U postgres", "5s", "3s", "5", "10s")
	case "mysql":
		return healthcheckYAML("mysqladmin ping -h localhost --silent", "5s", "3s", "5", "15s")
	case "mongodb":
		return healthcheckYAML("mongosh --eval 'db.runCommand(\"ping\").ok' --quiet", "5s", "3s", "5", "10s")
	case "redis":
		return healthcheckYAML("redis-cli ping | grep PONG", "5s", "3s", "5", "5s")
	case "rabbitmq":
		return healthcheckYAML("rabbitmq-diagnostics -q ping", "10s", "5s", "5", "20s")
	case "elasticsearch":
		return healthcheckYAML("curl -f http://localhost:9200/_cluster/health || exit 1", "10s", "5s", "5", "30s")
	default:
		return ""
	}
}

// healthcheckYAML builds the YAML string for a healthcheck configuration.
func healthcheckYAML(test, interval, timeout, retries, startPeriod string) string {
	return fmt.Sprintf(`    healthcheck:
      test: ["CMD-SHELL", "%s"]
      interval: %s
      timeout: %s
      retries: %s
      start_period: %s
`, test, interval, timeout, retries, startPeriod)
}
