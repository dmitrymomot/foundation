package router

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/dmitrymomot/gokit/core/handler"
)

var (
	// Mux errors
	ErrNoContextFactory = errors.New("no context factory provided")
	ErrMethodNotAllowed = errors.New("method not allowed")
	ErrNotFound         = errors.New("not found")
	ErrNilResponse      = errors.New("nil response")
	ErrInvalidMethod    = errors.New("invalid http method")
	ErrNilRouter        = errors.New("nil router")
	ErrNilSubrouter     = errors.New("nil subrouter")
	ErrInvalidPattern   = errors.New("invalid route path pattern")

	// Tree errors
	ErrInvalidRegexp    = errors.New("invalid route path pattern regexp")
	ErrMissingChild     = errors.New("missing child router")
	ErrWildcardPosition = errors.New("wildcard position must be last")
	ErrParamDelimiter   = errors.New("param delimiter must be unique")
	ErrDuplicateParam   = errors.New("duplicate parameter name")
)

// defaultErrorHandler provides default error handling.
func defaultErrorHandler[C handler.Context](ctx C, err error) {
	w := ctx.ResponseWriter()

	// Prevent double-writing responses which causes HTTP protocol errors
	if ww, ok := w.(*responseWriter); ok && ww.Written() {
		return
	}

	http.Error(w, err.Error(), http.StatusInternalServerError)
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
