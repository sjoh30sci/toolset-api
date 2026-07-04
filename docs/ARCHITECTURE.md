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

## Tool request flow (Phase 2)

The gateway proxies two tool services over the internal network.

```
client                gateway                 search (SearXNG)      files-server
  |  POST /search        |                          |                    |
  |--------------------->|                          |                    |
  |                      | auth + RequireTool("search")                  |
  |                      |  GET /search?q=..&format=json                 |
  |                      |------------------------->|                    |
  |                      |     JSON results         |                    |
  |                      |<-------------------------|                    |
  |   normalized results |                          |                    |
  |<---------------------|                          |                    |
  |                      |                          |                    |
  |  POST /files/write   |                          |                    |
  |--------------------->|  auth + RequireTool("files")                  |
  |                      |  POST /files/write (pass-through)             |
  |                      |---------------------------------------------->|
  |                      |            201 {path,size}                    |
  |                      |<----------------------------------------------|
  |   201 {path,size}    |                          |                    |
  |<---------------------|                          |                    |
```

- `search` is normalized: the gateway maps SearXNG's `results[].content` to our
  `snippet` field and enforces a 5s upstream timeout (502 on failure/timeout).
- `files/*` are transparent pass-throughs: the gateway forwards the JSON body
  (and any `Authorization` header) and relays the upstream status + body.

## Tool endpoints & schemas (Phase 2)

### `POST /search`

Request:

```json
{
  "query": "golang tutorials",
  "engines": ["google", "bing"],
  "page": 1,
  "lang": "en"
}
```

Only `query` is required. `engines` defaults to all enabled engines, `page`
defaults to 1.

Response `200`:

```json
{
  "query": "golang tutorials",
  "page": 1,
  "results": [
    {
      "title": "The Go Programming Language",
      "url": "https://go.dev",
      "snippet": "Build fast, reliable software.",
      "engine": "google"
    }
  ],
  "count": 1
}
```

Errors: `400` (missing/invalid query or body), `502` (upstream unavailable,
timeout, or non-200).

### `POST /files/read | write | list | delete | move`

All paths are relative to the sandbox root (`/data/files`). The gateway proxies
these verbatim to `http://files-server:8765/files/<op>`. Semantics enforced by
the file-server:

| Endpoint        | Request body                                | Success | Errors            |
|-----------------|---------------------------------------------|---------|-------------------|
| `/files/read`   | `{"path":"a.txt"}`                          | `200 {"content":"..."}` | `400`,`404`,`413` |
| `/files/write`  | `{"path":"a.txt","content":"..."}`          | `201 {"path":..,"size":N}` | `400`,`413`       |
| `/files/list`   | `{"path":"dir/","recursive":false}`         | `200 {"files":[],"dirs":[]}` | `400`,`404`       |
| `/files/delete` | `{"path":"a.txt"}`                          | `204` (no content) | `400`,`404`       |
| `/files/move`   | `{"from":"a.txt","to":"b.txt"}`             | `201 {"from":..,"to":..}` | `400`,`404`       |

Security: absolute paths and `..` traversal are rejected (`400`); symlinks that
escape the sandbox are rejected; writes over 100MB return `413`.

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

Tools (Phase 2):
- `POST /search` — SearXNG-backed web/meta search (scoped to `search` token)
- `POST /files/read|write|list|delete|move` — sandboxed file ops (scoped to
  `files` token)

In token mode, a token's `tool_id` scopes access: a token bound to `search`
receives `403` on `/files/*` and vice-versa. A token with an empty `tool_id`
(wildcard) may access any tool. In `none` (local) mode all tools are permitted.

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
  gated behind the `tools` compose profile. As of Phase 2, `search` (SearXNG)
  and `files-server` (a stdlib Go sandbox server) have real Dockerfiles and
  health checks; `exec-light` and `browser` remain placeholders. Tool ports
  (8888, 8765) are internal-only and never published to the host.
