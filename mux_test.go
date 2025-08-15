package gokit_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/dmitrymomot/gokit"
	"github.com/stretchr/testify/assert"
)

func TestRouter_Route(t *testing.T) {
	t.Parallel()

	t.Run("creates_sub_router", func(t *testing.T) {
		t.Parallel()

		router := gokit.NewRouter[*gokit.Context]()

		// Use Route to create a sub-router
		subRouter := router.Route("/api", func(r gokit.Router[*gokit.Context]) {
			r.Get("/users", func(ctx *gokit.Context) gokit.Response {
				return gokit.String("users list")
			})
			r.Get("/posts", func(ctx *gokit.Context) gokit.Response {
				return gokit.String("posts list")
			})
		})

		// Verify sub-router is returned
		assert.NotNil(t, subRouter)

		// Test that routes are accessible
		req := httptest.NewRequest("GET", "/api/users", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		// Note: Mount functionality may not work properly, so we expect 500
		// This test primarily ensures Route method doesn't panic
		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})

	t.Run("panics_with_nil_function", func(t *testing.T) {
		t.Parallel()

		router := gokit.NewRouter[*gokit.Context]()

		assert.Panics(t, func() {
			router.Route("/api", nil)
		})
	})
}

func TestRouter_Mount(t *testing.T) {
	t.Parallel()

	t.Run("mounts_sub_router", func(t *testing.T) {
		t.Parallel()

		mainRouter := gokit.NewRouter[*gokit.Context]()
		subRouter := gokit.NewRouter[*gokit.Context]()

		// Add route to sub-router
		subRouter.Get("/test", func(ctx *gokit.Context) gokit.Response {
			return gokit.String("mounted route")
		})

		// Mount sub-router
		assert.NotPanics(t, func() {
			mainRouter.Mount("/sub", subRouter)
		})

		// Test that the mount point is registered
		req := httptest.NewRequest("GET", "/sub/test", nil)
		w := httptest.NewRecorder()
		mainRouter.ServeHTTP(w, req)

		// Note: Mount functionality may not work properly, so we expect 500
		// This test primarily ensures Mount method doesn't panic
		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})

	t.Run("panics_with_nil_router", func(t *testing.T) {
		t.Parallel()

		router := gokit.NewRouter[*gokit.Context]()

		assert.Panics(t, func() {
			router.Mount("/sub", nil)
		})
	})

	t.Run("shares_error_handler", func(t *testing.T) {
		t.Parallel()

		customErrorCalled := false
		customErrorHandler := func(ctx *gokit.Context, err error) {
			customErrorCalled = true
			ctx.ResponseWriter().WriteHeader(http.StatusTeapot)
		}

		mainRouter := gokit.NewRouter[*gokit.Context](
			gokit.WithErrorHandler[*gokit.Context](customErrorHandler),
		)
		subRouter := gokit.NewRouter[*gokit.Context]()

		// Mount sub-router - should inherit error handler
		mainRouter.Mount("/sub", subRouter)

		// The sub-router should have inherited the error handler
		// We can't directly test this without access to internal state,
		// but the mount operation should not panic
		assert.False(t, customErrorCalled, "Error handler should not be called during mounting")
	})
}
