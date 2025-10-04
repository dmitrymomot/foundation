package sessiontransport

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/dmitrymomot/foundation/core/cookie"
	"github.com/dmitrymomot/foundation/core/session"
	"github.com/dmitrymomot/foundation/pkg/fingerprint"
)

// Cookie provides HTTP cookie-based session transport.
// It stores Session.Token as the cookie value (signed via cookie.Manager).
type Cookie[Data any] struct {
	manager   *session.Manager[Data]
	cookieMgr *cookie.Manager
	name      string
}

// NewCookie creates a new cookie-based session transport.
func NewCookie[Data any](mgr *session.Manager[Data], cookieMgr *cookie.Manager, name string) *Cookie[Data] {
	return &Cookie[Data]{
		manager:   mgr,
		cookieMgr: cookieMgr,
		name:      name,
	}
}

// Load session from cookie. Creates new anonymous session if no cookie or invalid.
// This provides graceful degradation - always returns a valid session.
func (c *Cookie[Data]) Load(ctx context.Context, r *http.Request) (session.Session[Data], error) {
	token, err := c.cookieMgr.GetSigned(r, c.name)
	if err != nil {
		return c.manager.New(ctx, fingerprint.Cookie(r))
	}

	sess, err := c.manager.GetByToken(ctx, token)
	if err != nil {
		return c.manager.New(ctx, fingerprint.Cookie(r))
	}

	return sess, nil
}

// Save session to cookie using signed cookie.
func (c *Cookie[Data]) Save(w http.ResponseWriter, r *http.Request, sess session.Session[Data]) error {
	until := time.Until(sess.ExpiresAt)
	if until <= 0 {
		return fmt.Errorf("cannot save expired session (expired %v ago)", -until)
	}
	maxAge := int(until.Seconds())

	return c.cookieMgr.SetSigned(w, r, c.name, sess.Token,
		cookie.WithHTTPOnly(true),
		cookie.WithSecure(true),
		cookie.WithSameSite(http.SameSiteLaxMode),
		cookie.WithMaxAge(maxAge),
		cookie.WithEssential(),
	)
}

// Authenticate user. Calls manager.Authenticate and sets new token in cookie.
// Returns the authenticated session with rotated token.
func (c *Cookie[Data]) Authenticate(ctx context.Context, w http.ResponseWriter, r *http.Request, userID uuid.UUID) (session.Session[Data], error) {
	currentSess, err := c.Load(ctx, r)
	if err != nil {
		return session.Session[Data]{}, err
	}

	// Rotates token for security
	authSess, err := c.manager.Authenticate(ctx, currentSess, userID)
	if err != nil {
		return session.Session[Data]{}, err
	}

	if err := c.Save(w, r, authSess); err != nil {
		return session.Session[Data]{}, err
	}

	return authSess, nil
}

// Logout user. Calls manager.Logout and sets new token in cookie.
// Returns the anonymous session with rotated token.
func (c *Cookie[Data]) Logout(ctx context.Context, w http.ResponseWriter, r *http.Request) (session.Session[Data], error) {
	currentSess, err := c.Load(ctx, r)
	if err != nil {
		return session.Session[Data]{}, err
	}

	// Rotates token for security
	anonSess, err := c.manager.Logout(ctx, currentSess)
	if err != nil {
		return session.Session[Data]{}, err
	}

	if err := c.Save(w, r, anonSess); err != nil {
		return session.Session[Data]{}, err
	}

	return anonSess, nil
}

// Delete session. Deletes cookie and session from store.
func (c *Cookie[Data]) Delete(ctx context.Context, w http.ResponseWriter, r *http.Request) error {
	currentSess, err := c.Load(ctx, r)
	if err != nil {
		return err
	}

	if err := c.manager.Delete(ctx, currentSess.ID); err != nil {
		return err
	}

	c.cookieMgr.Delete(w, c.name)

	return nil
}

// Touch updates session expiration if the touch interval has elapsed.
// This is called by the session middleware after each request to extend session lifetime.
//
// The method retrieves the session from the store (which automatically touches it if needed)
// and saves the updated cookie if the session was refreshed. This ensures the client's
// cookie MaxAge stays in sync with the server-side session expiration.
func (c *Cookie[Data]) Touch(w http.ResponseWriter, r *http.Request, sess session.Session[Data]) error {
	// GetByID triggers automatic touch if TouchInterval has passed
	refreshed, err := c.manager.GetByID(r.Context(), sess.ID)
	if err != nil {
		return err
	}

	// If session was touched (UpdatedAt changed), update the cookie
	if refreshed.UpdatedAt.After(sess.UpdatedAt) {
		return c.Save(w, r, refreshed)
	}

	return nil
}
