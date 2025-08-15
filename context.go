package gokit

import (
	"net/http"
	"time"
)

// Context is the default context implementation.
// It delegates all context.Context methods to the request's context.
type Context struct {
	w      http.ResponseWriter
	r      *http.Request
	params map[string]string
}

// Deadline returns the time when work done on behalf of this context
// should be canceled. Delegates to r.Context().
func (c *Context) Deadline() (deadline time.Time, ok bool) {
	return c.r.Context().Deadline()
}

// Done returns a channel that's closed when work done on behalf of this
// context should be canceled. Delegates to r.Context().
func (c *Context) Done() <-chan struct{} {
	return c.r.Context().Done()
}

// Err returns a non-nil error value after Done is closed. Delegates to r.Context().
func (c *Context) Err() error {
	return c.r.Context().Err()
}

// Value returns the value associated with this context for key, or nil
// if no value is associated with key. Delegates to r.Context().
func (c *Context) Value(key any) any {
	return c.r.Context().Value(key)
}

// Request returns the *http.Request associated with the context.
func (c *Context) Request() *http.Request {
	return c.r
}

// ResponseWriter returns the http.ResponseWriter associated with the context.
func (c *Context) ResponseWriter() http.ResponseWriter {
	return c.w
}

// Param returns the value of the URL parameter by key.
func (c *Context) Param(key string) string {
	if c.params == nil {
		return ""
	}
	return c.params[key]
}

// setParam sets a URL parameter value.
func (c *Context) setParam(key, value string) {
	if c.params == nil {
		c.params = make(map[string]string)
	}
	c.params[key] = value
}

// NewContext creates a new Context instance.
func NewContext(w http.ResponseWriter, r *http.Request) *Context {
	return &Context{
		w:      w,
		r:      r,
		params: make(map[string]string),
	}
}
