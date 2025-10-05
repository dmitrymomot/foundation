package main

import (
	"context"
	"net/http"
	"time"

	"github.com/dmitrymomot/foundation/core/binder"
	"github.com/dmitrymomot/foundation/core/sanitizer"
	"github.com/dmitrymomot/foundation/core/session"
	"github.com/dmitrymomot/foundation/core/validator"
	"github.com/dmitrymomot/foundation/middleware"
	"github.com/google/uuid"
)

// Context is the default context implementation that delegates to the request's context.
type Context struct {
	w      http.ResponseWriter
	r      *http.Request
	params map[string]string
}

// Deadline returns the time when work done on behalf of this context should be canceled.
func (c *Context) Deadline() (deadline time.Time, ok bool) {
	return c.r.Context().Deadline()
}

// Done returns a channel that's closed when work done on behalf of this context should be canceled.
func (c *Context) Done() <-chan struct{} {
	return c.r.Context().Done()
}

// Err returns a non-nil error value after Done is closed.
func (c *Context) Err() error {
	return c.r.Context().Err()
}

// Value returns the value associated with this context for key, or nil if no value is associated with key.
func (c *Context) Value(key any) any {
	return c.r.Context().Value(key)
}

// SetValue stores a value in the request's context.
// The value can be retrieved using the Value method.
func (c *Context) SetValue(key, val any) {
	ctx := context.WithValue(c.r.Context(), key, val)
	c.r = c.r.WithContext(ctx)
}

// Request returns the HTTP request associated with this context.
func (c *Context) Request() *http.Request {
	return c.r
}

// ResponseWriter returns the HTTP response writer associated with this context.
func (c *Context) ResponseWriter() http.ResponseWriter {
	return c.w
}

// Param returns the value of the URL parameter for the given key.
func (c *Context) Param(key string) string {
	if c.params == nil {
		return ""
	}
	return c.params[key]
}

// Session retrieves the current session from the request context.
// Returns the session and a boolean indicating whether it was found.
func (c *Context) Session() (session.Session[SessionData], bool) {
	return middleware.GetSession[SessionData](c)
}

// UserID returns the authenticated user ID from the session.
// This method assumes the session middleware has already validated authentication.
// It should only be used in routes protected by session middleware with RequireAuth: true.
func (c *Context) UserID() uuid.UUID {
	sess, _ := c.Session()
	return sess.UserID
}

// Bind binds, sanitizes, and validates request data into the provided struct.
// It automatically selects the appropriate binder based on:
// - Content-Type header (JSON, Form)
// - HTTP method (Query for GET/DELETE)
// - Path parameters (always applied)
//
// After binding, it:
// 1. Sanitizes the struct using `sanitize` struct tags (e.g., `sanitize:"trim,lower"`)
// 2. Validates the struct using `validate` struct tags (e.g., `validate:"required;min:2"`)
//
// Returns validation errors in a structured format compatible with response.Error.
//
// Example usage:
//
//	type SignupRequest struct {
//	    Name     string `json:"name" sanitize:"trim,title" validate:"required;min:2"`
//	    Email    string `json:"email" sanitize:"email" validate:"required;email"`
//	    Password string `json:"password" validate:"required;min:8"`
//	    Username string `json:"username" sanitize:"trim,lower,alphanum" validate:"required;min:3;max:20"`
//	}
//
//	var req SignupRequest
//	if err := ctx.Bind(&req); err != nil {
//	    return response.Error(response.ErrBadRequest.WithError(err))
//	}
func (c *Context) Bind(v any) error {
	// Always bind path parameters first (if available)
	if len(c.params) > 0 {
		pathBinder := binder.Path(func(r *http.Request, fieldName string) string {
			return c.Param(fieldName)
		})
		if err := pathBinder(c.r, v); err != nil && err != binder.ErrBinderNotApplicable {
			return err
		}
	}

	// Bind query parameters for GET/DELETE methods
	if c.r.Method == http.MethodGet || c.r.Method == http.MethodDelete {
		if err := binder.Query()(c.r, v); err != nil && err != binder.ErrBinderNotApplicable {
			return err
		}
	}

	// Bind JSON data (API only uses JSON)
	if err := binder.JSON()(c.r, v); err != nil && err != binder.ErrBinderNotApplicable {
		return err
	}

	// Sanitize using struct tags
	if err := sanitizer.SanitizeStruct(v); err != nil {
		return err
	}

	// Validate using struct tags
	if err := validator.ValidateStruct(v); err != nil {
		return err
	}

	return nil
}

// Logout logs out the current user by marking the session for deletion.
// The session middleware automatically handles the deletion.
func (c *Context) Logout() error {
	sess := middleware.MustGetSession[SessionData](c)
	sess.Logout()
	middleware.SetSession[SessionData](c, sess)
	return nil
}

// newContext creates a new Context instance.
func newContext() func(http.ResponseWriter, *http.Request, map[string]string) *Context {
	return func(w http.ResponseWriter, r *http.Request, params map[string]string) *Context {
		return &Context{
			w:      w,
			r:      r,
			params: params,
		}
	}
}
