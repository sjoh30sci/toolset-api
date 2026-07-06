# Quick Start

## Installation

### Option 1: Docker (Recommended)

```bash
docker-compose up -d
curl http://localhost:8080/health
```

> The default `docker-compose.yml` does not publish the gateway port to the
> host. Copy `docker-compose.override.yml.example` to
> `docker-compose.override.yml` to expose `8080` locally.

### Option 2: Pre-built Binary

Download from [GitHub Releases](https://github.com/yourusername/toolset-api/releases):

```bash
wget https://github.com/yourusername/toolset-api/releases/download/v0.1.0/toolset_0.1.0_linux_amd64.tar.gz
tar -xzf toolset_0.1.0_linux_amd64.tar.gz
cd toolset
docker-compose up -d
```

### Option 3: Build from Source

```bash
git clone https://github.com/yourusername/toolset-api
cd toolset-api
make build-cli   # builds ./bin/toolset
make build       # builds ./bin/gateway
make docker-build
make docker-up
```

## Basic Usage

### Web Search

```bash
curl -X POST http://localhost:8080/search \
  -H "Content-Type: application/json" \
  -d '{"query": "golang tutorials"}'
```

### Code Execution

```bash
curl -X POST http://localhost:8080/exec \
  -H "Content-Type: application/json" \
  -d '{
    "code": "print(\"Hello from Python\")",
    "language": "python"
  }'
```

### File Operations

```bash
# Write
curl -X POST http://localhost:8080/files/write \
  -H "Content-Type: application/json" \
  -d '{"path": "test.txt", "content": "Hello World"}'

# Read
curl -X POST http://localhost:8080/files/read \
  -H "Content-Type: application/json" \
  -d '{"path": "test.txt"}'
```

### Browser Automation

```bash
# Create session
SESSION=$(curl -X POST http://localhost:8080/browser/session \
  -H "Content-Type: application/json" \
  -d '{"browserType": "chromium"}' | jq -r '.id')

# Navigate
curl -X POST http://localhost:8080/browser/action \
  -H "Content-Type: application/json" \
  -d "{
    \"session_id\": \"$SESSION\",
    \"action\": {
      \"type\": \"navigate\",
      \"url\": \"https://example.com\"
    }
  }"
```

## CLI

```bash
toolset init      # scaffold config.yaml, .env.local, data/, logs/
toolset up        # docker-compose up -d + wait for health
toolset status    # query /health and print tool readiness
toolset logs gateway
toolset down
toolset package ./my-toolset.tar.gz   # portable distribution tarball
```

## MCP Integration

Use with Claude or other MCP clients:

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

See [MCP_SPEC.md](MCP_SPEC.md) for the full protocol surface.

## Configuration

Edit `config.yaml` (copy from `config.yaml.example`):

- `server.port` — API port (default `8080`)
- `auth.mode` — `none` or `token`
- `exec.queue_workers` — async job workers
- `browser.max_sessions` — browser pool size

Environment variables (`TOOLSET_*`) always override `config.yaml`.
