package response

import "net/http"

// HTTPError represents a structured error response that implements the error interface.
type HTTPError struct {
	Status  int            `json:"-"`                 // HTTP status code (not in JSON)
	Code    string         `json:"code"`              // Machine-readable error code
	Message string         `json:"message"`           // Human-readable message
	Details map[string]any `json:"details,omitempty"` // Optional context
}

// NewHTTPError creates a new Error with a custom message and default internal server error status.
// The error will have a 500 status code and "internal_server_error" code.
func NewHTTPError(message string) HTTPError {
	return HTTPError{
		Status:  http.StatusInternalServerError,
		Code:    "internal_server_error",
		Message: message,
	}
}

// Error implements the error interface.
func (e HTTPError) Error() string {
	return e.Message
}

// StatusCode returns the HTTP status code for the error.
// This allows HTTPError to work with the router's statusCode interface.
func (e HTTPError) StatusCode() int {
	return e.Status
}

// WithMessage returns a copy of the error with a custom message.
func (e HTTPError) WithMessage(message string) HTTPError {
	e.Message = message
	return e
}

// WithDetails returns a copy of the error with additional details.
func (e HTTPError) WithDetails(details map[string]any) HTTPError {
	e.Details = details
	return e
}

// WithError returns a copy of the error with an error cause.
func (e HTTPError) WithError(err error) HTTPError {
	if e.Details == nil {
		e.Details = map[string]any{"cause": err.Error()}
	} else {
		e.Details["cause"] = err.Error()
	}
	return e
}

// Predefined HTTP errors using http.StatusText for default messages.
var (
	// 4xx Client Errors
	ErrBadRequest = HTTPError{
		Status:  http.StatusBadRequest,
		Code:    "bad_request",
		Message: http.StatusText(http.StatusBadRequest),
	}

	ErrUnauthorized = HTTPError{
		Status:  http.StatusUnauthorized,
		Code:    "unauthorized",
		Message: http.StatusText(http.StatusUnauthorized),
	}

	ErrPaymentRequired = HTTPError{
		Status:  http.StatusPaymentRequired,
		Code:    "payment_required",
		Message: http.StatusText(http.StatusPaymentRequired),
	}

	ErrForbidden = HTTPError{
		Status:  http.StatusForbidden,
		Code:    "forbidden",
		Message: http.StatusText(http.StatusForbidden),
	}

	ErrNotFound = HTTPError{
		Status:  http.StatusNotFound,
		Code:    "not_found",
		Message: http.StatusText(http.StatusNotFound),
	}

	ErrMethodNotAllowed = HTTPError{
		Status:  http.StatusMethodNotAllowed,
		Code:    "method_not_allowed",
		Message: http.StatusText(http.StatusMethodNotAllowed),
	}

	ErrNotAcceptable = HTTPError{
		Status:  http.StatusNotAcceptable,
		Code:    "not_acceptable",
		Message: http.StatusText(http.StatusNotAcceptable),
	}

	ErrProxyAuthRequired = HTTPError{
		Status:  http.StatusProxyAuthRequired,
		Code:    "proxy_auth_required",
		Message: http.StatusText(http.StatusProxyAuthRequired),
	}

	ErrRequestTimeout = HTTPError{
		Status:  http.StatusRequestTimeout,
		Code:    "request_timeout",
		Message: http.StatusText(http.StatusRequestTimeout),
	}

	ErrConflict = HTTPError{
		Status:  http.StatusConflict,
		Code:    "conflict",
		Message: http.StatusText(http.StatusConflict),
	}

	ErrGone = HTTPError{
		Status:  http.StatusGone,
		Code:    "gone",
		Message: http.StatusText(http.StatusGone),
	}

	ErrLengthRequired = HTTPError{
		Status:  http.StatusLengthRequired,
		Code:    "length_required",
		Message: http.StatusText(http.StatusLengthRequired),
	}

	ErrPreconditionFailed = HTTPError{
		Status:  http.StatusPreconditionFailed,
		Code:    "precondition_failed",
		Message: http.StatusText(http.StatusPreconditionFailed),
	}

	ErrRequestEntityTooLarge = HTTPError{
		Status:  http.StatusRequestEntityTooLarge,
		Code:    "request_entity_too_large",
		Message: http.StatusText(http.StatusRequestEntityTooLarge),
	}

	ErrRequestURITooLong = HTTPError{
		Status:  http.StatusRequestURITooLong,
		Code:    "request_uri_too_long",
		Message: http.StatusText(http.StatusRequestURITooLong),
	}

	ErrUnsupportedMediaType = HTTPError{
		Status:  http.StatusUnsupportedMediaType,
		Code:    "unsupported_media_type",
		Message: http.StatusText(http.StatusUnsupportedMediaType),
	}

	ErrRequestedRangeNotSatisfiable = HTTPError{
		Status:  http.StatusRequestedRangeNotSatisfiable,
		Code:    "requested_range_not_satisfiable",
		Message: http.StatusText(http.StatusRequestedRangeNotSatisfiable),
	}

	ErrExpectationFailed = HTTPError{
		Status:  http.StatusExpectationFailed,
		Code:    "expectation_failed",
		Message: http.StatusText(http.StatusExpectationFailed),
	}

	ErrTeapot = HTTPError{
		Status:  http.StatusTeapot,
		Code:    "teapot",
		Message: "I'm a teapot",
	}

	ErrMisdirectedRequest = HTTPError{
		Status:  http.StatusMisdirectedRequest,
		Code:    "misdirected_request",
		Message: http.StatusText(http.StatusMisdirectedRequest),
	}

	ErrUnprocessableEntity = HTTPError{
		Status:  http.StatusUnprocessableEntity,
		Code:    "unprocessable_entity",
		Message: http.StatusText(http.StatusUnprocessableEntity),
	}

	ErrLocked = HTTPError{
		Status:  http.StatusLocked,
		Code:    "locked",
		Message: http.StatusText(http.StatusLocked),
	}

	ErrFailedDependency = HTTPError{
		Status:  http.StatusFailedDependency,
		Code:    "failed_dependency",
		Message: http.StatusText(http.StatusFailedDependency),
	}

	ErrTooEarly = HTTPError{
		Status:  http.StatusTooEarly,
		Code:    "too_early",
		Message: http.StatusText(http.StatusTooEarly),
	}

	ErrUpgradeRequired = HTTPError{
		Status:  http.StatusUpgradeRequired,
		Code:    "upgrade_required",
		Message: http.StatusText(http.StatusUpgradeRequired),
	}

	ErrPreconditionRequired = HTTPError{
		Status:  http.StatusPreconditionRequired,
		Code:    "precondition_required",
		Message: http.StatusText(http.StatusPreconditionRequired),
	}

	ErrTooManyRequests = HTTPError{
		Status:  http.StatusTooManyRequests,
		Code:    "too_many_requests",
		Message: http.StatusText(http.StatusTooManyRequests),
	}

	ErrRequestHeaderFieldsTooLarge = HTTPError{
		Status:  http.StatusRequestHeaderFieldsTooLarge,
		Code:    "request_header_fields_too_large",
		Message: http.StatusText(http.StatusRequestHeaderFieldsTooLarge),
	}

	ErrUnavailableForLegalReasons = HTTPError{
		Status:  http.StatusUnavailableForLegalReasons,
		Code:    "unavailable_for_legal_reasons",
		Message: http.StatusText(http.StatusUnavailableForLegalReasons),
	}

	// 5xx Server Errors
	ErrInternalServerError = HTTPError{
		Status:  http.StatusInternalServerError,
		Code:    "internal_server_error",
		Message: http.StatusText(http.StatusInternalServerError),
	}

	ErrNotImplemented = HTTPError{
		Status:  http.StatusNotImplemented,
		Code:    "not_implemented",
		Message: http.StatusText(http.StatusNotImplemented),
	}

	ErrBadGateway = HTTPError{
		Status:  http.StatusBadGateway,
		Code:    "bad_gateway",
		Message: http.StatusText(http.StatusBadGateway),
	}

	ErrServiceUnavailable = HTTPError{
		Status:  http.StatusServiceUnavailable,
		Code:    "service_unavailable",
		Message: http.StatusText(http.StatusServiceUnavailable),
	}

	ErrGatewayTimeout = HTTPError{
		Status:  http.StatusGatewayTimeout,
		Code:    "gateway_timeout",
		Message: http.StatusText(http.StatusGatewayTimeout),
	}

	ErrHTTPVersionNotSupported = HTTPError{
		Status:  http.StatusHTTPVersionNotSupported,
		Code:    "http_version_not_supported",
		Message: http.StatusText(http.StatusHTTPVersionNotSupported),
	}

	ErrVariantAlsoNegotiates = HTTPError{
		Status:  http.StatusVariantAlsoNegotiates,
		Code:    "variant_also_negotiates",
		Message: http.StatusText(http.StatusVariantAlsoNegotiates),
	}

	ErrInsufficientStorage = HTTPError{
		Status:  http.StatusInsufficientStorage,
		Code:    "insufficient_storage",
		Message: http.StatusText(http.StatusInsufficientStorage),
	}

	ErrLoopDetected = HTTPError{
		Status:  http.StatusLoopDetected,
		Code:    "loop_detected",
		Message: http.StatusText(http.StatusLoopDetected),
	}

	ErrNotExtended = HTTPError{
		Status:  http.StatusNotExtended,
		Code:    "not_extended",
		Message: http.StatusText(http.StatusNotExtended),
	}

	ErrNetworkAuthenticationRequired = HTTPError{
		Status:  http.StatusNetworkAuthenticationRequired,
		Code:    "network_authentication_required",
		Message: http.StatusText(http.StatusNetworkAuthenticationRequired),
	}
)

// httpErrorsByStatus maps HTTP status codes to their corresponding HTTPError values
var httpErrorsByStatus = map[int]HTTPError{
	http.StatusBadRequest:                    ErrBadRequest,
	http.StatusUnauthorized:                  ErrUnauthorized,
	http.StatusPaymentRequired:               ErrPaymentRequired,
	http.StatusForbidden:                     ErrForbidden,
	http.StatusNotFound:                      ErrNotFound,
	http.StatusMethodNotAllowed:              ErrMethodNotAllowed,
	http.StatusNotAcceptable:                 ErrNotAcceptable,
	http.StatusProxyAuthRequired:             ErrProxyAuthRequired,
	http.StatusRequestTimeout:                ErrRequestTimeout,
	http.StatusConflict:                      ErrConflict,
	http.StatusGone:                          ErrGone,
	http.StatusLengthRequired:                ErrLengthRequired,
	http.StatusPreconditionFailed:            ErrPreconditionFailed,
	http.StatusRequestEntityTooLarge:         ErrRequestEntityTooLarge,
	http.StatusRequestURITooLong:             ErrRequestURITooLong,
	http.StatusUnsupportedMediaType:          ErrUnsupportedMediaType,
	http.StatusRequestedRangeNotSatisfiable:  ErrRequestedRangeNotSatisfiable,
	http.StatusExpectationFailed:             ErrExpectationFailed,
	http.StatusTeapot:                        ErrTeapot,
	http.StatusMisdirectedRequest:            ErrMisdirectedRequest,
	http.StatusUnprocessableEntity:           ErrUnprocessableEntity,
	http.StatusLocked:                        ErrLocked,
	http.StatusFailedDependency:              ErrFailedDependency,
	http.StatusTooEarly:                      ErrTooEarly,
	http.StatusUpgradeRequired:               ErrUpgradeRequired,
	http.StatusPreconditionRequired:          ErrPreconditionRequired,
	http.StatusTooManyRequests:               ErrTooManyRequests,
	http.StatusRequestHeaderFieldsTooLarge:   ErrRequestHeaderFieldsTooLarge,
	http.StatusUnavailableForLegalReasons:    ErrUnavailableForLegalReasons,
	http.StatusInternalServerError:           ErrInternalServerError,
	http.StatusNotImplemented:                ErrNotImplemented,
	http.StatusBadGateway:                    ErrBadGateway,
	http.StatusServiceUnavailable:            ErrServiceUnavailable,
	http.StatusGatewayTimeout:                ErrGatewayTimeout,
	http.StatusHTTPVersionNotSupported:       ErrHTTPVersionNotSupported,
	http.StatusVariantAlsoNegotiates:         ErrVariantAlsoNegotiates,
	http.StatusInsufficientStorage:           ErrInsufficientStorage,
	http.StatusLoopDetected:                  ErrLoopDetected,
	http.StatusNotExtended:                   ErrNotExtended,
	http.StatusNetworkAuthenticationRequired: ErrNetworkAuthenticationRequired,
}
