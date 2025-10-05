package sessiontransport

import (
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/dmitrymomot/foundation/core/cookie"
	"github.com/dmitrymomot/foundation/core/handler"
	"github.com/dmitrymomot/foundation/core/session"
	"github.com/dmitrymomot/foundation/pkg/clientip"
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
func (c *Cookie[Data]) Load(ctx handler.Context) (session.Session[Data], error) {
	token, err := c.cookieMgr.GetSigned(ctx.Request(), c.name)
	if err != nil {
		return session.New[Data](session.NewSessionParams{
			Fingerprint: fingerprint.Cookie(ctx.Request()),
			IP:          clientip.GetIP(ctx.Request()),
			UserAgent:   ctx.Request().Header.Get("User-Agent"),
		}, c.manager.GetTTL())
	}

	sess, err := c.manager.GetByToken(ctx, token)
	if err != nil {
		return session.New[Data](session.NewSessionParams{
			Fingerprint: fingerprint.Cookie(ctx.Request()),
			IP:          clientip.GetIP(ctx.Request()),
			UserAgent:   ctx.Request().Header.Get("User-Agent"),
		}, c.manager.GetTTL())
	}

	return sess, nil
}

// Save session to cookie using signed cookie.
func (c *Cookie[Data]) Save(ctx handler.Context, sess session.Session[Data]) error {
	until := time.Until(sess.ExpiresAt)
	if until <= 0 {
		return fmt.Errorf("cannot save expired session (expired %v ago)", -until)
	}
	maxAge := int(until.Seconds())

	return c.cookieMgr.SetSigned(ctx.ResponseWriter(), ctx.Request(), c.name, sess.Token,
		cookie.WithMaxAge(maxAge),
		cookie.WithEssential(),
	)
}

// Authenticate user. Calls sess.Authenticate() and sets new token in cookie.
// Returns the authenticated session with rotated token.
// Optional data parameter allows setting session data during authentication.
func (c *Cookie[Data]) Authenticate(ctx handler.Context, userID uuid.UUID, data ...Data) (session.Session[Data], error) {
	currentSess, err := c.Load(ctx)
	if err != nil {
		return session.Session[Data]{}, err
	}

	if err := currentSess.Authenticate(userID, data...); err != nil {
		return session.Session[Data]{}, err
	}

	if err := c.manager.Store(ctx, currentSess); err != nil {
		return session.Session[Data]{}, err
	}

	if err := c.Save(ctx, currentSess); err != nil {
		return session.Session[Data]{}, err
	}

	return currentSess, nil
}

// Logout user. Calls sess.Logout() to mark session for deletion.
// Returns an empty session.
func (c *Cookie[Data]) Logout(ctx handler.Context) (session.Session[Data], error) {
	currentSess, err := c.Load(ctx)
	if err != nil {
		return session.Session[Data]{}, err
	}

	currentSess.Logout()

	// Store will delete the session because IsDeleted() is true
	if err := c.manager.Store(ctx, currentSess); err != nil && !errors.Is(err, session.ErrNotAuthenticated) {
		return session.Session[Data]{}, err
	}

	// Cookie will be deleted by Store method
	c.cookieMgr.Delete(ctx.ResponseWriter(), c.name)

	return session.Session[Data]{}, nil
}

// Delete session. Deletes cookie and session from store.
func (c *Cookie[Data]) Delete(ctx handler.Context) error {
	currentSess, err := c.Load(ctx)
	if err != nil {
		return err
	}

	currentSess.Logout() // Mark for deletion

	// Store will delete the session
	if err := c.manager.Store(ctx, currentSess); err != nil && !errors.Is(err, session.ErrNotAuthenticated) {
		return err
	}

	c.cookieMgr.Delete(ctx.ResponseWriter(), c.name)

	return nil
}

// Store persists session state and updates the cookie.
func (c *Cookie[Data]) Store(ctx handler.Context, sess session.Session[Data]) error {
	err := c.manager.Store(ctx, sess)

	// Handle deletion: remove cookie and propagate error
	if errors.Is(err, session.ErrNotAuthenticated) {
		c.cookieMgr.Delete(ctx.ResponseWriter(), c.name)
		return err
	}

	if err != nil {
		return err
	}

	// Update cookie with current session
	return c.Save(ctx, sess)
}
