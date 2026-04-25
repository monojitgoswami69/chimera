# Chimera

<div align="center">

**Autonomous Environment Orchestration for Any GitHub Repository**

[![Go Version](https://img.shields.io/badge/Go-1.21+-00ADD8?style=flat&logo=go)](https://go.dev/)
[![License](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![PRs Welcome](https://img.shields.io/badge/PRs-welcome-brightgreen.svg)](CONTRIBUTING.md)

*Clone any GitHub repository and get a fully containerized local development environment in under 2 minutes — with zero manual configuration.*

[Quick Start](#-quick-start) • [Features](#-features) • [Commands](#-commands) • [Architecture](#-architecture) • [Contributing](#-contributing)

</div>

---

## 📋 Table of Contents

- [Overview](#-overview)
- [Quick Start](#-quick-start)
- [Installation](#-installation)
- [Commands](#-commands)
  - [setup](#setup)
  - [init](#init)
  - [help](#help)
- [Features](#-features)
- [Supported Technologies](#-supported-technologies)
- [Architecture](#-architecture)
- [Configuration](#-configuration)
- [Output Structure](#-output-structure)
- [Examples](#-examples)
- [Troubleshooting](#-troubleshooting)
- [Contributing](#-contributing)
- [License](#-license)

---

## 🎯 Overview

Chimera is a developer CLI tool that autonomously analyzes GitHub repositories and generates production-ready Docker configurations. It combines static analysis with LLM-powered validation to understand your project's technology stack, dependencies, and environment requirements — then creates everything you need to run it locally.

### What Chimera Does

1. **Clones** any public GitHub repository
2. **Analyzes** the codebase to detect technologies, frameworks, and services
3. **Validates** findings using AI (OpenAI, Anthropic, Groq, Gemini, or Ollama)
4. **Generates** optimized Dockerfiles and docker-compose.yml
5. **Extracts** environment variables and creates .env.example templates
6. **Produces** a quick start guide for immediate development

### Why Chimera?

- **Zero Configuration**: No manual Dockerfile writing or dependency hunting
- **AI-Powered**: LLM validation ensures accurate detection and optimal configurations
- **Multi-Framework**: Supports Node.js, Express, Next.js, React, Python, FastAPI, Flask, Django
- **Production-Ready**: Generates multi-stage builds, proper networking, and security best practices
- **Time-Saving**: From clone to running containers in under 2 minutes

---

## 🚀 Quick Start

### Prerequisites

- **Go 1.21+** ([install](https://go.dev/doc/install))
- **Git** ([install](https://git-scm.com/downloads))
- **Docker** ([install](https://docs.docker.com/get-docker/))
- **LLM API Key** (OpenAI, Anthropic, Groq, Gemini, or Ollama)

### Installation

#### Option 1: Build from Source (Recommended)

```bash
# Clone the repository
git clone https://github.com/monojitgoswami69/chimera.git
cd chimera

# Build the binary
go build -o chimera .

# Move to PATH (optional)
sudo mv chimera /usr/local/bin/
# OR for user-only install
mkdir -p ~/.local/bin
mv chimera ~/.local/bin/
export PATH="$HOME/.local/bin:$PATH"
```

#### Option 2: One-Line Install Script

```bash
curl -fsSL https://raw.githubusercontent.com/monojitgoswami69/chimera/main/install.sh | bash
```

### First-Time Setup

```bash
# Run the interactive setup wizard
chimera setup
```

The setup wizard will guide you through:
1. Selecting your LLM provider (OpenAI, Anthropic, Groq, Gemini, Ollama)
2. Choosing a model
3. Entering your API key
4. (Optional) Adding a GitHub Personal Access Token for private repos

### Your First Project

```bash
# Initialize any GitHub repository
chimera init https://github.com/tiangolo/fastapi

# Navigate to the output directory
cd ~/.chimera/repos/fastapi/chimera-outputs

# Review and configure environment variables
cp env-vars/root/.env.example ../.env
nano ../.env  # Add your secrets

# Start the containers
docker compose up
```

That's it! Your application is now running in Docker.

---

## 📦 Installation

### Building from Source

```bash
# Clone the repository
git clone https://github.com/monojitgoswami69/chimera.git
cd chimera

# Install dependencies
go mod download

# Build the binary
go build -o chimera .

# Verify installation
./chimera --version
```

### Development Build

```bash
# Build with version information
go build -ldflags "-X chimera/cmd.Version=v0.1.0 -X chimera/cmd.BuildTime=$(date -u +%Y-%m-%dT%H:%M:%SZ)" -o chimera .
```

### System-Wide Installation

```bash
# Linux/macOS
sudo cp chimera /usr/local/bin/

# Verify
chimera --version
```

---

## 🎮 Commands

### `setup`

Interactive configuration wizard for first-time setup.

```bash
chimera setup
```

**What it does:**
- Prompts for LLM provider selection (OpenAI, Anthropic, Groq, Gemini, Ollama)
- Fetches available models from the provider's API
- Securely stores API key and configuration
- Optionally configures GitHub Personal Access Token

**Configuration Location:** `~/.chimera/config.json`

**Example:**
```bash
$ chimera setup

██████╗██╗  ██╗██╗███╗   ███╗███████╗██████╗  █████╗ 
██╔════╝██║  ██║██║████╗ ████║██╔════╝██╔══██╗██╔══██╗
██║     ███████║██║██╔████╔██║█████╗  ██████╔╝███████║
██║     ██╔══██║██║██║╚██╔╝██║██╔══╝  ██╔══██╗██╔══██║
╚██████╗██║  ██║██║██║ ╚═╝ ██║███████╗██║  ██║██║  ██║
 ╚═════╝╚═╝  ╚═╝╚═╝╚═╝     ╚═╝╚══════╝╚═╝  ╚═╝╚═╝  ╚═╝

Select LLM Provider:
  ❯ OpenAI
    Anthropic
    Groq
    Gemini
    Ollama
```

---

### `init`

Clone a GitHub repository and generate a complete Docker environment.

```bash
chimera init <github-url> [flags]
```

**Flags:**
- `--force` - Force re-initialization even if directory exists
- `--no-agent` - Disable LLM validation (use static analysis only)
- `-v, --verbose` - Enable verbose output (show LLM responses, file reads)
- `-q, --quiet` - Suppress non-error output (minimal mode)

**What it does:**

1. **Clone Repository** - Downloads the repo to `~/.chimera/repos/<repo-name>/`
2. **Generate File Tree** - Creates a smart tree (excludes node_modules, venv, .git, etc.)
3. **Detect Technologies** - Static analysis identifies frameworks and languages
4. **LLM Validation** - AI validates detection and requests additional files if needed
5. **Generate Dockerfiles** - Creates optimized multi-stage Dockerfiles
6. **Generate Compose** - Produces docker-compose.yml with proper networking
7. **Extract Environment Variables** - Scans code for env var patterns
8. **LLM Enhancement** - AI generates comprehensive .env.example files
9. **Write Outputs** - Saves everything to `chimera-outputs/`

**Examples:**

```bash
# Basic usage
chimera init https://github.com/tiangolo/fastapi

# Force re-initialization
chimera init https://github.com/user/repo --force

# Skip LLM validation (faster, less accurate)
chimera init https://github.com/user/repo --no-agent

# Quiet mode (minimal output)
chimera init https://github.com/user/repo --quiet

# Verbose mode (show all LLM interactions)
chimera init https://github.com/user/repo --verbose

# Combined flags
chimera init https://github.com/user/repo --force --verbose
```

**Output:**
```
✓ Cloned fastapi (127 files, 2.3 MB)
✓ Generated tree (245 lines)
✓ Detected 2 service(s) via static analysis
✓ LLM provided corrections
✓ Generated Dockerfile and docker-compose.yml
✓ LLM corrected Dockerfile
✓ Detected 8 environment variable(s)
✓ LLM enhanced environment variables

Files written to: ~/.chimera/repos/fastapi/chimera-outputs
```

---

### `help`

Display help information about Chimera and its commands.

```bash
chimera help
chimera <command> --help
```

**Examples:**
```bash
chimera help
chimera init --help
chimera setup --help
```

---

### Global Flags

Available for all commands:

- `-v, --verbose` - Enable verbose output (show LLM responses, detailed logs)
- `-q, --quiet` - Suppress non-error output
- `--version` - Show version and build information

```bash
chimera --version
chimera init <url> --verbose
chimera init <url> --quiet
```

---

## ✨ Features

### 🤖 AI-Powered Analysis

- **Multi-Provider Support**: OpenAI, Anthropic, Groq, Gemini, Ollama
- **Iterative Validation**: LLM can request up to 5 files per iteration, 3 attempts max
- **Smart Corrections**: AI validates and corrects technology detection
- **Context-Aware**: Reads actual source files to make informed decisions

### 🔍 Technology Detection

**Supported Frameworks:**
- **JavaScript/TypeScript**: Node.js, Express, Next.js, React
- **Python**: FastAPI, Flask, Django

**Detection Methods:**
- Package.json analysis (dependencies, scripts, devDependencies)
- Requirements.txt parsing
- File structure patterns (pages/, app/, api/)
- Configuration file detection (next.config.js, manage.py, etc.)

### 🐳 Docker Generation

**Dockerfile Features:**
- Multi-stage builds for optimized image size
- Framework-specific optimizations
- Proper layer caching
- Security best practices (non-root users)
- Production-ready configurations

**docker-compose.yml Features:**
- Service orchestration
- Network configuration
- Volume management
- Port mapping
- Environment variable injection
- Health checks

### 🔐 Environment Variable Management

**Detection Patterns:**
- Python: `os.getenv()`, `os.environ[]`, `config()`, `settings.`
- JavaScript: `process.env.`, `import.meta.env.`
- Configuration files: .env.example, .env.sample

**LLM Enhancement:**
- Reads existing .env files
- Generates comprehensive .env.example
- Adds descriptive comments
- Suggests default values
- Groups related variables

### 📊 Smart File Tree

**Exclusions:**
- node_modules/, venv/, __pycache__/
- .git/, .next/, .nuxt/, dist/, build/
- Binary files and large assets
- IDE configurations

**Features:**
- Configurable depth limit (default: 10,000 lines)
- Size-aware truncation
- Clean, readable output

### 🎨 Beautiful CLI

- **Colored Output**: Syntax-highlighted, easy-to-read terminal UI
- **Progress Indicators**: Real-time feedback on operations
- **Interactive Selectors**: Arrow-key navigation for choices
- **Error Handling**: Clear, actionable error messages
- **Responsive Design**: Adapts to terminal width

---

## 🛠 Supported Technologies

### Languages & Runtimes

| Language | Versions | Frameworks |
|----------|----------|------------|
| **JavaScript/TypeScript** | Node.js 18+ | Express, Next.js, React |
| **Python** | 3.9+ | FastAPI, Flask, Django |

### LLM Providers

| Provider | Models | API Key Required |
|----------|--------|------------------|
| **OpenAI** | GPT-4, GPT-3.5-turbo | Yes |
| **Anthropic** | Claude 3 (Opus, Sonnet, Haiku) | Yes |
| **Groq** | Llama 3, Mixtral | Yes |
| **Google Gemini** | Gemini Pro, Gemini Flash | Yes |
| **Ollama** | Any local model | No (local) |

### Coming Soon

- Go (Gin, Echo, Fiber)
- Ruby (Rails, Sinatra)
- Java (Spring Boot)
- PHP (Laravel, Symfony)
- Rust (Actix, Rocket)

---

## 🏗 Architecture

### Project Structure

```
chimera/
├── cmd/                    # Command implementations
│   ├── root.go            # Root command & global flags
│   ├── init.go            # Init command (main workflow)
│   ├── setup.go           # Setup wizard
│   └── help.go            # Help command
├── internal/              # Internal packages
│   ├── config/            # Configuration management
│   │   └── config.go      # Load/save config, paths
│   ├── detector/          # Technology detection
│   │   └── detector.go    # Static analysis engine
│   ├── envvar/            # Environment variable extraction
│   │   └── envvar.go      # Pattern matching, generation
│   ├── generator/         # Docker file generation
│   │   ├── dockerfile.go  # Dockerfile templates
│   │   └── compose.go     # docker-compose.yml generation
│   ├── git/               # Git operations
│   │   └── git.go         # Clone, validation, file counting
│   ├── llm/               # LLM client
│   │   └── client.go      # Multi-provider API client
│   ├── tree/              # File tree generation
│   │   └── tree.go        # Smart tree with exclusions
│   └── ui/                # Terminal UI components
│       ├── banner.go      # ASCII art headers
│       ├── colors.go      # Color schemes
│       ├── spinner.go     # Loading indicators
│       └── table.go       # Table rendering
├── main.go                # Entry point
├── go.mod                 # Go module definition
└── README.md              # This file
```

### Workflow Diagram

```
┌─────────────────────────────────────────────────────────────┐
│                     chimera init <url>                       │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
                    ┌─────────────────┐
                    │  Load Config    │
                    │  Validate URL   │
                    └────────┬────────┘
                             │
                             ▼
                    ┌─────────────────┐
                    │   Git Clone     │
                    │   Repository    │
                    └────────┬────────┘
                             │
                             ▼
                    ┌─────────────────┐
                    │  Generate Tree  │
                    │  (exclude dirs) │
                    └────────┬────────┘
                             │
                             ▼
                    ┌─────────────────┐
                    │ Static Analysis │
                    │ Detect Services │
                    └────────┬────────┘
                             │
                             ▼
                    ┌─────────────────┐
                    │ LLM Validation  │◄──┐
                    │ Request Files   │   │ Up to 3
                    │ Correct Results │───┘ iterations
                    └────────┬────────┘
                             │
                             ▼
                    ┌─────────────────┐
                    │    Generate     │
                    │   Dockerfile    │
                    │ docker-compose  │
                    └────────┬────────┘
                             │
                             ▼
                    ┌─────────────────┐
                    │ LLM Validation  │
                    │ Correct Configs │
                    └────────┬────────┘
                             │
                             ▼
                    ┌─────────────────┐
                    │  Extract EnvVars│
                    │  Pattern Match  │
                    └────────┬────────┘
                             │
                             ▼
                    ┌─────────────────┐
                    │ LLM Enhancement │
                    │ Generate .env   │
                    └────────┬────────┘
                             │
                             ▼
                    ┌─────────────────┐
                    │  Write Outputs  │
                    │ chimera-outputs/│
                    └─────────────────┘
```

### Key Design Decisions

1. **Static Analysis First**: Fast initial detection without API calls
2. **LLM Validation**: AI confirms and corrects findings for accuracy
3. **Iterative File Requests**: LLM can request specific files to make informed decisions
4. **Multi-Stage Dockerfiles**: Optimized for production (smaller images, faster builds)
5. **Progressive CLI**: Natural scrolling output, no screen clearing
6. **Platform-Universal**: Works on Linux, macOS, Windows (via WSL)

---

## ⚙️ Configuration

### Configuration File

Location: `~/.chimera/config.json`

```json
{
  "llm_provider": "openai",
  "llm_model": "gpt-4",
  "llm_api_key": "sk-...",
  "github_pat": "ghp_..."
}
```

### Supported Providers

#### OpenAI
```bash
export OPENAI_API_KEY="sk-..."
chimera setup  # Select OpenAI
```

#### Anthropic
```bash
export ANTHROPIC_API_KEY="sk-ant-..."
chimera setup  # Select Anthropic
```

#### Groq
```bash
export GROQ_API_KEY="gsk_..."
chimera setup  # Select Groq
```

#### Google Gemini
```bash
export GEMINI_API_KEY="..."
chimera setup  # Select Gemini
```

#### Ollama (Local)
```bash
# Start Ollama server
ollama serve

# Pull a model
ollama pull llama3

# Configure Chimera
chimera setup  # Select Ollama
```

### GitHub Personal Access Token

For private repositories:

1. Go to https://github.com/settings/tokens
2. Generate new token (classic)
3. Select scope: `repo` (Full control of private repositories)
4. Run `chimera setup` and enter the token when prompted

---

## 📁 Output Structure

After running `chimera init`, outputs are saved to:

```
~/.chimera/repos/<repo-name>/chimera-outputs/
├── Dockerfile                          # Multi-stage Dockerfile
├── docker-compose.yml                  # Service orchestration
├── .gitignore                          # Ignore all outputs
├── quick_start_guide.txt               # Getting started instructions
└── env-vars/                           # Environment variables by service
    ├── backend/
    │   └── .env.example
    ├── frontend/
    │   └── .env.example
    └── root/
        └── .env.example
```

### Example Dockerfile

```dockerfile
# Backend Stage
FROM python:3.11-slim AS backend
WORKDIR /app/backend
COPY backend/requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt
COPY backend/ .
EXPOSE 8000
CMD ["uvicorn", "main:app", "--host", "0.0.0.0", "--port", "8000"]

# Frontend Stage
FROM node:18-alpine AS frontend
WORKDIR /app/frontend
COPY frontend/package*.json ./
RUN npm ci --only=production
COPY frontend/ .
RUN npm run build
EXPOSE 3000
CMD ["npm", "start"]
```

### Example docker-compose.yml

```yaml
version: '3.8'

services:
  backend:
    build:
      context: ..
      dockerfile: chimera-outputs/Dockerfile
      target: backend
    ports:
      - "8000:8000"
    environment:
      - DATABASE_URL=${DATABASE_URL}
      - SECRET_KEY=${SECRET_KEY}
    networks:
      - app-network

  frontend:
    build:
      context: ..
      dockerfile: chimera-outputs/Dockerfile
      target: frontend
    ports:
      - "3000:3000"
    environment:
      - NEXT_PUBLIC_API_URL=http://backend:8000
    depends_on:
      - backend
    networks:
      - app-network

networks:
  app-network:
    driver: bridge
```

### Example .env.example

```bash
# Database Configuration
DATABASE_URL=postgresql://user:password@localhost:5432/dbname

# API Keys
SECRET_KEY=your-secret-key-here
API_KEY=your-api-key-here

# External Services
REDIS_URL=redis://localhost:6379
SMTP_HOST=smtp.gmail.com
SMTP_PORT=587

# Application Settings
DEBUG=false
LOG_LEVEL=info
```

---

## 📚 Examples

### Example 1: FastAPI Backend

```bash
chimera init https://github.com/tiangolo/fastapi
cd ~/.chimera/repos/fastapi/chimera-outputs
cp env-vars/root/.env.example ../.env
docker compose up
```

**Output:**
- Detects: Python, FastAPI
- Generates: Dockerfile with uvicorn, docker-compose.yml
- Extracts: DATABASE_URL, SECRET_KEY, etc.

### Example 2: Next.js + Express

```bash
chimera init https://github.com/vercel/next.js/tree/canary/examples/with-docker
cd ~/.chimera/repos/with-docker/chimera-outputs
docker compose up
```

**Output:**
- Detects: Next.js (frontend), Express (backend)
- Generates: Multi-stage Dockerfile, docker-compose.yml with 2 services
- Extracts: NEXT_PUBLIC_*, API_URL, etc.

### Example 3: Django Project

```bash
chimera init https://github.com/django/django
cd ~/.chimera/repos/django/chimera-outputs
cp env-vars/root/.env.example ../.env
nano ../.env  # Configure DATABASE_URL, SECRET_KEY
docker compose up
```

**Output:**
- Detects: Python, Django
- Generates: Dockerfile with gunicorn, docker-compose.yml
- Extracts: SECRET_KEY, DATABASE_URL, ALLOWED_HOSTS

### Example 4: Verbose Mode

```bash
chimera init https://github.com/user/repo --verbose
```

**Shows:**
- LLM request/response payloads
- File reads: `→ Reading package.json`
- Detailed validation steps
- Raw API responses

### Example 5: Quiet Mode

```bash
chimera init https://github.com/user/repo --quiet
```

**Shows:**
- Only errors
- No progress indicators
- No completion box
- Minimal output for scripting

---

## 🐛 Troubleshooting

### Common Issues

#### 1. "Chimera is not configured"

**Solution:**
```bash
chimera setup
```

#### 2. "Invalid GitHub URL"

**Solution:** Ensure URL format is `https://github.com/<owner>/<repo>`

```bash
# ✓ Correct
chimera init https://github.com/tiangolo/fastapi

# ✗ Incorrect
chimera init github.com/tiangolo/fastapi
chimera init git@github.com:tiangolo/fastapi.git
```

#### 3. "Directory already exists"

**Solution:** Use `--force` flag

```bash
chimera init https://github.com/user/repo --force
```

#### 4. "LLM validation failed"

**Possible causes:**
- Invalid API key
- Rate limiting
- Network issues

**Solution:**
```bash
# Skip LLM validation
chimera init https://github.com/user/repo --no-agent

# Or reconfigure
chimera setup
```

#### 5. "No supported project types detected"

**Cause:** Repository doesn't contain supported frameworks

**Solution:**
- Check if repo uses Node.js, Python, Express, Next.js, FastAPI, Flask, or Django
- Open an issue if you believe detection failed incorrectly

#### 6. Docker build fails

**Solution:**
```bash
# Check Dockerfile syntax
cd ~/.chimera/repos/<repo-name>/chimera-outputs
docker build -f Dockerfile ..

# Check logs
docker compose logs
```

### Debug Mode

```bash
# Enable verbose output
chimera init <url> --verbose

# Check configuration
cat ~/.chimera/config.json

# Check logs
ls -la ~/.chimera/repos/<repo-name>/
```

### Getting Help

1. Check this README
2. Run `chimera <command> --help`
3. Open an issue: https://github.com/monojitgoswami69/chimera/issues
4. Check existing issues for solutions

---

## 🤝 Contributing

We welcome contributions! Here's how you can help:

### Reporting Bugs

1. Check existing issues
2. Create a new issue with:
   - Clear title
   - Steps to reproduce
   - Expected vs actual behavior
   - Chimera version (`chimera --version`)
   - OS and Go version

### Suggesting Features

1. Open an issue with `[Feature Request]` prefix
2. Describe the feature and use case
3. Explain why it would be useful

### Contributing Code

1. Fork the repository
2. Create a feature branch: `git checkout -b feature/amazing-feature`
3. Make your changes
4. Write tests if applicable
5. Commit: `git commit -m 'Add amazing feature'`
6. Push: `git push origin feature/amazing-feature`
7. Open a Pull Request

### Development Setup

```bash
# Clone your fork
git clone https://github.com/YOUR_USERNAME/chimera.git
cd chimera

# Install dependencies
go mod download

# Build
go build -o chimera .

# Run tests
go test ./...

# Run with local changes
./chimera init <url>
```

### Code Style

- Follow Go conventions
- Use `gofmt` for formatting
- Add comments for exported functions
- Keep functions focused and small

---

## 📄 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

---

## 🙏 Acknowledgments

- [Cobra](https://github.com/spf13/cobra) - CLI framework
- [Bubble Tea](https://github.com/charmbracelet/bubbletea) - TUI framework
- [Lipgloss](https://github.com/charmbracelet/lipgloss) - Terminal styling
- All LLM providers for their APIs
- The open-source community

---

## 📞 Contact

- **GitHub**: [@monojitgoswami69](https://github.com/monojitgoswami69)
- **Issues**: [GitHub Issues](https://github.com/monojitgoswami69/chimera/issues)
- **Discussions**: [GitHub Discussions](https://github.com/monojitgoswami69/chimera/discussions)

---

<div align="center">

**Made with ❤️ by developers, for developers**

[⬆ Back to Top](#chimera)

</div>
