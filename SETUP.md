# Local Setup Guide

## Prerequisites

| Tool | Version | Purpose |
|------|---------|---------|
| Docker & Docker Compose | Latest | Container runtime & orchestration |
| Go | 1.22+ | Building the gateway & CLI |
| Node.js | 18+ | Building the browser service |
| Git | Latest | Version control |

> **Windows users:** Use [Docker Desktop](https://www.docker.com/products/docker-desktop/)
> with WSL 2 backend. All examples assume a Unix-like shell (Git Bash, WSL, or
> PowerShell). PowerShell-specific commands are noted where applicable.

---

## Quick Start (5 minutes)

### Step 1: Clone the repository

```bash
cd /path/to/projects
git clone https://github.com/sjoh30sci/toolset-api.git
cd toolset-api
```

### Step 2: Prepare environment

```bash
# Copy example files
cp .env.example .env.local
cp docker-compose.override.yml.example docker-compose.override.yml
cp config.yaml.example config.yaml

# (Optional) Edit .env.local for custom ports, auth tokens
```

### Step 3: Build Docker images

```bash
make docker-build
# Or: docker-compose build
```

Expected output:

| Image | Approximate size |
|-------|-----------------|
| `toolset/gateway:latest` | ~20 MB |
| `toolset/search:latest` (SearXNG) | ~200 MB |
| `toolset/files-server:latest` | ~50 MB |
| `toolset/exec-light:latest` | ~800 MB |
| `toolset/browser:latest` (Playwright) | ~1.2 GB |

### Step 4: Start the stack

```bash
make docker-up
# Or: docker-compose up -d
```

Expected output:

```
Creating toolset-network ...
Creating toolset-gateway ...
Creating toolset-search ...
Creating toolset-files-server ...
Creating toolset-exec-light ...
Creating toolset-browser ...
```

> **Note:** The first startup may take 30–60 seconds for health checks to pass.

### Step 5: Verify services are healthy

```bash
docker-compose ps
# All services should show "Up" and HEALTHY status

# Or check the health endpoint:
curl http://localhost:8080/health
# Expected: {"status":"ok","tools":{...},"version":"...","uptime_sec":...}
```

### Step 6: Test the API

```bash
# Web search
curl -X POST http://localhost:8080/search \
  -H "Content-Type: application/json" \
  -d '{"query": "golang tutorials", "page": 1}'

# Code execution
curl -X POST http://localhost:8080/exec \
  -H "Content-Type: application/json" \
  -d '{"code": "print(\"Hello from Python\")", "language": "python", "timeout": 10}'

# File write
curl -X POST http://localhost:8080/files/write \
  -H "Content-Type: application/json" \
  -d '{"path": "test.txt", "content": "Hello World"}'

# File read
curl -X POST http://localhost:8080/files/read \
  -H "Content-Type: application/json" \
  -d '{"path": "test.txt"}'
```

---

## Automated Setup

### macOS / Linux

```bash
bash QUICKSTART.sh
```

### Windows (PowerShell)

```powershell
powershell -ExecutionPolicy Bypass -File QUICKSTART.ps1
```

The scripts will:
1. Check prerequisites (Docker, Docker Compose, Git)
2. Copy example files (skips if already present)
3. Build all Docker images
4. Start the stack
5. Wait for services to become healthy
6. Run smoke tests against `/health` and `/exec`

---

## Detailed Setup

### Environment Variables (.env.local)

```bash
# Network & ports
TOOLSET_PORT=8080

# Auth mode: 'none' or 'token'
TOOLSET_AUTH_MODE=none

# Database
TOOLSET_DB_PATH=/data/toolset.db

# Logging
TOOLSET_LOG_LEVEL=info

# Auth tokens (only used when TOOLSET_AUTH_MODE=token)
TOOLSET_TOKEN_SEARCH=sk_search_changeme
TOOLSET_TOKEN_EXEC=sk_exec_changeme

# SearXNG secret (generate with: openssl rand -hex 16)
SEARXNG_SECRET=changeme_run_openssl_rand_hex_16

# File server
FILES_PORT=8765
FILES_SANDBOX_ROOT=/data/files
```

| Variable | Default | Description |
|----------|---------|-------------|
| `TOOLSET_PORT` | `8080` | Gateway HTTP port |
| `TOOLSET_AUTH_MODE` | `none` | Authentication mode (`none` or `token`) |
| `TOOLSET_LOG_LEVEL` | `info` | Log verbosity (`debug`, `info`, `warn`, `error`) |
| `TOOLSET_DB_PATH` | `/data/toolset.db` | SQLite database path (inside container) |
| `SEARXNG_SECRET` | *(changeme)* | Secret key for SearXNG instance |
| `FILES_PORT` | `8765` | Internal port for files-server |
| `FILES_SANDBOX_ROOT` | `/data/files` | Sandbox root directory for file operations |

### Configuration (config.yaml)

Copy `config.yaml.example` to `config.yaml` and edit to customize behavior:

```yaml
server:
  port: 8080
  fallback_port: 18080
  log_level: info                # debug | info | warn | error

auth:
  mode: none                     # 'none' for local dev, 'token' for production
  api_tokens:
    search: "sk_search_xxx"
    exec: "sk_exec_xxx"
  rate_limit: 100                # requests/minute per token or IP

db:
  path: /data/toolset.db
  max_connections: 10

tools:
  search:
    enabled: true
    timeout: 5s                  # per-request upstream timeout
    engines:                     # empty = all enabled
      - google
      - bing
      - duckduckgo
      - brave

  files:
    enabled: true
    sandbox_root: /data/files
    max_file_size: 104857600     # 100MB, in bytes

  exec:
    light_enabled: true
    heavy_enabled: false         # Disable if you don't need .NET/Java/Rust
    default_timeout: 30
    max_timeout: 300
    queue_workers: 2
    languages:
      light: [python, node, bash, c, cpp, assembly]
      heavy: [dotnet, java, rust, csharp]

  browser:
    enabled: true
    max_sessions: 10
    session_timeout_minutes: 30
    browsers:
      chromium: true
      firefox: true
      webkit: false              # Smaller image / faster launch if disabled
```

> **Note:** Environment variables (`TOOLSET_*`) always override values in
> `config.yaml`. The config file is read from the current working directory,
> falling back to `/etc/toolset/config.yaml` then `~/.config/toolset/config.yaml`.

### Docker Compose Override

The `docker-compose.override.yml` file exposes internal ports to the host for
local development. Copy the example:

```bash
cp docker-compose.override.yml.example docker-compose.override.yml
```

The override:
- Maps the gateway port `8080` to the host
- Attaches services to a non-internal bridge network (`toolset-external`)
- Removes `profiles` restrictions so all tool services start with `docker-compose up`

---

## Service Reference

| Service | Container Name | Internal Port | Description |
|---------|---------------|---------------|-------------|
| `gateway` | `toolset-gateway` | 8080 | Go HTTP + MCP gateway |
| `search` | `toolset-search` | 8888 | SearXNG search aggregation |
| `files-server` | `toolset-files-server` | 8765 | Sandboxed file operations |
| `exec-light` | `toolset-exec-light` | 8765 | Code execution (light runtimes) |
| `exec-heavy` | `toolset-exec-heavy` | 8766 | Code execution (heavy runtimes, profile: `heavy`) |
| `browser` | `toolset-browser` | 3000 | Playwright browser automation |

---

## Common Tasks

### View logs

```bash
# All services
docker-compose logs -f

# Specific service
docker-compose logs -f gateway
docker-compose logs -f search
docker-compose logs -f browser
```

### Stop the stack

```bash
make docker-down
# Or: docker-compose down
```

### Rebuild and restart

```bash
docker-compose down
make docker-build
docker-compose up -d
```

### Clean up everything (volumes included)

```bash
docker-compose down -v
# This removes the named volumes (data loss!)
```

### Manual cleanup (when containers/volumes conflict)

If you encounter errors like "container already in use" or "volume name conflicts",
run one of the included cleanup scripts:

**PowerShell:**
```powershell
powershell -ExecutionPolicy Bypass -File CLEANUP.ps1
```

**Command Prompt:**
```cmd
CLEANUP.bat
```

**Or run these commands directly:**
```powershell
# Stop and remove all containers with volumes
docker-compose down -v

# Force-remove any remaining containers
docker rm -f toolset-gateway
docker rm -f toolset-search
docker rm -f toolset-files-server
docker rm -f toolset-exec-light
docker rm -f toolset-exec-heavy
docker rm -f toolset-browser

# Remove orphan volumes
docker volume rm toolset-data toolset-logs
docker volume prune -f --filter "label=com.docker.compose.project=toolset-api"
docker volume prune -f --filter "label=com.docker.compose.project=toolsetapi"

# Remove networks
docker network rm toolset-network toolset-external
```

### Access the file sandbox

Files written via the API are stored in a Docker named volume:

```bash
# List files written via API
docker run --rm -v toolset-data:/data -v "$(pwd):/work" ubuntu ls -la /data/files/

# Copy from container to host
docker run --rm -v toolset-data:/data -v "$(pwd):/work" ubuntu cp -r /data/files/* /work/files-backup/
```

### Monitor resource usage

```bash
docker stats

# Watch specific container
docker stats gateway --no-stream
```

### Access the database directly

```bash
# Connect to SQLite via container
docker run --rm -v toolset-data:/data -it ubuntu sqlite3 /data/toolset.db

# Or extract and query locally
docker run --rm -v toolset-data:/data -v "$(pwd):/work" ubuntu cp /data/toolset.db /work/
sqlite3 toolset.db "SELECT * FROM tools;"
```

---

## Troubleshooting

### Port already in use

If port 8080 is busy, you have two options:

**Option A — Change the host port mapping:**

Edit `docker-compose.override.yml`:

```yaml
services:
  gateway:
    ports:
      - "9090:8080"    # Host port 9090 → container port 8080
```

Then access the API at `http://localhost:9090`.

**Option B — Use the fallback port:**

Set `TOOLSET_FALLBACK_PORT=18080` in `.env.local` and configure your Docker
override to map accordingly.

### Docker image build fails

- Ensure you have at least 20 GB free disk space (especially for
  `exec-light` + `browser` images).
- Verify the Docker daemon is running: `docker ps`
- Try a clean build: `docker-compose build --no-cache`
- For Windows: ensure WSL 2 integration is enabled and you are building from a
  WSL 2-backed directory.

### Services not starting

- Check service logs: `docker-compose logs gateway`
- Verify all images were built: `docker images | findstr toolset`
- Ensure port 8080 is free:
  - **macOS / Linux:** `lsof -i :8080`
  - **Windows:** `netstat -ano | findstr :8080`

### Health check failing

- Wait 30–60 seconds for services to stabilize (first startup is slow because
  images may need to initialize).
- Check individual service logs: `docker-compose logs <service>`
- Verify the gateway is reachable: `curl http://localhost:8080/health`
- If using a custom port, adjust the URL accordingly.

### Browser service memory issues

Playwright images are large (~1.2 GB). If you see OOM errors:

1. **Increase Docker memory limit:**
   - Docker Desktop → Settings → Resources → Memory (8 GB+ recommended)
2. **Disable WebKit in config.yaml:**
   ```yaml
   browser:
     browsers:
       webkit: false
   ```
3. **Disable the entire browser service:**
   ```yaml
   tools:
     browser:
       enabled: false
   ```

### Exec service compilation errors

If Python / Node / Rust won't compile:

- Verify the `exec-light` image built successfully:
  ```bash
  docker image inspect toolset/exec-light:latest
  ```
- Check logs: `docker-compose logs exec-light`
- For Rust: builds are slow the first time (~2 minutes); subsequent builds use
  Docker layer caching.

### Windows-specific issues

**Path separators in `.env.local`:**
Use forward slashes for paths inside containers (Linux). For host paths, use
Windows paths with backslashes or forward slashes depending on context.

**PowerShell quoting:**
When pasting `curl` examples in PowerShell, replace single quotes with double
quotes and escape inner double quotes with backticks or use `--%`:

```powershell
curl.exe -X POST http://localhost:8080/search --% -H "Content-Type: application/json" -d "{\"query\": \"golang\"}"
```

**Long path support:**
If you encounter `npm ERR!` during `browser` build with paths exceeding 260
characters, enable long path support in Windows:
1. Open **Group Policy Editor** → **Computer Configuration** → **Administrative Templates** → **System** → **Filesystem** → Enable **Win32 Long Paths**
2. Or run as Administrator: `New-ItemProperty -Path "HKLM:\SYSTEM\CurrentControlSet\Control\FileSystem" -Name "LongPathsEnabled" -Value 1 -PropertyType DWORD -Force`

---

## Next Steps

### Run tests

```bash
make test                 # Unit tests (requires Go)
make test-search          # Search handler tests
make test-files           # Files handler tests
make test-all             # Unit + search + files tests
make test-e2e             # End-to-end tests (requires services running)
```

### Build CLI locally

```bash
make build                # Build gateway binary → bin/gateway
make build-cli            # Build CLI binary → bin/toolset
```

### Use the CLI

```bash
./bin/toolset init        # Scaffold config.yaml, .env.local, data/, logs/
./bin/toolset status      # Query /health and print tool readiness
./bin/toolset logs gateway
./bin/toolset down
./bin/toolset package ./my-toolset.tar.gz
```

### Enable token-based auth

```bash
# Edit .env.local
TOOLSET_AUTH_MODE=token

# Restart
docker-compose down && docker-compose up -d

# Test with token
curl -X POST http://localhost:8080/search \
  -H "Authorization: Bearer sk_search_changeme" \
  -H "Content-Type: application/json" \
  -d '{"query": "golang tutorials"}'
```

### Use MCP with Claude

Add the following to your MCP client configuration:

```json
{
  "tools": [
    {
      "name": "toolset-api",
      "url": "http://localhost:8080/mcp"
    }
  ]
}
```

See [docs/MCP_SPEC.md](docs/MCP_SPEC.md) for the full protocol surface.

### Generate SDKs

```bash
make sdk-generate           # Generates TypeScript + Python SDKs from OpenAPI spec
```

### Package for distribution

```bash
make package                # Builds a portable tarball: toolset-latest.tar.gz
```

---

## Project Layout

```
.
├── docker-compose.yml                   # Service definitions
├── docker-compose.override.yml.example  # Local port mappings (copy to use)
├── config.yaml.example                  # Gateway config template
├── .env.example                         # Environment template
├── Makefile
├── gateway/                             # Go HTTP + MCP gateway
│   ├── main.go
│   ├── Dockerfile
│   ├── migrations/                      # golang-migrate SQL files
│   └── internal/{auth,config,db,handlers,registry}
├── cli/                                 # `toolset` CLI (cobra)
├── tools/                               # Tool service implementations
│   ├── search/                          # SearXNG search (Dockerfile + config)
│   ├── files/                           # Go file server
│   ├── exec-light/                      # Sandboxed code execution (light)
│   ├── exec-heavy/                      # Sandboxed code execution (heavy)
│   └── browser/                         # Playwright browser automation
├── sdk/
│   ├── ts/                              # TypeScript SDK
│   └── py/                              # Python SDK
├── scripts/                             # Helper scripts (test-e2e.sh)
├── docs/                                # Documentation
│   ├── ARCHITECTURE.md
│   ├── API_REFERENCE.md
│   ├── BROWSER_API.md
│   ├── EMBEDDING.md
│   ├── MCP_SPEC.md
│   ├── QUICKSTART.md
│   └── TROUBLESHOOTING.md
└── data/                                # Runtime data (gitignored)
```

---

## Support

- **📖 Full docs:** [docs/](docs/) directory
- **🐛 Issues:** https://github.com/sjoh30sci/toolset-api/issues
- **💬 Discussions:** https://github.com/sjoh30sci/toolset-api/discussions
- **🔒 Security:** See [SECURITY.md](SECURITY.md)
- **🤝 Contributing:** See [CONTRIBUTING.md](CONTRIBUTING.md)
