# MCP Compliance

The Toolset API gateway speaks the [Model Context Protocol](https://modelcontextprotocol.io/)
(MCP) revision **`2024-11-05`** over HTTP. All MCP endpoints live under
`/mcp/*` and are subject to the gateway's auth middleware (see the auth section
in the [README](../README.md)).

## Endpoints

| Method | Path                   | Purpose                                    |
| ------ | ---------------------- | ------------------------------------------ |
| POST   | `/mcp/initialize`      | Capability handshake                       |
| POST   | `/mcp/tools/list`      | List all tools + JSON Schema contracts     |
| POST   | `/mcp/tools/call`      | Dispatch a tool call to the right service  |
| POST   | `/mcp/resources/read`  | Read a resource (placeholder)              |
| GET    | `/mcp/schema`          | OpenAPI 3.0 description of the API surface |

`POST /tools/list` and `POST /tools/call` are also mounted as aliases for
backwards compatibility.

## initialize

**Request**

```json
POST /mcp/initialize
{
  "protocolVersion": "2024-11-05",
  "clientInfo": { "name": "my-agent", "version": "1.0.0" }
}
```

**Response**

```json
{
  "protocolVersion": "2024-11-05",
  "capabilities": {
    "tools": {},
    "resources": {}
  },
  "serverInfo": {
    "name": "toolset-api",
    "version": "0.1.0"
  }
}
```

## tools/list

Returns the full catalog. Each tool has a JSON Schema `inputSchema`.

```json
POST /mcp/tools/list
{}
```

```json
{
  "tools": [
    {
      "name": "search",
      "description": "Web/meta search via the SearXNG aggregator.",
      "inputSchema": {
        "type": "object",
        "properties": {
          "query":   { "type": "string",  "description": "Search query" },
          "engines": { "type": "array", "items": { "type": "string" } },
          "page":    { "type": "number", "description": "Result page (1-based)" },
          "lang":    { "type": "string" }
        },
        "required": ["query"]
      }
    }
  ]
}
```

### Tool catalog

| Tool                     | Required args         | Dispatches to                         |
| ------------------------ | --------------------- | ------------------------------------- |
| `search`                 | `query`               | SearXNG (`/search`)                   |
| `files_read`             | `path`                | file-server (`/files/read`)           |
| `files_write`            | `path`, `content`     | file-server (`/files/write`)          |
| `exec`                   | `language`, `code`    | exec-light / exec-heavy sandbox       |
| `browser_session_create` | –                     | browser service (`/session`)          |
| `browser_action`         | `session_id`, `type`  | browser service (`/session/:id/action`) |

## tools/call

**Request**

```json
POST /mcp/tools/call
{
  "params": {
    "name": "search",
    "arguments": { "query": "golang generics" }
  }
}
```

**Success response**

```json
{
  "jsonrpc": "2.0",
  "content": [
    { "type": "text", "text": "{\"results\":[...]}" }
  ]
}
```

**Error response** (unknown tool, bad arguments, upstream failure)

```json
{
  "jsonrpc": "2.0",
  "error": {
    "code": -32602,
    "message": "tool call failed",
    "data": "query is required"
  }
}
```

Error codes follow JSON-RPC 2.0: `-32602` invalid params, `-32603` internal
error, `-32601` method/tool not found.

## resources/read

Placeholder implementation. Echoes the requested URI as a `text/plain`
resource. Real resource resolution (file contents, execution artifacts) is
scheduled for a later phase.

```json
POST /mcp/resources/read
{ "params": { "uri": "file:///data/report.txt" } }
```

```json
{
  "contents": [
    { "uri": "file:///data/report.txt", "mimeType": "text/plain", "text": "Resource placeholder" }
  ]
}
```

## schema

`GET /mcp/schema` returns an OpenAPI 3.0 document enumerating every gateway
path (`/search`, `/files/*`, `/exec*`, `/browser/*`, `/mcp/*`). Use it for
client generation or discovery.
