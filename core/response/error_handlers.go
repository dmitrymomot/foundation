package response

import (
	"errors"
	"net/http"

	"github.com/dmitrymomot/gokit/core/handler"
)

// statusCode is an interface that errors can implement
// to provide a custom HTTP status code.
type statusCode interface {
	StatusCode() int
}

// convertToHTTPError converts any error to an HTTPError
func convertToHTTPError(err error) HTTPError {
	var httpErr HTTPError

	// First check if it's already an HTTPError
	if errors.As(err, &httpErr) {
		return httpErr
	}

	// Not an HTTPError, need to convert it
	status := http.StatusInternalServerError

	// Check for statusCode interface
	if sc, ok := err.(statusCode); ok {
		status = sc.StatusCode()
	}

	// Look up the appropriate HTTPError from the map
	baseErr, ok := httpErrorsByStatus[status]
	if !ok {
		// If status not in map, use internal server error
		baseErr = ErrInternalServerError
	}

	// Attach the original error
	return baseErr.WithError(err)
}

// ErrorHandler is the default error handler that returns plain text errors.
// It checks for HTTPError type first, then statusCode interface, and defaults to 500.
func ErrorHandler[C handler.Context](ctx C, err error) {
	httpErr := convertToHTTPError(err)
	Render(ctx, StringWithStatus(httpErr.Error(), httpErr.Status))
}

// JSONErrorHandler returns errors as JSON responses.
// It checks for HTTPError type first (to get structured data), then statusCode interface, and defaults to 500.
func JSONErrorHandler[C handler.Context](ctx C, err error) {
	httpErr := convertToHTTPError(err)
	Render(ctx, JSONWithStatus(httpErr, httpErr.Status))
}
