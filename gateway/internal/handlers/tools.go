package handlers

import (
	"net/http"

	"github.com/labstack/echo/v4"
)

// Tools handles GET /tools, returning the full registered tool list.
func (h *Handlers) Tools(c echo.Context) error {
	tools, err := h.Registry.List(c.Request().Context())
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, map[string]any{
			"error": "failed to list tools",
		})
	}
	return c.JSON(http.StatusOK, map[string]any{
		"count": len(tools),
		"tools": tools,
	})
}
