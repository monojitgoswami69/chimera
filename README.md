# Chimera

> Clone a GitHub repo, detect its stack, and walk away with a runnable Docker environment.

```
$ chimera init https://github.com/tiangolo/fastapi
[1/6] Clone repository                       ✓ fastapi cloned (3002 files · 47.8 MB)
[2/6] Generate repository tree               ✓ Tree generated (3392 lines)
[3/6] Detect services and frameworks         ✓ service-1 · backend · fastapi · port 8000
[4/6] Generate Dockerfile + docker-compose   ✓ chimera-outputs/Dockerfile.service-1
                                             ✓ chimera-outputs/docker-compose.yml
[5/6] Extract environment variables          ✓ Detected variables across 1 service
[6/6] Write outputs                          ✓ env-vars/service-1/.env.example
                                             ✓ quick_start.md
Init complete in 6.8s
```

Chimera does **static** detection by default; if you set up an LLM provider it
will also use that provider to double-check the detection and to enrich the
generated `.env.example` files. Pass `--no-agent` to skip the LLM entirely.

## Install

```bash
git clone https://github.com/monojitgoswami69/chimera.git
cd chimera
go build -o chimera .
sudo mv chimera /usr/local/bin/   # or ~/.local/bin
```

You need Go 1.21+, `git`, and (for actually running the generated containers)
Docker with Compose v2.

## Commands

```
chimera setup                       Configure provider, model, API key, GitHub PAT
chimera init <github-url>           Run the full pipeline against a repo
chimera help                        Show the in-app help
```

Global flags: `--verbose`, `--quiet`, `--no-color`, `--version`.

Useful `init` flags:

| flag | effect |
| --- | --- |
| `--no-agent` | Skip every LLM call. Works without `chimera setup`. |
| `--force` | Remove the previously cloned copy and start fresh. |
| `--output <dir>` | Override the output sub-directory (default `chimera-outputs`). |

Accepted URL shapes:

- `https://github.com/<owner>/<repo>`
- `https://github.com/<owner>/<repo>.git`
- `https://github.com/<owner>/<repo>/tree/<branch>[/<subdir>]` — clones the
  branch via `--single-branch --branch`
- `git@github.com:<owner>/<repo>[.git]` — normalised to the equivalent
  HTTPS clone URL

## Setup

`chimera setup` is an interactive wizard. It writes
`~/.chimera/config.json` (chmod 0600) with:

```json
{
  "llm_provider": "openai",
  "llm_model":    "gpt-4o",
  "llm_api_key":  "...",
  "github_pat":   "ghp_..."
}
```

Supported providers: OpenAI, Anthropic, Groq, Google Gemini, Ollama (local —
the API key step is skipped and the tool talks to `localhost:11434`).

The GitHub PAT step is optional. When a PAT is set, `git clone` is invoked with
an `Authorization: Basic` header injected through `git -c http.extraheader=…`,
so the token never appears on the command line or in `.git/config`. The header
config is removed from the cloned repo after success.

A `.chimera.env` file from older versions is read as a fallback the first time,
then replaced with `config.json` on the next `chimera setup`.

## How detection works

`chimera init` walks the cloned tree, ignoring `node_modules/`, `.venv/`,
`.git/`, `dist/`, `build/`, and similar generated/vendored directories. It
picks up:

- `package.json` → JavaScript/TypeScript service. Framework is inferred from
  `dependencies`/`devDependencies`: Next, NestJS, Fastify, Express, Vite,
  Create-React-App, plain React, or generic Node.
- `requirements.txt` / `pyproject.toml` / `Pipfile` → Python service. Framework
  is inferred from the file contents: FastAPI, Flask, Django, or generic
  Python. When both `requirements.txt` and `pyproject.toml` exist in the same
  directory, `requirements.txt` wins so we only emit one service.

For each detected service Chimera records a concrete service model:

| field | meaning |
| --- | --- |
| `ID` | Stable id (`service-1`, `service-2`, …) used as compose service name and Dockerfile suffix. |
| `Directory` | Path relative to repo root (`""` means root). |
| `Language` | `javascript`, `typescript`, or `python`. |
| `Framework` | `next`, `express`, `fastapi`, … |
| `PackageManager` | `npm` / `yarn` / `pnpm` / `pip` / `poetry`. |
| `InstallCmd`, `BuildCmd`, `StartCmd` | Concrete commands used in the Dockerfile. |
| `Port` | Container port the Dockerfile `EXPOSE`s and compose publishes. |
| `Confidence` | `high` / `medium` / `low`, surfaced in the detection table. |

If an LLM provider is configured and `--no-agent` is not set, the tool sends
the detection and tree to the LLM and accepts corrections to the service list.
The LLM may also request a small number of files (capped at 5 per round, 16 KiB
each). File reads are sandboxed through `internal/safefs` — paths must be
repo-relative and stay inside the cloned root; symlinks pointing outside are
rejected.

## Generated layout

```
chimera-outputs/
├── docker-compose.yml          # one services entry per detected service
├── Dockerfile.service-1        # one Dockerfile per service
├── Dockerfile.service-2
├── env-vars/
│   ├── service-1/.env.example
│   └── service-2/.env.example
├── quick_start.md
└── .gitignore
```

A few things worth knowing about the generated files:

- The build context for every service is `..` (i.e. the repo root). Each
  Dockerfile copies from the right sub-directory (`COPY apps/web/package*.json
  ./`), so monorepos work without ad-hoc compose tricks.
- `docker compose -f chimera-outputs/docker-compose.yml up --build` is the
  intended invocation, run from the repo root.
- Host port collisions across services are remapped automatically (the second
  service on port 3000 binds 3001, the third 3002, etc.).
- `env_file:` references each service's `.env.example`. Fill it in and copy to
  `.env` for production — the file is **only** a template.

## Status / non-goals

Chimera generates a starting point, not a guarantee. The Dockerfiles are
intentionally simple and based on framework conventions; you'll likely need to
tweak them for production use (caching, multi-stage trimming, non-root user,
healthchecks, secrets management). It does not:

- Set up databases, queues, or other infrastructure services automatically
  (compose is for the application services only).
- Run the containers for you.
- Provide telemetry, metrics, or healing — it's a one-shot generator.

## Development

```bash
go build ./...        # build everything
go test ./...         # run unit tests (no network)
go vet ./...          # static analysis
go run . help         # try the binary without installing
```

The test suite uses temp directories for fixtures, so it's safe to run
anywhere. Tests cover detector framework picks, env-var regex behaviour,
generator output (including compose YAML parsing), git URL parsing, safefs
escape attempts, and config round-trips.

## License

MIT.
