package response

import "net/http"

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

// WithError returns a copy of the error with an error cause.
func (e Error) WithError(err error) Error {
	if e.Details == nil {
		e.Details = map[string]any{"cause": err.Error()}
	} else {
		e.Details["cause"] = err.Error()
	}
	return e
}

// Render implements the Response interface by returning itself as an error.
// This allows Error to be used directly as a Response that will be handled
// by the global error handler.
func (e Error) Render(w http.ResponseWriter, req *http.Request) error {
	return e
}
