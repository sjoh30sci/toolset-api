package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
)

func newBrowserHandler() *Handlers {
	return &Handlers{Version: "test", Started: time.Now()}
}

// invokeBrowser runs a browser handler, routing :id params via echo when needed,
// and applies echo's default error handler on failure.
func invokeBrowser(t *testing.T, h *Handlers, method, path, body string, fn func(echo.Context) error, idParam string) *httptest.ResponseRecorder {
	t.Helper()
	e := echo.New()
	var reader *strings.Reader
	if body != "" {
		reader = strings.NewReader(body)
	} else {
		reader = strings.NewReader("")
	}
	req := httptest.NewRequest(method, path, reader)
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	if idParam != "" {
		c.SetParamNames("id")
		c.SetParamValues(idParam)
	}
	if err := fn(c); err != nil {
		e.HTTPErrorHandler(err, c)
	}
	return rec
}

func TestBrowserSessionCreate(t *testing.T) {
	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/session" {
			t.Errorf("expected POST /session, got %s %s", r.Method, r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":"abc123","browserType":"chromium"}`))
	}))
	defer mock.Close()

	orig := browserUpstream
	browserUpstream = mock.URL
	defer func() { browserUpstream = orig }()

	h := newBrowserHandler()
	rec := invokeBrowser(t, h, http.MethodPost, "/browser/session", `{"browserType":"chromium"}`, h.BrowserSessionCreate, "")
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "abc123") {
		t.Errorf("session id not passed through: %s", rec.Body.String())
	}
}

func TestBrowserSessionGet(t *testing.T) {
	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/session/abc123" {
			t.Errorf("expected GET /session/abc123, got %s %s", r.Method, r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"abc123","url":"https://go.dev","title":"Go"}`))
	}))
	defer mock.Close()

	orig := browserUpstream
	browserUpstream = mock.URL
	defer func() { browserUpstream = orig }()

	h := newBrowserHandler()
	rec := invokeBrowser(t, h, http.MethodGet, "/browser/session/abc123", "", h.BrowserSessionGet, "abc123")
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "go.dev") {
		t.Errorf("metadata not passed through: %s", rec.Body.String())
	}
}

func TestBrowserSessionDelete(t *testing.T) {
	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete || r.URL.Path != "/session/abc123" {
			t.Errorf("expected DELETE /session/abc123, got %s %s", r.Method, r.URL.Path)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer mock.Close()

	orig := browserUpstream
	browserUpstream = mock.URL
	defer func() { browserUpstream = orig }()

	h := newBrowserHandler()
	rec := invokeBrowser(t, h, http.MethodDelete, "/browser/session/abc123", "", h.BrowserSessionDelete, "abc123")
	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", rec.Code)
	}
}

func TestBrowserActionDispatch(t *testing.T) {
	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/session/abc123/action" {
			t.Errorf("expected POST /session/abc123/action, got %s %s", r.Method, r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"success","url":"https://go.dev","title":"Go"}`))
	}))
	defer mock.Close()

	orig := browserUpstream
	browserUpstream = mock.URL
	defer func() { browserUpstream = orig }()

	h := newBrowserHandler()
	body := `{"session_id":"abc123","type":"navigate","url":"https://go.dev"}`
	rec := invokeBrowser(t, h, http.MethodPost, "/browser/action", body, h.BrowserAction, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "success") {
		t.Errorf("action result not passed through: %s", rec.Body.String())
	}
}

func TestBrowserActionMissingSessionID(t *testing.T) {
	h := newBrowserHandler()
	rec := invokeBrowser(t, h, http.MethodPost, "/browser/action", `{"type":"navigate"}`, h.BrowserAction, "")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for missing session_id, got %d", rec.Code)
	}
}

func TestBrowserActionInvalidJSON(t *testing.T) {
	h := newBrowserHandler()
	rec := invokeBrowser(t, h, http.MethodPost, "/browser/action", `{not json`, h.BrowserAction, "")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid JSON, got %d", rec.Code)
	}
}

func TestBrowserUpstreamDown(t *testing.T) {
	orig := browserUpstream
	browserUpstream = "http://127.0.0.1:0"
	defer func() { browserUpstream = orig }()

	h := newBrowserHandler()
	rec := invokeBrowser(t, h, http.MethodPost, "/browser/session", `{}`, h.BrowserSessionCreate, "")
	if rec.Code != http.StatusBadGateway {
		t.Fatalf("expected 502 when upstream is down, got %d", rec.Code)
	}
}

func TestBrowserSessionGetMissingID(t *testing.T) {
	h := newBrowserHandler()
	rec := invokeBrowser(t, h, http.MethodGet, "/browser/session/", "", h.BrowserSessionGet, "")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for missing id, got %d", rec.Code)
	}
}

// sanity: create response is valid JSON we can decode.
func TestBrowserCreateResponseDecodes(t *testing.T) {
	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":"s1","browserType":"firefox"}`))
	}))
	defer mock.Close()

	orig := browserUpstream
	browserUpstream = mock.URL
	defer func() { browserUpstream = orig }()

	h := newBrowserHandler()
	rec := invokeBrowser(t, h, http.MethodPost, "/browser/session", `{"browserType":"firefox"}`, h.BrowserSessionCreate, "")
	var out map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("response not valid JSON: %v", err)
	}
	if out["browserType"] != "firefox" {
		t.Errorf("expected browserType firefox, got %v", out["browserType"])
	}
}
