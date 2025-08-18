package router

import (
	"net/http"

	"github.com/dmitrymomot/gokit/handler"
)

// Option configures a Router during creation.
type Option[C handler.Context] func(*mux[C])

// WithErrorHandler sets a custom error handler for the router.
func WithErrorHandler[C handler.Context](h handler.ErrorHandler[C]) Option[C] {
	return func(m *mux[C]) {
		if h != nil {
			m.errorHandler = h
		}
	}
}

// WithMiddleware adds middleware to the router.
func WithMiddleware[C handler.Context](middlewares ...handler.Middleware[C]) Option[C] {
	return func(m *mux[C]) {
		m.middlewares = append(m.middlewares, middlewares...)
	}
}

// WithContextFactory sets a custom context factory for the router.
func WithContextFactory[C handler.Context](f func(http.ResponseWriter, *http.Request, map[string]string) C) Option[C] {
	return func(m *mux[C]) {
		m.newContext = f
	}
}
