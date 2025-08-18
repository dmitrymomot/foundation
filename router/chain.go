package router

import "github.com/dmitrymomot/gokit/handler"

// chain builds a single handler from a middleware stack and endpoint.
func chain[C handler.Context](middlewares []handler.Middleware[C], endpoint handler.HandlerFunc[C]) handler.HandlerFunc[C] {
	// Start with the endpoint
	handler := endpoint

	// Wrap in middleware in reverse order
	// so the first middleware runs first
	for i := len(middlewares) - 1; i >= 0; i-- {
		handler = middlewares[i](handler)
	}

	return handler
}
