package gokit

import (
	"net/http"
	"time"
)

// baseContext is the default context implementation.
// It delegates all context.Context methods to the request's context.
type baseContext struct {
	w      http.ResponseWriter
	r      *http.Request
	params map[string]string
}

// Deadline returns the time when work done on behalf of this context
// should be canceled. Delegates to r.Context().
func (c *baseContext) Deadline() (deadline time.Time, ok bool) {
	return c.r.Context().Deadline()
}

// Done returns a channel that's closed when work done on behalf of this
// context should be canceled. Delegates to r.Context().
func (c *baseContext) Done() <-chan struct{} {
	return c.r.Context().Done()
}

// Err returns a non-nil error value after Done is closed. Delegates to r.Context().
func (c *baseContext) Err() error {
	return c.r.Context().Err()
}

// Value returns the value associated with this context for key, or nil
// if no value is associated with key. Delegates to r.Context().
func (c *baseContext) Value(key any) any {
	return c.r.Context().Value(key)
}

// Request returns the *http.Request associated with the context.
func (c *baseContext) Request() *http.Request {
	return c.r
}

// ResponseWriter returns the http.ResponseWriter associated with the context.
func (c *baseContext) ResponseWriter() http.ResponseWriter {
	return c.w
}

// Param returns the value of the URL parameter by key.
func (c *baseContext) Param(key string) string {
	if c.params == nil {
		return ""
	}
	return c.params[key]
}

// setParam sets a URL parameter value.
func (c *baseContext) setParam(key, value string) {
	if c.params == nil {
		c.params = make(map[string]string)
	}
	c.params[key] = value
}

// reset clears the context for reuse.
func (c *baseContext) reset() {
	c.w = nil
	c.r = nil
	if c.params != nil {
		// Clear the map for reuse
		for k := range c.params {
			delete(c.params, k)
		}
	}
}

// newBaseContext creates a new baseContext instance.
func newBaseContext(w http.ResponseWriter, r *http.Request) *baseContext {
	return &baseContext{
		w:      w,
		r:      r,
		params: make(map[string]string),
	}
}

// RouteParams holds URL parameters extracted from the route.
type RouteParams struct {
	Keys   []string
	Values []string
}

// responseWriter wraps http.ResponseWriter to track if response was written.
type responseWriter struct {
	http.ResponseWriter
	written bool
	status  int
}

func (w *responseWriter) WriteHeader(status int) {
	if !w.written {
		w.status = status
		w.written = true
		w.ResponseWriter.WriteHeader(status)
	}
}

func (w *responseWriter) Write(b []byte) (int, error) {
	if !w.written {
		w.WriteHeader(http.StatusOK)
	}
	return w.ResponseWriter.Write(b)
}

// Written returns true if the response has been written.
func (w *responseWriter) Written() bool {
	return w.written
}

// Status returns the HTTP status code of the response.
func (w *responseWriter) Status() int {
	return w.status
}
