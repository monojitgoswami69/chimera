# Project Chimera 🔥

**Autonomous Environment Orchestration CLI**

Chimera eliminates the friction of local development environment setup by transforming it into a fully automated, intelligent, and reproducible process. Clone any repository and get a fully functional containerized development environment with a single command.

[![Go Version](https://img.shields.io/badge/Go-1.25+-00ADD8?style=flat&logo=go)](https://golang.org)
[![License](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

## 🎯 What is Chimera?

Chimera is a CLI tool that accepts a GitHub repository URL and autonomously:

- 🔍 **Analyzes the codebase** to detect languages, frameworks, and dependencies
- 🐳 **Generates Docker configurations** (docker-compose.yml, Dockerfile, .env.example)
- 🔧 **Resolves port conflicts** automatically
- 🌐 **Sets up reverse proxy** with custom local domains (e.g., `http://project-name.local`)
- 🤖 **Self-heals failures** using AI-powered diagnostics
- 📊 **Monitors resources** with real-time telemetry
- 🧹 **Cleans up completely** with a single nuke command

## ✨ Features

### Core Functionality

#### 1. Intelligent Code Scanner
- Detects environment variables from source code patterns (`process.env`, `os.getenv`, etc.)
- Identifies database connection strings and service dependencies
- Infers required infrastructure (PostgreSQL, MongoDB, Redis, MySQL, RabbitMQ, Elasticsearch)
- Supports **Node.js**, **Python**, and **Go** ecosystems

#### 2. Automated Environment Provisioning
- Dynamically generates `docker-compose.yml` with appropriate service definitions
- Creates `Dockerfile` with correct base images and build steps
- Generates `.env.example` with all detected environment variables
- Implements dependency-aware startup with Docker healthchecks

#### 3. Port Conflict Resolution Engine
- Detects port conflicts on the host machine before deployment
- Suggests alternative ports automatically
- Updates configurations dynamically and notifies the user

#### 4. Automated Reverse Proxy ("Shadow Proxy")
- Provisions a Caddy reverse proxy container automatically
- Maps each project to a custom local domain (e.g., `http://my-app.local`)
- Handles routing and `/etc/hosts` resolution seamlessly

#### 5. AI-Powered Self-Healing System
- Monitors container failures during runtime
- Captures and analyzes error logs
- Integrates with LLM APIs (OpenAI, Gemini, Groq) to diagnose issues
- Suggests actionable fixes directly in the CLI

#### 6. Real-Time System Telemetry
- Live dashboard showing CPU usage, memory consumption, and network stats
- Fetches data directly from Docker daemon
- Updates every 2 seconds with terminal resize support

#### 7. Environment Cleanup Utility ("Nuke Command")
- Stops all related containers
- Removes volumes and orphaned images
- Cleans `/etc/hosts` entries
- Ensures the host system remains clean and resource-efficient

## 🚀 Quick Start

### Installation

```bash
# Clone the repository
git clone https://github.com/projectchimera/chimera.git
cd chimera

# Build the binary
make build

# Install to $GOPATH/bin (optional)
make install
```

### Setup AI Agent (Recommended)

Run the interactive setup wizard to configure AI-powered analysis:

```bash
./build/chimera setup
```

The wizard will:
1. Help you select an LLM provider (OpenAI, Gemini, or Groq)
2. Validate your API key
3. Let you choose from available models
4. Save configuration to `.chimera.env` in the current directory

### Basic Usage

```bash
# Initialize a new environment from any GitHub repository
chimera init https://github.com/user/repo

# With Docker execution
chimera init https://github.com/user/repo --docker-run

# With reverse proxy
chimera init https://github.com/user/repo --docker-run --create-proxy

# Without AI agent (template-based)
chimera init https://github.com/user/repo --no-agent

# View live container statistics
chimera stats

# Run AI diagnostics on failing containers
chimera diagnose

# Completely tear down the environment
chimera nuke
```

## 📖 Detailed Usage

### `chimera setup`

Interactive setup wizard for configuring AI agent.

```bash
chimera setup
```

**What it does:**
- Guides you through provider selection (OpenAI, Gemini, Groq)
- Validates your API key by fetching available models
- Lets you select a model with recommendations
- Saves configuration to `./.chimera.env`

**Configuration Priority:**
1. Environment variables (highest priority)
2. `./.chimera.env` (current directory)
3. `~/.chimera.env` (home directory)

### `chimera init`

Initialize a fully functional development environment from a GitHub repository.

```bash
chimera init <github-repo-url> [flags]

Flags:
  --cwd string         Working directory for the workspace (default: ./chimera-<repo-name>)
  --force              Force re-initialization even if workspace exists
  --docker-run         Start Docker containers after generating configs
  --create-proxy       Set up Caddy reverse proxy and /etc/hosts entry
  --no-agent           Disable AI agent (use template-based generation)
  -v, --verbose        Enable verbose output
  -q, --quiet          Suppress non-error output
```

**Default Behavior (without flags):**
- Uses AI agent mode (if configured via `chimera setup`)
- Generates configuration files only (no Docker execution)
- No reverse proxy setup
- Creates `quick_start_guide.txt` with detailed instructions

**If AI agent is not configured:**
- Automatically falls back to template-based generation
- Displays a message suggesting to run `chimera setup`
- Still generates working configurations

**Examples:**

```bash
# Basic initialization (generates files only)
chimera init https://github.com/user/my-app

# Generate and start Docker containers
chimera init https://github.com/user/my-app --docker-run

# Full setup with proxy and Docker
chimera init https://github.com/user/my-app --docker-run --create-proxy

# Template-based (no AI)
chimera init https://github.com/user/my-app --no-agent

# Private repository (requires GITHUB_TOKEN or SSH)
export GITHUB_TOKEN=ghp_xxxxx
chimera init https://github.com/user/private-repo

# Custom workspace directory
chimera init https://github.com/user/repo --cwd ./my-workspace
```

**What happens during init:**

1. ✅ Clones the repository (supports private repos via `GITHUB_TOKEN` or SSH)
2. ✅ Scans for languages, dependencies, and environment variables (supports monorepos)
3. ✅ Generates `docker-compose.yml`, `Dockerfile`, `.env.example`, and `quick_start_guide.txt`
4. ✅ Resolves port conflicts automatically
5. ✅ Optionally sets up Caddy reverse proxy and local domain (with `--create-proxy`)
6. ✅ Optionally starts all containers with health checks (with `--docker-run`)
7. ✅ Watches for failures and triggers AI diagnostics (when Docker is running)

### `chimera stats`

Display a live TUI dashboard with real-time container statistics.

```bash
chimera stats [flags]

Flags:
  --project string   Project name (auto-detected from .chimera file)
  --once             Print one snapshot and exit (for CI use)
```

**Example:**

```bash
# Launch interactive dashboard
chimera stats

# Print one-time snapshot
chimera stats --once
```

### `chimera diagnose`

Run AI-powered diagnostics on failing containers.

```bash
chimera diagnose [flags]

Flags:
  --project string   Project name (auto-detected from .chimera file)
```

**Setup AI Diagnostics:**

```bash
# OpenAI (default)
export CHIMERA_LLM_PROVIDER=openai
export OPENAI_API_KEY=sk-xxxxx

# Google Gemini
export CHIMERA_LLM_PROVIDER=gemini
export GEMINI_API_KEY=xxxxx

# Groq
export CHIMERA_LLM_PROVIDER=groq
export GROQ_API_KEY=xxxxx
```

### `chimera nuke`

Completely tear down and remove all containers, volumes, and hosts entries.

```bash
chimera nuke [flags]

Flags:
  --project string   Project name (auto-detected from .chimera file)
  --force            Skip confirmation prompt
```

**Example:**

```bash
# Interactive cleanup (asks for confirmation)
chimera nuke

# Force cleanup without confirmation
chimera nuke --force
```

## 🛠️ Configuration

### Environment Variables

Chimera can be configured via environment variables in `.chimera.env` files:

- **User-level config:** `~/.chimera.env`
- **Workspace-level config:** `./.chimera.env`

**Example `.chimera.env`:**

```bash
# AI Diagnostics
CHIMERA_LLM_PROVIDER=openai
OPENAI_API_KEY=sk-xxxxx

# GitHub Access (for private repos)
GITHUB_TOKEN=ghp_xxxxx

# Agent Mode (use AI for config generation)
CHIMERA_AGENT=1
```

### Supported Languages

| Language | Detection Files | Default Version |
|----------|----------------|-----------------|
| Node.js  | `package.json` | 20 |
| Python   | `requirements.txt`, `Pipfile`, `pyproject.toml` | 3.11 |
| Go       | `go.mod` | 1.21 |

### Supported Infrastructure

| Service | Detection Patterns |
|---------|-------------------|
| PostgreSQL | `pg`, `postgres`, `psycopg2`, `pgx` |
| MySQL | `mysql`, `mysql2`, `pymysql` |
| MongoDB | `mongoose`, `mongodb`, `pymongo` |
| Redis | `redis`, `ioredis`, `go-redis` |
| RabbitMQ | `amqplib`, `pika`, `amqp091-go` |
| Elasticsearch | `elasticsearch`, `@elastic/elasticsearch` |

## 🧪 Development

### Prerequisites

- Go 1.25+
- Docker 24.0+
- Make

### Build from Source

```bash
# Download dependencies
make deps

# Run tests
make test

# Build binary
make build

# Run with race detector
make dev

# Build for all platforms
make build-all
```

### Project Structure

```
chimera/
├── cmd/                    # CLI commands
│   ├── root.go            # Root command and global flags
│   ├── init.go            # Init command implementation
│   ├── stats.go           # Stats dashboard command
│   ├── diagnose.go        # AI diagnostics command
│   └── nuke.go            # Cleanup command
├── internal/              # Internal packages
│   ├── scanner/           # Code and dependency scanner
│   ├── compose/           # Docker Compose generator
│   ├── ports/             # Port conflict resolution
│   ├── proxy/             # Reverse proxy and hosts management
│   ├── healer/            # AI-powered self-healing
│   ├── docker/            # Docker client wrapper
│   ├── git/               # Git operations
│   ├── agent/             # AI agent for config generation
│   ├── nuke/              # Cleanup utilities
│   └── tui/               # Terminal UI components
├── main.go                # Entry point
├── Makefile               # Build automation
└── go.mod                 # Go module definition
```

### Running Tests

```bash
# Run all tests
make test

# Run tests with coverage
make test-coverage

# Run specific package tests
go test -v ./internal/scanner/...
```

## 🤝 Contributing

Contributions are welcome! Please follow these guidelines:

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

### Code Style

- Follow standard Go conventions
- Run `make lint` before committing
- Add tests for new features
- Update documentation as needed

## 📋 Requirements Checklist

### Core Functional Requirements ✅

- [x] **Intelligent Code Scanner** - Detects env vars, databases, and dependencies
- [x] **Automated Environment Provisioning** - Generates docker-compose.yml and .env.example
- [x] **Multi-Language Compatibility** - Supports Node.js, Python, and Go
- [x] **Dependency-Aware Startup** - Implements Docker healthchecks

### Advanced Features ✅

- [x] **Port Conflict Resolution Engine** - Detects and resolves conflicts automatically
- [x] **Automated Reverse Proxy** - Caddy proxy with custom local domains
- [x] **AI-Powered Self-Healing** - LLM integration for diagnostics
- [x] **Real-Time System Telemetry** - Live dashboard with Docker stats
- [x] **Environment Cleanup Utility** - Complete nuke command

## 🐛 Known Issues & Limitations

- `/etc/hosts` modification requires sudo on Linux/macOS
- AI diagnostics require API keys for OpenAI, Gemini, or Groq
- Port 80/443 may require elevated privileges
- Windows support is experimental

## 📄 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## 🙏 Acknowledgments

- [Cobra](https://github.com/spf13/cobra) - CLI framework
- [Bubble Tea](https://github.com/charmbracelet/bubbletea) - TUI framework
- [Docker SDK](https://github.com/docker/docker) - Docker API client
- [go-git](https://github.com/go-git/go-git) - Git operations

## 📞 Support

- 📧 Email: support@projectchimera.dev
- 🐛 Issues: [GitHub Issues](https://github.com/projectchimera/chimera/issues)
- 💬 Discussions: [GitHub Discussions](https://github.com/projectchimera/chimera/discussions)

---

**Made with ❤️ by the Chimera Team**
