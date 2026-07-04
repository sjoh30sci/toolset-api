package handlers

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
)

func newFilesHandler() *Handlers {
	return &Handlers{Version: "test", Started: time.Now()}
}

// invokeProxy calls the given files handler and applies echo's default error
// handler on failure so the recorder reflects the intended response.
func invokeProxy(t *testing.T, fn func(echo.Context) error, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	if err := fn(c); err != nil {
		e.HTTPErrorHandler(err, c)
	}
	return rec
}

func TestFilesReadProxy(t *testing.T) {
	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/files/read" {
			t.Errorf("expected /files/read, got %s", r.URL.Path)
		}
		b, _ := io.ReadAll(r.Body)
		if !strings.Contains(string(b), "test.txt") {
			t.Errorf("body not forwarded: %s", string(b))
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"content":"hello"}`))
	}))
	defer mock.Close()

	orig := filesUpstream
	filesUpstream = mock.URL
	defer func() { filesUpstream = orig }()

	rec := invokeProxy(t, newFilesHandler().FilesRead, "/files/read", `{"path":"test.txt"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "hello") {
		t.Errorf("response not passed through: %s", rec.Body.String())
	}
}

func TestFilesWriteProxy(t *testing.T) {
	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/files/write" {
			t.Errorf("expected /files/write, got %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"path":"test.txt","size":5}`))
	}))
	defer mock.Close()

	orig := filesUpstream
	filesUpstream = mock.URL
	defer func() { filesUpstream = orig }()

	rec := invokeProxy(t, newFilesHandler().FilesWrite, "/files/write", `{"path":"test.txt","content":"hello"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", rec.Code)
	}
}

func TestFilesListProxy(t *testing.T) {
	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/files/list" {
			t.Errorf("expected /files/list, got %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"files":["a.txt"],"dirs":["sub"]}`))
	}))
	defer mock.Close()

	orig := filesUpstream
	filesUpstream = mock.URL
	defer func() { filesUpstream = orig }()

	rec := invokeProxy(t, newFilesHandler().FilesList, "/files/list", `{"path":"."}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "a.txt") {
		t.Errorf("list not passed through: %s", rec.Body.String())
	}
}

func TestFilesDeleteProxy(t *testing.T) {
	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/files/delete" {
			t.Errorf("expected /files/delete, got %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer mock.Close()

	orig := filesUpstream
	filesUpstream = mock.URL
	defer func() { filesUpstream = orig }()

	rec := invokeProxy(t, newFilesHandler().FilesDelete, "/files/delete", `{"path":"test.txt"}`)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", rec.Code)
	}
}

func TestFilesMoveProxy(t *testing.T) {
	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/files/move" {
			t.Errorf("expected /files/move, got %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"from":"a.txt","to":"b.txt"}`))
	}))
	defer mock.Close()

	orig := filesUpstream
	filesUpstream = mock.URL
	defer func() { filesUpstream = orig }()

	rec := invokeProxy(t, newFilesHandler().FilesMove, "/files/move", `{"from":"a.txt","to":"b.txt"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", rec.Code)
	}
}

func TestFilesUpstreamError(t *testing.T) {
	// Point at an unreachable address to trigger a proxy error.
	orig := filesUpstream
	filesUpstream = "http://127.0.0.1:0"
	defer func() { filesUpstream = orig }()

	rec := invokeProxy(t, newFilesHandler().FilesRead, "/files/read", `{"path":"test.txt"}`)
	if rec.Code != http.StatusBadGateway {
		t.Fatalf("expected 502 on upstream error, got %d", rec.Code)
	}
}
