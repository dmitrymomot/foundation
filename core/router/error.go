package router

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/dmitrymomot/foundation/core/handler"
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

// statusCode is an unexported interface that errors can implement
// to provide a custom HTTP status code.
type statusCode interface {
	StatusCode() int
}

// defaultErrorHandler provides default error handling.
func defaultErrorHandler[C handler.Context](ctx C, err error) {
	w := ctx.ResponseWriter()

	// Prevent double-writing responses which causes HTTP protocol errors
	if ww, ok := w.(*responseWriter); ok && ww.Written() {
		return
	}

	// Check if error implements statusCode interface
	status := http.StatusInternalServerError
	if sc, ok := err.(statusCode); ok {
		status = sc.StatusCode()
	}

	http.Error(w, err.Error(), status)
}

// PanicError interface allows external error handlers to detect and handle panics.
// When a panic is recovered by the router, it's wrapped in an error that implements
// this interface, providing access to the original panic value and stack trace.
type PanicError interface {
	error
	// Value returns the original panic value.
	Value() any
	// Stack returns the stack trace captured at the panic point.
	Stack() []byte
}

// panicError is the private implementation of PanicError interface.
type panicError struct {
	value any
	stack []byte
}

// Error implements the error interface.
func (e *panicError) Error() string {
	return fmt.Sprintf("panic: %v", e.value)
}

// Value returns the original panic value.
func (e *panicError) Value() any {
	return e.value
}

// Stack returns the stack trace.
func (e *panicError) Stack() []byte {
	return e.stack
}

// Unwrap allows errors.Is/As to work with wrapped panics.
func (e *panicError) Unwrap() error {
	if err, ok := e.value.(error); ok {
		return err
	}
	return nil
}
