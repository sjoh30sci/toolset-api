# Troubleshooting

## Gateway won't start / port already in use

The gateway prefers `server.port` (default `8080`) and falls back to
`server.fallback_port` (default `18080`). If both are taken it exits with
`no available port`.

```bash
# Find what holds the port (Linux/macOS)
lsof -i :8080
# Windows (PowerShell)
Get-NetTCPConnection -LocalPort 8080
```

Change the port in `config.yaml` or via `TOOLSET_PORT`.

## `curl http://localhost:8080/health` refuses the connection

The default `docker-compose.yml` does **not** publish ports to the host. Copy
the override and restart:

```bash
cp docker-compose.override.yml.example docker-compose.override.yml
docker-compose up -d
```

## `401 Unauthorized` in token mode

- Send `Authorization: Bearer <token>`.
- Tokens are **scoped per tool**. A `search` token cannot call `/exec`.
- Verify the token exists in the `api_keys` table (see `config.yaml`
  `auth.api_tokens`).

## `403 Forbidden` in `none` auth mode

In `auth.mode=none` the gateway only accepts requests from `127.0.0.1`/`::1`.
Requests via a container network or LAN IP are rejected. Use `token` mode for
remote access.

## Code execution returns `503 execution not enabled`

- `exec.light_enabled` / `exec.heavy_enabled` must be `true` in `config.yaml`.
- Heavy runtimes (`dotnet`, `java`, `rust`, `csharp`) require
  `exec.heavy_enabled: true` and the `exec-heavy` container running.

## `400 unsupported language`

Check the language is in `exec.languages.light` or `.heavy`. Supported:
`python`, `node`, `bash`, `c`, `cpp`, `assembly` (light); `dotnet`, `java`,
`rust`, `csharp` (heavy).

## Browser actions time out

- Increase `browser.session_timeout_minutes`.
- Ensure the requested `browserType` is enabled under `browser.browsers`
  (`webkit` is disabled by default).
- The browser service pool is bounded by `browser.max_sessions`; free sessions
  with `DELETE /browser/session/{id}`.

## `make build` fails with a CGO/gcc error

The gateway links `go-sqlite3`, which requires CGO and a C compiler for a
native build. Install a C toolchain (e.g. `build-essential`, Xcode CLT, or
MSYS2/MinGW on Windows). The **CLI** builds with `CGO_ENABLED=0` and needs no C
compiler.

## GoReleaser: `main` builds fail

The `.goreleaser.yml` builds the CLI from `dir: cli` with `CGO_ENABLED=0`. It
does **not** build the gateway (which needs CGO cross-toolchains). The gateway
is distributed as a Docker image via `make docker-build`.

## SDK generation produces no output

`make sdk-generate` requires Docker (for `openapitools/openapi-generator-cli`).
It first runs `make openapi`, which requires a C compiler because the gateway
binary links sqlite. On CI (ubuntu-latest) gcc is preinstalled.
