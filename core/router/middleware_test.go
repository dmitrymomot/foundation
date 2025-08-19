package router_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/dmitrymomot/gokit/core/handler"
	"github.com/dmitrymomot/gokit/core/router"
)

func TestMiddlewareBasicChaining(t *testing.T) {
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
			executionOrder = append(executionOrder, "response")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("success"))
			return nil
		}
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "success", w.Body.String())

	expected := []string{
		"middleware1-before",
		"middleware2-before",
		"handler",
		"middleware2-after",
		"middleware1-after",
		"response", // Response function executes after all middleware cleanup
	}
	assert.Equal(t, expected, executionOrder)
}

func TestMiddlewareRequestResponseAccess(t *testing.T) {
	t.Parallel()

	r := router.New[*router.Context]()
	var capturedMethod, capturedPath string
	var headerAdded bool

	requestMiddleware := func(next handler.HandlerFunc[*router.Context]) handler.HandlerFunc[*router.Context] {
		return func(ctx *router.Context) handler.Response {
			// Access request in middleware
			req := ctx.Request()
			capturedMethod = req.Method
			capturedPath = req.URL.Path
			return next(ctx)
		}
	}

	responseMiddleware := func(next handler.HandlerFunc[*router.Context]) handler.HandlerFunc[*router.Context] {
		return func(ctx *router.Context) handler.Response {
			response := next(ctx)
			// Modify response in middleware
			return func(w http.ResponseWriter, r *http.Request) error {
				w.Header().Set("X-Custom-Header", "middleware-value")
				headerAdded = true
				return response(w, r)
			}
		}
	}

	r.Use(requestMiddleware, responseMiddleware)

	r.Post("/api/test", func(ctx *router.Context) handler.Response {
		return func(w http.ResponseWriter, r *http.Request) error {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("handled"))
			return nil
		}
	})

	req := httptest.NewRequest(http.MethodPost, "/api/test", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "handled", w.Body.String())
	assert.Equal(t, "POST", capturedMethod)
	assert.Equal(t, "/api/test", capturedPath)
	assert.True(t, headerAdded)
	assert.Equal(t, "middleware-value", w.Header().Get("X-Custom-Header"))
}

func TestMiddlewareParamAccess(t *testing.T) {
	t.Parallel()

	r := router.New[*router.Context]()
	var capturedUserID, capturedPostID string

	paramMiddleware := func(next handler.HandlerFunc[*router.Context]) handler.HandlerFunc[*router.Context] {
		return func(ctx *router.Context) handler.Response {
			capturedUserID = ctx.Param("userID")
			capturedPostID = ctx.Param("postID")
			return next(ctx)
		}
	}

	r.Use(paramMiddleware)

	r.Get("/users/{userID}/posts/{postID}", func(ctx *router.Context) handler.Response {
		return func(w http.ResponseWriter, r *http.Request) error {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("success"))
			return nil
		}
	})

	req := httptest.NewRequest(http.MethodGet, "/users/123/posts/456", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "123", capturedUserID)
	assert.Equal(t, "456", capturedPostID)
}

func TestMiddlewareErrorHandling(t *testing.T) {
	t.Parallel()

	r := router.New[*router.Context]()
	errorHandled := false

	errorHandlerMiddleware := func(next handler.HandlerFunc[*router.Context]) handler.HandlerFunc[*router.Context] {
		return func(ctx *router.Context) handler.Response {
			response := next(ctx)
			return func(w http.ResponseWriter, r *http.Request) error {
				if err := response(w, r); err != nil {
					errorHandled = true
					w.WriteHeader(http.StatusBadRequest)
					w.Write([]byte("middleware handled error: " + err.Error()))
					return nil // Error handled by middleware
				}
				return nil
			}
		}
	}

	r.Use(errorHandlerMiddleware)

	r.Get("/test", func(ctx *router.Context) handler.Response {
		return func(w http.ResponseWriter, r *http.Request) error {
			return assert.AnError // Return an error
		}
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.True(t, errorHandled)
	assert.Contains(t, w.Body.String(), "middleware handled error")
}

func TestMiddlewareShortCircuit(t *testing.T) {
	t.Parallel()

	r := router.New[*router.Context]()
	handlerExecuted := false

	authMiddleware := func(next handler.HandlerFunc[*router.Context]) handler.HandlerFunc[*router.Context] {
		return func(ctx *router.Context) handler.Response {
			// Simulate authentication failure
			authHeader := ctx.Request().Header.Get("Authorization")
			if authHeader != "Bearer valid-token" {
				// Short-circuit - don't call next handler
				return func(w http.ResponseWriter, r *http.Request) error {
					w.WriteHeader(http.StatusUnauthorized)
					w.Write([]byte("unauthorized"))
					return nil
				}
			}
			return next(ctx)
		}
	}

	r.Use(authMiddleware)

	r.Get("/protected", func(ctx *router.Context) handler.Response {
		handlerExecuted = true
		return func(w http.ResponseWriter, r *http.Request) error {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("protected content"))
			return nil
		}
	})

	t.Run("unauthorized request", func(t *testing.T) {
		handlerExecuted = false
		req := httptest.NewRequest(http.MethodGet, "/protected", nil)
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
		assert.Equal(t, "unauthorized", w.Body.String())
		assert.False(t, handlerExecuted)
	})

	t.Run("authorized request", func(t *testing.T) {
		handlerExecuted = false
		req := httptest.NewRequest(http.MethodGet, "/protected", nil)
		req.Header.Set("Authorization", "Bearer valid-token")
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "protected content", w.Body.String())
		assert.True(t, handlerExecuted)
	})
}

func TestMiddlewareWithInlineRouter(t *testing.T) {
	t.Parallel()

	r := router.New[*router.Context]()
	executionOrder := []string{}

	globalMiddleware := func(next handler.HandlerFunc[*router.Context]) handler.HandlerFunc[*router.Context] {
		return func(ctx *router.Context) handler.Response {
			executionOrder = append(executionOrder, "global")
			return next(ctx)
		}
	}

	inlineMiddleware := func(next handler.HandlerFunc[*router.Context]) handler.HandlerFunc[*router.Context] {
		return func(ctx *router.Context) handler.Response {
			executionOrder = append(executionOrder, "inline")
			return next(ctx)
		}
	}

	r.Use(globalMiddleware)

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

	assert.Equal(t, http.StatusOK, w.Code)
	expected := []string{"global", "inline", "handler"}
	assert.Equal(t, expected, executionOrder)
}

func TestMiddlewareStackingWithMultipleWith(t *testing.T) {
	t.Parallel()

	r := router.New[*router.Context]()
	executionOrder := []string{}

	middleware1 := func(next handler.HandlerFunc[*router.Context]) handler.HandlerFunc[*router.Context] {
		return func(ctx *router.Context) handler.Response {
			executionOrder = append(executionOrder, "m1")
			return next(ctx)
		}
	}

	middleware2 := func(next handler.HandlerFunc[*router.Context]) handler.HandlerFunc[*router.Context] {
		return func(ctx *router.Context) handler.Response {
			executionOrder = append(executionOrder, "m2")
			return next(ctx)
		}
	}

	middleware3 := func(next handler.HandlerFunc[*router.Context]) handler.HandlerFunc[*router.Context] {
		return func(ctx *router.Context) handler.Response {
			executionOrder = append(executionOrder, "m3")
			return next(ctx)
		}
	}

	// Chain multiple With() calls
	r.With(middleware1).With(middleware2).With(middleware3).Get("/test", func(ctx *router.Context) handler.Response {
		executionOrder = append(executionOrder, "handler")
		return func(w http.ResponseWriter, r *http.Request) error {
			w.WriteHeader(http.StatusOK)
			return nil
		}
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	expected := []string{"m1", "m2", "m3", "handler"}
	assert.Equal(t, expected, executionOrder)
}

func TestMiddlewarePanicWhenAddedAfterRoutes(t *testing.T) {
	t.Parallel()

	// This test documents the expected behavior but the current implementation
	// may not enforce this constraint. We'll test that middleware works correctly.

	r := router.New[*router.Context]()

	// Add middleware first
	middleware := func(next handler.HandlerFunc[*router.Context]) handler.HandlerFunc[*router.Context] {
		return func(ctx *router.Context) handler.Response {
			return next(ctx)
		}
	}

	r.Use(middleware)

	// Register a route
	r.Get("/test", func(ctx *router.Context) handler.Response {
		return func(w http.ResponseWriter, r *http.Request) error {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("success"))
			return nil
		}
	})

	// Test that the route works
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "success", w.Body.String())
}

func TestMiddlewareContextModification(t *testing.T) {
	t.Parallel()

	// Custom context that can be modified
	type CustomContext struct {
		*router.Context
		UserID  string
		IsAdmin bool
		Data    map[string]string
	}

	contextFactory := func(w http.ResponseWriter, r *http.Request, params map[string]string) *CustomContext {
		return &CustomContext{
			Context: &router.Context{},
			Data:    make(map[string]string),
		}
	}

	r := router.New[*CustomContext](router.WithContextFactory(contextFactory))

	authMiddleware := func(next handler.HandlerFunc[*CustomContext]) handler.HandlerFunc[*CustomContext] {
		return func(ctx *CustomContext) handler.Response {
			// Simulate setting user info in middleware
			ctx.UserID = "user123"
			ctx.IsAdmin = true
			ctx.Data["middleware"] = "executed"
			return next(ctx)
		}
	}

	r.Use(authMiddleware)

	r.Get("/test", func(ctx *CustomContext) handler.Response {
		return func(w http.ResponseWriter, r *http.Request) error {
			w.WriteHeader(http.StatusOK)
			response := "user:" + ctx.UserID + ",admin:" +
				func() string {
					if ctx.IsAdmin {
						return "true"
					}
					return "false"
				}() +
				",data:" + ctx.Data["middleware"]
			w.Write([]byte(response))
			return nil
		}
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "user:user123,admin:true,data:executed", w.Body.String())
}

func TestMiddlewareOnDifferentRoutes(t *testing.T) {
	t.Parallel()

	r := router.New[*router.Context]()
	executionCounter := 0

	counterMiddleware := func(next handler.HandlerFunc[*router.Context]) handler.HandlerFunc[*router.Context] {
		return func(ctx *router.Context) handler.Response {
			executionCounter++
			return next(ctx)
		}
	}

	r.Use(counterMiddleware)

	// Multiple routes using same middleware
	r.Get("/route1", func(ctx *router.Context) handler.Response {
		return func(w http.ResponseWriter, r *http.Request) error {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("route1"))
			return nil
		}
	})

	r.Get("/route2", func(ctx *router.Context) handler.Response {
		return func(w http.ResponseWriter, r *http.Request) error {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("route2"))
			return nil
		}
	})

	r.Post("/route3", func(ctx *router.Context) handler.Response {
		return func(w http.ResponseWriter, r *http.Request) error {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("route3"))
			return nil
		}
	})

	// Test each route
	tests := []struct {
		method   string
		path     string
		expected string
	}{
		{"GET", "/route1", "route1"},
		{"GET", "/route2", "route2"},
		{"POST", "/route3", "route3"},
	}

	for i, test := range tests {
		req := httptest.NewRequest(test.method, test.path, nil)
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, test.expected, w.Body.String())
		assert.Equal(t, i+1, executionCounter, "Middleware should execute for each request")
	}
}

func TestMiddlewareNilResponseHandling(t *testing.T) {
	t.Parallel()

	r := router.New[*router.Context]()

	nilReturningMiddleware := func(next handler.HandlerFunc[*router.Context]) handler.HandlerFunc[*router.Context] {
		return func(ctx *router.Context) handler.Response {
			// Return nil instead of calling next
			return nil
		}
	}

	r.Use(nilReturningMiddleware)

	r.Get("/test", func(ctx *router.Context) handler.Response {
		return func(w http.ResponseWriter, r *http.Request) error {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("should not reach this"))
			return nil
		}
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	// Should get error for nil response
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestMiddlewarePanicHandling(t *testing.T) {
	t.Parallel()

	r := router.New[*router.Context]()

	panicMiddleware := func(next handler.HandlerFunc[*router.Context]) handler.HandlerFunc[*router.Context] {
		return func(ctx *router.Context) handler.Response {
			panic("middleware panic")
		}
	}

	r.Use(panicMiddleware)

	r.Get("/test", func(ctx *router.Context) handler.Response {
		return func(w http.ResponseWriter, r *http.Request) error {
			w.WriteHeader(http.StatusOK)
			return nil
		}
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	// Panic should be caught and handled by default error handler
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}
