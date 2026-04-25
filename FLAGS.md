# Chimera v4 - Command Flags

## Overview

Chimera v4 supports command-line flags to customize behavior.

---

## `chimera setup` Flags

### `--force`

**Purpose:** Force reconfiguration even if config already exists

**Default:** `false`

**Behavior:**
- **Without `--force`**: If `~/.chimera/.chimera.env` exists, shows a message and exits
- **With `--force`**: Runs the setup wizard and overwrites existing configuration

**Use Cases:**
- Change LLM provider
- Update API key
- Change model
- Add or update GitHub PAT
- Fix corrupted configuration

**Examples:**

```bash
# First time setup
chimera setup

# Reconfigure (will show warning if config exists)
chimera setup

# Force reconfiguration (overwrites existing)
chimera setup --force
```

**Output (without --force, config exists):**

```
╭─────────────────────────────────────────────────╮
│  ⚠  Configuration already exists                │
╰─────────────────────────────────────────────────╯

  ℹ  Config file: /home/user/.chimera/.chimera.env

To reconfigure, run:
  chimera setup --force
```

**Output (with --force):**

```
  ⚠  Removing existing configuration (--force)

[Runs full setup wizard...]
```

---

## `chimera init` Flags

### `--force`

**Purpose:** Force re-initialization even if directory exists

**Default:** `false`

**Behavior:**
- **Without `--force`**: If `~/.chimera/repos/<repo-name>/` exists, shows error and exits
- **With `--force`**: Removes existing directory and clones fresh

**Use Cases:**
- Re-clone repository with latest changes
- Fix corrupted clone
- Start fresh after failed initialization
- Update repository to latest version

**Examples:**

```bash
# First time init
chimera init https://github.com/tiangolo/fastapi

# Try to init again (will fail)
chimera init https://github.com/tiangolo/fastapi
# Error: directory fastapi already exists. Use --force to re-initialize

# Force re-initialization
chimera init https://github.com/tiangolo/fastapi --force
```

**Output (without --force, directory exists):**

```
╭─────────────────────────────────────────────────╮
│ Error: directory fastapi already exists.       │
│ Use --force to re-initialize                   │
╰─────────────────────────────────────────────────╯
```

**Output (with --force):**

```
  ⚠  Removing existing directory fastapi (--force)
  ⠹  Cloning tiangolo/fastapi...
  ✓  Cloned fastapi (247 files, 4.1 MB)
```

---

### `--no-agent`

**Purpose:** Disable LLM validation (use static analysis only)

**Default:** `false`

**Behavior:**
- **Without `--no-agent`**: Uses LLM to validate detection, Dockerfiles, and env vars
- **With `--no-agent`**: Skips all LLM validation steps, relies on static analysis only

**Use Cases:**
- Save API costs
- Faster execution (no LLM calls)
- Offline usage (no internet required for LLM)
- Testing static analysis
- When LLM is unavailable or rate-limited

**Examples:**

```bash
# With LLM validation (default)
chimera init https://github.com/tiangolo/fastapi

# Without LLM validation
chimera init https://github.com/tiangolo/fastapi --no-agent
```

**Output (with LLM - default):**

```
  ⠹  Validating with LLM...
  ✓  LLM validation passed
```

**Output (with --no-agent):**

```
  ℹ  LLM validation skipped (--no-agent mode)
```

**What's Skipped:**
1. Detection validation (Step 4)
2. Dockerfile validation (Step 6)
3. Environment variable validation (Step 8)

**What Still Runs:**
1. Clone (Step 1)
2. Tree generation (Step 2)
3. Static analysis (Step 3)
4. Dockerfile generation (Step 5)
5. Env var detection (Step 7)
6. Write outputs (Step 9)

---

## Combining Flags

You can combine multiple flags:

```bash
# Force re-init without LLM
chimera init https://github.com/user/repo --force --no-agent

# Force re-init with LLM (default)
chimera init https://github.com/user/repo --force
```

---

## Flag Reference Table

| Command | Flag | Type | Default | Description |
|---------|------|------|---------|-------------|
| `setup` | `--force` | bool | `false` | Force reconfiguration |
| `init` | `--force` | bool | `false` | Force re-initialization |
| `init` | `--no-agent` | bool | `false` | Disable LLM validation |

---

## Help Text

### `chimera setup --help`

```
Interactive wizard to configure Chimera with your LLM provider, API key, and optional GitHub PAT.

This creates ~/.chimera/.chimera.env with your configuration.

Flags:
  --force    Force reconfiguration even if config already exists

Examples:
  chimera setup
  chimera setup --force

Use "chimera [command] --help" for more information about a command.
```

### `chimera init --help`

```
Clone a GitHub repository, analyze its stack, and generate Docker configurations.

By default, this command will:
  1. Clone the repository to ~/.chimera/repos/<repo-name>/
  2. Generate a smart file tree (excluding node_modules, venv, etc.)
  3. Detect technologies and frameworks (JS/TS, Python)
  4. Validate findings with LLM (unless --no-agent is used)
  5. Generate Dockerfile and docker-compose.yml
  6. Extract environment variables
  7. Write everything to chimera-outputs/

Flags:
  --force       Force re-initialization even if directory exists
  --no-agent    Disable LLM validation (use static analysis only)

Examples:
  chimera init https://github.com/tiangolo/fastapi
  chimera init https://github.com/user/repo --force
  chimera init https://github.com/user/repo --no-agent
  chimera init https://github.com/user/repo --force --no-agent

Use "chimera [command] --help" for more information about a command.
```

---

## Common Workflows

### 1. First Time Setup

```bash
# Configure Chimera
chimera setup

# Initialize a repository
chimera init https://github.com/user/repo
```

### 2. Change Configuration

```bash
# Reconfigure (will warn if exists)
chimera setup --force
```

### 3. Update Repository

```bash
# Re-clone with latest changes
chimera init https://github.com/user/repo --force
```

### 4. Fast Mode (No LLM)

```bash
# Skip LLM validation for speed
chimera init https://github.com/user/repo --no-agent
```

### 5. Complete Fresh Start

```bash
# Reconfigure and re-initialize
chimera setup --force
chimera init https://github.com/user/repo --force
```

### 6. Offline Mode

```bash
# Setup once (requires internet)
chimera setup

# Then use offline (no LLM calls)
chimera init https://github.com/user/repo --no-agent
```

---

## Error Handling

### Setup Without --force (Config Exists)

**Error:** None (just informational message)

**Exit Code:** 0

**Action:** Shows message and exits gracefully

### Init Without --force (Directory Exists)

**Error:** `directory <name> already exists. Use --force to re-initialize`

**Exit Code:** 1

**Action:** Shows error box and exits

### Init With --no-agent (No Config)

**Error:** `Chimera is not configured. Run chimera setup first`

**Exit Code:** 1

**Action:** Shows error box and exits

**Note:** Even with `--no-agent`, you still need to run `chimera setup` first because the config file is required for repository cloning and basic operations.

---

## Performance Comparison

### With LLM (Default)

```
Total Time: ~30-60 seconds
- Clone: 5-10s
- Tree: 1-2s
- Detection: 1-2s
- LLM Validation: 10-20s (3 calls)
- Generation: 1-2s
- Write: 1-2s
```

### Without LLM (--no-agent)

```
Total Time: ~10-20 seconds
- Clone: 5-10s
- Tree: 1-2s
- Detection: 1-2s
- LLM Validation: 0s (skipped)
- Generation: 1-2s
- Write: 1-2s
```

**Speed Improvement:** ~50-70% faster with `--no-agent`

---

## Best Practices

1. **Use `--force` sparingly** - Only when you need to overwrite
2. **Use `--no-agent` for testing** - Faster iteration during development
3. **Use LLM by default** - Better accuracy and validation
4. **Combine flags when needed** - `--force --no-agent` for fast re-runs
5. **Check help text** - `chimera init --help` for latest info

---

## Troubleshooting

### "Configuration already exists"

**Solution:** Use `chimera setup --force`

### "Directory already exists"

**Solution:** Use `chimera init <url> --force`

### "LLM validation failed"

**Solution:** Use `chimera init <url> --no-agent` to skip LLM

### "Rate limit exceeded"

**Solution:** Use `--no-agent` to avoid LLM calls

### "Offline / No internet"

**Solution:** Use `--no-agent` for offline usage

---

## Summary

Chimera v4 provides flexible flags to customize behavior:

- ✅ `--force` for setup and init
- ✅ `--no-agent` for faster, offline usage
- ✅ Clear error messages
- ✅ Graceful handling
- ✅ Combinable flags

**Use them wisely for the best experience!** 🚀
