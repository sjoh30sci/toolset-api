package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/yourusername/toolset-api/gateway/internal/mcp"
)

func newMCPHandler() *Handlers {
	return &Handlers{Version: "test", Started: time.Now()}
}

func invokeMCP(t *testing.T, method, path, body string, fn func(echo.Context) error) *httptest.ResponseRecorder {
	t.Helper()
	e := echo.New()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	if err := fn(c); err != nil {
		e.HTTPErrorHandler(err, c)
	}
	return rec
}

func TestMCPInitialize(t *testing.T) {
	h := newMCPHandler()
	rec := invokeMCP(t, http.MethodPost, "/mcp/initialize", `{"protocolVersion":"2024-11-05"}`, h.MCPInitialize)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var resp mcp.InitializeResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.ProtocolVersion != mcp.ProtocolVersion {
		t.Errorf("expected protocol %s, got %s", mcp.ProtocolVersion, resp.ProtocolVersion)
	}
	if resp.ServerInfo.Name != "toolset-api" {
		t.Errorf("expected server name toolset-api, got %s", resp.ServerInfo.Name)
	}
	if resp.Capabilities.Tools == nil || resp.Capabilities.Resources == nil {
		t.Errorf("expected tools+resources capabilities advertised")
	}
}

func TestMCPToolsList(t *testing.T) {
	h := newMCPHandler()
	rec := invokeMCP(t, http.MethodPost, "/mcp/tools/list", `{}`, h.MCPToolsList)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var resp mcp.ToolsListResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	want := map[string]bool{
		"search": false, "files_read": false, "files_write": false,
		"exec": false, "browser_session_create": false, "browser_action": false,
	}
	for _, tool := range resp.Tools {
		if _, ok := want[tool.Name]; ok {
			want[tool.Name] = true
		}
		if tool.InputSchema.Type != "object" {
			t.Errorf("tool %s: expected object input schema, got %q", tool.Name, tool.InputSchema.Type)
		}
	}
	for name, seen := range want {
		if !seen {
			t.Errorf("expected tool %q in catalog", name)
		}
	}
}

func TestMCPToolsCallUnknownTool(t *testing.T) {
	h := newMCPHandler()
	body := `{"params":{"name":"nope","arguments":{}}}`
	rec := invokeMCP(t, http.MethodPost, "/mcp/tools/call", body, h.MCPToolsCall)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for unknown tool, got %d", rec.Code)
	}
	var resp mcp.ToolsCallErrorResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.Error.Code != mcp.CodeInvalidParams {
		t.Errorf("expected code %d, got %d", mcp.CodeInvalidParams, resp.Error.Code)
	}
}

func TestMCPToolsCallSearch(t *testing.T) {
	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"results":[{"title":"Go","url":"https://go.dev","content":"x","engine":"google"}]}`))
	}))
	defer mock.Close()

	orig := searchUpstream
	searchUpstream = mock.URL
	defer func() { searchUpstream = orig }()

	h := newMCPHandler()
	body := `{"params":{"name":"search","arguments":{"query":"golang"}}}`
	rec := invokeMCP(t, http.MethodPost, "/mcp/tools/call", body, h.MCPToolsCall)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp mcp.ToolsCallResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(resp.Content) == 0 || !strings.Contains(resp.Content[0].Text, "go.dev") {
		t.Errorf("expected search result in content, got %+v", resp.Content)
	}
}

func TestMCPToolsCallBrowserSessionCreate(t *testing.T) {
	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/session" {
			t.Errorf("expected /session, got %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":"sess-1","browserType":"chromium"}`))
	}))
	defer mock.Close()

	orig := browserUpstream
	browserUpstream = mock.URL
	defer func() { browserUpstream = orig }()

	h := newMCPHandler()
	body := `{"params":{"name":"browser_session_create","arguments":{"browserType":"chromium"}}}`
	rec := invokeMCP(t, http.MethodPost, "/mcp/tools/call", body, h.MCPToolsCall)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "sess-1") {
		t.Errorf("expected session id in response, got %s", rec.Body.String())
	}
}

func TestMCPResourcesRead(t *testing.T) {
	h := newMCPHandler()
	body := `{"params":{"uri":"file:///data/x.txt"}}`
	rec := invokeMCP(t, http.MethodPost, "/mcp/resources/read", body, h.MCPResourcesRead)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var resp mcp.ResourcesReadResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(resp.Contents) != 1 || resp.Contents[0].Uri != "file:///data/x.txt" {
		t.Errorf("expected echoed uri, got %+v", resp.Contents)
	}
}

func TestMCPSchema(t *testing.T) {
	h := newMCPHandler()
	rec := invokeMCP(t, http.MethodGet, "/mcp/schema", "", h.MCPSchema)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var spec mcp.OpenAPISpec
	if err := json.Unmarshal(rec.Body.Bytes(), &spec); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if spec.OpenAPI != "3.0.0" {
		t.Errorf("expected openapi 3.0.0, got %s", spec.OpenAPI)
	}
	if _, ok := spec.Paths["/browser/action"]; !ok {
		t.Errorf("expected /browser/action path in schema")
	}
	if _, ok := spec.Paths["/mcp/tools/call"]; !ok {
		t.Errorf("expected /mcp/tools/call path in schema")
	}
}
