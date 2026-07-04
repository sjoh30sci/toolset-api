# Toolset API — Architecture

## Overview

The Toolset API is a local-first gateway that exposes a curated set of tools to
AI agents. A single Go gateway fronts several containerized tool services over
an internal Docker network. The gateway speaks both a small REST API and the
Model Context Protocol (MCP).

## Service diagram

```
                          Host
   +-------------------------------------------------------------+
   |  AI agent / client                                          |
   |        |  REST + MCP (HTTP)                                 |
   |        v                                                    |
   |  +-----------------------------+                            |
   |  |  gateway  (toolset/gateway) |  :8080                     |
   |  |  - RequestID middleware     |                            |
   |  |  - Auth middleware          |                            |
   |  |  - Rate limiter             |                            |
   |  |  - Echo router              |                            |
   |  |  - SQLite (/data)           |                            |
   |  +--------------+--------------+                            |
   |                 | toolset-network (internal bridge)         |
   |   +-------------+-----------+-------------+---------------+  |
   |   v             v           v             v               v  |
   | search      exec-light   files-server   browser        (vols)|
   | (search)    (nsjail)     (sandbox FS)   (Playwright)         |
   +-------------------------------------------------------------+

   Volumes: toolset-data (/data: SQLite + files), toolset-logs (/logs)
```

By default `toolset-network` is `internal: true` (no host exposure). Developers
copy `docker-compose.override.yml.example` to add host port mappings and an
external bridge.

## Request flow

1. Client sends an HTTP request (REST endpoint or MCP JSON-RPC-style POST).
2. `RequestID` middleware assigns/propagates `X-Request-ID` for tracing.
3. `Recover` middleware guards against panics.
4. `Auth` middleware runs:
   - **none mode:** verifies loopback origin, tags context `auth_mode=local`.
   - **token mode:** extracts bearer token, validates against `api_keys`,
     checks expiry, enforces rate limit, tags `token`/`tool_id`.
5. Router dispatches to a handler (`/health`, `/tools`, `/mcp/*`).
6. Handler consults the `registry` (backed by SQLite) and, in later phases,
   proxies to the appropriate tool container.
7. Structured JSON logs are emitted per request with the request ID.

Public endpoints (`/` and `/health`) bypass auth so orchestrators and Docker
health checks can probe them.

## Auth modes

| Mode  | Origin check      | Credential                     | Rate limit key |
|-------|-------------------|--------------------------------|----------------|
| none  | loopback only     | none                           | client IP      |
| token | any               | `Authorization: Bearer <tok>`  | token          |

Rate limiting in Phase 1 is a fixed-window in-memory limiter (default
100 req/min). The `rate_limits` table exists for a future persistent limiter.

## Endpoints

REST:
- `GET /` — welcome + endpoint index
- `GET /health` — gateway status + per-tool readiness
- `GET /tools` — full registered tool list
- `POST /tools/list`, `POST /tools/call` — MCP-compatible aliases

MCP:
- `POST /mcp/initialize` — protocol handshake + capabilities
- `POST /mcp/tools/list` — list tools
- `POST /mcp/tools/call` — invoke a tool (stub in Phase 1)
- `POST /mcp/resources/read` — read a resource (stub in Phase 1)
- `GET /mcp/schema` — OpenAPI schema (stub in Phase 1)

## Database schema

SQLite at `/data/toolset.db` (WAL mode, foreign keys on). Migrations are managed
by `golang-migrate` and applied automatically on startup.

- **tools** — registered tools (id, name, description, category, status,
  health_check_url, container_name, timestamps).
- **api_keys** — bearer tokens (token, tool_id FK, rate_limit, expires_at).
- **executions** — audit trail of tool invocations (status, timings, error).
- **rate_limits** — persistent rate-limit windows (future use).

Indexes: `idx_tools_status`, `idx_api_keys_token`, `idx_executions_tool`.

## Configuration precedence

1. Environment variables (`TOOLSET_*`)
2. `./config.yaml`
3. `/etc/toolset/config.yaml`
4. `~/.config/toolset/config.yaml`
5. Built-in defaults

## Build & deployment

- Gateway is built as a static (musl) CGO binary for `go-sqlite3` and shipped in
  a minimal Alpine runtime image (~15MB) running as a non-root user.
- The `gateway healthcheck` subcommand backs the Docker `HEALTHCHECK`.
- Tool services (`search`, `exec-light`, `files-server`, `browser`) are
  placeholders in Phase 1 and gated behind the `tools` compose profile.
