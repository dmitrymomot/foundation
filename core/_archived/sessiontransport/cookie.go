package sessiontransport

import (
	"errors"
	"net/http"
	"time"

	"github.com/dmitrymomot/foundation/core/cookie"
	"github.com/dmitrymomot/foundation/core/session"
)

// CookieTransport implements session.Transport using encrypted cookies.
type CookieTransport struct {
	manager    *cookie.Manager
	cookieName string
	options    []cookie.Option
}

// CookieOption configures the cookie transport.
type CookieOption func(*CookieTransport)

// WithCookieName sets a custom cookie name.
func WithCookieName(name string) CookieOption {
	return func(t *CookieTransport) {
		if name != "" {
			t.cookieName = name
		}
	}
}

// WithCookieOptions adds cookie options to be applied when setting cookies.
func WithCookieOptions(opts ...cookie.Option) CookieOption {
	return func(t *CookieTransport) {
		t.options = append(t.options, opts...)
	}
}

// NewCookie creates a new cookie-based session transport.
func NewCookie(manager *cookie.Manager, opts ...CookieOption) *CookieTransport {
	t := &CookieTransport{
		manager:    manager,
		cookieName: "__session",
		options:    []cookie.Option{cookie.WithEssential()}, // Sessions are essential
	}

	for _, opt := range opts {
		opt(t)
	}

	return t
}

// Extract retrieves the session token from the cookie.
func (t *CookieTransport) Extract(r *http.Request) (string, error) {
	token, err := t.manager.GetEncrypted(r, t.cookieName)
	if err != nil {
		if errors.Is(err, cookie.ErrCookieNotFound) {
			return "", session.ErrNoToken
		}
		// Decryption failed or invalid format
		if errors.Is(err, cookie.ErrDecryptionFailed) ||
			errors.Is(err, cookie.ErrInvalidFormat) ||
			errors.Is(err, cookie.ErrInvalidSignature) {
			return "", session.ErrInvalidToken
		}
		// Other infrastructure errors
		return "", session.ErrTransportFailed
	}

	if token == "" {
		return "", session.ErrNoToken
	}

	return token, nil
}

// Embed stores the session token in an encrypted cookie.
func (t *CookieTransport) Embed(w http.ResponseWriter, r *http.Request, token string, ttl time.Duration) error {
	// Calculate MaxAge in seconds
	maxAge := int(ttl.Seconds())

	// Combine default options with MaxAge and any custom options
	opts := append(t.options, cookie.WithMaxAge(maxAge))

	err := t.manager.SetEncrypted(w, r, t.cookieName, token, opts...)
	if err != nil {
		// Check if it's a cookie size error
		var cookieTooLarge cookie.ErrCookieTooLarge
		if errors.As(err, &cookieTooLarge) {
			return session.ErrInvalidToken // Token too large
		}
		return session.ErrTransportFailed
	}

	return nil
}

// Revoke removes the session cookie.
// The request parameter is unused but required by the Transport interface.
func (t *CookieTransport) Revoke(w http.ResponseWriter, r *http.Request) error {
	t.manager.Delete(w, t.cookieName)
	return nil // Always succeed - idempotent operation
}
