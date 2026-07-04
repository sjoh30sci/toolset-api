// Package auth provides authentication middleware, token validation, and
// rate-limiting hooks for the gateway.
package auth

import "time"

// AuthConfig configures the auth middleware.
type AuthConfig struct {
	// Mode is "none" or "token".
	Mode string
	// DefaultRateLimit is the requests-per-minute applied when a token/IP has
	// no explicit limit.
	DefaultRateLimit int
}

// Auth modes.
const (
	ModeNone  = "none"
	ModeToken = "token"
)

// Token represents a row in the api_keys table.
type Token struct {
	ID        string
	Token     string
	ToolID    string
	RateLimit int
	CreatedAt time.Time
	ExpiresAt *time.Time
}

// Expired reports whether the token has passed its expiry time.
func (t Token) Expired(now time.Time) bool {
	return t.ExpiresAt != nil && now.After(*t.ExpiresAt)
}
