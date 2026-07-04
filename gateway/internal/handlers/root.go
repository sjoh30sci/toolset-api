package handlers

import (
	"net/http"

	"github.com/labstack/echo/v4"
)

// Root handles GET /, returning a welcome payload and API docs pointer.
func (h *Handlers) Root(c echo.Context) error {
	return c.JSON(http.StatusOK, map[string]any{
		"name":    "toolset-api",
		"version": h.Version,
		"message": "Local API toolset for AI agents.",
		"docs":    "/mcp/schema",
		"endpoints": map[string]any{
			"health":    "GET /health",
			"tools":     "GET /tools",
			"mcp":       "POST /mcp/initialize, /mcp/tools/list, /mcp/tools/call, /mcp/resources/read",
			"mcpSchema": "GET /mcp/schema",
		},
	})
}
