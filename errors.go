package gokit

import (
	"errors"
	"fmt"
	"net/http"
)

// Standard router errors.
var (
	ErrNotFound         = errors.New("route not found")
	ErrMethodNotAllowed = errors.New("method not allowed")
	ErrNilResponse      = errors.New("handler returned nil response")

	// Configuration errors
	ErrNoContextFactory = errors.New("no context factory provided and C is not *baseContext")
	ErrInvalidMethod    = errors.New("invalid http method")
	ErrInvalidPattern   = errors.New("routing pattern must begin with '/'")
	ErrNilRouter        = errors.New("cannot mount nil router")
	ErrNilSubrouter     = errors.New("subrouter function cannot be nil")

	// Pattern parsing errors
	ErrInvalidRegexp    = errors.New("invalid regexp pattern in route param")
	ErrMissingChild     = errors.New("replacing missing child")
	ErrWildcardPosition = errors.New("wildcard '*' must be the last pattern in a route")
	ErrParamDelimiter   = errors.New("route param closing delimiter '}' is missing")
	ErrDuplicateParam   = errors.New("routing pattern contains duplicate param key")
)

// defaultErrorHandler provides default error handling.
func defaultErrorHandler[C Context](ctx C, err error) {
	w := ctx.ResponseWriter()

	// Check if response already written
	if ww, ok := w.(*responseWriter); ok && ww.Written() {
		// Log error but don't write response
		return
	}

	switch {
	case errors.Is(err, ErrNotFound):
		http.Error(w, "404 Not Found", http.StatusNotFound)
	case errors.Is(err, ErrMethodNotAllowed):
		// Allow header should already be set by the caller if applicable
		http.Error(w, "405 Method Not Allowed", http.StatusMethodNotAllowed)
	case errors.Is(err, ErrNilResponse):
		http.Error(w, "500 Internal Server Error - nil response", http.StatusInternalServerError)
	default:
		http.Error(w, "500 Internal Server Error", http.StatusInternalServerError)
	}
}

// toError converts any value to an error.
func toError(v any) error {
	switch e := v.(type) {
	case error:
		return e
	case string:
		return errors.New(e)
	default:
		return fmt.Errorf("panic: %v", e)
	}
}
