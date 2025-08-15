package gokit

import (
	"context"
	"net/http"
)

// HandlerFunc provides type-safe HTTP request handling with custom context support.
// C must implement the contextInterface interface, Response must implement the Response interface.
type HandlerFunc[C contexter] func(ctx C) Response

// ErrorHandler handles errors from binding or rendering.
type ErrorHandler[C contexter] func(ctx C, err error)

// Middleware wraps a HandlerFunc with additional functionality.
type Middleware[C contexter] func(next HandlerFunc[C]) HandlerFunc[C]

// Response renders itself to an http.ResponseWriter.
// Implementations should set headers, status code, and write body.
// Errors are handled by the framework (returns 500).
type Response interface {
	Render(w http.ResponseWriter, r *http.Request) error
}

// contexter wraps http.Request and http.ResponseWriter with context.Context.
// It embeds the request's context and provides access to HTTP components.
// This is a private interface - use Context for the default implementation.
type contexter interface {
	context.Context
	Request() *http.Request
	ResponseWriter() http.ResponseWriter
	Param(key string) string
}

// Router is the main routing interface for handling HTTP requests.
type Router[C contexter] interface {
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

// NewRouter creates a new router with the given options.
func NewRouter[C contexter](opts ...Option[C]) Router[C] {
	return newMux[C](opts...)
}
