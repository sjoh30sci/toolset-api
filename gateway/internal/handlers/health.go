// Package handlers implements the gateway's HTTP endpoints.
package handlers

import (
	"net/http"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/yourusername/toolset-api/gateway/internal/registry"
)

// Handlers bundles the dependencies shared across HTTP handlers.
type Handlers struct {
	Registry *registry.Registry
	Version  string
	Started  time.Time
}

// New creates a Handlers instance.
func New(reg *registry.Registry, version string) *Handlers {
	return &Handlers{
		Registry: reg,
		Version:  version,
		Started:  time.Now().UTC(),
	}
}

// Health handles GET /health. It reports gateway status and per-tool readiness.
func (h *Handlers) Health(c echo.Context) error {
	tools, err := h.Registry.List(c.Request().Context())
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, map[string]any{
			"error": "failed to read tool registry",
		})
	}

	toolStatus := map[string]string{}
	for _, t := range tools {
		toolStatus[t.Name] = string(t.Status)
	}

	return c.JSON(http.StatusOK, map[string]any{
		"status":     "ok",
		"version":    h.Version,
		"uptime_sec": int(time.Since(h.Started).Seconds()),
		"tools":      toolStatus,
	})
}

// --- MCP endpoint stubs -----------------------------------------------------

// MCPInitialize handles POST /mcp/initialize.
func (h *Handlers) MCPInitialize(c echo.Context) error {
	return c.JSON(http.StatusOK, map[string]any{
		"protocolVersion": "2024-11-05",
		"serverInfo": map[string]any{
			"name":    "toolset-api",
			"version": h.Version,
		},
		"capabilities": map[string]any{
			"tools":     map[string]any{},
			"resources": map[string]any{},
		},
	})
}

// MCPToolsList handles POST /mcp/tools/list and POST /tools/list.
func (h *Handlers) MCPToolsList(c echo.Context) error {
	tools, err := h.Registry.List(c.Request().Context())
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, map[string]any{
			"error": "failed to list tools",
		})
	}
	list := make([]map[string]any, 0, len(tools))
	for _, t := range tools {
		list = append(list, map[string]any{
			"name":        t.Name,
			"description": t.Description,
		})
	}
	return c.JSON(http.StatusOK, map[string]any{"tools": list})
}

// MCPToolsCall handles POST /mcp/tools/call and POST /tools/call (stub).
func (h *Handlers) MCPToolsCall(c echo.Context) error {
	return c.JSON(http.StatusNotImplemented, map[string]any{
		"error": "tools/call is not implemented in Phase 1",
	})
}

// MCPResourcesRead handles POST /mcp/resources/read (stub).
func (h *Handlers) MCPResourcesRead(c echo.Context) error {
	return c.JSON(http.StatusNotImplemented, map[string]any{
		"error": "resources/read is not implemented in Phase 1",
	})
}

// MCPSchema handles GET /mcp/schema. Returns a minimal OpenAPI stub.
func (h *Handlers) MCPSchema(c echo.Context) error {
	return c.JSON(http.StatusOK, map[string]any{
		"openapi": "3.1.0",
		"info": map[string]any{
			"title":   "Toolset API",
			"version": h.Version,
		},
		"paths": map[string]any{},
	})
}
