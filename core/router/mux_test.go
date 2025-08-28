package router_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/dmitrymomot/foundation/core/handler"
	"github.com/dmitrymomot/foundation/core/router"
)

// testCustomContext is a custom context type for testing
type testCustomContext struct {
	w           http.ResponseWriter
	r           *http.Request
	params      map[string]string
	CustomField string
}

// Implement handler.Context interface
func (c *testCustomContext) Deadline() (deadline time.Time, ok bool) {
	return c.r.Context().Deadline()
}
func (c *testCustomContext) Done() <-chan struct{} {
	return c.r.Context().Done()
}
func (c *testCustomContext) Err() error {
	return c.r.Context().Err()
}
func (c *testCustomContext) Value(key any) any {
	return c.r.Context().Value(key)
}
func (c *testCustomContext) Request() *http.Request {
	return c.r
}
func (c *testCustomContext) ResponseWriter() http.ResponseWriter {
	return c.w
}
func (c *testCustomContext) Param(key string) string {
	if c.params != nil {
		return c.params[key]
	}
	return ""
}
func (c *testCustomContext) SetValue(key, val any) {
	ctx := context.WithValue(c.r.Context(), key, val)
	c.r = c.r.WithContext(ctx)
}

func TestMuxServeHTTP(t *testing.T) {
	t.Parallel()

	t.Run("successful request handling", func(t *testing.T) {
		t.Parallel()

		r := router.New[*router.Context]()
		responseBody := "Hello World"

		r.Get("/test", func(ctx *router.Context) handler.Response {
			return func(w http.ResponseWriter, r *http.Request) error {
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(responseBody))
				return nil
			}
		})

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, responseBody, w.Body.String())
	})

	t.Run("handles empty path as root", func(t *testing.T) {
		t.Parallel()

		r := router.New[*router.Context]()
		executed := false

		r.Get("/", func(ctx *router.Context) handler.Response {
			executed = true
			return func(w http.ResponseWriter, r *http.Request) error {
				w.WriteHeader(http.StatusOK)
				return nil
			}
		})

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		assert.True(t, executed)
		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("handles invalid HTTP method", func(t *testing.T) {
		t.Parallel()

		r := router.New[*router.Context]()
		req := httptest.NewRequest("INVALID", "/test", nil)
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusInternalServerError, w.Code) // Default error handler
	})

	t.Run("handles response write error", func(t *testing.T) {
		t.Parallel()

		r := router.New[*router.Context]()

		r.Get("/test", func(ctx *router.Context) handler.Response {
			return func(w http.ResponseWriter, r *http.Request) error {
				return errors.New("response write error")
			}
		})

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})

	t.Run("handles nil response", func(t *testing.T) {
		t.Parallel()

		r := router.New[*router.Context]()

		r.Get("/test", func(ctx *router.Context) handler.Response {
			return nil // Return nil response
		})

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})

	t.Run("handles panic in handler", func(t *testing.T) {
		t.Parallel()

		r := router.New[*router.Context]()

		r.Get("/test", func(ctx *router.Context) handler.Response {
			panic("test panic")
		})

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})

	t.Run("handles panic in response function", func(t *testing.T) {
		t.Parallel()

		r := router.New[*router.Context]()

		r.Get("/test", func(ctx *router.Context) handler.Response {
			return func(w http.ResponseWriter, r *http.Request) error {
				panic("response panic")
			}
		})

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})
}

func TestMuxWithCustomErrorHandler(t *testing.T) {
	t.Parallel()

	errorHandlerCalled := false
	var capturedError error

	customErrorHandler := func(ctx *router.Context, err error) {
		errorHandlerCalled = true
		capturedError = err
		w := ctx.ResponseWriter()
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("Custom error: " + err.Error()))
	}

	r := router.New[*router.Context](router.WithErrorHandler(customErrorHandler))

	r.Get("/test", func(ctx *router.Context) handler.Response {
		return func(w http.ResponseWriter, r *http.Request) error {
			return errors.New("test error")
		}
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.True(t, errorHandlerCalled)
	assert.Equal(t, "test error", capturedError.Error())
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Equal(t, "Custom error: test error", w.Body.String())
}

func TestMuxMethodNotAllowedAllowHeader(t *testing.T) {
	t.Parallel()

	r := router.New[*router.Context]()

	// Register GET and POST handlers
	r.Get("/test", func(ctx *router.Context) handler.Response {
		return func(w http.ResponseWriter, r *http.Request) error {
			w.WriteHeader(http.StatusOK)
			return nil
		}
	})

	r.Post("/test", func(ctx *router.Context) handler.Response {
		return func(w http.ResponseWriter, r *http.Request) error {
			w.WriteHeader(http.StatusOK)
			return nil
		}
	})

	// Test PUT request should set Allow header with GET and POST
	req := httptest.NewRequest(http.MethodPut, "/test", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	allowHeader := w.Header().Get("Allow")
	assert.Contains(t, allowHeader, "GET")
	assert.Contains(t, allowHeader, "POST")
}

func TestMuxUseMiddleware(t *testing.T) {
	t.Parallel()

	t.Run("adds middleware to router", func(t *testing.T) {
		t.Parallel()

		r := router.New[*router.Context]()
		executionOrder := []string{}

		middleware1 := func(next handler.HandlerFunc[*router.Context]) handler.HandlerFunc[*router.Context] {
			return func(ctx *router.Context) handler.Response {
				executionOrder = append(executionOrder, "middleware1-before")
				response := next(ctx)
				executionOrder = append(executionOrder, "middleware1-after")
				return response
			}
		}

		middleware2 := func(next handler.HandlerFunc[*router.Context]) handler.HandlerFunc[*router.Context] {
			return func(ctx *router.Context) handler.Response {
				executionOrder = append(executionOrder, "middleware2-before")
				response := next(ctx)
				executionOrder = append(executionOrder, "middleware2-after")
				return response
			}
		}

		r.Use(middleware1, middleware2)

		r.Get("/test", func(ctx *router.Context) handler.Response {
			executionOrder = append(executionOrder, "handler")
			return func(w http.ResponseWriter, r *http.Request) error {
				w.WriteHeader(http.StatusOK)
				return nil
			}
		})

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		expected := []string{
			"middleware1-before",
			"middleware2-before",
			"handler",
			"middleware2-after",
			"middleware1-after",
		}
		assert.Equal(t, expected, executionOrder)
	})

	t.Run("panics when middleware added after routes", func(t *testing.T) {
		t.Parallel()

		r := router.New[*router.Context]()

		// Register a route first
		r.Get("/test", func(ctx *router.Context) handler.Response { return nil })

		// Adding middleware after route should panic
		assert.Panics(t, func() {
			r.Use(func(next handler.HandlerFunc[*router.Context]) handler.HandlerFunc[*router.Context] {
				return next
			})
		})
	})
}

func TestMuxWithInlineRouter(t *testing.T) {
	t.Parallel()

	r := router.New[*router.Context]()
	executionOrder := []string{}

	baseMiddleware := func(next handler.HandlerFunc[*router.Context]) handler.HandlerFunc[*router.Context] {
		return func(ctx *router.Context) handler.Response {
			executionOrder = append(executionOrder, "base-middleware")
			return next(ctx)
		}
	}

	inlineMiddleware := func(next handler.HandlerFunc[*router.Context]) handler.HandlerFunc[*router.Context] {
		return func(ctx *router.Context) handler.Response {
			executionOrder = append(executionOrder, "inline-middleware")
			return next(ctx)
		}
	}

	r.Use(baseMiddleware)

	// Create inline router with additional middleware
	inlineRouter := r.With(inlineMiddleware)

	inlineRouter.Get("/test", func(ctx *router.Context) handler.Response {
		executionOrder = append(executionOrder, "handler")
		return func(w http.ResponseWriter, r *http.Request) error {
			w.WriteHeader(http.StatusOK)
			return nil
		}
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	expected := []string{
		"base-middleware",
		"inline-middleware",
		"handler",
	}
	assert.Equal(t, expected, executionOrder)
}

func TestMuxWithCustomContextFactory(t *testing.T) {
	t.Parallel()

	factoryCalled := false
	contextFactory := func(w http.ResponseWriter, r *http.Request, params map[string]string) *testCustomContext {
		factoryCalled = true
		ctx := &testCustomContext{
			w:           w,
			r:           r,
			params:      params,
			CustomField: "custom_value",
		}
		return ctx
	}

	r := router.New[*testCustomContext](router.WithContextFactory(contextFactory))

	var contextReceived *testCustomContext
	r.Get("/test", func(ctx *testCustomContext) handler.Response {
		contextReceived = ctx
		return func(w http.ResponseWriter, r *http.Request) error {
			w.WriteHeader(http.StatusOK)
			return nil
		}
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.True(t, factoryCalled)
	assert.NotNil(t, contextReceived)
	assert.Equal(t, "custom_value", contextReceived.CustomField)
}

func TestMuxPanicWhenNoContextFactory(t *testing.T) {
	t.Parallel()

	// Custom context type without factory
	type UnsupportedContext struct {
		*router.Context
	}

	// This should work with default context auto-detection
	assert.NotPanics(t, func() {
		r := router.New[*router.Context]()
		r.Get("/test", func(ctx *router.Context) handler.Response { return nil })

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
	})

	// This should panic when trying to create unsupported context type
	assert.Panics(t, func() {
		r := router.New[*UnsupportedContext]()
		r.Get("/test", func(ctx *UnsupportedContext) handler.Response { return nil })

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
	})
}
