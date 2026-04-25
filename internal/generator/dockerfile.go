package generator

import (
	"fmt"
	"strings"

	"chimera/internal/detector"
)

// Dockerfile templates
const (
	nextTemplate = `FROM node:18-alpine AS deps
WORKDIR /app
COPY package*.json ./
RUN npm ci

FROM node:18-alpine AS builder
WORKDIR /app
COPY --from=deps /app/node_modules ./node_modules
COPY . .
RUN npm run build

FROM node:18-alpine AS runner
WORKDIR /app
ENV NODE_ENV production
COPY --from=builder /app/.next ./.next
COPY --from=builder /app/node_modules ./node_modules
COPY --from=builder /app/package.json ./
EXPOSE 3000
CMD ["npm", "start"]`

	expressTemplate = `FROM node:18-alpine
WORKDIR /app
COPY package*.json ./
RUN npm ci --only=production
COPY . .
EXPOSE 4000
CMD ["node", "server.js"]`

	fastapiTemplate = `FROM python:3.11-slim
WORKDIR /app
COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt
COPY . .
EXPOSE 8000
CMD ["uvicorn", "main:app", "--host", "0.0.0.0", "--port", "8000"]`

	flaskTemplate = `FROM python:3.11-slim
WORKDIR /app
COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt
COPY . .
EXPOSE 5000
ENV FLASK_APP=app.py
CMD ["flask", "run", "--host=0.0.0.0"]`

	djangoTemplate = `FROM python:3.11-slim
WORKDIR /app
COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt
COPY . .
EXPOSE 8000
CMD ["python", "manage.py", "runserver", "0.0.0.0:8000"]`
)

// GenerateDockerfile generates a Dockerfile for a service
func GenerateDockerfile(service detector.Service) string {
	switch service.Framework {
	case "next":
		return nextTemplate
	case "express":
		return expressTemplate
	case "fastapi":
		return fastapiTemplate
	case "flask":
		return flaskTemplate
	case "django":
		return djangoTemplate
	case "react":
		return nextTemplate // Use Next.js template for React
	default:
		if service.Language == "python" {
			return fastapiTemplate
		}
		return expressTemplate
	}
}

// GenerateMultiStage generates multi-stage Dockerfile for monorepos
func GenerateMultiStage(services []detector.Service) string {
	if len(services) == 1 {
		return GenerateDockerfile(services[0])
	}

	var b strings.Builder
	for i, svc := range services {
		b.WriteString(fmt.Sprintf("# Stage %d: %s (%s)\n", i+1, svc.ID, svc.Framework))
		dockerfile := GenerateDockerfile(svc)
		lines := strings.Split(dockerfile, "\n")
		if len(lines) > 0 && strings.HasPrefix(lines[0], "FROM") {
			lines[0] = lines[0] + fmt.Sprintf(" AS %s", svc.ID)
		}
		b.WriteString(strings.Join(lines, "\n"))
		b.WriteString("\n\n")
	}
	return b.String()
}

// GetDefaultPort returns default port for framework
func GetDefaultPort(framework string) int {
	ports := map[string]int{
		"next":    3000,
		"express": 4000,
		"fastapi": 8000,
		"flask":   5000,
		"django":  8000,
		"react":   3000,
	}
	if port, ok := ports[framework]; ok {
		return port
	}
	return 3000
}
