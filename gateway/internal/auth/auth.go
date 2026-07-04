package auth

import (
	"database/sql"
	"errors"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

// Context keys used to stash auth information on the request context.
const (
	CtxAuthMode  = "auth_mode"
	CtxToken     = "token"
	CtxToolID    = "tool_id"
	CtxRequestID = "request_id"
)

// RateLimiter is the hook interface used by the middleware. Phase 1 ships an
// in-memory implementation; later phases may back it with the rate_limits table.
type RateLimiter interface {
	// Allow reports whether a request keyed by key is permitted given limit
	// requests per minute. It returns the remaining allowance.
	Allow(key string, limit int) (allowed bool, remaining int)
}

// Authenticator holds dependencies for the auth middleware.
type Authenticator struct {
	cfg     AuthConfig
	db      *sql.DB
	limiter RateLimiter
}

// New creates an Authenticator. If limiter is nil an in-memory limiter is used.
func New(cfg AuthConfig, db *sql.DB, limiter RateLimiter) *Authenticator {
	if limiter == nil {
		limiter = NewMemoryLimiter()
	}
	if cfg.DefaultRateLimit <= 0 {
		cfg.DefaultRateLimit = 100
	}
	return &Authenticator{cfg: cfg, db: db, limiter: limiter}
}

// RequestID middleware assigns a request ID (honoring an inbound X-Request-ID)
// and stores it on the context and response header for tracing.
func RequestID() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			rid := c.Request().Header.Get(echo.HeaderXRequestID)
			if rid == "" {
				rid = uuid.NewString()
			}
			c.Set(CtxRequestID, rid)
			c.Response().Header().Set(echo.HeaderXRequestID, rid)
			return next(c)
		}
	}
}

// Middleware returns the auth+rate-limit middleware chain.
func (a *Authenticator) Middleware() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			token := extractToken(c.Request())

			switch a.cfg.Mode {
			case ModeToken:
				return a.handleTokenMode(c, next, token)
			default: // ModeNone
				return a.handleNoneMode(c, next)
			}
		}
	}
}

// handleNoneMode requires the request to originate from localhost.
func (a *Authenticator) handleNoneMode(c echo.Context, next echo.HandlerFunc) error {
	if !isLocalRequest(c.Request()) {
		return echo.NewHTTPError(http.StatusForbidden, map[string]any{
			"error": "auth_mode=none only accepts requests from 127.0.0.1 or ::1; " +
				"switch to auth_mode=token to allow remote access",
		})
	}
	c.Set(CtxAuthMode, "local")

	// Rate-limit by client IP even in local mode.
	ip := clientIP(c.Request())
	if allowed, _ := a.limiter.Allow(ip, a.cfg.DefaultRateLimit); !allowed {
		return rateLimited(c)
	}
	return next(c)
}

// handleTokenMode validates the bearer token against the api_keys table.
func (a *Authenticator) handleTokenMode(c echo.Context, next echo.HandlerFunc, token string) error {
	if token == "" {
		return echo.NewHTTPError(http.StatusUnauthorized, map[string]any{
			"error": "missing bearer token; provide Authorization: Bearer <token> or ?token=",
		})
	}

	tok, err := a.lookupToken(token)
	if errors.Is(err, sql.ErrNoRows) || tok == nil {
		return echo.NewHTTPError(http.StatusUnauthorized, map[string]any{
			"error": "invalid token",
		})
	}
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, map[string]any{
			"error": "token validation failed",
		})
	}
	if tok.Expired(time.Now().UTC()) {
		return echo.NewHTTPError(http.StatusUnauthorized, map[string]any{
			"error": "token expired",
		})
	}

	limit := tok.RateLimit
	if limit <= 0 {
		limit = a.cfg.DefaultRateLimit
	}
	if allowed, _ := a.limiter.Allow(tok.Token, limit); !allowed {
		return rateLimited(c)
	}

	c.Set(CtxAuthMode, "token")
	c.Set(CtxToken, tok.Token)
	c.Set(CtxToolID, tok.ToolID)
	return next(c)
}

// lookupToken loads a token record from the database. Returns (nil, nil) when
// no DB is configured (Phase 1 scaffolding).
func (a *Authenticator) lookupToken(token string) (*Token, error) {
	if a.db == nil {
		return nil, nil
	}
	const q = `
SELECT id, token, COALESCE(tool_id,''), COALESCE(rate_limit,0), created_at, expires_at
FROM api_keys WHERE token=?;`

	var t Token
	var expires sql.NullTime
	err := a.db.QueryRow(q, token).Scan(&t.ID, &t.Token, &t.ToolID, &t.RateLimit, &t.CreatedAt, &expires)
	if err != nil {
		return nil, err
	}
	if expires.Valid {
		t.ExpiresAt = &expires.Time
	}
	return &t, nil
}

func rateLimited(c echo.Context) error {
	c.Response().Header().Set("Retry-After", "60")
	return echo.NewHTTPError(http.StatusTooManyRequests, map[string]any{
		"error": "rate limit exceeded",
	})
}

// extractToken pulls a bearer token from the Authorization header or ?token=.
func extractToken(r *http.Request) string {
	if h := r.Header.Get(echo.HeaderAuthorization); h != "" {
		if strings.HasPrefix(strings.ToLower(h), "bearer ") {
			return strings.TrimSpace(h[len("bearer "):])
		}
	}
	return strings.TrimSpace(r.URL.Query().Get("token"))
}

// isLocalRequest reports whether the request originates from loopback.
func isLocalRequest(r *http.Request) bool {
	ip := net.ParseIP(clientIP(r))
	return ip != nil && ip.IsLoopback()
}

// clientIP extracts the best-effort client IP from the request.
func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	// Strip IPv6 brackets if present.
	host = strings.Trim(host, "[]")
	return host
}

// --- In-memory rate limiter -------------------------------------------------

// MemoryLimiter is a simple fixed-window, in-memory rate limiter.
type MemoryLimiter struct {
	mu      sync.Mutex
	windows map[string]*window
}

type window struct {
	start time.Time
	count int
}

// NewMemoryLimiter creates an empty in-memory limiter.
func NewMemoryLimiter() *MemoryLimiter {
	return &MemoryLimiter{windows: make(map[string]*window)}
}

// Allow implements RateLimiter using a 1-minute fixed window.
func (m *MemoryLimiter) Allow(key string, limit int) (bool, int) {
	if limit <= 0 {
		return true, 0
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	w, ok := m.windows[key]
	if !ok || now.Sub(w.start) >= time.Minute {
		m.windows[key] = &window{start: now, count: 1}
		return true, limit - 1
	}
	if w.count >= limit {
		return false, 0
	}
	w.count++
	return true, limit - w.count
}
