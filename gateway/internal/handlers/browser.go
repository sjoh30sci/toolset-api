package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
)

// browserUpstream is the base URL of the Playwright browser service on the
// internal network. Overridable via a package var so tests can point at a mock.
var browserUpstream = "http://browser:3000"

// browserTimeout bounds a single upstream browser operation. Browser actions
// (navigate/screenshot/pdf) can be slow, so this is generous.
const browserTimeout = 30 * time.Second

// browserActionRequest is the inbound POST /browser/action body. It carries the
// target session id plus an arbitrary action payload forwarded to the service.
type browserActionRequest struct {
	SessionID string `json:"session_id"`
}

// BrowserSessionCreate handles POST /browser/session by proxying to the browser
// service's POST /session endpoint.
func (h *Handlers) BrowserSessionCreate(c echo.Context) error {
	body, err := io.ReadAll(c.Request().Body)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, map[string]any{
			"error": "failed to read request body",
		})
	}
	defer c.Request().Body.Close()
	if len(body) == 0 {
		body = []byte("{}")
	}
	return h.proxyBrowser(c, http.MethodPost, "/session", body)
}

// BrowserSessionGet handles GET /browser/session/:id by proxying to the browser
// service's GET /session/:id endpoint.
func (h *Handlers) BrowserSessionGet(c echo.Context) error {
	id := c.Param("id")
	if id == "" {
		return echo.NewHTTPError(http.StatusBadRequest, map[string]any{
			"error": "session id required",
		})
	}
	return h.proxyBrowser(c, http.MethodGet, "/session/"+id, nil)
}

// BrowserSessionDelete handles DELETE /browser/session/:id by proxying to the
// browser service's DELETE /session/:id endpoint.
func (h *Handlers) BrowserSessionDelete(c echo.Context) error {
	id := c.Param("id")
	if id == "" {
		return echo.NewHTTPError(http.StatusBadRequest, map[string]any{
			"error": "session id required",
		})
	}
	return h.proxyBrowser(c, http.MethodDelete, "/session/"+id, nil)
}

// BrowserAction handles POST /browser/action. The body must include a
// "session_id"; the remaining fields form the action and are forwarded verbatim
// to the browser service's POST /session/:id/action endpoint.
func (h *Handlers) BrowserAction(c echo.Context) error {
	body, err := io.ReadAll(c.Request().Body)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, map[string]any{
			"error": "failed to read request body",
		})
	}
	defer c.Request().Body.Close()

	var meta browserActionRequest
	if err := json.Unmarshal(body, &meta); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, map[string]any{
			"error": "invalid JSON body",
		})
	}
	if meta.SessionID == "" {
		return echo.NewHTTPError(http.StatusBadRequest, map[string]any{
			"error": "session_id is required",
		})
	}

	return h.proxyBrowser(c, http.MethodPost, "/session/"+meta.SessionID+"/action", body)
}

// proxyBrowser forwards a request to the browser service and streams the
// response (status + body) back to the caller. A nil body sends no payload.
func (h *Handlers) proxyBrowser(c echo.Context, method, path string, body []byte) error {
	ctx, cancel := context.WithTimeout(c.Request().Context(), browserTimeout)
	defer cancel()

	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}

	httpReq, err := http.NewRequestWithContext(ctx, method, browserUpstream+path, reader)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, map[string]any{
			"error": "failed to build upstream request",
		})
	}
	if body != nil {
		httpReq.Header.Set("Content-Type", "application/json")
	}

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadGateway, map[string]any{
			"error": "browser upstream unavailable",
		})
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadGateway, map[string]any{
			"error": "failed to read upstream response",
		})
	}

	// 204 No Content has no body; return it as-is.
	if resp.StatusCode == http.StatusNoContent || len(respBody) == 0 {
		return c.NoContent(resp.StatusCode)
	}

	ct := resp.Header.Get("Content-Type")
	if ct == "" {
		ct = "application/json"
	}
	return c.Blob(resp.StatusCode, ct, respBody)
}
