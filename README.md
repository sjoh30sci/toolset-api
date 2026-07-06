# Toolset API

A local API toolset that exposes tools (web search, code execution, file
operations, browser automation) to local AI agents over REST and
[MCP](https://modelcontextprotocol.io/).

> **Status:** Phase 1 — Foundation. Service implementations are placeholders;
> this phase delivers the project skeleton, gateway, database, auth, and CLI.

## Architecture

```
                 +------------------------------------------+
   AI agent ---> |  Go gateway (:8080)                      |
   (REST/MCP)    |   auth -> rate-limit -> router -> handler |
                 +-------------------+----------------------+
                                     | (internal bridge)
        +-------------+--------------+--------------+-------------+
        |             |              |              |             |
    search       exec-light     files-server     browser      (SQLite:
  (placeholder) (nsjail, later) (sandbox, later) (Playwright)   /data)
```

See [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) for details.

## Features

- **Search** — web/meta search via SearXNG.
- **Files** — sandboxed read/write/list/delete/move.
- **Exec** — sandboxed code execution across 9+ language runtimes (sync + async).
- **Browser** — Playwright automation: sessions, DOM actions, screenshots, PDF
  export, multi-browser (Chromium/Firefox/WebKit). See
  [docs/BROWSER_API.md](docs/BROWSER_API.md).
- **MCP** — full Model Context Protocol compliance (`2024-11-05`). See
  [docs/MCP_SPEC.md](docs/MCP_SPEC.md).

## Quick start

```bash
# 1. Scaffold local config/env and directories
make build-cli && ./bin/toolset init

# 2. Build the gateway binary
make build

# 3. Build & run the stack
make docker-build
make docker-up

# 4. Check status
./bin/toolset status
# or: curl http://127.0.0.1:8080/health   (requires the override file for ports)
```

## Layout

```
.
├── docker-compose.yml                 # service definitions
├── docker-compose.override.yml.example# local port mappings (copy to use)
├── config.yaml.example                # gateway config template
├── .env.example                       # environment template
├── Makefile
├── gateway/                           # Go HTTP + MCP gateway
│   ├── main.go
│   ├── Dockerfile
│   ├── migrations/                    # golang-migrate SQL files
│   └── internal/{auth,config,db,handlers,registry}
├── cli/                               # `toolset` CLI (cobra)
└── docs/ARCHITECTURE.md
```

## Configuration

Config resolves in priority order: environment (`TOOLSET_*`) → `config.yaml`
(cwd) → `/etc/toolset/config.yaml` → `~/.config/toolset/config.yaml` →
defaults. Key env vars: `TOOLSET_AUTH_MODE`, `TOOLSET_PORT`,
`TOOLSET_LOG_LEVEL`, `TOOLSET_DB_PATH`.

## Auth modes

- **none** (default): only accepts requests from `127.0.0.1` / `::1`.
- **token**: requires `Authorization: Bearer <token>` validated against the
  `api_keys` table, with per-token rate limiting.

## Local Development

### Quick Start (30 seconds)

**macOS / Linux:**

```bash
git clone https://github.com/sjoh30sci/toolset-api.git
cd toolset-api
bash QUICKSTART.sh
```

**Windows (PowerShell):**

```powershell
git clone https://github.com/sjoh30sci/toolset-api.git
cd toolset-api
powershell -ExecutionPolicy Bypass -File QUICKSTART.ps1
```

Then visit `http://localhost:8080/health` to verify.

### Manual Setup

See [SETUP.md](SETUP.md) for step-by-step instructions, troubleshooting, and
advanced configuration.

### Development Workflow

```bash
# Build binaries
make build          # Gateway binary → bin/gateway
make build-cli      # CLI tool → bin/toolset

# Run tests
make test           # Unit tests
make test-all       # All tests
make test-e2e       # End-to-end (requires services running)

# Manage services
make docker-build   # Build all images
make docker-up      # Start stack
make docker-down    # Stop stack
make compose-config # Validate compose file

# Generate SDKs
make sdk-generate   # TypeScript + Python SDKs

# Release
make release        # GoReleaser (creates artifacts)
```

See [Makefile](Makefile) for all available targets.
