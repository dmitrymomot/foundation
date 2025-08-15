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
