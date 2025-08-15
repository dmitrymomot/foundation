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

// NewError creates a new Error with a custom message and default internal server error status.
// The error will have a 500 status code and "internal_server_error" code.
func NewError(message string) Error {
	return Error{
		Status:  http.StatusInternalServerError,
		Code:    "internal_server_error",
		Message: message,
	}
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

// Render implements the Response interface by returning itself as an error.
// This allows Error to be used directly as a Response that will be handled
// by the global error handler.
func (e Error) Render(w http.ResponseWriter, req *http.Request) error {
	return e
}

// Predefined HTTP errors using http.StatusText for default messages.
var (
	// 4xx Client Errors
	ErrBadRequest                   = Error{Status: http.StatusBadRequest, Code: "bad_request", Message: http.StatusText(http.StatusBadRequest)}
	ErrUnauthorized                 = Error{Status: http.StatusUnauthorized, Code: "unauthorized", Message: http.StatusText(http.StatusUnauthorized)}
	ErrPaymentRequired              = Error{Status: http.StatusPaymentRequired, Code: "payment_required", Message: http.StatusText(http.StatusPaymentRequired)}
	ErrForbidden                    = Error{Status: http.StatusForbidden, Code: "forbidden", Message: http.StatusText(http.StatusForbidden)}
	ErrNotFound                     = Error{Status: http.StatusNotFound, Code: "not_found", Message: http.StatusText(http.StatusNotFound)}
	ErrMethodNotAllowed             = Error{Status: http.StatusMethodNotAllowed, Code: "method_not_allowed", Message: http.StatusText(http.StatusMethodNotAllowed)}
	ErrNotAcceptable                = Error{Status: http.StatusNotAcceptable, Code: "not_acceptable", Message: http.StatusText(http.StatusNotAcceptable)}
	ErrProxyAuthRequired            = Error{Status: http.StatusProxyAuthRequired, Code: "proxy_auth_required", Message: http.StatusText(http.StatusProxyAuthRequired)}
	ErrRequestTimeout               = Error{Status: http.StatusRequestTimeout, Code: "request_timeout", Message: http.StatusText(http.StatusRequestTimeout)}
	ErrConflict                     = Error{Status: http.StatusConflict, Code: "conflict", Message: http.StatusText(http.StatusConflict)}
	ErrGone                         = Error{Status: http.StatusGone, Code: "gone", Message: http.StatusText(http.StatusGone)}
	ErrLengthRequired               = Error{Status: http.StatusLengthRequired, Code: "length_required", Message: http.StatusText(http.StatusLengthRequired)}
	ErrPreconditionFailed           = Error{Status: http.StatusPreconditionFailed, Code: "precondition_failed", Message: http.StatusText(http.StatusPreconditionFailed)}
	ErrRequestEntityTooLarge        = Error{Status: http.StatusRequestEntityTooLarge, Code: "request_entity_too_large", Message: http.StatusText(http.StatusRequestEntityTooLarge)}
	ErrRequestURITooLong            = Error{Status: http.StatusRequestURITooLong, Code: "request_uri_too_long", Message: http.StatusText(http.StatusRequestURITooLong)}
	ErrUnsupportedMediaType         = Error{Status: http.StatusUnsupportedMediaType, Code: "unsupported_media_type", Message: http.StatusText(http.StatusUnsupportedMediaType)}
	ErrRequestedRangeNotSatisfiable = Error{Status: http.StatusRequestedRangeNotSatisfiable, Code: "requested_range_not_satisfiable", Message: http.StatusText(http.StatusRequestedRangeNotSatisfiable)}
	ErrExpectationFailed            = Error{Status: http.StatusExpectationFailed, Code: "expectation_failed", Message: http.StatusText(http.StatusExpectationFailed)}
	ErrTeapot                       = Error{Status: http.StatusTeapot, Code: "teapot", Message: "I'm a teapot"}
	ErrMisdirectedRequest           = Error{Status: http.StatusMisdirectedRequest, Code: "misdirected_request", Message: http.StatusText(http.StatusMisdirectedRequest)}
	ErrUnprocessableEntity          = Error{Status: http.StatusUnprocessableEntity, Code: "unprocessable_entity", Message: http.StatusText(http.StatusUnprocessableEntity)}
	ErrLocked                       = Error{Status: http.StatusLocked, Code: "locked", Message: http.StatusText(http.StatusLocked)}
	ErrFailedDependency             = Error{Status: http.StatusFailedDependency, Code: "failed_dependency", Message: http.StatusText(http.StatusFailedDependency)}
	ErrTooEarly                     = Error{Status: http.StatusTooEarly, Code: "too_early", Message: http.StatusText(http.StatusTooEarly)}
	ErrUpgradeRequired              = Error{Status: http.StatusUpgradeRequired, Code: "upgrade_required", Message: http.StatusText(http.StatusUpgradeRequired)}
	ErrPreconditionRequired         = Error{Status: http.StatusPreconditionRequired, Code: "precondition_required", Message: http.StatusText(http.StatusPreconditionRequired)}
	ErrTooManyRequests              = Error{Status: http.StatusTooManyRequests, Code: "too_many_requests", Message: http.StatusText(http.StatusTooManyRequests)}
	ErrRequestHeaderFieldsTooLarge  = Error{Status: http.StatusRequestHeaderFieldsTooLarge, Code: "request_header_fields_too_large", Message: http.StatusText(http.StatusRequestHeaderFieldsTooLarge)}
	ErrUnavailableForLegalReasons   = Error{Status: http.StatusUnavailableForLegalReasons, Code: "unavailable_for_legal_reasons", Message: http.StatusText(http.StatusUnavailableForLegalReasons)}

	// 5xx Server Errors
	ErrInternalServerError           = Error{Status: http.StatusInternalServerError, Code: "internal_server_error", Message: http.StatusText(http.StatusInternalServerError)}
	ErrNotImplemented                = Error{Status: http.StatusNotImplemented, Code: "not_implemented", Message: http.StatusText(http.StatusNotImplemented)}
	ErrBadGateway                    = Error{Status: http.StatusBadGateway, Code: "bad_gateway", Message: http.StatusText(http.StatusBadGateway)}
	ErrServiceUnavailable            = Error{Status: http.StatusServiceUnavailable, Code: "service_unavailable", Message: http.StatusText(http.StatusServiceUnavailable)}
	ErrGatewayTimeout                = Error{Status: http.StatusGatewayTimeout, Code: "gateway_timeout", Message: http.StatusText(http.StatusGatewayTimeout)}
	ErrHTTPVersionNotSupported       = Error{Status: http.StatusHTTPVersionNotSupported, Code: "http_version_not_supported", Message: http.StatusText(http.StatusHTTPVersionNotSupported)}
	ErrVariantAlsoNegotiates         = Error{Status: http.StatusVariantAlsoNegotiates, Code: "variant_also_negotiates", Message: http.StatusText(http.StatusVariantAlsoNegotiates)}
	ErrInsufficientStorage           = Error{Status: http.StatusInsufficientStorage, Code: "insufficient_storage", Message: http.StatusText(http.StatusInsufficientStorage)}
	ErrLoopDetected                  = Error{Status: http.StatusLoopDetected, Code: "loop_detected", Message: http.StatusText(http.StatusLoopDetected)}
	ErrNotExtended                   = Error{Status: http.StatusNotExtended, Code: "not_extended", Message: http.StatusText(http.StatusNotExtended)}
	ErrNetworkAuthenticationRequired = Error{Status: http.StatusNetworkAuthenticationRequired, Code: "network_authentication_required", Message: http.StatusText(http.StatusNetworkAuthenticationRequired)}
)

// Standard router errors.
var (
	ErrNilResponse = Error{Status: http.StatusInternalServerError, Code: "nil_response", Message: "handler returned nil response"}

	// Configuration errors
	ErrNoContextFactory = NewError("no context factory provided and C is not *Context")
	ErrInvalidMethod    = NewError("invalid http method")
	ErrInvalidPattern   = NewError("routing pattern must begin with '/'")
	ErrNilRouter        = NewError("cannot mount nil router")
	ErrNilSubrouter     = NewError("subrouter function cannot be nil")

	// Pattern parsing errors
	ErrInvalidRegexp    = NewError("invalid regexp pattern in route param")
	ErrMissingChild     = NewError("replacing missing child")
	ErrWildcardPosition = NewError("wildcard '*' must be the last pattern in a route")
	ErrParamDelimiter   = NewError("route param closing delimiter '}' is missing")
	ErrDuplicateParam   = NewError("routing pattern contains duplicate param key")
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

	// Fallback for non-Error types
	http.Error(w, "500 Internal Server Error", http.StatusInternalServerError)
}

// toError converts any value to an error.
func toError(v any) error {
	switch e := v.(type) {
	case error:
		return e
	case string:
		return NewError(e)
	default:
		return fmt.Errorf("panic: %v", e)
	}
}
