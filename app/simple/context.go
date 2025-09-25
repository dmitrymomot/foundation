package simple

import (
	"context"
	"net/http"
	"time"
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

// newContext creates a new Context instance.
func newContext(w http.ResponseWriter, r *http.Request, params map[string]string) *Context {
	return &Context{
		w:      w,
		r:      r,
		params: params,
	}
}
