package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/labstack/echo/v4"

	"github.com/yourusername/toolset-api/gateway/internal/executor"
	"github.com/yourusername/toolset-api/gateway/internal/mcp"
)

// MCPInitialize handles POST /mcp/initialize, returning the server's protocol
// version, identity, and advertised capabilities.
func (h *Handlers) MCPInitialize(c echo.Context) error {
	var req mcp.InitializeRequest
	_ = json.NewDecoder(c.Request().Body).Decode(&req)

	resp := mcp.InitializeResponse{
		ProtocolVersion: mcp.ProtocolVersion,
		Capabilities: mcp.ServerCapabilities{
			Tools:     &mcp.ToolsCapability{},
			Resources: &mcp.ResourcesCapability{},
		},
		ServerInfo: mcp.ServerInfo{
			Name:    "toolset-api",
			Version: h.Version,
		},
	}
	return c.JSON(http.StatusOK, resp)
}

// MCPToolsList handles POST /mcp/tools/list and POST /tools/list, returning the
// full tool catalog with JSON Schema input contracts.
func (h *Handlers) MCPToolsList(c echo.Context) error {
	var req mcp.ToolsListRequest
	_ = json.NewDecoder(c.Request().Body).Decode(&req)

	return c.JSON(http.StatusOK, mcp.ToolsListResponse{
		Tools: mcp.Catalog(),
	})
}

// MCPToolsCall handles POST /mcp/tools/call and POST /tools/call. It dispatches
// to the appropriate downstream service based on the requested tool name.
func (h *Handlers) MCPToolsCall(c echo.Context) error {
	var req mcp.ToolsCallRequest
	if err := json.NewDecoder(c.Request().Body).Decode(&req); err != nil {
		return c.JSON(http.StatusBadRequest, mcp.ToolsCallErrorResponse{
			JsonRpc: "2.0",
			Error: mcp.Error{
				Code:    mcp.CodeInvalidParams,
				Message: "invalid request body",
				Data:    err.Error(),
			},
		})
	}

	ctx, cancel := context.WithTimeout(c.Request().Context(), browserTimeout)
	defer cancel()

	var result string
	var err error
	switch req.Params.Name {
	case "search":
		result, err = h.mcpExecuteSearch(ctx, req.Params.Arguments)
	case "files_read":
		result, err = h.mcpProxyJSON(ctx, filesUpstream+"/files/read", req.Params.Arguments)
	case "files_write":
		result, err = h.mcpProxyJSON(ctx, filesUpstream+"/files/write", req.Params.Arguments)
	case "exec":
		result, err = h.mcpExecuteExec(ctx, req.Params.Arguments)
	case "browser_session_create":
		result, err = h.mcpProxyJSON(ctx, browserUpstream+"/session", req.Params.Arguments)
	case "browser_action":
		result, err = h.mcpExecuteBrowserAction(ctx, req.Params.Arguments)
	default:
		err = fmt.Errorf("unknown tool: %s", req.Params.Name)
	}

	if err != nil {
		return c.JSON(http.StatusBadRequest, mcp.ToolsCallErrorResponse{
			JsonRpc: "2.0",
			Error: mcp.Error{
				Code:    mcp.CodeInvalidParams,
				Message: "tool call failed",
				Data:    err.Error(),
			},
		})
	}

	return c.JSON(http.StatusOK, mcp.ToolsCallResponse{
		JsonRpc: "2.0",
		Content: []mcp.Content{{Type: "text", Text: result}},
	})
}

// MCPResourcesRead handles POST /mcp/resources/read. This is a placeholder that
// echoes the requested URI; real resource resolution lands in a later phase.
func (h *Handlers) MCPResourcesRead(c echo.Context) error {
	var req mcp.ResourcesReadRequest
	_ = json.NewDecoder(c.Request().Body).Decode(&req)

	return c.JSON(http.StatusOK, mcp.ResourcesReadResponse{
		Contents: []mcp.ResourceContent{
			{
				Uri:      req.Params.Uri,
				MimeType: "text/plain",
				Text:     "Resource placeholder",
			},
		},
	})
}

// MCPSchema handles GET /mcp/schema, returning an OpenAPI 3.0 description of the
// toolset API surface.
func (h *Handlers) MCPSchema(c echo.Context) error {
	spec := mcp.OpenAPISpec{
		OpenAPI: "3.0.0",
		Info: mcp.OpenAPIInfo{
			Title:       "Toolset API",
			Version:     h.Version,
			Description: "Search, files, code execution, and browser automation gateway.",
		},
		Paths: openAPIPaths(),
	}
	return c.JSON(http.StatusOK, spec)
}

// --- tools/call dispatch helpers --------------------------------------------

// mcpExecuteSearch runs a search via SearXNG and returns a JSON summary string.
func (h *Handlers) mcpExecuteSearch(ctx context.Context, args json.RawMessage) (string, error) {
	var in struct {
		Query   string   `json:"query"`
		Engines []string `json:"engines"`
		Page    int      `json:"page"`
		Lang    string   `json:"lang"`
	}
	if err := json.Unmarshal(args, &in); err != nil {
		return "", fmt.Errorf("invalid search arguments: %w", err)
	}
	if strings.TrimSpace(in.Query) == "" {
		return "", fmt.Errorf("query is required")
	}
	if in.Page <= 0 {
		in.Page = 1
	}

	q := url.Values{}
	q.Set("q", in.Query)
	q.Set("format", "json")
	q.Set("pageno", strconv.Itoa(in.Page))
	if len(in.Engines) > 0 {
		q.Set("engines", strings.Join(in.Engines, ","))
	}
	if in.Lang != "" {
		q.Set("language", in.Lang)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, searchUpstream+"/search?"+q.Encode(), nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/json")
	return doAndReadString(req)
}

// mcpExecuteExec runs code via the executor client and returns a JSON string.
func (h *Handlers) mcpExecuteExec(ctx context.Context, args json.RawMessage) (string, error) {
	if h.Exec == nil {
		return "", fmt.Errorf("code execution is not enabled")
	}
	var req executor.ExecRequest
	if err := json.Unmarshal(args, &req); err != nil {
		return "", fmt.Errorf("invalid exec arguments: %w", err)
	}
	if req.Code == "" || req.Language == "" {
		return "", fmt.Errorf("code and language are required")
	}
	res, err := h.Exec.Execute(ctx, req)
	if err != nil {
		return "", err
	}
	out, _ := json.Marshal(res)
	return string(out), nil
}

// mcpExecuteBrowserAction forwards a browser action, extracting session_id.
func (h *Handlers) mcpExecuteBrowserAction(ctx context.Context, args json.RawMessage) (string, error) {
	var meta struct {
		SessionID string `json:"session_id"`
	}
	if err := json.Unmarshal(args, &meta); err != nil {
		return "", fmt.Errorf("invalid browser_action arguments: %w", err)
	}
	if meta.SessionID == "" {
		return "", fmt.Errorf("session_id is required")
	}
	return h.mcpProxyJSON(ctx, browserUpstream+"/session/"+meta.SessionID+"/action", args)
}

// mcpProxyJSON POSTs the given JSON arguments to a URL and returns the response
// body as a string.
func (h *Handlers) mcpProxyJSON(ctx context.Context, target string, args json.RawMessage) (string, error) {
	body := []byte("{}")
	if len(args) > 0 {
		body = args
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, target, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	return doAndReadString(req)
}

// doAndReadString performs the request and returns the body as a string, failing
// on non-2xx status.
func doAndReadString(req *http.Request) (string, error) {
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("upstream unavailable: %w", err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read upstream response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("upstream status %d: %s", resp.StatusCode, string(raw))
	}
	return string(raw), nil
}

// openAPIPaths returns the OpenAPI path objects for the gateway surface.
func openAPIPaths() map[string]interface{} {
	post := func(summary string) map[string]interface{} {
		return map[string]interface{}{
			"post": map[string]interface{}{
				"summary":   summary,
				"responses": map[string]interface{}{"200": map[string]interface{}{"description": "OK"}},
			},
		}
	}
	get := func(summary string) map[string]interface{} {
		return map[string]interface{}{
			"get": map[string]interface{}{
				"summary":   summary,
				"responses": map[string]interface{}{"200": map[string]interface{}{"description": "OK"}},
			},
		}
	}
	del := func(summary string) map[string]interface{} {
		return map[string]interface{}{
			"delete": map[string]interface{}{
				"summary":   summary,
				"responses": map[string]interface{}{"204": map[string]interface{}{"description": "No Content"}},
			},
		}
	}

	return map[string]interface{}{
		"/health":               get("Gateway health and tool readiness"),
		"/tools":                get("List registered tools"),
		"/search":               post("Web search via SearXNG"),
		"/files/read":           post("Read a sandbox file"),
		"/files/write":          post("Write a sandbox file"),
		"/files/list":           post("List a sandbox directory"),
		"/files/delete":         post("Delete a sandbox file"),
		"/files/move":           post("Move/rename a sandbox file"),
		"/exec":                 post("Execute code synchronously"),
		"/exec/async":           post("Enqueue code for async execution"),
		"/browser/session":      post("Create a browser session"),
		"/browser/session/{id}": mergeOps(get("Get browser session metadata"), del("Destroy a browser session")),
		"/browser/action":       post("Execute a browser DOM action"),
		"/mcp/initialize":       post("MCP capability handshake"),
		"/mcp/tools/list":       post("List MCP tools"),
		"/mcp/tools/call":       post("Invoke an MCP tool"),
		"/mcp/resources/read":   post("Read an MCP resource"),
		"/mcp/schema":           get("This OpenAPI document"),
	}
}

// mergeOps combines multiple single-operation path maps into one path item.
func mergeOps(ops ...map[string]interface{}) map[string]interface{} {
	out := map[string]interface{}{}
	for _, op := range ops {
		for k, v := range op {
			out[k] = v
		}
	}
	return out
}
