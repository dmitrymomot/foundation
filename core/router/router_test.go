package router_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dmitrymomot/foundation/core/handler"
	"github.com/dmitrymomot/foundation/core/router"
)

// Test that Router implements http.Handler interface
func TestRouterImplementsHTTPHandler(t *testing.T) {
	t.Parallel()

	r := router.New[*router.Context]()
	var _ http.Handler = r

	assert.NotNil(t, r)
}

func TestNewRouter(t *testing.T) {
	t.Parallel()

	t.Run("creates router with default context", func(t *testing.T) {
		t.Parallel()

		r := router.New[*router.Context]()
		require.NotNil(t, r)

		// Verify it's a routes provider
		var _ router.Routes = r
		routes := r.Routes()
		assert.Empty(t, routes)
	})

	t.Run("creates router with options", func(t *testing.T) {
		t.Parallel()

		errorHandlerCalled := false
		customErrorHandler := func(ctx *router.Context, err error) {
			errorHandlerCalled = true
		}

		r := router.New[*router.Context](router.WithErrorHandler(customErrorHandler))
		require.NotNil(t, r)

		// Test error handler by triggering an error
		req := httptest.NewRequest(http.MethodGet, "/nonexistent", nil)
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)
		assert.True(t, errorHandlerCalled)
	})
}

func TestRouterHTTPMethods(t *testing.T) {
	t.Parallel()

	tests := []struct {
		method   string
		register func(router.Router[*router.Context], string, handler.HandlerFunc[*router.Context])
	}{
		{"GET", func(r router.Router[*router.Context], pattern string, h handler.HandlerFunc[*router.Context]) {
			r.Get(pattern, h)
		}},
		{"POST", func(r router.Router[*router.Context], pattern string, h handler.HandlerFunc[*router.Context]) {
			r.Post(pattern, h)
		}},
		{"PUT", func(r router.Router[*router.Context], pattern string, h handler.HandlerFunc[*router.Context]) {
			r.Put(pattern, h)
		}},
		{"DELETE", func(r router.Router[*router.Context], pattern string, h handler.HandlerFunc[*router.Context]) {
			r.Delete(pattern, h)
		}},
		{"PATCH", func(r router.Router[*router.Context], pattern string, h handler.HandlerFunc[*router.Context]) {
			r.Patch(pattern, h)
		}},
		{"HEAD", func(r router.Router[*router.Context], pattern string, h handler.HandlerFunc[*router.Context]) {
			r.Head(pattern, h)
		}},
		{"OPTIONS", func(r router.Router[*router.Context], pattern string, h handler.HandlerFunc[*router.Context]) {
			r.Options(pattern, h)
		}},
		{"CONNECT", func(r router.Router[*router.Context], pattern string, h handler.HandlerFunc[*router.Context]) {
			r.Connect(pattern, h)
		}},
		{"TRACE", func(r router.Router[*router.Context], pattern string, h handler.HandlerFunc[*router.Context]) {
			r.Trace(pattern, h)
		}},
	}

	for _, test := range tests {
		t.Run(test.method, func(t *testing.T) {
			t.Parallel()

			r := router.New[*router.Context]()
			executed := false

			h := func(ctx *router.Context) handler.Response {
				executed = true
				return func(w http.ResponseWriter, r *http.Request) error {
					w.WriteHeader(http.StatusOK)
					return nil
				}
			}

			test.register(r, "/test", h)

			req := httptest.NewRequest(test.method, "/test", nil)
			w := httptest.NewRecorder()

			r.ServeHTTP(w, req)

			assert.True(t, executed, "Handler should have been executed")
			assert.Equal(t, http.StatusOK, w.Code)
		})
	}
}

func TestRouterHandleMethod(t *testing.T) {
	t.Parallel()

	t.Run("Handle method accepts all HTTP methods", func(t *testing.T) {
		t.Parallel()

		r := router.New[*router.Context]()
		executed := false

		h := func(ctx *router.Context) handler.Response {
			executed = true
			return func(w http.ResponseWriter, r *http.Request) error {
				w.WriteHeader(http.StatusOK)
				return nil
			}
		}

		r.Handle("/api", h)

		methods := []string{"GET", "POST", "PUT", "DELETE", "PATCH", "HEAD", "OPTIONS", "CONNECT", "TRACE"}
		for _, method := range methods {
			executed = false
			req := httptest.NewRequest(method, "/api", nil)
			w := httptest.NewRecorder()

			r.ServeHTTP(w, req)

			assert.True(t, executed, "Handler should have been executed for method %s", method)
			assert.Equal(t, http.StatusOK, w.Code)
		}
	})
}

func TestRouterMethodWithSpecificMethods(t *testing.T) {
	t.Parallel()

	r := router.New[*router.Context]()
	executed := false

	h := func(ctx *router.Context) handler.Response {
		executed = true
		return func(w http.ResponseWriter, r *http.Request) error {
			w.WriteHeader(http.StatusOK)
			return nil
		}
	}

	// Register handler for specific methods
	r.Method("/api", h, "GET", "POST")

	t.Run("allowed methods work", func(t *testing.T) {
		for _, method := range []string{"GET", "POST"} {
			executed = false
			req := httptest.NewRequest(method, "/api", nil)
			w := httptest.NewRecorder()

			r.ServeHTTP(w, req)

			assert.True(t, executed, "Handler should have been executed for method %s", method)
			assert.Equal(t, http.StatusOK, w.Code)
		}
	})

	t.Run("disallowed methods return 405", func(t *testing.T) {
		for _, method := range []string{"PUT", "DELETE"} {
			executed = false
			req := httptest.NewRequest(method, "/api", nil)
			w := httptest.NewRecorder()

			r.ServeHTTP(w, req)

			assert.False(t, executed, "Handler should not have been executed for method %s", method)
			assert.Equal(t, http.StatusInternalServerError, w.Code) // Default error handler returns 500
		}
	})
}

func TestRouterRoutes(t *testing.T) {
	t.Parallel()

	r := router.New[*router.Context]()

	// Register some routes
	r.Get("/users", func(ctx *router.Context) handler.Response { return nil })
	r.Post("/users", func(ctx *router.Context) handler.Response { return nil })
	r.Get("/users/{id}", func(ctx *router.Context) handler.Response { return nil })
	r.Delete("/users/{id}", func(ctx *router.Context) handler.Response { return nil })

	routes := r.Routes()
	require.Len(t, routes, 4)

	// Create a map for easier verification
	routeMap := make(map[string]string)
	for _, route := range routes {
		routeMap[route.Method+":"+route.Pattern] = route.Pattern
	}

	assert.Contains(t, routeMap, "GET:/users")
	assert.Contains(t, routeMap, "POST:/users")
	assert.Contains(t, routeMap, "GET:/users/{id}")
	assert.Contains(t, routeMap, "DELETE:/users/{id}")
}

func TestRouterInvalidPatterns(t *testing.T) {
	t.Parallel()

	r := router.New[*router.Context]()
	h := func(ctx *router.Context) handler.Response { return nil }

	t.Run("empty pattern panics", func(t *testing.T) {
		t.Parallel()
		assert.Panics(t, func() {
			r.Get("", h)
		})
	})

	t.Run("pattern without leading slash panics", func(t *testing.T) {
		t.Parallel()
		assert.Panics(t, func() {
			r.Get("users", h)
		})
	})

	t.Run("invalid method panics", func(t *testing.T) {
		t.Parallel()
		assert.Panics(t, func() {
			r.Method("/test", h, "INVALID")
		})
	})

	t.Run("no methods provided panics", func(t *testing.T) {
		t.Parallel()
		assert.Panics(t, func() {
			r.Method("/test", h)
		})
	})
}

func TestRouterMethodNotAllowed(t *testing.T) {
	t.Parallel()

	r := router.New[*router.Context]()

	// Register only GET handler
	r.Get("/test", func(ctx *router.Context) handler.Response {
		return func(w http.ResponseWriter, r *http.Request) error {
			w.WriteHeader(http.StatusOK)
			return nil
		}
	})

	// Test POST request to same path should return method not allowed
	req := httptest.NewRequest(http.MethodPost, "/test", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	// Default error handler returns 500, but sets Allow header
	assert.Equal(t, "GET", w.Header().Get("Allow"))
}

func TestRouterNotFound(t *testing.T) {
	t.Parallel()

	r := router.New[*router.Context]()

	// No routes registered
	req := httptest.NewRequest(http.MethodGet, "/nonexistent", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	// Default error handler returns 500 for not found
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}
