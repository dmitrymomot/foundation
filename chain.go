package gokit

// chain builds a single handler from a middleware stack and endpoint.
func chain[C Context](middlewares []Middleware[C], endpoint HandlerFunc[C]) HandlerFunc[C] {
	// Start with the endpoint
	handler := endpoint

	// Wrap in middleware in reverse order
	// so the first middleware runs first
	for i := len(middlewares) - 1; i >= 0; i-- {
		handler = middlewares[i](handler)
	}

	return handler
}
