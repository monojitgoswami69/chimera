// Package generator builds Dockerfiles and docker-compose.yml from detected services.
package generator

import (
	"fmt"
	"strings"

	"chimera/internal/detector"
)

// Output bundles everything the writer needs to lay down on disk.
type Output struct {
	// Files maps relative paths (within chimera-outputs/) to file contents.
	Files map[string]string
}

// Build constructs the full set of generated files. RepoName is used for the
// docker-compose network name. The OutputSubdir is the directory (relative to
// repo root) where these files will be written — typically "chimera-outputs".
func Build(services []detector.Service, repoName, outputSubdir string) *Output {
	out := &Output{Files: map[string]string{}}
	// Copy services so we can adjust runtime ports without mutating the caller's slice.
	runtime := make([]detector.Service, len(services))
	copy(runtime, services)
	for i := range runtime {
		// Static-served SPAs run on nginx port 80 in their runtime stage.
		switch runtime[i].Framework {
		case "react", "vite", "cra":
			runtime[i].Port = 80
		}
	}
	for _, svc := range runtime {
		out.Files["Dockerfile."+svc.ID] = renderDockerfile(svc)
	}
	out.Files["docker-compose.yml"] = renderCompose(runtime, repoName, outputSubdir)
	return out
}

// renderDockerfile generates a Dockerfile assuming the build context is the
// repository root. Paths are sub-directory aware so monorepos work cleanly.
func renderDockerfile(svc detector.Service) string {
	prefix := ""
	if svc.Directory != "" {
		prefix = strings.TrimSuffix(svc.Directory, "/") + "/"
	}

	switch svc.Framework {
	case "next":
		return renderNext(svc, prefix)
	case "nest":
		return renderNest(svc, prefix)
	case "react", "vite", "cra":
		return renderReact(svc, prefix)
	case "express", "fastify", "node":
		return renderNode(svc, prefix)
	case "fastapi":
		return renderFastAPI(svc, prefix)
	case "flask":
		return renderFlask(svc, prefix)
	case "django":
		return renderDjango(svc, prefix)
	default:
		if svc.Language == "python" {
			return renderPython(svc, prefix)
		}
		return renderNode(svc, prefix)
	}
}

func renderNext(svc detector.Service, prefix string) string {
	pm := svc.PackageManager
	if pm == "" {
		pm = "npm"
	}
	installCmd := joinArgs(svc.InstallCmd)
	buildCmd := joinArgs(svc.BuildCmd)
	startCmd := dockerCMD(svc.StartCmd)
	if buildCmd == "" {
		buildCmd = pm + " run build"
	}
	return fmt.Sprintf(`# syntax=docker/dockerfile:1.6
FROM node:20-alpine AS deps
WORKDIR /app
COPY %[1]spackage*.json ./
%[3]s
RUN %[5]s

FROM node:20-alpine AS builder
WORKDIR /app
COPY --from=deps /app/node_modules ./node_modules
COPY %[1]s. ./
RUN %[6]s

FROM node:20-alpine AS runner
ENV NODE_ENV=production
WORKDIR /app
COPY --from=builder /app/.next ./.next
COPY --from=builder /app/node_modules ./node_modules
COPY --from=builder /app/package.json ./
COPY --from=builder /app/public ./public 2>/dev/null || true
EXPOSE %[2]d
%[4]s
`, prefix, svc.Port, copyLockfile(prefix, pm), startCmd, installCmd, buildCmd)
}

func renderNest(svc detector.Service, prefix string) string {
	pm := svc.PackageManager
	if pm == "" {
		pm = "npm"
	}
	return fmt.Sprintf(`# syntax=docker/dockerfile:1.6
FROM node:20-alpine AS deps
WORKDIR /app
COPY %[1]spackage*.json ./
%[3]s
RUN %[4]s

FROM node:20-alpine AS builder
WORKDIR /app
COPY --from=deps /app/node_modules ./node_modules
COPY %[1]s. ./
RUN %[5]s

FROM node:20-alpine AS runner
ENV NODE_ENV=production
WORKDIR /app
COPY --from=builder /app/dist ./dist
COPY --from=builder /app/node_modules ./node_modules
COPY --from=builder /app/package.json ./
EXPOSE %[2]d
%[6]s
`, prefix, svc.Port, copyLockfile(prefix, pm), joinArgs(svc.InstallCmd), joinArgs(svc.BuildCmd), dockerCMD(svc.StartCmd))
}

func renderReact(svc detector.Service, prefix string) string {
	pm := svc.PackageManager
	if pm == "" {
		pm = "npm"
	}
	return fmt.Sprintf(`# syntax=docker/dockerfile:1.6
FROM node:20-alpine AS builder
WORKDIR /app
COPY %[1]spackage*.json ./
%[3]s
RUN %[4]s
COPY %[1]s. ./
RUN %[5]s

FROM nginx:alpine AS runner
COPY --from=builder /app/dist /usr/share/nginx/html
COPY --from=builder /app/build /usr/share/nginx/html
EXPOSE 80
CMD ["nginx", "-g", "daemon off;"]
# Original dev start: %[6]s (port %[2]d)
`, prefix, svc.Port, copyLockfile(prefix, pm), joinArgs(svc.InstallCmd), joinArgs(svc.BuildCmd), strings.Join(svc.StartCmd, " "))
}

func renderNode(svc detector.Service, prefix string) string {
	pm := svc.PackageManager
	if pm == "" {
		pm = "npm"
	}
	return fmt.Sprintf(`# syntax=docker/dockerfile:1.6
FROM node:20-alpine
WORKDIR /app
COPY %[1]spackage*.json ./
%[3]s
RUN %[4]s
COPY %[1]s. ./
EXPOSE %[2]d
%[5]s
`, prefix, svc.Port, copyLockfile(prefix, pm), joinArgs(svc.InstallCmd), dockerCMD(svc.StartCmd))
}

func renderFastAPI(svc detector.Service, prefix string) string {
	return fmt.Sprintf(`# syntax=docker/dockerfile:1.6
FROM python:3.12-slim
WORKDIR /app
COPY %[1]srequirements.txt ./
RUN %[3]s
COPY %[1]s. ./
EXPOSE %[2]d
%[4]s
`, prefix, svc.Port, joinArgs(svc.InstallCmd), dockerCMD(svc.StartCmd))
}

func renderFlask(svc detector.Service, prefix string) string {
	return fmt.Sprintf(`# syntax=docker/dockerfile:1.6
FROM python:3.12-slim
WORKDIR /app
COPY %[1]srequirements.txt ./
RUN %[3]s
COPY %[1]s. ./
ENV FLASK_RUN_HOST=0.0.0.0
EXPOSE %[2]d
%[4]s
`, prefix, svc.Port, joinArgs(svc.InstallCmd), dockerCMD(svc.StartCmd))
}

func renderDjango(svc detector.Service, prefix string) string {
	return fmt.Sprintf(`# syntax=docker/dockerfile:1.6
FROM python:3.12-slim
WORKDIR /app
COPY %[1]srequirements.txt ./
RUN %[3]s
COPY %[1]s. ./
EXPOSE %[2]d
%[4]s
`, prefix, svc.Port, joinArgs(svc.InstallCmd), dockerCMD(svc.StartCmd))
}

func renderPython(svc detector.Service, prefix string) string {
	return fmt.Sprintf(`# syntax=docker/dockerfile:1.6
FROM python:3.12-slim
WORKDIR /app
COPY %[1]srequirements.txt ./
RUN pip install --no-cache-dir -r requirements.txt
COPY %[1]s. ./
EXPOSE %[2]d
%[3]s
`, prefix, svc.Port, dockerCMD(svc.StartCmd))
}

func copyLockfile(prefix, pm string) string {
	switch pm {
	case "pnpm":
		return fmt.Sprintf("COPY %spnpm-lock.yaml ./", prefix)
	case "yarn":
		return fmt.Sprintf("COPY %syarn.lock ./", prefix)
	}
	return ""
}

func joinArgs(args []string) string {
	if len(args) == 0 {
		return ""
	}
	return strings.Join(args, " ")
}

func dockerCMD(args []string) string {
	if len(args) == 0 {
		return `CMD ["sh", "-c", "echo no start command detected; sleep infinity"]`
	}
	parts := make([]string, len(args))
	for i, a := range args {
		parts[i] = fmt.Sprintf("%q", a)
	}
	return "CMD [" + strings.Join(parts, ", ") + "]"
}
