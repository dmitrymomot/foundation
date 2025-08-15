package gokit_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/dmitrymomot/gokit"
	"github.com/stretchr/testify/assert"
)

func TestWithMiddleware(t *testing.T) {
	t.Parallel()

	t.Run("middleware_is_applied", func(t *testing.T) {
		t.Parallel()

		middlewareCalled := false
		middleware := func(next gokit.HandlerFunc[*gokit.Context]) gokit.HandlerFunc[*gokit.Context] {
			return func(ctx *gokit.Context) gokit.Response {
				middlewareCalled = true
				return next(ctx)
			}
		}

		router := gokit.NewRouter[*gokit.Context](
			gokit.WithMiddleware(middleware),
		)

		router.Get("/test", func(ctx *gokit.Context) gokit.Response {
			return gokit.String("test response")
		})

		req := httptest.NewRequest("GET", "/test", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.True(t, middlewareCalled, "Middleware should have been called")
		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "test response", w.Body.String())
	})

	t.Run("multiple_middleware_order", func(t *testing.T) {
		t.Parallel()

		var order []string

		middleware1 := func(next gokit.HandlerFunc[*gokit.Context]) gokit.HandlerFunc[*gokit.Context] {
			return func(ctx *gokit.Context) gokit.Response {
				order = append(order, "m1-before")
				resp := next(ctx)
				order = append(order, "m1-after")
				return resp
			}
		}

		middleware2 := func(next gokit.HandlerFunc[*gokit.Context]) gokit.HandlerFunc[*gokit.Context] {
			return func(ctx *gokit.Context) gokit.Response {
				order = append(order, "m2-before")
				resp := next(ctx)
				order = append(order, "m2-after")
				return resp
			}
		}

		router := gokit.NewRouter[*gokit.Context](
			gokit.WithMiddleware(middleware1, middleware2),
		)

		router.Get("/test", func(ctx *gokit.Context) gokit.Response {
			order = append(order, "handler")
			return gokit.String("test")
		})

		req := httptest.NewRequest("GET", "/test", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		expectedOrder := []string{
			"m1-before",
			"m2-before",
			"handler",
			"m2-after",
			"m1-after",
		}
		assert.Equal(t, expectedOrder, order)
	})

	t.Run("middleware_with_empty_list", func(t *testing.T) {
		t.Parallel()

		// Should not panic with empty middleware list
		router := gokit.NewRouter[*gokit.Context](
			gokit.WithMiddleware[*gokit.Context](),
		)

		router.Get("/test", func(ctx *gokit.Context) gokit.Response {
			return gokit.String("test")
		})

		req := httptest.NewRequest("GET", "/test", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "test", w.Body.String())
	})
}
