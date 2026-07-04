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

## Development

```bash
make build          # build gateway -> bin/gateway
make test           # run unit tests
make compose-config # validate docker-compose syntax
make clean          # remove build artifacts
```
