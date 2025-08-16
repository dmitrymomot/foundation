// Package gokit provides a high-performance HTTP router and toolkit for building
// JSON APIs, web services, and HTTP-based applications in Go. It features type-safe
// request handlers, structured error handling, streaming responses, and a powerful
// middleware system.
package gokit

import (
	"context"
	"net/http"
)

// HandlerFunc is a type-safe HTTP request handler with custom context support.
type HandlerFunc[C contexter] func(ctx C) Response

// ErrorHandler handles errors during request processing.
type ErrorHandler[C contexter] func(ctx C, err error)

// Middleware wraps handlers to add cross-cutting functionality.
type Middleware[C contexter] func(next HandlerFunc[C]) HandlerFunc[C]

// Response renders HTTP responses. Implementations should set headers, status, and body.
// Rendering errors are handled by the framework's error handler.
type Response interface {
	Render(w http.ResponseWriter, r *http.Request) error
}

// contexter defines the contract for request contexts in the framework.
// Use Context for the default implementation.
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
	Method(pattern string, handler HandlerFunc[C], methods ...string)

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
