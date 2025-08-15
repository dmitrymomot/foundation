package gokit

import "net/http"

// Option configures a Router during creation.
type Option[C contexter] func(*mux[C])

// WithErrorHandler sets a custom error handler for the router.
func WithErrorHandler[C contexter](h ErrorHandler[C]) Option[C] {
	return func(m *mux[C]) {
		if h != nil {
			m.errorHandler = h
		}
	}
}

// WithMiddleware adds middleware to the router.
func WithMiddleware[C contexter](middlewares ...Middleware[C]) Option[C] {
	return func(m *mux[C]) {
		m.middlewares = append(m.middlewares, middlewares...)
	}
}

// WithContextFactory sets a custom context factory for the router.
func WithContextFactory[C contexter](f func(http.ResponseWriter, *http.Request) C) Option[C] {
	return func(m *mux[C]) {
		m.newContext = f
	}
}
