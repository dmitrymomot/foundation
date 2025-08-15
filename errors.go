package gokit

import (
	"errors"
	"fmt"
	"net/http"
)

// Error represents a structured error response that implements the error interface.
type Error struct {
	Status  int            `json:"-"`                 // HTTP status code (not in JSON)
	Code    string         `json:"code"`              // Machine-readable error code
	Message string         `json:"message"`           // Human-readable message
	Details map[string]any `json:"details,omitempty"` // Optional context
}

// Error implements the error interface.
func (e Error) Error() string {
	return e.Message
}

// WithMessage returns a copy of the error with a custom message.
func (e Error) WithMessage(message string) Error {
	e.Message = message
	return e
}

// WithDetails returns a copy of the error with additional details.
func (e Error) WithDetails(details map[string]any) Error {
	e.Details = details
	return e
}

// Predefined HTTP errors using http.StatusText for default messages.
var (
	ErrBadRequest           = Error{Status: http.StatusBadRequest, Code: "BAD_REQUEST", Message: http.StatusText(http.StatusBadRequest)}
	ErrUnauthorized         = Error{Status: http.StatusUnauthorized, Code: "UNAUTHORIZED", Message: http.StatusText(http.StatusUnauthorized)}
	ErrForbidden            = Error{Status: http.StatusForbidden, Code: "FORBIDDEN", Message: http.StatusText(http.StatusForbidden)}
	ErrNotFoundHTTP         = Error{Status: http.StatusNotFound, Code: "NOT_FOUND", Message: http.StatusText(http.StatusNotFound)}
	ErrMethodNotAllowedHTTP = Error{Status: http.StatusMethodNotAllowed, Code: "METHOD_NOT_ALLOWED", Message: http.StatusText(http.StatusMethodNotAllowed)}
	ErrConflict             = Error{Status: http.StatusConflict, Code: "CONFLICT", Message: http.StatusText(http.StatusConflict)}
	ErrUnprocessableEntity  = Error{Status: http.StatusUnprocessableEntity, Code: "UNPROCESSABLE_ENTITY", Message: http.StatusText(http.StatusUnprocessableEntity)}
	ErrTooManyRequests      = Error{Status: http.StatusTooManyRequests, Code: "TOO_MANY_REQUESTS", Message: http.StatusText(http.StatusTooManyRequests)}
	ErrInternalServerError  = Error{Status: http.StatusInternalServerError, Code: "INTERNAL_SERVER_ERROR", Message: http.StatusText(http.StatusInternalServerError)}
	ErrNotImplemented       = Error{Status: http.StatusNotImplemented, Code: "NOT_IMPLEMENTED", Message: http.StatusText(http.StatusNotImplemented)}
	ErrBadGateway           = Error{Status: http.StatusBadGateway, Code: "BAD_GATEWAY", Message: http.StatusText(http.StatusBadGateway)}
	ErrServiceUnavailable   = Error{Status: http.StatusServiceUnavailable, Code: "SERVICE_UNAVAILABLE", Message: http.StatusText(http.StatusServiceUnavailable)}
	ErrGatewayTimeout       = Error{Status: http.StatusGatewayTimeout, Code: "GATEWAY_TIMEOUT", Message: http.StatusText(http.StatusGatewayTimeout)}
)

// Standard router errors.
var (
	ErrNotFound         = errors.New("route not found")
	ErrMethodNotAllowed = errors.New("method not allowed")
	ErrNilResponse      = errors.New("handler returned nil response")

	// Configuration errors
	ErrNoContextFactory = errors.New("no context factory provided and C is not *Context")
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

	// Streaming errors
	ErrStreamEncode = errors.New("failed to encode stream item")
	ErrStreamWrite  = errors.New("failed to write stream data")

	// SSE errors
	ErrSSEWrite     = errors.New("failed to write SSE event")
	ErrSSEMarshal   = errors.New("failed to marshal SSE data")
	ErrSSEKeepAlive = errors.New("failed to send keepalive")
)

// defaultErrorHandler provides default error handling.
func defaultErrorHandler[C contexter](ctx C, err error) {
	w := ctx.ResponseWriter()

	// Prevent double-writing responses which causes HTTP protocol errors
	if ww, ok := w.(*responseWriter); ok && ww.Written() {
		return
	}

	var appErr Error
	if errors.As(err, &appErr) {
		http.Error(w, appErr.Message, appErr.Status)
		return
	}

	switch {
	case errors.Is(err, ErrNotFound):
		http.Error(w, "404 Not Found", http.StatusNotFound)
	case errors.Is(err, ErrMethodNotAllowed):
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
