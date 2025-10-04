package sessiontransport

import (
	"fmt"
	"time"

	"github.com/dmitrymomot/foundation/core/cookie"
	"github.com/dmitrymomot/foundation/core/session"
)

// CookieConfig provides environment-based configuration for cookie-based session transport.
type CookieConfig struct {
	// CookieName is the name of the session cookie
	CookieName string `env:"SESSION_COOKIE_NAME" envDefault:"__session"`
}

// DefaultCookieConfig returns a CookieConfig with sensible defaults.
func DefaultCookieConfig() CookieConfig {
	return CookieConfig{
		CookieName: "__session",
	}
}

// NewCookieFromConfig creates a cookie-based session transport from configuration.
// The session.Manager and cookie.Manager must be provided by the caller.
func NewCookieFromConfig[Data any](cfg CookieConfig, mgr *session.Manager[Data], cookieMgr *cookie.Manager) *Cookie[Data] {
	return NewCookie[Data](mgr, cookieMgr, cfg.CookieName)
}

// JWTConfig provides environment-based configuration for JWT-based session transport.
type JWTConfig struct {
	// SecretKey is the JWT signing secret (required, no default)
	SecretKey string `env:"SESSION_JWT_SECRET" envDefault:""`

	// AccessTTL is the access token duration in seconds
	AccessTTL int `env:"SESSION_JWT_ACCESS_TTL" envDefault:"900"` // 15 minutes

	// Issuer is the JWT issuer claim
	Issuer string `env:"SESSION_JWT_ISSUER" envDefault:"foundation"`
}

// DefaultJWTConfig returns a JWTConfig with sensible defaults.
// Note: SecretKey must be set explicitly - it has no default.
func DefaultJWTConfig() JWTConfig {
	return JWTConfig{
		AccessTTL: 900, // 15 minutes
		Issuer:    "foundation",
	}
}

// NewJWTFromConfig creates a JWT-based session transport from configuration.
// Returns an error if SecretKey is empty or JWT creation fails.
func NewJWTFromConfig[Data any](cfg JWTConfig, mgr *session.Manager[Data]) (*JWT[Data], error) {
	if cfg.SecretKey == "" {
		return nil, fmt.Errorf("JWT secret key is required")
	}

	accessTTL := time.Duration(cfg.AccessTTL) * time.Second

	return NewJWT(mgr, cfg.SecretKey, accessTTL, cfg.Issuer)
}
