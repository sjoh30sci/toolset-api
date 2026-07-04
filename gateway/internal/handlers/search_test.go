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

// newSearchHandler builds a Handlers with no registry dependency (search does
// not touch the registry) for isolated endpoint testing.
func newSearchHandler() *Handlers {
	return &Handlers{Version: "test", Started: time.Now()}
}

// searxngFixture is a canned SearXNG JSON response.
const searxngFixture = `{
  "results": [
    {"title": "Go Programming", "url": "https://go.dev", "content": "The Go language", "engine": "google"},
    {"title": "Effective Go", "url": "https://go.dev/doc/effective_go", "content": "Tips", "engine": "bing"}
  ]
}`

// invokeSearch runs Search and, if it returns an HTTPError, applies echo's
// default error handler so the status/body land in the recorder.
func invokeSearch(t *testing.T, h *Handlers, body string) *httptest.ResponseRecorder {
	t.Helper()
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/search", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	if err := h.Search(c); err != nil {
		e.HTTPErrorHandler(err, c)
	}
	return rec
}

func TestSearchValidQuery(t *testing.T) {
	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("q"); got != "golang" {
			t.Errorf("expected q=golang, got %q", got)
		}
		if got := r.URL.Query().Get("format"); got != "json" {
			t.Errorf("expected format=json, got %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(searxngFixture))
	}))
	defer mock.Close()

	orig := searchUpstream
	searchUpstream = mock.URL
	defer func() { searchUpstream = orig }()

	rec := invokeSearch(t, newSearchHandler(), `{"query":"golang"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp searchResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.Query != "golang" {
		t.Errorf("expected query golang, got %q", resp.Query)
	}
	if resp.Page != 1 {
		t.Errorf("expected page 1, got %d", resp.Page)
	}
	if resp.Count != 2 || len(resp.Results) != 2 {
		t.Fatalf("expected 2 results, got %d", resp.Count)
	}
	if resp.Results[0].Title != "Go Programming" || resp.Results[0].Snippet != "The Go language" {
		t.Errorf("unexpected first result: %+v", resp.Results[0])
	}
	if resp.Results[0].Engine != "google" {
		t.Errorf("expected engine google, got %q", resp.Results[0].Engine)
	}
}

func TestSearchEmptyQuery(t *testing.T) {
	rec := invokeSearch(t, newSearchHandler(), `{"query":""}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for empty query, got %d", rec.Code)
	}
}

func TestSearchMalformedJSON(t *testing.T) {
	rec := invokeSearch(t, newSearchHandler(), `{not json`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for malformed JSON, got %d", rec.Code)
	}
}

func TestSearchTimeout(t *testing.T) {
	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(searchTimeout + 2*time.Second)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(searxngFixture))
	}))
	defer mock.Close()

	orig := searchUpstream
	searchUpstream = mock.URL
	defer func() { searchUpstream = orig }()

	rec := invokeSearch(t, newSearchHandler(), `{"query":"golang"}`)
	if rec.Code != http.StatusBadGateway {
		t.Fatalf("expected 502 on upstream timeout, got %d", rec.Code)
	}
}

func TestSearchUpstreamError(t *testing.T) {
	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer mock.Close()

	orig := searchUpstream
	searchUpstream = mock.URL
	defer func() { searchUpstream = orig }()

	rec := invokeSearch(t, newSearchHandler(), `{"query":"golang"}`)
	if rec.Code != http.StatusBadGateway {
		t.Fatalf("expected 502 on upstream error, got %d", rec.Code)
	}
}
