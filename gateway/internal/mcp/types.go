// Package mcp defines the Model Context Protocol (MCP) wire types used by the
// gateway's /mcp/* endpoints. It targets protocol version 2024-11-05.
//
// Reference: https://spec.modelcontextprotocol.io/
//
// Only the request/response shapes the gateway actually serves are modeled.
// JSON-RPC framing fields (jsonrpc, id) are included where the spec requires
// them on responses; inbound framing is tolerated but not strictly validated.
package mcp

import "encoding/json"

// ProtocolVersion is the MCP revision this server implements.
const ProtocolVersion = "2024-11-05"

// --- initialize --------------------------------------------------------------

// InitializeRequest is the client's POST /mcp/initialize body.
type InitializeRequest struct {
	ProtocolVersion string             `json:"protocolVersion,omitempty"`
	Capabilities    ClientCapabilities `json:"capabilities,omitempty"`
	ClientInfo      ClientInfo         `json:"clientInfo,omitempty"`
}

// ClientCapabilities advertises what the client supports (informational here).
type ClientCapabilities struct {
	Experimental map[string]any `json:"experimental,omitempty"`
	Sampling     map[string]any `json:"sampling,omitempty"`
}

// ClientInfo identifies the connecting client.
type ClientInfo struct {
	Name    string `json:"name,omitempty"`
	Version string `json:"version,omitempty"`
}

// InitializeResponse is the server's capabilities handshake.
type InitializeResponse struct {
	ProtocolVersion string             `json:"protocolVersion"`
	Capabilities    ServerCapabilities `json:"capabilities"`
	ServerInfo      ServerInfo         `json:"serverInfo"`
}

// ServerCapabilities describes which MCP feature groups are available.
type ServerCapabilities struct {
	Tools     *ToolsCapability     `json:"tools,omitempty"`
	Resources *ResourcesCapability `json:"resources,omitempty"`
}

// ToolsCapability signals tool support. ListChanged advertises change notifs.
type ToolsCapability struct {
	ListChanged bool `json:"listChanged,omitempty"`
}

// ResourcesCapability signals resource support.
type ResourcesCapability struct {
	Subscribe   bool `json:"subscribe,omitempty"`
	ListChanged bool `json:"listChanged,omitempty"`
}

// ServerInfo identifies this server.
type ServerInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// --- tools/list --------------------------------------------------------------

// ToolsListRequest is the POST /mcp/tools/list body (cursor for pagination).
type ToolsListRequest struct {
	Cursor string `json:"cursor,omitempty"`
}

// ToolsListResponse returns the advertised tools.
type ToolsListResponse struct {
	Tools      []Tool `json:"tools"`
	NextCursor string `json:"nextCursor,omitempty"`
}

// Tool describes a callable tool and its JSON Schema input contract.
type Tool struct {
	Name        string    `json:"name"`
	Description string    `json:"description"`
	InputSchema ToolInput `json:"inputSchema"`
}

// ToolInput is a JSON Schema object describing a tool's arguments.
type ToolInput struct {
	Type       string                 `json:"type"`
	Properties map[string]interface{} `json:"properties"`
	Required   []string               `json:"required,omitempty"`
}

// --- tools/call --------------------------------------------------------------

// ToolsCallRequest is the POST /mcp/tools/call body.
type ToolsCallRequest struct {
	Params ToolsCallParams `json:"params"`
}

// ToolsCallParams carries the tool name and its arguments.
type ToolsCallParams struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

// ToolsCallResponse is the successful result envelope for a tool call.
type ToolsCallResponse struct {
	JsonRpc string    `json:"jsonrpc"`
	Content []Content `json:"content"`
	IsError bool      `json:"isError,omitempty"`
}

// Content is a single content block returned by a tool call.
type Content struct {
	Type     string `json:"type"`               // text | image | resource
	Text     string `json:"text,omitempty"`     // for type=text
	Data     string `json:"data,omitempty"`     // base64 for type=image
	MimeType string `json:"mimeType,omitempty"` // for type=image/resource
}

// ToolsCallErrorResponse is the JSON-RPC error envelope for a failed call.
type ToolsCallErrorResponse struct {
	JsonRpc string `json:"jsonrpc"`
	Error   Error  `json:"error"`
}

// Error is a JSON-RPC 2.0 error object.
type Error struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    string `json:"data,omitempty"`
}

// Standard JSON-RPC / MCP error codes used by the gateway.
const (
	CodeInvalidParams  = -32602
	CodeInternalError  = -32603
	CodeMethodNotFound = -32601
)

// --- resources/read ----------------------------------------------------------

// ResourcesReadRequest is the POST /mcp/resources/read body.
type ResourcesReadRequest struct {
	Params ResourcesReadParams `json:"params"`
}

// ResourcesReadParams identifies the resource to read.
type ResourcesReadParams struct {
	Uri string `json:"uri"`
}

// ResourcesReadResponse returns one or more resource content blocks.
type ResourcesReadResponse struct {
	Contents []ResourceContent `json:"contents"`
}

// ResourceContent is a single resource payload.
type ResourceContent struct {
	Uri      string `json:"uri"`
	MimeType string `json:"mimeType,omitempty"`
	Text     string `json:"text,omitempty"`
	Blob     string `json:"blob,omitempty"` // base64 for binary resources
}

// --- OpenAPI schema (GET /mcp/schema) ----------------------------------------

// OpenAPISpec is a minimal OpenAPI 3.0 document describing the toolset API.
type OpenAPISpec struct {
	OpenAPI string                 `json:"openapi"`
	Info    OpenAPIInfo            `json:"info"`
	Paths   map[string]interface{} `json:"paths"`
}

// OpenAPIInfo holds the spec's title/version metadata.
type OpenAPIInfo struct {
	Title       string `json:"title"`
	Version     string `json:"version"`
	Description string `json:"description,omitempty"`
}
