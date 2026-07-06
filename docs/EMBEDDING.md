# Embedding Toolset in Your Project

There are three common ways to embed the Toolset API into an existing project,
plus first-class SDKs for TypeScript and Python.

## Option 1: Docker Compose Include

Include the toolset stack from your own `docker-compose.yml` (Compose v2.20+):

```yaml
services:
  my-app:
    image: my-app:latest
    depends_on:
      - gateway

include:
  - path: toolset/docker-compose.yml
```

## Option 2: Portable Package

Ship the whole stack as a single tarball:

```bash
toolset package ./my-deployment.tar.gz
# Share my-deployment.tar.gz. Recipients extract and run:
tar -xzf my-deployment.tar.gz
cd toolset
docker-compose up -d
```

The package contains `docker-compose.yml`, configs, docs, tool definitions, and
the `toolset` CLI binary at `toolset/bin/toolset`.

## Option 3: Official CLI

```bash
# Install (downloads the release binary for your platform)
curl -fsSL https://raw.githubusercontent.com/yourusername/toolset-api/main/INSTALL.sh | bash

# Run
toolset up
toolset status
toolset logs gateway
```

## SDK Usage

### TypeScript

```bash
npm install @yourusername/toolset-api
```

```typescript
import { ToolsetAPI } from "@yourusername/toolset-api";

const client = new ToolsetAPI({ baseURL: "http://localhost:8080" });

// Search
const results = await client.search({ query: "golang" });

// Execute code
const output = await client.exec({
  code: 'print("Hello")',
  language: "python",
});
```

### Python

```bash
pip install toolset-api
```

```python
from toolset_api import ToolsetAPI

client = ToolsetAPI(base_url="http://localhost:8080")

# Search
results = client.search(query="golang")

# Execute code
output = client.exec(code='print("Hello")', language="python")
```

## Authentication

When the gateway runs in `auth.mode=token`, pass a per-tool bearer token:

```typescript
const client = new ToolsetAPI({ baseURL: "...", token: "sk_search_xxx" });
```

```python
client = ToolsetAPI(base_url="...", token="sk_search_xxx")
```
