package router

import (
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"runtime/debug"
	"strings"

	"github.com/dmitrymomot/foundation/core/handler"
)

// mux is the private implementation of Router interface.
type mux[C handler.Context] struct {
	tree         *node[C]
	middlewares  []handler.Middleware[C]
	errorHandler handler.ErrorHandler[C]
	newContext   func(http.ResponseWriter, *http.Request, map[string]string) C
	logger       *slog.Logger
	parent       *mux[C] // for sub-routers
	inline       bool    // for inline groups
	handler      handler.HandlerFunc[C]
}

// newMux creates a new router instance.
func newMux[C handler.Context](opts ...Option[C]) *mux[C] {
	m := &mux[C]{
		tree:         &node[C]{},
		errorHandler: defaultErrorHandler[C],
		logger:       slog.New(slog.NewTextHandler(io.Discard, nil)), // No-op logger by default
	}

	for _, opt := range opts {
		opt(m)
	}

	// If no context factory provided, require it for non-default contexts
	if m.newContext == nil {
		m.newContext = func(w http.ResponseWriter, r *http.Request, params map[string]string) C {
			// Only support default *Context type without factory
			// For custom contexts, user must provide a factory
			var zero C
			if _, ok := any(zero).(*Context); ok {
				return any(newContext(w, r, params)).(C)
			}
			panic(ErrNoContextFactory)
		}
	}

	return m
}

// ServeHTTP implements http.Handler interface.
func (m *mux[C]) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ww := newResponseWriter(w)

	// Use RawPath if available to preserve URL encoding
	path := r.URL.Path
	if r.URL.RawPath != "" {
		path = r.URL.RawPath
	}
	if path == "" {
		path = "/"
	}

	method, ok := methodMap[r.Method]
	if !ok {
		// Create context with empty params for error handling
		ctx := m.newContext(ww, r, nil)
		m.errorHandler(ctx, ErrMethodNotAllowed)
		return
	}

	// Find route and extract params
	rn, eps, fn, params := m.tree.findRoute(method, path)

	// Build params map
	var paramsMap map[string]string
	if len(params.Keys) > 0 {
		paramsMap = make(map[string]string, len(params.Keys))
		for i, key := range params.Keys {
			if i < len(params.Values) {
				paramsMap[key] = params.Values[i]
			}
		}
	}

	// Create context with params
	ctx := m.newContext(ww, r, paramsMap)

	// Recover from panics to prevent server crashes
	defer func() {
		if p := recover(); p != nil {
			// Wrap panic in error with stack trace
			panicErr := &panicError{
				value: p,
				stack: debug.Stack(),
			}

			// Check if response has already been written
			if ww.Written() {
				// Can't send error response, just log the panic
				m.logger.Error("panic after response written",
					"value", panicErr.value,
					"stack", string(panicErr.stack),
					"path", r.URL.Path,
					"method", r.Method,
					"status", ww.Status(),
				)
			} else {
				// Response not written, can use error handler
				m.errorHandler(ctx, panicErr)
			}
		}
	}()

	// Check if we hit a mounted subrouter
	if rn != nil && rn.subroutes != nil {
		// Calculate the remaining path after the mount point
		mountPath := ""
		if rn.endpoints[mSTUB] != nil {
			mountPath = rn.endpoints[mSTUB].pattern
		}

		// Strip the mount path from the request path
		subPath := path
		if mountPath != "" && mountPath != "/" {
			// Remove trailing wildcard from mount pattern if present
			if strings.HasSuffix(mountPath, "/*") {
				mountPath = mountPath[:len(mountPath)-2]
			} else if strings.HasSuffix(mountPath, "*") {
				mountPath = mountPath[:len(mountPath)-1]
			}

			if strings.HasPrefix(path, mountPath) {
				subPath = path[len(mountPath):]
				if subPath == "" {
					subPath = "/"
				} else if subPath[0] != '/' {
					subPath = "/" + subPath
				}
			}
		}

		// Update request with the sub-path and delegate to subrouter
		r2 := r.Clone(r.Context())
		r2.URL.Path = subPath
		rn.subroutes.ServeHTTP(w, r2)
		return
	}

	if fn == nil {
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
			// Set Allow header per RFC 7231 before responding with 405
			if !ww.Written() {
				ww.Header().Set("Allow", strings.Join(allowed, ", "))
			}
			m.errorHandler(ctx, ErrMethodNotAllowed)
		} else {
			m.errorHandler(ctx, ErrNotFound)
		}
		return
	}

	if len(m.middlewares) > 0 {
		fn = chain(m.middlewares, fn)
	}

	response := fn(ctx)
	if response == nil {
		m.errorHandler(ctx, ErrNilResponse)
		return
	}

	if err := response(ww, r); err != nil {
		m.errorHandler(ctx, err)
		return
	}
}

// Get registers a handler for GET requests.
func (m *mux[C]) Get(pattern string, handler handler.HandlerFunc[C]) {
	m.handle(mGET, pattern, handler)
}

// Post registers a handler for POST requests.
func (m *mux[C]) Post(pattern string, handler handler.HandlerFunc[C]) {
	m.handle(mPOST, pattern, handler)
}

// Put registers a handler for PUT requests.
func (m *mux[C]) Put(pattern string, handler handler.HandlerFunc[C]) {
	m.handle(mPUT, pattern, handler)
}

// Delete registers a handler for DELETE requests.
func (m *mux[C]) Delete(pattern string, handler handler.HandlerFunc[C]) {
	m.handle(mDELETE, pattern, handler)
}

// Patch registers a handler for PATCH requests.
func (m *mux[C]) Patch(pattern string, handler handler.HandlerFunc[C]) {
	m.handle(mPATCH, pattern, handler)
}

// Head registers a handler for HEAD requests.
func (m *mux[C]) Head(pattern string, handler handler.HandlerFunc[C]) {
	m.handle(mHEAD, pattern, handler)
}

// Options registers a handler for OPTIONS requests.
func (m *mux[C]) Options(pattern string, handler handler.HandlerFunc[C]) {
	m.handle(mOPTIONS, pattern, handler)
}

// Connect registers a handler for CONNECT requests.
func (m *mux[C]) Connect(pattern string, handler handler.HandlerFunc[C]) {
	m.handle(mCONNECT, pattern, handler)
}

// Trace registers a handler for TRACE requests.
func (m *mux[C]) Trace(pattern string, handler handler.HandlerFunc[C]) {
	m.handle(mTRACE, pattern, handler)
}

// Handle registers a handler for all HTTP methods.
func (m *mux[C]) Handle(pattern string, handler handler.HandlerFunc[C]) {
	m.handle(mALL, pattern, handler)
}

// Method registers a handler for one or more specific HTTP methods.
func (m *mux[C]) Method(pattern string, handler handler.HandlerFunc[C], methods ...string) {
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
func (m *mux[C]) Use(middlewares ...handler.Middleware[C]) {
	if m.handler != nil {
		panic("foundation: all middlewares must be defined before routes on a mux")
	}
	m.middlewares = append(m.middlewares, middlewares...)
}

// With creates a new inline router with additional middleware.
func (m *mux[C]) With(middlewares ...handler.Middleware[C]) Router[C] {
	// Only store the additional middlewares, not parent ones
	// They will be chained at registration time
	im := &mux[C]{
		inline:       true,
		parent:       m,
		tree:         m.tree,
		middlewares:  middlewares,
		errorHandler: m.errorHandler,
		newContext:   m.newContext,
		logger:       m.logger,
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

	subRouter.errorHandler = m.errorHandler
	subRouter.newContext = m.newContext
	subRouter.logger = m.logger

	fn(subRouter)
	m.Mount(pattern, subRouter)
	return subRouter
}

// Mount attaches a sub-router at the given pattern.
func (m *mux[C]) Mount(pattern string, sub Router[C]) {
	if sub == nil {
		panic(fmt.Errorf("%w on '%s'", ErrNilRouter, pattern))
	}

	subMux, ok := sub.(*mux[C])
	if !ok {
		panic("foundation: can only mount *mux[C] routers")
	}

	// Always inherit parent's error handler, logger, and context factory for consistency
	// This ensures mounted subrouters behave predictably
	subMux.errorHandler = m.errorHandler
	subMux.logger = m.logger
	subMux.newContext = m.newContext

	// Stub handler - actual routing is handled by the tree traversal
	mountHandler := func(ctx C) handler.Response {
		return nil
	}

	// Store all nodes that need the subrouter reference
	var nodes []*node[C]

	if pattern == "" || pattern[len(pattern)-1] != '/' {
		n1 := m.handle(mALL|mSTUB, pattern, mountHandler)
		if n1 != nil {
			nodes = append(nodes, n1)
		}
		n2 := m.handle(mALL|mSTUB, pattern+"/", mountHandler)
		if n2 != nil {
			nodes = append(nodes, n2)
		}
		pattern += "/"
	}

	n := m.handle(mALL|mSTUB, pattern+"*", mountHandler)
	if n != nil {
		nodes = append(nodes, n)
	}

	// Set subrouter on all mount nodes
	for _, node := range nodes {
		node.subroutes = sub
	}
}

// Routes returns all registered routes.
func (m *mux[C]) Routes() []Route {
	return m.tree.routes()
}

// handle registers a handler in the routing tree.
func (m *mux[C]) handle(method methodTyp, pattern string, fn handler.HandlerFunc[C]) *node[C] {
	if len(pattern) == 0 || pattern[0] != '/' {
		panic(fmt.Errorf("%w: '%s'", ErrInvalidPattern, pattern))
	}

	// Mark that routes have been added (for middleware validation)
	if !m.inline && m.handler == nil {
		m.handler = fn // Just use as a flag that routes exist
	}

	// For inline routers, collect all middlewares from parent chain
	var h handler.HandlerFunc[C]
	if m.inline {
		// Collect middlewares from parent inline routers
		var allMiddlewares []handler.Middleware[C]
		curr := m
		for curr != nil && curr.inline {
			// Prepend parent middlewares to maintain order
			if len(curr.middlewares) > 0 {
				allMiddlewares = append(curr.middlewares, allMiddlewares...)
			}
			curr = curr.parent
		}
		if len(allMiddlewares) > 0 {
			h = chain(allMiddlewares, fn)
		} else {
			h = fn
		}
	} else {
		h = fn
	}

	return m.tree.insertRoute(method, pattern, h)
}
