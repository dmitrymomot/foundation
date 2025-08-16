package gokit_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/dmitrymomot/gokit"
	"github.com/stretchr/testify/assert"
)

func TestError_ImplementsErrorInterface(t *testing.T) {
	t.Parallel()

	err := gokit.Error{
		Status:  http.StatusNotFound,
		Code:    "NOT_FOUND",
		Message: "Resource not found",
	}

	// Should implement error interface
	var _ error = err
	assert.Equal(t, "Resource not found", err.Error())
}

func TestError_WithMessage(t *testing.T) {
	t.Parallel()

	original := gokit.ErrNotFound
	modified := original.WithMessage("User not found")

	// Original should not be modified
	assert.Equal(t, http.StatusText(http.StatusNotFound), original.Message)

	// Modified should have new message
	assert.Equal(t, "User not found", modified.Message)
	assert.Equal(t, original.Status, modified.Status)
	assert.Equal(t, original.Code, modified.Code)
}

func TestError_WithDetails(t *testing.T) {
	t.Parallel()

	original := gokit.ErrBadRequest
	details := map[string]any{
		"field": "email",
		"error": "invalid format",
	}
	modified := original.WithDetails(details)

	// Original should not have details
	assert.Nil(t, original.Details)

	// Modified should have details
	assert.Equal(t, details, modified.Details)
	assert.Equal(t, original.Status, modified.Status)
	assert.Equal(t, original.Code, modified.Code)
	assert.Equal(t, original.Message, modified.Message)
}

func TestError_CanBeCaughtWithErrorsAs(t *testing.T) {
	t.Parallel()

	// Simulate panic and recovery
	defer func() {
		if r := recover(); r != nil {
			// Convert to error
			err := toError(r)

			// Should be able to extract Error type using errors.As
			var appErr gokit.Error
			if errors.As(err, &appErr) {
				assert.Equal(t, http.StatusInternalServerError, appErr.Status)
				assert.Equal(t, "test_error", appErr.Code)
				assert.Equal(t, "Test error message", appErr.Message)
			} else {
				t.Fatal("Failed to extract Error type using errors.As")
			}
		}
	}()

	// Panic with Error type
	panic(gokit.Error{
		Status:  http.StatusInternalServerError,
		Code:    "test_error",
		Message: "Test error message",
	})
}

func TestPredefinedErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		err    gokit.Error
		status int
		code   string
	}{
		{gokit.ErrBadRequest, http.StatusBadRequest, "bad_request"},
		{gokit.ErrUnauthorized, http.StatusUnauthorized, "unauthorized"},
		{gokit.ErrForbidden, http.StatusForbidden, "forbidden"},
		{gokit.ErrNotFound, http.StatusNotFound, "not_found"},
		{gokit.ErrMethodNotAllowed, http.StatusMethodNotAllowed, "method_not_allowed"},
		{gokit.ErrConflict, http.StatusConflict, "conflict"},
		{gokit.ErrUnprocessableEntity, http.StatusUnprocessableEntity, "unprocessable_entity"},
		{gokit.ErrTooManyRequests, http.StatusTooManyRequests, "too_many_requests"},
		{gokit.ErrInternalServerError, http.StatusInternalServerError, "internal_server_error"},
		{gokit.ErrNotImplemented, http.StatusNotImplemented, "not_implemented"},
		{gokit.ErrBadGateway, http.StatusBadGateway, "bad_gateway"},
		{gokit.ErrServiceUnavailable, http.StatusServiceUnavailable, "service_unavailable"},
		{gokit.ErrGatewayTimeout, http.StatusGatewayTimeout, "gateway_timeout"},
	}

	for _, tt := range tests {
		t.Run(tt.code, func(t *testing.T) {
			assert.Equal(t, tt.status, tt.err.Status)
			assert.Equal(t, tt.code, tt.err.Code)
			assert.Equal(t, http.StatusText(tt.status), tt.err.Message)
		})
	}
}

func TestError_ImplementsResponseInterface(t *testing.T) {
	t.Parallel()

	// Test that Error implements Response interface
	err := gokit.ErrNotFound.WithMessage("Custom not found message")

	// Should implement Response interface
	var _ gokit.Response = err

	// Test Render method returns the error itself
	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)

	renderErr := err.Render(w, req)

	// Should return itself as the error
	assert.Equal(t, err, renderErr)

	// Verify error details are preserved
	var returnedErr gokit.Error
	assert.True(t, errors.As(renderErr, &returnedErr))
	assert.Equal(t, http.StatusNotFound, returnedErr.Status)
	assert.Equal(t, "not_found", returnedErr.Code)
	assert.Equal(t, "Custom not found message", returnedErr.Message)
}

func TestNewPredefinedErrors(t *testing.T) {
	t.Parallel()

	// Test some of the new predefined errors
	tests := []struct {
		err    gokit.Error
		status int
		code   string
	}{
		{gokit.ErrPaymentRequired, http.StatusPaymentRequired, "payment_required"},
		{gokit.ErrNotAcceptable, http.StatusNotAcceptable, "not_acceptable"},
		{gokit.ErrProxyAuthRequired, http.StatusProxyAuthRequired, "proxy_auth_required"},
		{gokit.ErrRequestTimeout, http.StatusRequestTimeout, "request_timeout"},
		{gokit.ErrGone, http.StatusGone, "gone"},
		{gokit.ErrTeapot, http.StatusTeapot, "teapot"},
		{gokit.ErrLocked, http.StatusLocked, "locked"},
		{gokit.ErrTooEarly, http.StatusTooEarly, "too_early"},
		{gokit.ErrHTTPVersionNotSupported, http.StatusHTTPVersionNotSupported, "http_version_not_supported"},
		{gokit.ErrVariantAlsoNegotiates, http.StatusVariantAlsoNegotiates, "variant_also_negotiates"},
		{gokit.ErrNetworkAuthenticationRequired, http.StatusNetworkAuthenticationRequired, "network_authentication_required"},
	}

	for _, tt := range tests {
		t.Run(tt.code, func(t *testing.T) {
			assert.Equal(t, tt.status, tt.err.Status)
			assert.Equal(t, tt.code, tt.err.Code)
		})
	}
}

func TestRouterErrors(t *testing.T) {
	t.Parallel()

	// Test that router errors are now Error types
	tests := []struct {
		err     gokit.Error
		status  int
		code    string
		message string
	}{
		{gokit.ErrNotFound, http.StatusNotFound, "not_found", "Not Found"},
		{gokit.ErrMethodNotAllowed, http.StatusMethodNotAllowed, "method_not_allowed", "Method Not Allowed"},
		{gokit.ErrNilResponse, http.StatusInternalServerError, "internal_server_error", "handler returned nil response"},
	}

	for _, tt := range tests {
		t.Run(tt.code, func(t *testing.T) {
			assert.Equal(t, tt.status, tt.err.Status)
			assert.Equal(t, tt.code, tt.err.Code)
			assert.Equal(t, tt.message, tt.err.Message)

			// Verify they implement error interface
			var _ error = tt.err
			assert.Equal(t, tt.message, tt.err.Error())
		})
	}
}

// Helper function similar to the one in errors.go
func toError(v any) error {
	switch e := v.(type) {
	case error:
		return e
	case string:
		return errors.New(e)
	default:
		return errors.New("unknown error")
	}
}
