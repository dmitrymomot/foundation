package gokit

import (
	"fmt"
	"net/http"
	"strings"
)

// mux is the private implementation of Router interface.
type mux[C contexter] struct {
	tree         *node[C]
	middlewares  []Middleware[C]
	errorHandler ErrorHandler[C]
	newContext   func(http.ResponseWriter, *http.Request) C
	parent       *mux[C] // for sub-routers
	inline       bool    // for inline groups
	handler      HandlerFunc[C]
}

// newMux creates a new router instance.
func newMux[C contexter](opts ...Option[C]) *mux[C] {
	m := &mux[C]{
		tree:         &node[C]{},
		errorHandler: defaultErrorHandler[C],
	}

	// Apply options
	for _, opt := range opts {
		opt(m)
	}

	// If no context factory provided, check if C is *Context
	if m.newContext == nil {
		m.newContext = func(w http.ResponseWriter, r *http.Request) C {
			// Try to create a Context if C is compatible
			var zero C
			if _, ok := any(zero).(*Context); ok {
				return any(NewContext(w, r)).(C)
			}
			// Otherwise panic - user must provide a context factory
			panic(ErrNoContextFactory)
		}
	}

	return m
}

// ServeHTTP implements http.Handler interface.
func (m *mux[C]) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Track response state
	ww := &responseWriter{ResponseWriter: w}

	ctx := m.newContext(ww, r)

	// Panic recovery
	defer func() {
		if err := recover(); err != nil {
			if !ww.Written() {
				m.errorHandler(ctx, toError(err))
			}
		}
	}()

	// Route path
	path := r.URL.Path
	if path == "" {
		path = "/"
	}

	// Find route
	method, ok := methodMap[r.Method]
	if !ok {
		m.errorHandler(ctx, ErrMethodNotAllowed)
		return
	}

	_, eps, handler, params := m.tree.findRoute(method, path)

	if bc, ok := any(ctx).(*Context); ok && len(params.Keys) > 0 {
		for i, key := range params.Keys {
			if i < len(params.Values) {
				bc.setParam(key, params.Values[i])
			}
		}
	}

	if handler == nil {
		allowed := []string{}
		for mt := range eps {
			if mt == mALL || mt == mSTUB {
				continue
			}
			if eps[mt] != nil && eps[mt].handler != nil {
				allowed = append(allowed, reverseMethodMap[mt])
			}
		}

		if len(allowed) > 0 {
			// Set Allow header before calling error handler
			w := ctx.ResponseWriter()
			if ww, ok := w.(*responseWriter); ok && !ww.Written() {
				ww.Header().Set("Allow", strings.Join(allowed, ", "))
			}
			m.errorHandler(ctx, ErrMethodNotAllowed)
		} else {
			m.errorHandler(ctx, ErrNotFound)
		}
		return
	}

	// Apply middleware chain
	if len(m.middlewares) > 0 {
		handler = chain(m.middlewares, handler)
	}

	// Execute handler
	response := handler(ctx)
	if response == nil {
		m.errorHandler(ctx, ErrNilResponse)
		return
	}

	// Render response
	if err := response.Render(ww, r); err != nil {
		if !ww.Written() {
			m.errorHandler(ctx, err)
		}
	}
}

// Get registers a handler for GET requests.
func (m *mux[C]) Get(pattern string, handler HandlerFunc[C]) {
	m.handle(mGET, pattern, handler)
}

// Post registers a handler for POST requests.
func (m *mux[C]) Post(pattern string, handler HandlerFunc[C]) {
	m.handle(mPOST, pattern, handler)
}

// Put registers a handler for PUT requests.
func (m *mux[C]) Put(pattern string, handler HandlerFunc[C]) {
	m.handle(mPUT, pattern, handler)
}

// Delete registers a handler for DELETE requests.
func (m *mux[C]) Delete(pattern string, handler HandlerFunc[C]) {
	m.handle(mDELETE, pattern, handler)
}

// Patch registers a handler for PATCH requests.
func (m *mux[C]) Patch(pattern string, handler HandlerFunc[C]) {
	m.handle(mPATCH, pattern, handler)
}

// Head registers a handler for HEAD requests.
func (m *mux[C]) Head(pattern string, handler HandlerFunc[C]) {
	m.handle(mHEAD, pattern, handler)
}

// Options registers a handler for OPTIONS requests.
func (m *mux[C]) Options(pattern string, handler HandlerFunc[C]) {
	m.handle(mOPTIONS, pattern, handler)
}

// Connect registers a handler for CONNECT requests.
func (m *mux[C]) Connect(pattern string, handler HandlerFunc[C]) {
	m.handle(mCONNECT, pattern, handler)
}

// Trace registers a handler for TRACE requests.
func (m *mux[C]) Trace(pattern string, handler HandlerFunc[C]) {
	m.handle(mTRACE, pattern, handler)
}

// Handle registers a handler for all HTTP methods.
func (m *mux[C]) Handle(pattern string, handler HandlerFunc[C]) {
	m.handle(mALL, pattern, handler)
}

// Method registers a handler for one or more specific HTTP methods.
func (m *mux[C]) Method(pattern string, handler HandlerFunc[C], methods ...string) {
	if len(methods) == 0 {
		panic(fmt.Errorf("%w: no methods provided", ErrInvalidMethod))
	}

	seen := make(map[methodTyp]bool)
	for _, method := range methods {
		mt, ok := methodMap[strings.ToUpper(method)]
		if !ok {
			panic(fmt.Errorf("%w: %s", ErrInvalidMethod, method))
		}
		if seen[mt] {
			continue
		}
		seen[mt] = true
		m.handle(mt, pattern, handler)
	}
}

// Use appends middleware to the router.
func (m *mux[C]) Use(middlewares ...Middleware[C]) {
	if m.handler != nil {
		panic("gokit: all middlewares must be defined before routes on a mux")
	}
	m.middlewares = append(m.middlewares, middlewares...)
}

// With creates a new inline router with additional middleware.
func (m *mux[C]) With(middlewares ...Middleware[C]) Router[C] {
	// Copy middlewares from parent
	mws := make([]Middleware[C], len(m.middlewares))
	copy(mws, m.middlewares)
	mws = append(mws, middlewares...)

	im := &mux[C]{
		inline:       true,
		parent:       m,
		tree:         m.tree,
		middlewares:  mws,
		errorHandler: m.errorHandler,
		newContext:   m.newContext,
	}

	return im
}

// Group creates a new inline router for grouping routes.
func (m *mux[C]) Group(fn func(r Router[C])) Router[C] {
	im := m.With()
	if fn != nil {
		fn(im)
	}
	return im
}

// Route creates a new sub-router mounted at the given pattern.
func (m *mux[C]) Route(pattern string, fn func(r Router[C])) Router[C] {
	if fn == nil {
		panic(fmt.Errorf("%w on '%s'", ErrNilSubrouter, pattern))
	}
	subRouter := newMux[C]()

	// Copy configuration from parent
	subRouter.errorHandler = m.errorHandler
	subRouter.newContext = m.newContext

	fn(subRouter)
	m.Mount(pattern, subRouter)
	return subRouter
}

// Mount attaches a sub-router at the given pattern.
func (m *mux[C]) Mount(pattern string, sub Router[C]) {
	if sub == nil {
		panic(fmt.Errorf("%w on '%s'", ErrNilRouter, pattern))
	}

	// Type assertion to access private mux
	subMux, ok := sub.(*mux[C])
	if !ok {
		panic("gokit: can only mount *mux[C] routers")
	}

	// Share error handler if sub doesn't have one
	if subMux.errorHandler == nil {
		subMux.errorHandler = m.errorHandler
	}

	// Create mount handler
	mountHandler := func(ctx C) Response {
		// This is a stub handler for mounting
		// The actual routing is handled by the tree structure
		return nil
	}

	// Ensure pattern ends with /*
	if pattern == "" || pattern[len(pattern)-1] != '/' {
		m.handle(mALL|mSTUB, pattern, mountHandler)
		m.handle(mALL|mSTUB, pattern+"/", mountHandler)
		pattern += "/"
	}

	// Register the mount point with wildcard
	n := m.handle(mALL|mSTUB, pattern+"*", mountHandler)

	// Attach the sub-router to the node
	if n != nil {
		n.subroutes = sub
	}
}

// Routes returns all registered routes.
func (m *mux[C]) Routes() []Route {
	return m.tree.routes()
}

// handle registers a handler in the routing tree.
func (m *mux[C]) handle(method methodTyp, pattern string, handler HandlerFunc[C]) *node[C] {
	if len(pattern) == 0 || pattern[0] != '/' {
		panic(fmt.Errorf("%w: '%s'", ErrInvalidPattern, pattern))
	}

	// Build endpoint handler with inline middlewares for the route
	var h HandlerFunc[C]
	if m.inline {
		h = chain(m.middlewares, handler)
	} else {
		h = handler
	}

	// Add the endpoint to the tree and return the node
	return m.tree.insertRoute(method, pattern, h)
}
