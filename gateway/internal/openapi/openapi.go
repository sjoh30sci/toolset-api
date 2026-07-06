// Package openapi produces the OpenAPI 3.0 specification for the Toolset API
// gateway. The spec is generated from hand-authored definitions that mirror the
// live handlers (search, files, exec, browser, mcp) so it can drive SDK
// generation (TypeScript + Python) without pulling in a heavy OpenAPI library.
//
// The output is emitted by `gateway --openapi-spec` and consumed by the
// openapi-generator toolchain in CI (see .github/workflows/test.yml and the
// `sdk-generate` Makefile target).
package openapi

import (
	"encoding/json"
)

// Spec is the top-level OpenAPI document. It is intentionally modeled with
// plain maps for the schema bodies so we can express arbitrary JSON Schema
// without a large dependency, while keeping the structural fields typed.
type Spec struct {
	OpenAPI    string         `json:"openapi"`
	Info       Info           `json:"info"`
	Servers    []Server       `json:"servers"`
	Paths      map[string]any `json:"paths"`
	Components Components     `json:"components"`
}

// Info is the OpenAPI info object.
type Info struct {
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Version     string   `json:"version"`
	License     *License `json:"license,omitempty"`
}

// License is the OpenAPI license object.
type License struct {
	Name string `json:"name"`
	URL  string `json:"url,omitempty"`
}

// Server is an OpenAPI server entry.
type Server struct {
	URL         string `json:"url"`
	Description string `json:"description,omitempty"`
}

// Components holds reusable schemas and security schemes.
type Components struct {
	Schemas         map[string]any `json:"schemas"`
	SecuritySchemes map[string]any `json:"securitySchemes"`
}

// Version is the API version advertised in the spec. It is overridable by the
// caller (gateway main) so the spec version tracks the build version.
var Version = "0.1.0"

// GenerateSpec returns the full OpenAPI 3.0 document for the gateway.
func GenerateSpec() *Spec {
	return &Spec{
		OpenAPI: "3.0.3",
		Info: Info{
			Title:       "Toolset API",
			Description: "Local API toolset that exposes web search, file operations, code execution, and browser automation to AI agents over REST and MCP.",
			Version:     Version,
			License:     &License{Name: "MIT", URL: "https://opensource.org/licenses/MIT"},
		},
		Servers: []Server{
			{URL: "http://localhost:8080", Description: "Local gateway"},
		},
		Components: Components{
			SecuritySchemes: map[string]any{
				"bearerAuth": map[string]any{
					"type":        "http",
					"scheme":      "bearer",
					"description": "Per-tool bearer token. Only required when the gateway runs in auth.mode=token.",
				},
			},
			Schemas: schemas(),
		},
		Paths: paths(),
	}
}

// JSON returns the indented JSON encoding of the spec.
func JSON() ([]byte, error) {
	return json.MarshalIndent(GenerateSpec(), "", "  ")
}

// obj is a small helper to build JSON Schema / OpenAPI fragments concisely.
func obj(m map[string]any) map[string]any { return m }

// jsonBody wraps a schema reference as an application/json request/response body.
func jsonContent(schema map[string]any) map[string]any {
	return map[string]any{
		"application/json": map[string]any{"schema": schema},
	}
}

// ref returns a $ref to a component schema.
func ref(name string) map[string]any {
	return map[string]any{"$ref": "#/components/schemas/" + name}
}

// paths returns the complete set of documented API paths.
func paths() map[string]any {
	return map[string]any{
		"/health": obj(map[string]any{
			"get": obj(map[string]any{
				"operationId": "getHealth",
				"tags":        []string{"system"},
				"summary":     "Liveness and tool readiness probe",
				"security":    []any{},
				"responses": obj(map[string]any{
					"200": obj(map[string]any{
						"description": "Gateway is healthy",
						"content":     jsonContent(ref("HealthResponse")),
					}),
				}),
			}),
		}),
		"/tools": obj(map[string]any{
			"get": obj(map[string]any{
				"operationId": "listTools",
				"tags":        []string{"system"},
				"summary":     "List registered tools and their status",
				"responses": obj(map[string]any{
					"200": obj(map[string]any{
						"description": "Registered tools",
						"content":     jsonContent(ref("ToolsResponse")),
					}),
				}),
			}),
		}),
		"/search": obj(map[string]any{
			"post": obj(map[string]any{
				"operationId": "search",
				"tags":        []string{"search"},
				"summary":     "Perform a web/meta search via SearXNG",
				"requestBody": obj(map[string]any{
					"required": true,
					"content":  jsonContent(ref("SearchRequest")),
				}),
				"responses": obj(map[string]any{
					"200": obj(map[string]any{
						"description": "Search results",
						"content":     jsonContent(ref("SearchResponse")),
					}),
					"400": errorResp("Invalid request"),
					"502": errorResp("Search upstream unavailable"),
				}),
			}),
		}),
		"/files/read":   fileOp("filesRead", "Read a file from the sandbox"),
		"/files/write":  fileOp("filesWrite", "Write a file to the sandbox"),
		"/files/list":   fileOp("filesList", "List files in a sandbox directory"),
		"/files/delete": fileOp("filesDelete", "Delete a file from the sandbox"),
		"/files/move":   fileOp("filesMove", "Move/rename a file within the sandbox"),
		"/exec": obj(map[string]any{
			"post": obj(map[string]any{
				"operationId": "exec",
				"tags":        []string{"exec"},
				"summary":     "Execute code synchronously in a sandbox",
				"requestBody": obj(map[string]any{
					"required": true,
					"content":  jsonContent(ref("ExecRequest")),
				}),
				"responses": obj(map[string]any{
					"200": obj(map[string]any{
						"description": "Execution result",
						"content":     jsonContent(ref("ExecResult")),
					}),
					"400": errorResp("Invalid request"),
					"503": errorResp("Execution not enabled"),
				}),
			}),
		}),
		"/exec/async": obj(map[string]any{
			"post": obj(map[string]any{
				"operationId": "execAsync",
				"tags":        []string{"exec"},
				"summary":     "Enqueue code for asynchronous execution",
				"requestBody": obj(map[string]any{
					"required": true,
					"content":  jsonContent(ref("ExecRequest")),
				}),
				"responses": obj(map[string]any{
					"202": obj(map[string]any{
						"description": "Job queued",
						"content":     jsonContent(ref("ExecJob")),
					}),
				}),
			}),
		}),
		"/exec/{id}": obj(map[string]any{
			"get": obj(map[string]any{
				"operationId": "execStatus",
				"tags":        []string{"exec"},
				"summary":     "Poll an async execution job",
				"parameters":  []any{idParam()},
				"responses": obj(map[string]any{
					"200": obj(map[string]any{
						"description": "Job status",
						"content":     jsonContent(ref("ExecJob")),
					}),
					"404": errorResp("Job not found"),
				}),
			}),
			"delete": obj(map[string]any{
				"operationId": "execCancel",
				"tags":        []string{"exec"},
				"summary":     "Cancel a pending or running job",
				"parameters":  []any{idParam()},
				"responses": obj(map[string]any{
					"200": obj(map[string]any{"description": "Job cancelled"}),
					"409": errorResp("Job not cancellable"),
				}),
			}),
		}),
		"/browser/session": obj(map[string]any{
			"post": obj(map[string]any{
				"operationId": "browserSessionCreate",
				"tags":        []string{"browser"},
				"summary":     "Create a browser session",
				"requestBody": obj(map[string]any{
					"required": true,
					"content":  jsonContent(ref("BrowserSessionRequest")),
				}),
				"responses": obj(map[string]any{
					"200": obj(map[string]any{"description": "Session created"}),
				}),
			}),
		}),
		"/browser/session/{id}": obj(map[string]any{
			"get": obj(map[string]any{
				"operationId": "browserSessionGet",
				"tags":        []string{"browser"},
				"summary":     "Get browser session details",
				"parameters":  []any{idParam()},
				"responses": obj(map[string]any{
					"200": obj(map[string]any{"description": "Session details"}),
					"404": errorResp("Session not found"),
				}),
			}),
			"delete": obj(map[string]any{
				"operationId": "browserSessionDelete",
				"tags":        []string{"browser"},
				"summary":     "Close a browser session",
				"parameters":  []any{idParam()},
				"responses": obj(map[string]any{
					"200": obj(map[string]any{"description": "Session closed"}),
				}),
			}),
		}),
		"/browser/action": obj(map[string]any{
			"post": obj(map[string]any{
				"operationId": "browserAction",
				"tags":        []string{"browser"},
				"summary":     "Execute an action within a browser session",
				"requestBody": obj(map[string]any{
					"required": true,
					"content":  jsonContent(ref("BrowserActionRequest")),
				}),
				"responses": obj(map[string]any{
					"200": obj(map[string]any{"description": "Action result"}),
				}),
			}),
		}),
		"/mcp/initialize":     mcpOp("mcpInitialize", "MCP initialize handshake"),
		"/mcp/tools/list":     mcpOp("mcpToolsList", "List MCP tools"),
		"/mcp/tools/call":     mcpOp("mcpToolsCall", "Call an MCP tool"),
		"/mcp/resources/read": mcpOp("mcpResourcesRead", "Read an MCP resource"),
		"/mcp/schema": obj(map[string]any{
			"get": obj(map[string]any{
				"operationId": "mcpSchema",
				"tags":        []string{"mcp"},
				"summary":     "Fetch the MCP tool schema",
				"responses": obj(map[string]any{
					"200": obj(map[string]any{"description": "MCP schema"}),
				}),
			}),
		}),
	}
}

// fileOp builds a standard POST file-operation path item.
func fileOp(opID, summary string) map[string]any {
	return obj(map[string]any{
		"post": obj(map[string]any{
			"operationId": opID,
			"tags":        []string{"files"},
			"summary":     summary,
			"requestBody": obj(map[string]any{
				"required": true,
				"content":  jsonContent(ref("FileRequest")),
			}),
			"responses": obj(map[string]any{
				"200": obj(map[string]any{
					"description": "Operation result",
					"content":     jsonContent(ref("FileResponse")),
				}),
				"400": errorResp("Invalid request"),
			}),
		}),
	})
}

// mcpOp builds a standard MCP POST path item accepting an arbitrary JSON-RPC body.
func mcpOp(opID, summary string) map[string]any {
	return obj(map[string]any{
		"post": obj(map[string]any{
			"operationId": opID,
			"tags":        []string{"mcp"},
			"summary":     summary,
			"requestBody": obj(map[string]any{
				"required": true,
				"content":  jsonContent(obj(map[string]any{"type": "object"})),
			}),
			"responses": obj(map[string]any{
				"200": obj(map[string]any{"description": "MCP response"}),
			}),
		}),
	})
}

// idParam is the shared {id} path parameter.
func idParam() map[string]any {
	return obj(map[string]any{
		"name":     "id",
		"in":       "path",
		"required": true,
		"schema":   obj(map[string]any{"type": "string"}),
	})
}

// errorResp builds a standard error response referencing the Error schema.
func errorResp(desc string) map[string]any {
	return obj(map[string]any{
		"description": desc,
		"content":     jsonContent(ref("Error")),
	})
}

// schemas returns all reusable component schemas.
func schemas() map[string]any {
	return map[string]any{
		"Error": obj(map[string]any{
			"type": "object",
			"properties": obj(map[string]any{
				"error": obj(map[string]any{"type": "string"}),
			}),
			"required": []string{"error"},
		}),
		"HealthResponse": obj(map[string]any{
			"type": "object",
			"properties": obj(map[string]any{
				"status":  obj(map[string]any{"type": "string", "example": "ok"}),
				"version": obj(map[string]any{"type": "string"}),
				"tools":   obj(map[string]any{"type": "object"}),
			}),
		}),
		"ToolsResponse": obj(map[string]any{
			"type": "object",
			"properties": obj(map[string]any{
				"tools": obj(map[string]any{"type": "array", "items": obj(map[string]any{"type": "object"})}),
			}),
		}),
		"SearchRequest": obj(map[string]any{
			"type": "object",
			"properties": obj(map[string]any{
				"query":   obj(map[string]any{"type": "string", "description": "Search query"}),
				"engines": obj(map[string]any{"type": "array", "items": obj(map[string]any{"type": "string"})}),
				"page":    obj(map[string]any{"type": "integer", "minimum": 1}),
				"lang":    obj(map[string]any{"type": "string"}),
			},
			),
			"required": []string{"query"},
		}),
		"SearchResult": obj(map[string]any{
			"type": "object",
			"properties": obj(map[string]any{
				"title":   obj(map[string]any{"type": "string"}),
				"url":     obj(map[string]any{"type": "string"}),
				"snippet": obj(map[string]any{"type": "string"}),
				"engine":  obj(map[string]any{"type": "string"}),
			}),
		}),
		"SearchResponse": obj(map[string]any{
			"type": "object",
			"properties": obj(map[string]any{
				"query":   obj(map[string]any{"type": "string"}),
				"page":    obj(map[string]any{"type": "integer"}),
				"count":   obj(map[string]any{"type": "integer"}),
				"results": obj(map[string]any{"type": "array", "items": ref("SearchResult")}),
			}),
		}),
		"FileRequest": obj(map[string]any{
			"type": "object",
			"properties": obj(map[string]any{
				"path":        obj(map[string]any{"type": "string", "description": "Sandbox-relative path"}),
				"content":     obj(map[string]any{"type": "string", "description": "File content (write)"}),
				"destination": obj(map[string]any{"type": "string", "description": "Destination path (move)"}),
			}),
			"required": []string{"path"},
		}),
		"FileResponse": obj(map[string]any{
			"type": "object",
			"properties": obj(map[string]any{
				"path":    obj(map[string]any{"type": "string"}),
				"content": obj(map[string]any{"type": "string"}),
				"entries": obj(map[string]any{"type": "array", "items": obj(map[string]any{"type": "object"})}),
			}),
		}),
		"ExecRequest": obj(map[string]any{
			"type": "object",
			"properties": obj(map[string]any{
				"code":     obj(map[string]any{"type": "string", "description": "Source code to execute"}),
				"language": obj(map[string]any{"type": "string", "description": "Runtime language", "example": "python"}),
				"stdin":    obj(map[string]any{"type": "string"}),
				"timeout":  obj(map[string]any{"type": "integer", "description": "Seconds"}),
			}),
			"required": []string{"code", "language"},
		}),
		"ExecResult": obj(map[string]any{
			"type": "object",
			"properties": obj(map[string]any{
				"stdout":    obj(map[string]any{"type": "string"}),
				"stderr":    obj(map[string]any{"type": "string"}),
				"exit_code": obj(map[string]any{"type": "integer"}),
				"duration":  obj(map[string]any{"type": "number"}),
			}),
		}),
		"ExecJob": obj(map[string]any{
			"type": "object",
			"properties": obj(map[string]any{
				"job_id":       obj(map[string]any{"type": "string"}),
				"execution_id": obj(map[string]any{"type": "string"}),
				"status":       obj(map[string]any{"type": "string", "enum": []string{"queued", "running", "completed", "failed", "cancelled"}}),
				"position":     obj(map[string]any{"type": "integer"}),
				"language":     obj(map[string]any{"type": "string"}),
				"result":       ref("ExecResult"),
			}),
		}),
		"BrowserSessionRequest": obj(map[string]any{
			"type": "object",
			"properties": obj(map[string]any{
				"browserType": obj(map[string]any{"type": "string", "enum": []string{"chromium", "firefox", "webkit"}}),
			}),
		}),
		"BrowserActionRequest": obj(map[string]any{
			"type": "object",
			"properties": obj(map[string]any{
				"session_id": obj(map[string]any{"type": "string"}),
				"action":     obj(map[string]any{"type": "object", "description": "Action payload (navigate, click, screenshot, pdf, ...)"}),
			}),
			"required": []string{"session_id", "action"},
		}),
	}
}
