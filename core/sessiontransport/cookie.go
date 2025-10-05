package sessiontransport

import (
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/dmitrymomot/foundation/core/cookie"
	"github.com/dmitrymomot/foundation/core/handler"
	"github.com/dmitrymomot/foundation/core/session"
	"github.com/dmitrymomot/foundation/pkg/clientip"
	"github.com/dmitrymomot/foundation/pkg/fingerprint"
)

// CookieManager defines the interface for managing HTTP cookies.
// This interface allows for testing with mock implementations.
type CookieManager interface {
	GetSigned(r *http.Request, name string) (string, error)
	SetSigned(w http.ResponseWriter, r *http.Request, name, value string, opts ...cookie.Option) error
	Delete(w http.ResponseWriter, name string)
}

// Cookie provides HTTP cookie-based session transport.
// It stores Session.Token as the cookie value (signed via cookie.Manager).
type Cookie[Data any] struct {
	manager   *session.Manager[Data]
	cookieMgr CookieManager
	name      string
}

// NewCookie creates a new cookie-based session transport.
func NewCookie[Data any](mgr *session.Manager[Data], cookieMgr CookieManager, name string) *Cookie[Data] {
	return &Cookie[Data]{
		manager:   mgr,
		cookieMgr: cookieMgr,
		name:      name,
	}
}

// Load session from cookie. Creates new anonymous session if no cookie or invalid
// to provide graceful degradation - always returns a valid session.
func (c *Cookie[Data]) Load(ctx handler.Context) (*session.Session[Data], error) {
	token, err := c.cookieMgr.GetSigned(ctx.Request(), c.name)
	if err != nil {
		sess, newErr := session.New[Data](session.NewSessionParams{
			Fingerprint: fingerprint.Cookie(ctx.Request()),
			IP:          clientip.GetIP(ctx.Request()),
			UserAgent:   ctx.Request().Header.Get("User-Agent"),
		}, c.manager.GetTTL())
		if newErr != nil {
			return nil, newErr
		}
		return &sess, nil
	}

	sess, err := c.manager.GetByToken(ctx, token)
	if err != nil {
		sess, newErr := session.New[Data](session.NewSessionParams{
			Fingerprint: fingerprint.Cookie(ctx.Request()),
			IP:          clientip.GetIP(ctx.Request()),
			UserAgent:   ctx.Request().Header.Get("User-Agent"),
		}, c.manager.GetTTL())
		if newErr != nil {
			return nil, newErr
		}
		return &sess, nil
	}

	return sess, nil
}

// Save writes the session token to a signed, essential cookie.
func (c *Cookie[Data]) Save(ctx handler.Context, sess *session.Session[Data]) error {
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

// Authenticate creates an authenticated session with rotated token.
// Optional data parameter allows setting session data during authentication.
func (c *Cookie[Data]) Authenticate(ctx handler.Context, userID uuid.UUID, data ...Data) (*session.Session[Data], error) {
	currentSess, err := c.Load(ctx)
	if err != nil {
		return nil, err
	}

	if err := currentSess.Authenticate(userID, data...); err != nil {
		return nil, err
	}

	if err := c.manager.Store(ctx, currentSess); err != nil {
		return nil, err
	}

	if err := c.Save(ctx, currentSess); err != nil {
		return nil, err
	}

	return currentSess, nil
}

// Logout deletes the session from store and removes the cookie.
func (c *Cookie[Data]) Logout(ctx handler.Context) (*session.Session[Data], error) {
	currentSess, err := c.Load(ctx)
	if err != nil {
		return nil, err
	}

	currentSess.Logout()

	if err := c.manager.Store(ctx, currentSess); err != nil && !errors.Is(err, session.ErrNotAuthenticated) {
		return nil, err
	}

	c.cookieMgr.Delete(ctx.ResponseWriter(), c.name)

	return nil, nil
}

// Delete removes the session from store and deletes the cookie.
func (c *Cookie[Data]) Delete(ctx handler.Context) error {
	currentSess, err := c.Load(ctx)
	if err != nil {
		return err
	}

	currentSess.Logout()

	if err := c.manager.Store(ctx, currentSess); err != nil && !errors.Is(err, session.ErrNotAuthenticated) {
		return err
	}

	c.cookieMgr.Delete(ctx.ResponseWriter(), c.name)

	return nil
}

// Store persists session state and updates the cookie.
func (c *Cookie[Data]) Store(ctx handler.Context, sess *session.Session[Data]) error {
	err := c.manager.Store(ctx, sess)

	if errors.Is(err, session.ErrNotAuthenticated) {
		c.cookieMgr.Delete(ctx.ResponseWriter(), c.name)
		return err
	}

	if err != nil {
		return err
	}

	return c.Save(ctx, sess)
}
