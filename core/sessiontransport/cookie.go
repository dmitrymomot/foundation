package sessiontransport

import (
	"context"
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/dmitrymomot/foundation/core/cookie"
	"github.com/dmitrymomot/foundation/core/session"
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
		return c.manager.New(ctx)
	}

	sess, err := c.manager.GetByToken(ctx, token)
	if err != nil {
		return c.manager.New(ctx)
	}

	return sess, nil
}

// Save session to cookie using signed cookie.
func (c *Cookie[Data]) Save(ctx context.Context, w http.ResponseWriter, sess session.Session[Data]) error {
	maxAge := int(time.Until(sess.ExpiresAt).Seconds())
	if maxAge < 0 {
		maxAge = 0
	}

	return c.cookieMgr.SetSigned(w, nil, c.name, sess.Token,
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
		var empty session.Session[Data]
		return empty, err
	}

	// Rotates token for security
	authSess, err := c.manager.Authenticate(ctx, currentSess, userID)
	if err != nil {
		var empty session.Session[Data]
		return empty, err
	}

	if err := c.Save(ctx, w, authSess); err != nil {
		var empty session.Session[Data]
		return empty, err
	}

	return authSess, nil
}

// Logout user. Calls manager.Logout and sets new token in cookie.
// Returns the anonymous session with rotated token.
func (c *Cookie[Data]) Logout(ctx context.Context, w http.ResponseWriter, r *http.Request) (session.Session[Data], error) {
	currentSess, err := c.Load(ctx, r)
	if err != nil {
		var empty session.Session[Data]
		return empty, err
	}

	// Rotates token for security
	anonSess, err := c.manager.Logout(ctx, currentSess)
	if err != nil {
		var empty session.Session[Data]
		return empty, err
	}

	if err := c.Save(ctx, w, anonSess); err != nil {
		var empty session.Session[Data]
		return empty, err
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

// Touch updates session expiration if interval has passed.
// Note: GetByID/GetByToken already handle touch logic internally,
// so this method is primarily for explicit session extension.
func (c *Cookie[Data]) Touch(ctx context.Context, w http.ResponseWriter, sess session.Session[Data]) error {
	// GetByID triggers automatic touch if TouchInterval has passed
	refreshed, err := c.manager.GetByID(ctx, sess.ID)
	if err != nil {
		return err
	}

	if refreshed.UpdatedAt.After(sess.UpdatedAt) {
		return c.Save(ctx, w, refreshed)
	}

	return nil
}
