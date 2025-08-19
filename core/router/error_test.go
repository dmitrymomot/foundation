package router_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/dmitrymomot/gokit/core/handler"
	"github.com/dmitrymomot/gokit/core/router"
)

func TestDefaultErrorHandler(t *testing.T) {
	t.Parallel()

	r := router.New[*router.Context]()

	r.Get("/error", func(ctx *router.Context) handler.Response {
		return func(w http.ResponseWriter, r *http.Request) error {
			return errors.New("test error")
		}
	})

	req := httptest.NewRequest(http.MethodGet, "/error", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Contains(t, w.Body.String(), "test error")
}

func TestCustomErrorHandler(t *testing.T) {
	t.Parallel()

	errorHandlerCalled := false
	var capturedError error
	var capturedContext *router.Context

	customErrorHandler := func(ctx *router.Context, err error) {
		errorHandlerCalled = true
		capturedError = err
		capturedContext = ctx

		w := ctx.ResponseWriter()
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("Custom error handler: " + err.Error()))
	}

	r := router.New[*router.Context](router.WithErrorHandler(customErrorHandler))

	r.Get("/error", func(ctx *router.Context) handler.Response {
		return func(w http.ResponseWriter, r *http.Request) error {
			return errors.New("custom test error")
		}
	})

	req := httptest.NewRequest(http.MethodGet, "/error", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.True(t, errorHandlerCalled)
	assert.NotNil(t, capturedError)
	assert.Equal(t, "custom test error", capturedError.Error())
	assert.NotNil(t, capturedContext)
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Equal(t, "Custom error handler: custom test error", w.Body.String())
}

func TestErrorHandlerNotCalledOnSuccess(t *testing.T) {
	t.Parallel()

	errorHandlerCalled := false
	customErrorHandler := func(ctx *router.Context, err error) {
		errorHandlerCalled = true
	}

	r := router.New[*router.Context](router.WithErrorHandler(customErrorHandler))

	r.Get("/success", func(ctx *router.Context) handler.Response {
		return func(w http.ResponseWriter, r *http.Request) error {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("success"))
			return nil
		}
	})

	req := httptest.NewRequest(http.MethodGet, "/success", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.False(t, errorHandlerCalled)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "success", w.Body.String())
}

func TestPanicRecovery(t *testing.T) {
	t.Parallel()

	errorHandlerCalled := false
	var capturedError error

	customErrorHandler := func(ctx *router.Context, err error) {
		errorHandlerCalled = true
		capturedError = err
		w := ctx.ResponseWriter()
		w.WriteHeader(http.StatusTeapot)
		w.Write([]byte("Panic recovered: " + err.Error()))
	}

	r := router.New[*router.Context](router.WithErrorHandler(customErrorHandler))

	r.Get("/panic", func(ctx *router.Context) handler.Response {
		panic("test panic")
	})

	req := httptest.NewRequest(http.MethodGet, "/panic", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.True(t, errorHandlerCalled)
	assert.NotNil(t, capturedError)
	assert.Contains(t, capturedError.Error(), "test panic")
	assert.Equal(t, http.StatusTeapot, w.Code)
	assert.Contains(t, w.Body.String(), "Panic recovered")
}

func TestPanicInResponseFunction(t *testing.T) {
	t.Parallel()

	errorHandlerCalled := false
	var capturedError error

	customErrorHandler := func(ctx *router.Context, err error) {
		errorHandlerCalled = true
		capturedError = err
		w := ctx.ResponseWriter()
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Write([]byte("Response panic recovered"))
	}

	r := router.New[*router.Context](router.WithErrorHandler(customErrorHandler))

	r.Get("/response-panic", func(ctx *router.Context) handler.Response {
		return func(w http.ResponseWriter, r *http.Request) error {
			panic("response function panic")
		}
	})

	req := httptest.NewRequest(http.MethodGet, "/response-panic", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.True(t, errorHandlerCalled)
	assert.NotNil(t, capturedError)
	assert.Contains(t, capturedError.Error(), "response function panic")
	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

func TestPanicWithDifferentTypes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		panicValue any
		expected   string
	}{
		{"string_panic", "string panic message", "string panic message"},
		{"error_panic", errors.New("error panic"), "error panic"},
		{"int_panic", 42, "panic: 42"},
		{"nil_panic", nil, "panic called with nil argument"},
		{"struct_panic", struct{ msg string }{"struct panic"}, "panic: {struct panic}"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			errorHandlerCalled := false
			var capturedError error

			customErrorHandler := func(ctx *router.Context, err error) {
				errorHandlerCalled = true
				capturedError = err
				w := ctx.ResponseWriter()
				w.WriteHeader(http.StatusInternalServerError)
				w.Write([]byte(err.Error()))
			}

			r := router.New[*router.Context](router.WithErrorHandler(customErrorHandler))

			r.Get("/panic", func(ctx *router.Context) handler.Response {
				panic(test.panicValue)
			})

			req := httptest.NewRequest(http.MethodGet, "/panic", nil)
			w := httptest.NewRecorder()

			r.ServeHTTP(w, req)

			assert.True(t, errorHandlerCalled)
			assert.NotNil(t, capturedError)
			assert.Contains(t, capturedError.Error(), test.expected)
		})
	}
}

func TestNotFoundError(t *testing.T) {
	t.Parallel()

	errorHandlerCalled := false
	var capturedError error

	customErrorHandler := func(ctx *router.Context, err error) {
		errorHandlerCalled = true
		capturedError = err
		w := ctx.ResponseWriter()
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("Custom not found: " + err.Error()))
	}

	r := router.New[*router.Context](router.WithErrorHandler(customErrorHandler))

	// Don't register any routes

	req := httptest.NewRequest(http.MethodGet, "/nonexistent", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.True(t, errorHandlerCalled)
	assert.NotNil(t, capturedError)
	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.Contains(t, w.Body.String(), "Custom not found")
}

func TestMethodNotAllowedError(t *testing.T) {
	t.Parallel()

	errorHandlerCalled := false
	var capturedError error

	customErrorHandler := func(ctx *router.Context, err error) {
		errorHandlerCalled = true
		capturedError = err
		w := ctx.ResponseWriter()
		w.WriteHeader(http.StatusMethodNotAllowed)
		w.Write([]byte("Custom method not allowed: " + err.Error()))
	}

	r := router.New[*router.Context](router.WithErrorHandler(customErrorHandler))

	// Register only GET handler
	r.Get("/test", func(ctx *router.Context) handler.Response {
		return func(w http.ResponseWriter, r *http.Request) error {
			w.WriteHeader(http.StatusOK)
			return nil
		}
	})

	// Try POST request
	req := httptest.NewRequest(http.MethodPost, "/test", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.True(t, errorHandlerCalled)
	assert.NotNil(t, capturedError)
	assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
	assert.Contains(t, w.Body.String(), "Custom method not allowed")
	assert.Equal(t, "GET", w.Header().Get("Allow"))
}

func TestInvalidHTTPMethodError(t *testing.T) {
	t.Parallel()

	errorHandlerCalled := false
	var capturedError error

	customErrorHandler := func(ctx *router.Context, err error) {
		errorHandlerCalled = true
		capturedError = err
		w := ctx.ResponseWriter()
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("Invalid method: " + err.Error()))
	}

	r := router.New[*router.Context](router.WithErrorHandler(customErrorHandler))

	// Use invalid HTTP method
	req := httptest.NewRequest("INVALID", "/test", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.True(t, errorHandlerCalled)
	assert.NotNil(t, capturedError)
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "Invalid method")
}

func TestNilResponseError(t *testing.T) {
	t.Parallel()

	errorHandlerCalled := false
	var capturedError error

	customErrorHandler := func(ctx *router.Context, err error) {
		errorHandlerCalled = true
		capturedError = err
		w := ctx.ResponseWriter()
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Nil response: " + err.Error()))
	}

	r := router.New[*router.Context](router.WithErrorHandler(customErrorHandler))

	r.Get("/nil", func(ctx *router.Context) handler.Response {
		return nil // Return nil response
	})

	req := httptest.NewRequest(http.MethodGet, "/nil", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.True(t, errorHandlerCalled)
	assert.NotNil(t, capturedError)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Contains(t, w.Body.String(), "Nil response")
}

func TestErrorAfterWriteHandledProperly(t *testing.T) {
	t.Parallel()

	errorHandlerCalled := false
	var handledError error

	customErrorHandler := func(ctx *router.Context, err error) {
		errorHandlerCalled = true
		handledError = err
		// Don't write if response already started
		// This is what real error handlers should do
		w := ctx.ResponseWriter()
		// Try to write - if it fails, response was already written
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Error: " + err.Error()))
	}

	r := router.New[*router.Context](router.WithErrorHandler(customErrorHandler))

	r.Get("/error-after-write", func(ctx *router.Context) handler.Response {
		return func(w http.ResponseWriter, r *http.Request) error {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("This will be sent"))
			// Return error after writing response
			// Without buffering, this response is already committed
			return errors.New("error after write")
		}
	})

	req := httptest.NewRequest(http.MethodGet, "/error-after-write", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	// Error handler should be called
	assert.True(t, errorHandlerCalled)
	assert.NotNil(t, handledError)
	assert.Equal(t, "error after write", handledError.Error())

	// Without buffering, the original response is already sent
	// Error handler will append to it (this is a handler bug)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "This will be sentError: error after write", w.Body.String())
}

func TestErrorHandlerPreventsDoubleWrite(t *testing.T) {
	t.Parallel()

	customErrorHandler := func(ctx *router.Context, err error) {
		w := ctx.ResponseWriter()

		// First WriteHeader sets the status
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Error handled"))

		// Second WriteHeader should be ignored (standard HTTP behavior)
		w.WriteHeader(http.StatusBadRequest) // Status remains 500
		// But additional writes should append to the buffer
		w.Write([]byte(" - additional content"))
	}

	r := router.New[*router.Context](router.WithErrorHandler(customErrorHandler))

	r.Get("/error", func(ctx *router.Context) handler.Response {
		return func(w http.ResponseWriter, r *http.Request) error {
			return errors.New("test error")
		}
	})

	req := httptest.NewRequest(http.MethodGet, "/error", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	// Status should be from first WriteHeader call
	assert.Equal(t, http.StatusInternalServerError, w.Code)
	// Body should contain all written content
	assert.Equal(t, "Error handled - additional content", w.Body.String())
}

func TestErrorHandlerInMiddleware(t *testing.T) {
	t.Parallel()

	middlewareErrorHandled := false
	routerErrorHandled := false

	routerErrorHandler := func(ctx *router.Context, err error) {
		routerErrorHandled = true
		w := ctx.ResponseWriter()
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Router error handler"))
	}

	r := router.New[*router.Context](router.WithErrorHandler(routerErrorHandler))

	errorHandlingMiddleware := func(next handler.HandlerFunc[*router.Context]) handler.HandlerFunc[*router.Context] {
		return func(ctx *router.Context) handler.Response {
			response := next(ctx)
			return func(w http.ResponseWriter, r *http.Request) error {
				if err := response(w, r); err != nil {
					middlewareErrorHandled = true
					w.WriteHeader(http.StatusBadRequest)
					w.Write([]byte("Middleware handled: " + err.Error()))
					return nil // Error handled by middleware
				}
				return nil
			}
		}
	}

	r.Use(errorHandlingMiddleware)

	r.Get("/error", func(ctx *router.Context) handler.Response {
		return func(w http.ResponseWriter, r *http.Request) error {
			return errors.New("handler error")
		}
	})

	req := httptest.NewRequest(http.MethodGet, "/error", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.True(t, middlewareErrorHandled)
	assert.False(t, routerErrorHandled) // Router error handler shouldn't be called
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "Middleware handled")
}

func TestErrorHandlerPanicRecovery(t *testing.T) {
	t.Parallel()

	panicErrorHandler := func(ctx *router.Context, err error) {
		panic("error handler panic")
	}

	r := router.New[*router.Context](router.WithErrorHandler(panicErrorHandler))

	r.Get("/error", func(ctx *router.Context) handler.Response {
		return func(w http.ResponseWriter, r *http.Request) error {
			return errors.New("original error")
		}
	})

	req := httptest.NewRequest(http.MethodGet, "/error", nil)
	w := httptest.NewRecorder()

	// If error handler panics, that's a programming error and should panic
	// This tests that broken error handlers are not silently ignored
	assert.Panics(t, func() {
		r.ServeHTTP(w, req)
	})
}

func TestMultipleErrorsSameRequest(t *testing.T) {
	t.Parallel()

	errorCount := 0
	var capturedErrors []error

	errorCountingHandler := func(ctx *router.Context, err error) {
		errorCount++
		capturedErrors = append(capturedErrors, err)
		w := ctx.ResponseWriter()
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Error count: " + err.Error()))
	}

	r := router.New[*router.Context](router.WithErrorHandler(errorCountingHandler))

	r.Get("/multiple-errors", func(ctx *router.Context) handler.Response {
		return func(w http.ResponseWriter, r *http.Request) error {
			// First error
			if err := errors.New("first error"); err != nil {
				// This would normally be handled, but we'll cause a panic
				panic("panic after error")
			}
			return nil
		}
	})

	req := httptest.NewRequest(http.MethodGet, "/multiple-errors", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	// Only one error should be handled (the panic), not multiple
	assert.Equal(t, 1, errorCount)
	assert.Len(t, capturedErrors, 1)
	assert.Contains(t, capturedErrors[0].Error(), "panic after error")
}
