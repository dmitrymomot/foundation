package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dmitrymomot/gokit/core/handler"
	"github.com/dmitrymomot/gokit/core/router"
	"github.com/dmitrymomot/gokit/middleware"
)

func TestRequestIDDefaultConfiguration(t *testing.T) {
	t.Parallel()

	r := router.New[*router.Context]()

	requestIDMiddleware := middleware.RequestID[*router.Context]()
	r.Use(requestIDMiddleware)

	var capturedID string
	r.Get("/test", func(ctx *router.Context) handler.Response {
		id, ok := middleware.GetRequestID(ctx)
		assert.True(t, ok, "Request ID should be present in context")
		capturedID = id
		return func(w http.ResponseWriter, r *http.Request) error {
			w.WriteHeader(http.StatusOK)
			return nil
		}
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.NotEmpty(t, capturedID, "Request ID should be generated")
	assert.Equal(t, capturedID, w.Header().Get("X-Request-ID"), "Request ID should be in response header")

	// Validate UUID format (default generator)
	assert.Len(t, capturedID, 36, "Default ID should be UUID v4 format")
	assert.Contains(t, capturedID, "-", "UUID should contain hyphens")
}

func TestRequestIDCustomGenerator(t *testing.T) {
	t.Parallel()

	r := router.New[*router.Context]()

	customID := "custom-123-456"
	requestIDMiddleware := middleware.RequestIDWithConfig[*router.Context](middleware.RequestIDConfig{
		Generator: func() string {
			return customID
		},
	})
	r.Use(requestIDMiddleware)

	var capturedID string
	r.Get("/test", func(ctx *router.Context) handler.Response {
		id, ok := middleware.GetRequestID(ctx)
		assert.True(t, ok)
		capturedID = id
		return func(w http.ResponseWriter, r *http.Request) error {
			w.WriteHeader(http.StatusOK)
			return nil
		}
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, customID, capturedID, "Should use custom generator")
	assert.Equal(t, customID, w.Header().Get("X-Request-ID"))
}

func TestRequestIDCustomHeaderName(t *testing.T) {
	t.Parallel()

	r := router.New[*router.Context]()

	customHeaderName := "X-Trace-ID"
	requestIDMiddleware := middleware.RequestIDWithConfig[*router.Context](middleware.RequestIDConfig{
		HeaderName: customHeaderName,
	})
	r.Use(requestIDMiddleware)

	r.Get("/test", func(ctx *router.Context) handler.Response {
		return func(w http.ResponseWriter, r *http.Request) error {
			w.WriteHeader(http.StatusOK)
			return nil
		}
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.NotEmpty(t, w.Header().Get(customHeaderName), "Custom header should be set")
	assert.Empty(t, w.Header().Get("X-Request-ID"), "Default header should not be set")
}

func TestRequestIDUseExisting(t *testing.T) {
	t.Parallel()

	r := router.New[*router.Context]()

	requestIDMiddleware := middleware.RequestIDWithConfig[*router.Context](middleware.RequestIDConfig{
		UseExisting: true,
	})
	r.Use(requestIDMiddleware)

	var capturedID string
	r.Get("/test", func(ctx *router.Context) handler.Response {
		id, _ := middleware.GetRequestID(ctx)
		capturedID = id
		return func(w http.ResponseWriter, r *http.Request) error {
			w.WriteHeader(http.StatusOK)
			return nil
		}
	})

	t.Run("with existing header", func(t *testing.T) {
		existingID := "existing-request-id-123"
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.Header.Set("X-Request-ID", existingID)
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, existingID, capturedID, "Should use existing request ID")
		assert.Equal(t, existingID, w.Header().Get("X-Request-ID"))
	})

	t.Run("without existing header", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.NotEmpty(t, capturedID, "Should generate new ID when no existing header")
		assert.Equal(t, capturedID, w.Header().Get("X-Request-ID"))
	})
}

func TestRequestIDSkipFunctionality(t *testing.T) {
	t.Parallel()

	r := router.New[*router.Context]()

	requestIDMiddleware := middleware.RequestIDWithConfig[*router.Context](middleware.RequestIDConfig{
		Skip: func(ctx handler.Context) bool {
			// Skip middleware for health check endpoints
			return strings.HasPrefix(ctx.Request().URL.Path, "/health")
		},
	})
	r.Use(requestIDMiddleware)

	r.Get("/health", func(ctx *router.Context) handler.Response {
		id, ok := middleware.GetRequestID(ctx)
		assert.False(t, ok, "Request ID should not be present for skipped routes")
		assert.Empty(t, id)
		return func(w http.ResponseWriter, r *http.Request) error {
			w.WriteHeader(http.StatusOK)
			return nil
		}
	})

	r.Get("/api/test", func(ctx *router.Context) handler.Response {
		id, ok := middleware.GetRequestID(ctx)
		assert.True(t, ok, "Request ID should be present for non-skipped routes")
		assert.NotEmpty(t, id)
		return func(w http.ResponseWriter, r *http.Request) error {
			w.WriteHeader(http.StatusOK)
			return nil
		}
	})

	t.Run("skip health endpoint", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/health", nil)
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Empty(t, w.Header().Get("X-Request-ID"), "Request ID header should not be set for skipped routes")
	})

	t.Run("process api endpoint", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.NotEmpty(t, w.Header().Get("X-Request-ID"), "Request ID header should be set for non-skipped routes")
	})
}

func TestRequestIDMultipleRequests(t *testing.T) {
	t.Parallel()

	r := router.New[*router.Context]()

	requestIDMiddleware := middleware.RequestID[*router.Context]()
	r.Use(requestIDMiddleware)

	requestIDs := make([]string, 0, 3)
	r.Get("/test", func(ctx *router.Context) handler.Response {
		id, _ := middleware.GetRequestID(ctx)
		requestIDs = append(requestIDs, id)
		return func(w http.ResponseWriter, r *http.Request) error {
			w.WriteHeader(http.StatusOK)
			return nil
		}
	})

	// Make multiple requests
	for i := 0; i < 3; i++ {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
	}

	// Ensure all request IDs are unique
	require.Len(t, requestIDs, 3)
	assert.NotEqual(t, requestIDs[0], requestIDs[1], "Each request should have unique ID")
	assert.NotEqual(t, requestIDs[1], requestIDs[2], "Each request should have unique ID")
	assert.NotEqual(t, requestIDs[0], requestIDs[2], "Each request should have unique ID")
}

func TestRequestIDWithMultipleMiddleware(t *testing.T) {
	t.Parallel()

	r := router.New[*router.Context]()

	executionOrder := []string{}
	var requestIDInMiddleware2, requestIDInHandler string

	middleware1 := middleware.RequestID[*router.Context]()

	middleware2 := func(next handler.HandlerFunc[*router.Context]) handler.HandlerFunc[*router.Context] {
		return func(ctx *router.Context) handler.Response {
			executionOrder = append(executionOrder, "middleware2")
			id, ok := middleware.GetRequestID(ctx)
			assert.True(t, ok, "Request ID should be available in subsequent middleware")
			requestIDInMiddleware2 = id
			return next(ctx)
		}
	}

	r.Use(middleware1, middleware2)

	r.Get("/test", func(ctx *router.Context) handler.Response {
		executionOrder = append(executionOrder, "handler")
		id, _ := middleware.GetRequestID(ctx)
		requestIDInHandler = id
		return func(w http.ResponseWriter, r *http.Request) error {
			w.WriteHeader(http.StatusOK)
			return nil
		}
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, []string{"middleware2", "handler"}, executionOrder)
	assert.NotEmpty(t, requestIDInMiddleware2)
	assert.Equal(t, requestIDInMiddleware2, requestIDInHandler, "Request ID should be consistent across middleware and handler")
	assert.Equal(t, requestIDInHandler, w.Header().Get("X-Request-ID"))
}

func TestRequestIDEmptyExistingHeader(t *testing.T) {
	t.Parallel()

	r := router.New[*router.Context]()

	requestIDMiddleware := middleware.RequestIDWithConfig[*router.Context](middleware.RequestIDConfig{
		UseExisting: true,
	})
	r.Use(requestIDMiddleware)

	var capturedID string
	r.Get("/test", func(ctx *router.Context) handler.Response {
		id, _ := middleware.GetRequestID(ctx)
		capturedID = id
		return func(w http.ResponseWriter, r *http.Request) error {
			w.WriteHeader(http.StatusOK)
			return nil
		}
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-Request-ID", "") // Empty header value
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.NotEmpty(t, capturedID, "Should generate new ID for empty existing header")
	assert.Equal(t, capturedID, w.Header().Get("X-Request-ID"))
}

func TestRequestIDContextNotFound(t *testing.T) {
	t.Parallel()

	r := router.New[*router.Context]()

	// Handler without request ID middleware
	r.Get("/test", func(ctx *router.Context) handler.Response {
		id, ok := middleware.GetRequestID(ctx)
		assert.False(t, ok, "Request ID should not be found when middleware not used")
		assert.Empty(t, id, "ID should be empty when not found")
		return func(w http.ResponseWriter, r *http.Request) error {
			w.WriteHeader(http.StatusOK)
			return nil
		}
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Empty(t, w.Header().Get("X-Request-ID"), "Header should not be set without middleware")
}

func TestRequestIDIncrementing(t *testing.T) {
	t.Parallel()

	r := router.New[*router.Context]()

	counter := 0
	requestIDMiddleware := middleware.RequestIDWithConfig[*router.Context](middleware.RequestIDConfig{
		Generator: func() string {
			counter++
			return strings.Join([]string{"req", string(rune('0' + counter))}, "-")
		},
	})
	r.Use(requestIDMiddleware)

	r.Get("/test", func(ctx *router.Context) handler.Response {
		return func(w http.ResponseWriter, r *http.Request) error {
			w.WriteHeader(http.StatusOK)
			return nil
		}
	})

	// First request
	req1 := httptest.NewRequest(http.MethodGet, "/test", nil)
	w1 := httptest.NewRecorder()
	r.ServeHTTP(w1, req1)
	assert.Equal(t, "req-1", w1.Header().Get("X-Request-ID"))

	// Second request
	req2 := httptest.NewRequest(http.MethodGet, "/test", nil)
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)
	assert.Equal(t, "req-2", w2.Header().Get("X-Request-ID"))

	// Third request
	req3 := httptest.NewRequest(http.MethodGet, "/test", nil)
	w3 := httptest.NewRecorder()
	r.ServeHTTP(w3, req3)
	assert.Equal(t, "req-3", w3.Header().Get("X-Request-ID"))
}

func BenchmarkRequestIDDefault(b *testing.B) {
	r := router.New[*router.Context]()

	requestIDMiddleware := middleware.RequestID[*router.Context]()
	r.Use(requestIDMiddleware)

	r.Get("/test", func(ctx *router.Context) handler.Response {
		return func(w http.ResponseWriter, r *http.Request) error {
			w.WriteHeader(http.StatusOK)
			return nil
		}
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
	}
}

func BenchmarkRequestIDWithExisting(b *testing.B) {
	r := router.New[*router.Context]()

	requestIDMiddleware := middleware.RequestIDWithConfig[*router.Context](middleware.RequestIDConfig{
		UseExisting: true,
	})
	r.Use(requestIDMiddleware)

	r.Get("/test", func(ctx *router.Context) handler.Response {
		return func(w http.ResponseWriter, r *http.Request) error {
			w.WriteHeader(http.StatusOK)
			return nil
		}
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-Request-ID", "existing-id-123")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
	}
}
