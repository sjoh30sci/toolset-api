// Package handlers implements the gateway's HTTP endpoints.
package handlers

import (
	"net/http"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/yourusername/toolset-api/gateway/internal/executor"
	"github.com/yourusername/toolset-api/gateway/internal/registry"
)

// Handlers bundles the dependencies shared across HTTP handlers.
type Handlers struct {
	Registry *registry.Registry
	Version  string
	Started  time.Time

	// Exec is the sandbox client used by the /exec endpoints (optional; nil
	// disables synchronous execution).
	Exec *executor.Client
	// Queue backs the async /exec endpoints (optional; nil disables async).
	Queue *executor.Queue
	// ExecToolID is the registry ID of the exec tool, stamped onto jobs.
	ExecToolID string
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
