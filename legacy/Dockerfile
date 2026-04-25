# Multi-stage Dockerfile for Project Chimera
# Produces a minimal static binary

# Stage 1: Build
FROM golang:1.21-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git make

# Set working directory
WORKDIR /build

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build static binary with CGO_ENABLED=0
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-w -s -X 'github.com/projectchimera/chimera/cmd.Version=docker' -X 'github.com/projectchimera/chimera/cmd.BuildTime=$(date -u '+%Y-%m-%d_%H:%M:%S')'" \
    -o chimera \
    .

# Stage 2: Runtime
FROM alpine:latest

# Install runtime dependencies
RUN apk add --no-cache \
    ca-certificates \
    git \
    docker-cli

# Create non-root user
RUN addgroup -g 1000 chimera && \
    adduser -D -u 1000 -G chimera chimera

# Copy binary from builder
COPY --from=builder /build/chimera /usr/local/bin/chimera

# Set user
USER chimera

# Set working directory
WORKDIR /workspace

# Entry point
ENTRYPOINT ["chimera"]
CMD ["--help"]
