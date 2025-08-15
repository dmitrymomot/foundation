package gokit

import (
	"context"
	"net/http"
)

// HandlerFunc provides type-safe HTTP request handling with custom context support.
// C must implement the Context interface, Response must implement the Response interface.
type HandlerFunc[C Context] func(ctx C) Response

// ErrorHandler handles errors from binding or rendering.
type ErrorHandler[C Context] func(ctx C, err error)

// Middleware wraps a HandlerFunc with additional functionality.
type Middleware[C Context] func(next HandlerFunc[C]) HandlerFunc[C]

// Response renders itself to an http.ResponseWriter.
// Implementations should set headers, status code, and write body.
// Errors are handled by the framework (returns 500).
type Response interface {
	Render(w http.ResponseWriter, r *http.Request) error
}

// Context wraps http.Request and http.ResponseWriter with context.Context.
// It embeds the request's context and provides access to HTTP components.
type Context interface {
	context.Context
	Request() *http.Request
	ResponseWriter() http.ResponseWriter
}

// Router is the main routing interface for handling HTTP requests.
type Router[C Context] interface {
	http.Handler
	Routes

	// HTTP method handlers
	Get(pattern string, handler HandlerFunc[C])
	Post(pattern string, handler HandlerFunc[C])
	Put(pattern string, handler HandlerFunc[C])
	Delete(pattern string, handler HandlerFunc[C])
	Patch(pattern string, handler HandlerFunc[C])
	Head(pattern string, handler HandlerFunc[C])
	Options(pattern string, handler HandlerFunc[C])
	Connect(pattern string, handler HandlerFunc[C])
	Trace(pattern string, handler HandlerFunc[C])

	// Generic handlers
	Handle(pattern string, handler HandlerFunc[C])
	Method(method, pattern string, handler HandlerFunc[C])

	// Middleware
	Use(middlewares ...Middleware[C])
	With(middlewares ...Middleware[C]) Router[C]

	// Grouping and mounting
	Group(fn func(r Router[C])) Router[C]
	Route(pattern string, fn func(r Router[C])) Router[C]
	Mount(pattern string, sub Router[C])
}

// Routes provides route introspection capabilities.
type Routes interface {
	Routes() []Route
}

// Route describes a single route in the router.
type Route struct {
	Method  string
	Pattern string
}

// Option configures a Router during creation.
type Option[C Context] func(*mux[C])

// NewRouter creates a new router with the given options.
func NewRouter[C Context](opts ...Option[C]) Router[C] {
	return newMux[C](opts...)
}

// WithErrorHandler sets a custom error handler for the router.
func WithErrorHandler[C Context](h ErrorHandler[C]) Option[C] {
	return func(m *mux[C]) {
		if h != nil {
			m.errorHandler = h
		}
	}
}

// WithMiddleware adds middleware to the router.
func WithMiddleware[C Context](middlewares ...Middleware[C]) Option[C] {
	return func(m *mux[C]) {
		m.middlewares = append(m.middlewares, middlewares...)
	}
}
