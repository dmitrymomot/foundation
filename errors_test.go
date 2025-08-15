package gokit_test

import (
	"errors"
	"net/http"
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

	original := gokit.ErrNotFoundHTTP
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
				assert.Equal(t, "TEST_ERROR", appErr.Code)
				assert.Equal(t, "Test error message", appErr.Message)
			} else {
				t.Fatal("Failed to extract Error type using errors.As")
			}
		}
	}()

	// Panic with Error type
	panic(gokit.Error{
		Status:  http.StatusInternalServerError,
		Code:    "TEST_ERROR",
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
		{gokit.ErrBadRequest, http.StatusBadRequest, "BAD_REQUEST"},
		{gokit.ErrUnauthorized, http.StatusUnauthorized, "UNAUTHORIZED"},
		{gokit.ErrForbidden, http.StatusForbidden, "FORBIDDEN"},
		{gokit.ErrNotFoundHTTP, http.StatusNotFound, "NOT_FOUND"},
		{gokit.ErrMethodNotAllowedHTTP, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED"},
		{gokit.ErrConflict, http.StatusConflict, "CONFLICT"},
		{gokit.ErrUnprocessableEntity, http.StatusUnprocessableEntity, "UNPROCESSABLE_ENTITY"},
		{gokit.ErrTooManyRequests, http.StatusTooManyRequests, "TOO_MANY_REQUESTS"},
		{gokit.ErrInternalServerError, http.StatusInternalServerError, "INTERNAL_SERVER_ERROR"},
		{gokit.ErrNotImplemented, http.StatusNotImplemented, "NOT_IMPLEMENTED"},
		{gokit.ErrBadGateway, http.StatusBadGateway, "BAD_GATEWAY"},
		{gokit.ErrServiceUnavailable, http.StatusServiceUnavailable, "SERVICE_UNAVAILABLE"},
		{gokit.ErrGatewayTimeout, http.StatusGatewayTimeout, "GATEWAY_TIMEOUT"},
	}

	for _, tt := range tests {
		t.Run(tt.code, func(t *testing.T) {
			assert.Equal(t, tt.status, tt.err.Status)
			assert.Equal(t, tt.code, tt.err.Code)
			assert.Equal(t, http.StatusText(tt.status), tt.err.Message)
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
