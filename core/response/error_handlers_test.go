package response_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dmitrymomot/gokit/core/handler"
	"github.com/dmitrymomot/gokit/core/response"
	"github.com/dmitrymomot/gokit/core/router"
)

// testContext is a simple test implementation of handler.Context
type testContext struct {
	w http.ResponseWriter
	r *http.Request
}

func (tc *testContext) Deadline() (deadline time.Time, ok bool) {
	return tc.r.Context().Deadline()
}

func (tc *testContext) Done() <-chan struct{} {
	return tc.r.Context().Done()
}

func (tc *testContext) Err() error {
	return tc.r.Context().Err()
}

func (tc *testContext) Value(key any) any {
	return tc.r.Context().Value(key)
}

func (tc *testContext) SetValue(key, val any) {
	// Not needed for tests
}

func (tc *testContext) Request() *http.Request {
	return tc.r
}

func (tc *testContext) ResponseWriter() http.ResponseWriter {
	return tc.w
}

func (tc *testContext) Param(key string) string {
	// Not needed for tests
	return ""
}

// customStatusError is a test error that implements StatusCode() int
type customStatusError struct {
	message string
	status  int
}

func (e customStatusError) Error() string {
	return e.message
}

func (e customStatusError) StatusCode() int {
	return e.status
}

func TestErrorHandler(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		error          error
		expectedStatus int
		expectedBody   string
	}{
		{
			name:           "regular error returns 500",
			error:          errors.New("internal error"),
			expectedStatus: http.StatusInternalServerError,
			expectedBody:   "Internal Server Error\n", // Now returns HTTPError's message
		},
		{
			name:           "HTTPError with 401",
			error:          response.ErrUnauthorized.WithMessage("invalid credentials"),
			expectedStatus: http.StatusUnauthorized,
			expectedBody:   "invalid credentials\n", // No trailing newline
		},
		{
			name:           "HTTPError with 404",
			error:          response.ErrNotFound.WithMessage("resource not found"),
			expectedStatus: http.StatusNotFound,
			expectedBody:   "resource not found\n", // No trailing newline
		},
		{
			name:           "HTTPError with 400",
			error:          response.ErrBadRequest.WithMessage("bad request"),
			expectedStatus: http.StatusBadRequest,
			expectedBody:   "bad request\n", // No trailing newline
		},
		{
			name:           "custom error with StatusCode interface",
			error:          customStatusError{message: "custom error", status: http.StatusTeapot},
			expectedStatus: http.StatusTeapot,
			expectedBody:   "I'm a teapot\n", // Now returns the HTTPError's message
		},
		{
			name:           "HTTPError takes precedence over StatusCode interface",
			error:          response.ErrForbidden.WithMessage("access denied"),
			expectedStatus: http.StatusForbidden,
			expectedBody:   "access denied\n", // No trailing newline
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a test context
			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			w := httptest.NewRecorder()
			testCtx := &testContext{w: w, r: req}

			// Call ErrorHandler
			response.ErrorHandler(testCtx, tt.error)

			// Check response
			assert.Equal(t, tt.expectedStatus, w.Code)
			assert.Equal(t, "text/plain; charset=utf-8", w.Header().Get("Content-Type"))
			assert.Equal(t, tt.expectedBody+"\n", w.Body.String())
		})
	}
}

func TestJSONErrorHandler(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		error          error
		expectedStatus int
		expectedJSON   map[string]any
		checkDetails   bool
	}{
		{
			name:           "regular error returns 500",
			error:          errors.New("internal error"),
			expectedStatus: http.StatusInternalServerError,
			expectedJSON: map[string]any{
				"code":    "internal_server_error",
				"message": "Internal Server Error",
				"details": map[string]any{
					"cause": "internal error",
				},
			},
			checkDetails: true,
		},
		{
			name:           "HTTPError with structure",
			error:          response.ErrUnauthorized.WithMessage("invalid token"),
			expectedStatus: http.StatusUnauthorized,
			expectedJSON: map[string]any{
				"code":    "unauthorized",
				"message": "invalid token",
			},
		},
		{
			name: "HTTPError with details",
			error: response.ErrUnprocessableEntity.WithMessage("validation failed").WithDetails(map[string]any{
				"field":  "email",
				"reason": "invalid format",
			}),
			expectedStatus: http.StatusUnprocessableEntity,
			expectedJSON: map[string]any{
				"code":    "unprocessable_entity",
				"message": "validation failed",
				"details": map[string]any{
					"field":  "email",
					"reason": "invalid format",
				},
			},
			checkDetails: true,
		},
		{
			name:           "HTTPError with error cause in details",
			error:          response.ErrBadRequest.WithMessage("request failed").WithError(errors.New("underlying cause")),
			expectedStatus: http.StatusBadRequest,
			expectedJSON: map[string]any{
				"code":    "bad_request",
				"message": "request failed",
				"details": map[string]any{
					"cause": "underlying cause",
				},
			},
			checkDetails: true,
		},
		{
			name:           "custom error with StatusCode interface",
			error:          customStatusError{message: "custom error", status: http.StatusTeapot},
			expectedStatus: http.StatusTeapot,
			expectedJSON: map[string]any{
				"code":    "teapot",
				"message": "I'm a teapot",
				"details": map[string]any{
					"cause": "custom error",
				},
			},
			checkDetails: true,
		},
		{
			name:           "HTTPError takes precedence",
			error:          response.ErrForbidden.WithMessage("no access"),
			expectedStatus: http.StatusForbidden,
			expectedJSON: map[string]any{
				"code":    "forbidden",
				"message": "no access",
			},
		},
		{
			name:           "HTTPError without custom message uses default",
			error:          response.ErrNotFound,
			expectedStatus: http.StatusNotFound,
			expectedJSON: map[string]any{
				"code":    "not_found",
				"message": "Not Found",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a test context
			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			w := httptest.NewRecorder()
			testCtx := &testContext{w: w, r: req}

			// Call JSONErrorHandler
			response.JSONErrorHandler(testCtx, tt.error)

			// Check response
			assert.Equal(t, tt.expectedStatus, w.Code)
			assert.Equal(t, "application/json; charset=utf-8", w.Header().Get("Content-Type"))

			// Parse JSON response
			var result map[string]any
			err := json.NewDecoder(w.Body).Decode(&result)
			require.NoError(t, err)

			// Check JSON structure
			if tt.checkDetails {
				// Check all fields including details
				assert.Equal(t, tt.expectedJSON, result)
			} else {
				// Check fields except details (if present)
				for key, expectedValue := range tt.expectedJSON {
					if key != "details" {
						assert.Equal(t, expectedValue, result[key], "field %s mismatch", key)
					}
				}
			}
		})
	}
}

func TestErrorHandlersWithRouter(t *testing.T) {
	t.Parallel()

	t.Run("ErrorHandler with router", func(t *testing.T) {
		r := router.New[*router.Context](
			router.WithErrorHandler(response.ErrorHandler[*router.Context]),
		)

		r.Get("/error", func(ctx *router.Context) handler.Response {
			return response.Error(response.ErrUnauthorized.WithMessage("need auth"))
		})

		req := httptest.NewRequest(http.MethodGet, "/error", nil)
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
		assert.Equal(t, "text/plain; charset=utf-8", w.Header().Get("Content-Type"))
		assert.Equal(t, "need auth\n", w.Body.String())
	})

	t.Run("JSONErrorHandler with router", func(t *testing.T) {
		r := router.New[*router.Context](
			router.WithErrorHandler(response.JSONErrorHandler[*router.Context]),
		)

		r.Get("/error", func(ctx *router.Context) handler.Response {
			return response.Error(
				response.ErrBadRequest.WithMessage("invalid input").WithDetails(map[string]any{
					"field":      "username",
					"min_length": 3,
				}),
			)
		})

		req := httptest.NewRequest(http.MethodGet, "/error", nil)
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		assert.Equal(t, "application/json; charset=utf-8", w.Header().Get("Content-Type"))

		var result map[string]any
		err := json.NewDecoder(w.Body).Decode(&result)
		require.NoError(t, err)

		assert.Equal(t, "bad_request", result["code"])
		assert.Equal(t, "invalid input", result["message"])
		assert.NotNil(t, result["details"])

		details := result["details"].(map[string]any)
		assert.Equal(t, "username", details["field"])
		assert.Equal(t, float64(3), details["min_length"]) // JSON numbers decode as float64
	})
}

func TestErrorHandlersContentNegotiation(t *testing.T) {
	t.Parallel()

	// Create a content-negotiating error handler
	negotiatingErrorHandler := func(ctx *router.Context, err error) {
		accept := ctx.Request().Header.Get("Accept")

		if accept == "" || accept == "*/*" {
			// Default to plain text
			response.ErrorHandler(ctx, err)
		} else if accept == "application/json" {
			response.JSONErrorHandler(ctx, err)
		} else {
			response.ErrorHandler(ctx, err)
		}
	}

	r := router.New[*router.Context](
		router.WithErrorHandler(negotiatingErrorHandler),
	)

	r.Get("/error", func(ctx *router.Context) handler.Response {
		return response.Error(response.ErrNotFound.WithMessage("not found"))
	})

	tests := []struct {
		name         string
		acceptHeader string
		contentType  string
		bodyContains string
	}{
		{
			name:         "JSON when Accept is application/json",
			acceptHeader: "application/json",
			contentType:  "application/json; charset=utf-8",
			bodyContains: `"code":"not_found"`,
		},
		{
			name:         "Plain text when Accept is empty",
			acceptHeader: "",
			contentType:  "text/plain; charset=utf-8",
			bodyContains: "not found\n",
		},
		{
			name:         "Plain text when Accept is */*",
			acceptHeader: "*/*",
			contentType:  "text/plain; charset=utf-8",
			bodyContains: "not found\n",
		},
		{
			name:         "Plain text for other Accept values",
			acceptHeader: "text/html",
			contentType:  "text/plain; charset=utf-8",
			bodyContains: "not found\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/error", nil)
			if tt.acceptHeader != "" {
				req.Header.Set("Accept", tt.acceptHeader)
			}
			w := httptest.NewRecorder()

			r.ServeHTTP(w, req)

			assert.Equal(t, http.StatusNotFound, w.Code)
			assert.Equal(t, tt.contentType, w.Header().Get("Content-Type"))
			assert.Contains(t, w.Body.String(), tt.bodyContains)
		})
	}
}
