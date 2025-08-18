package router

import (
	"net/http"

	"github.com/dmitrymomot/gokit/handler"
)

// Router is the main routing interface for handling HTTP requests.
type Router[C handler.Context] interface {
	http.Handler
	Routes

	// HTTP method handlers
	Get(pattern string, h handler.HandlerFunc[C])
	Post(pattern string, h handler.HandlerFunc[C])
	Put(pattern string, h handler.HandlerFunc[C])
	Delete(pattern string, h handler.HandlerFunc[C])
	Patch(pattern string, h handler.HandlerFunc[C])
	Head(pattern string, h handler.HandlerFunc[C])
	Options(pattern string, h handler.HandlerFunc[C])
	Connect(pattern string, h handler.HandlerFunc[C])
	Trace(pattern string, h handler.HandlerFunc[C])

	// Generic handlers
	Handle(pattern string, h handler.HandlerFunc[C])
	Method(pattern string, h handler.HandlerFunc[C], methods ...string)

	// Middleware
	Use(middlewares ...handler.Middleware[C])
	With(middlewares ...handler.Middleware[C]) Router[C]

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

// New creates a new router with the given options.
func New[C handler.Context](opts ...Option[C]) Router[C] {
	return newMux[C](opts...)
}
