package handlers

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
)

// filesUpstream is the base URL of the file-server on the internal network.
// Overridable via FILES_UPSTREAM_URL (used by tests to point at a mock).
var filesUpstream = "http://files-server:8765"

// filesTimeout bounds a single upstream file operation.
const filesTimeout = 30 * time.Second

// FilesRead proxies POST /files/read to the file-server.
func (h *Handlers) FilesRead(c echo.Context) error { return h.proxyFiles(c, "/files/read") }

// FilesWrite proxies POST /files/write to the file-server.
func (h *Handlers) FilesWrite(c echo.Context) error { return h.proxyFiles(c, "/files/write") }

// FilesList proxies POST /files/list to the file-server.
func (h *Handlers) FilesList(c echo.Context) error { return h.proxyFiles(c, "/files/list") }

// FilesDelete proxies POST /files/delete to the file-server.
func (h *Handlers) FilesDelete(c echo.Context) error { return h.proxyFiles(c, "/files/delete") }

// FilesMove proxies POST /files/move to the file-server.
func (h *Handlers) FilesMove(c echo.Context) error { return h.proxyFiles(c, "/files/move") }

// proxyFiles forwards the request body to the file-server op endpoint and
// streams the response (status + body) back to the caller. Any inbound
// Authorization header is passed through.
func (h *Handlers) proxyFiles(c echo.Context, op string) error {
	body, err := io.ReadAll(c.Request().Body)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, map[string]any{
			"error": "failed to read request body",
		})
	}
	defer c.Request().Body.Close()

	ctx, cancel := context.WithTimeout(c.Request().Context(), filesTimeout)
	defer cancel()

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, filesUpstream+op, bytes.NewReader(body))
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, map[string]any{
			"error": "failed to build upstream request",
		})
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if authz := c.Request().Header.Get(echo.HeaderAuthorization); authz != "" {
		httpReq.Header.Set(echo.HeaderAuthorization, authz)
	}

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadGateway, map[string]any{
			"error": "files upstream unavailable",
		})
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadGateway, map[string]any{
			"error": "failed to read upstream response",
		})
	}

	ct := resp.Header.Get("Content-Type")
	if ct == "" {
		ct = "application/json"
	}
	return c.Blob(resp.StatusCode, ct, respBody)
}
