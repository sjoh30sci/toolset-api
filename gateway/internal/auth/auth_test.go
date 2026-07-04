package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
)

// runRequireTool builds a context tagged with the given auth mode + tool ID and
// runs RequireTool(toolName), returning the resulting HTTP status.
func runRequireTool(t *testing.T, mode, tokenToolID, requiredTool string) int {
	t.Helper()
	a := New(AuthConfig{Mode: ModeToken}, nil, nil)

	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/search", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.Set(CtxAuthMode, mode)
	c.Set(CtxToolID, tokenToolID)

	handlerRan := false
	final := func(c echo.Context) error {
		handlerRan = true
		return c.NoContent(http.StatusOK)
	}

	mw := a.RequireTool(requiredTool)
	if err := mw(final)(c); err != nil {
		e.HTTPErrorHandler(err, c)
	}
	_ = handlerRan
	return rec.Code
}

func TestRequireToolGrantsMatchingTool(t *testing.T) {
	if code := runRequireTool(t, "token", "search", "search"); code != http.StatusOK {
		t.Fatalf("expected 200 when token grants search, got %d", code)
	}
}

func TestRequireToolDeniesOtherTool(t *testing.T) {
	// Token scoped to "search" must be denied on the "exec" tool.
	if code := runRequireTool(t, "token", "search", "exec"); code != http.StatusForbidden {
		t.Fatalf("expected 403 when search token hits exec, got %d", code)
	}
}

func TestRequireToolWildcardTokenAllowed(t *testing.T) {
	// Empty tool_id = wildcard: allowed on any tool.
	if code := runRequireTool(t, "token", "", "exec"); code != http.StatusOK {
		t.Fatalf("expected 200 for wildcard token, got %d", code)
	}
}

func TestRequireToolLocalModeAllowed(t *testing.T) {
	// In local (none) mode all tools are permitted.
	if code := runRequireTool(t, "local", "", "exec"); code != http.StatusOK {
		t.Fatalf("expected 200 in local mode, got %d", code)
	}
}

func TestExtractToken(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set(echo.HeaderAuthorization, "Bearer abc123")
	if got := extractToken(req); got != "abc123" {
		t.Errorf("expected abc123 from header, got %q", got)
	}

	req2 := httptest.NewRequest(http.MethodGet, "/?token=xyz789", nil)
	if got := extractToken(req2); got != "xyz789" {
		t.Errorf("expected xyz789 from query, got %q", got)
	}
}
