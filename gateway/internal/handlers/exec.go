package handlers

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/yourusername/toolset-api/gateway/internal/executor"
)

// execSyncTimeout bounds a synchronous /exec call end-to-end.
const execSyncTimeout = 300 * time.Second

// ExecSync handles POST /exec: run code synchronously in the sandbox and return
// the result. The sandbox tier (light/heavy) is chosen by language.
func (h *Handlers) ExecSync(c echo.Context) error {
	if h.Exec == nil {
		return echo.NewHTTPError(http.StatusServiceUnavailable, map[string]any{
			"error": "code execution is not enabled",
		})
	}

	var req executor.ExecRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, map[string]any{
			"error": "invalid request body",
		})
	}
	if req.Code == "" || req.Language == "" {
		return echo.NewHTTPError(http.StatusBadRequest, map[string]any{
			"error": "code and language are required",
		})
	}

	ctx, cancel := context.WithTimeout(c.Request().Context(), execSyncTimeout)
	defer cancel()

	res, err := h.Exec.Execute(ctx, req)
	if err != nil {
		return mapExecError(err)
	}
	return c.JSON(http.StatusOK, res)
}

// ExecAsync handles POST /exec/async: enqueue code for background execution and
// return a job id immediately.
func (h *Handlers) ExecAsync(c echo.Context) error {
	if h.Queue == nil {
		return echo.NewHTTPError(http.StatusServiceUnavailable, map[string]any{
			"error": "async execution is not enabled",
		})
	}

	var req executor.ExecRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, map[string]any{
			"error": "invalid request body",
		})
	}
	if req.Code == "" || req.Language == "" {
		return echo.NewHTTPError(http.StatusBadRequest, map[string]any{
			"error": "code and language are required",
		})
	}

	job, err := h.Queue.Submit(c.Request().Context(), h.ExecToolID, req)
	if err != nil {
		return mapExecError(err)
	}
	return c.JSON(http.StatusAccepted, map[string]any{
		"job_id":       job.JobID,
		"execution_id": job.ExecutionID,
		"status":       "queued",
		"position":     job.Position,
		"language":     job.Language,
	})
}

// ExecStatus handles GET /exec/:id: poll an async job's status/result.
func (h *Handlers) ExecStatus(c echo.Context) error {
	if h.Queue == nil {
		return echo.NewHTTPError(http.StatusServiceUnavailable, map[string]any{
			"error": "async execution is not enabled",
		})
	}
	jobID := c.Param("id")
	if jobID == "" {
		return echo.NewHTTPError(http.StatusBadRequest, map[string]any{"error": "job id required"})
	}

	job, err := h.Queue.Get(c.Request().Context(), jobID)
	if err != nil {
		return mapExecError(err)
	}
	return c.JSON(http.StatusOK, job)
}

// ExecCancel handles DELETE /exec/:id: cancel a pending or running job.
func (h *Handlers) ExecCancel(c echo.Context) error {
	if h.Queue == nil {
		return echo.NewHTTPError(http.StatusServiceUnavailable, map[string]any{
			"error": "async execution is not enabled",
		})
	}
	jobID := c.Param("id")
	if jobID == "" {
		return echo.NewHTTPError(http.StatusBadRequest, map[string]any{"error": "job id required"})
	}

	job, err := h.Queue.Cancel(c.Request().Context(), jobID)
	if err != nil {
		return mapExecError(err)
	}
	return c.JSON(http.StatusOK, map[string]any{
		"job_id": job.JobID,
		"status": string(job.Status),
	})
}

// mapExecError converts executor domain errors into HTTP responses.
func mapExecError(err error) error {
	switch {
	case errors.Is(err, executor.ErrUnsupportedLanguage):
		return echo.NewHTTPError(http.StatusBadRequest, map[string]any{"error": err.Error()})
	case errors.Is(err, executor.ErrTierDisabled):
		return echo.NewHTTPError(http.StatusServiceUnavailable, map[string]any{"error": err.Error()})
	case errors.Is(err, executor.ErrJobNotFound):
		return echo.NewHTTPError(http.StatusNotFound, map[string]any{"error": "job not found"})
	case errors.Is(err, executor.ErrJobNotCancellable):
		return echo.NewHTTPError(http.StatusConflict, map[string]any{"error": "job is not cancellable"})
	default:
		return echo.NewHTTPError(http.StatusBadGateway, map[string]any{"error": err.Error()})
	}
}
