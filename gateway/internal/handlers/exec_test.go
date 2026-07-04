package handlers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/yourusername/toolset-api/gateway/internal/executor"
)

// newExecHandler builds a Handlers wired to a client pointed at the given light
// and heavy mock base URLs.
func newExecHandler(lightURL, heavyURL string, lightOn, heavyOn bool) *Handlers {
	c := executor.NewClient()
	c.LightURL = lightURL
	c.HeavyURL = heavyURL
	c.LightEnabled = lightOn
	c.HeavyEnabled = heavyOn
	c.HTTP = &http.Client{Timeout: 5 * time.Second}
	return &Handlers{Version: "test", Started: time.Now(), Exec: c}
}

// invokeExec calls an exec handler and applies echo's error handler on failure.
func invokeExec(t *testing.T, h *Handlers, fn func(echo.Context) error, method, path, body string) *httptest.ResponseRecorder {
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

func TestExecSyncSuccess(t *testing.T) {
	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/exec" {
			t.Errorf("expected /exec, got %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"success","exit_code":0,"stdout":"hi\n","stderr":"","duration_seconds":0.1}`))
	}))
	defer mock.Close()

	h := newExecHandler(mock.URL, "", true, false)
	rec := invokeExec(t, h, h.ExecSync, http.MethodPost, "/exec", `{"code":"print('hi')","language":"python"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (%s)", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "hi") {
		t.Errorf("stdout not passed through: %s", rec.Body.String())
	}
}

func TestExecSyncCompileError(t *testing.T) {
	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"error","error":"Compilation failed","stderr":"syntax error"}`))
	}))
	defer mock.Close()

	h := newExecHandler(mock.URL, "", true, false)
	rec := invokeExec(t, h, h.ExecSync, http.MethodPost, "/exec", `{"code":"int main(){","language":"c"}`)
	// Upstream returned 200 with an error status body; the gateway relays it.
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "Compilation failed") {
		t.Errorf("compile error not relayed: %s", rec.Body.String())
	}
}

func TestExecSyncTimeoutStatus(t *testing.T) {
	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"timeout","error":"Execution timeout after 1 seconds"}`))
	}))
	defer mock.Close()

	h := newExecHandler(mock.URL, "", true, false)
	rec := invokeExec(t, h, h.ExecSync, http.MethodPost, "/exec", `{"code":"while True: pass","language":"python","timeout":1}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "timeout") {
		t.Errorf("timeout status not relayed: %s", rec.Body.String())
	}
}

func TestExecSyncUnsupportedLanguage(t *testing.T) {
	h := newExecHandler("http://unused", "", true, false)
	rec := invokeExec(t, h, h.ExecSync, http.MethodPost, "/exec", `{"code":"x","language":"cobol"}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for unsupported language, got %d (%s)", rec.Code, rec.Body.String())
	}
}

func TestExecSyncMissingFields(t *testing.T) {
	h := newExecHandler("http://unused", "", true, false)
	rec := invokeExec(t, h, h.ExecSync, http.MethodPost, "/exec", `{"language":"python"}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for missing code, got %d", rec.Code)
	}
}

// TestExecLanguageRoutingLight verifies python routes to the light URL.
func TestExecLanguageRoutingLight(t *testing.T) {
	hit := ""
	light := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hit = "light"
		_, _ = w.Write([]byte(`{"status":"success"}`))
	}))
	defer light.Close()
	heavy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hit = "heavy"
		_, _ = w.Write([]byte(`{"status":"success"}`))
	}))
	defer heavy.Close()

	h := newExecHandler(light.URL, heavy.URL, true, true)
	invokeExec(t, h, h.ExecSync, http.MethodPost, "/exec", `{"code":"print(1)","language":"python"}`)
	if hit != "light" {
		t.Fatalf("python should route to light tier, hit=%q", hit)
	}
}

// TestExecLanguageRoutingHeavy verifies java routes to the heavy URL.
func TestExecLanguageRoutingHeavy(t *testing.T) {
	hit := ""
	light := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hit = "light"
		_, _ = w.Write([]byte(`{"status":"success"}`))
	}))
	defer light.Close()
	heavy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hit = "heavy"
		_, _ = w.Write([]byte(`{"status":"success"}`))
	}))
	defer heavy.Close()

	h := newExecHandler(light.URL, heavy.URL, true, true)
	invokeExec(t, h, h.ExecSync, http.MethodPost, "/exec", `{"code":"class Main{}","language":"java"}`)
	if hit != "heavy" {
		t.Fatalf("java should route to heavy tier, hit=%q", hit)
	}
}

// TestExecHeavyDisabled verifies a heavy language 503s when heavy is disabled.
func TestExecHeavyDisabled(t *testing.T) {
	h := newExecHandler("http://unused", "http://unused", true, false)
	rec := invokeExec(t, h, h.ExecSync, http.MethodPost, "/exec", `{"code":"class Main{}","language":"java"}`)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 when heavy tier disabled, got %d", rec.Code)
	}
}

// TestExecUpstreamDown verifies a 502 when the sandbox is unreachable.
func TestExecUpstreamDown(t *testing.T) {
	h := newExecHandler("http://127.0.0.1:0", "", true, false)
	rec := invokeExec(t, h, h.ExecSync, http.MethodPost, "/exec", `{"code":"print(1)","language":"python"}`)
	if rec.Code != http.StatusBadGateway {
		t.Fatalf("expected 502 on upstream error, got %d", rec.Code)
	}
}

// TestExecDisabled verifies /exec 503s when no client is configured.
func TestExecDisabled(t *testing.T) {
	h := &Handlers{Version: "test", Started: time.Now()}
	rec := invokeExec(t, h, h.ExecSync, http.MethodPost, "/exec", `{"code":"x","language":"python"}`)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 when exec disabled, got %d", rec.Code)
	}
}
