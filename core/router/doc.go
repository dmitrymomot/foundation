// Package router provides a high-performance, type-safe HTTP router with middleware support
// built on top of a radix tree for efficient path matching.
//
// The router supports modern web development patterns including middleware chaining,
// route grouping, sub-router mounting, and WebSocket upgrades. It uses Go generics
// to provide type-safe context handling while maintaining excellent performance
// through its radix tree implementation.
//
// Basic usage:
//
//	import (
//		"net/http"
//		"github.com/dmitrymomot/foundation/core/handler"
//		"github.com/dmitrymomot/foundation/core/router"
//	)
//
//	func main() {
//		r := router.New[*router.Context]()
//
//		r.Get("/", func(ctx *router.Context) handler.Response {
//			return func(w http.ResponseWriter, r *http.Request) error {
//				w.Write([]byte("Hello World"))
//				return nil
//			}
//		})
//
//		r.Get("/users/{id}", func(ctx *router.Context) handler.Response {
//			return func(w http.ResponseWriter, r *http.Request) error {
//				id := ctx.Param("id")
//				w.Write([]byte("User ID: " + id))
//				return nil
//			}
//		})
//
//		http.ListenAndServe(":8080", r)
//	}
//
// # Route Patterns
//
// The router supports several URL pattern types:
//
//   - Static routes: /users/profile
//   - Parameters: /users/{id} or /users/{id:[0-9]+} with regex constraints
//   - Wildcards: /files/* (catches all remaining path segments)
//
// Parameters are extracted and made available via ctx.Param("name").
//
// # Middleware Support
//
// Middleware can be applied globally or to specific route groups:
//
//	// Global middleware
//	r.Use(loggingMiddleware, authMiddleware)
//
//	// Route-specific middleware
//	r.With(adminMiddleware).Get("/admin", adminHandler)
//
//	// Route groups with shared middleware
//	r.Route("/api/v1", func(r router.Router[*router.Context]) {
//		r.Use(apiMiddleware)
//		r.Get("/users", listUsersHandler)
//		r.Post("/users", createUserHandler)
//	})
//
// # Error Handling
//
// The router provides comprehensive error handling with panic recovery:
//
//	errorHandler := func(ctx *router.Context, err error) {
//		if panicErr, ok := err.(router.PanicError); ok {
//			log.Printf("Panic: %v\nStack: %s", panicErr.Value(), panicErr.Stack())
//		}
//		http.Error(ctx.ResponseWriter(), err.Error(), http.StatusInternalServerError)
//	}
//
//	r := router.New[*router.Context](
//		router.WithErrorHandler(errorHandler),
//	)
//
// # Custom Context Types
//
// Use custom context types for application-specific data:
//
//	type AppContext struct {
//		*router.Context
//		User *User
//		DB   *sql.DB
//	}
//
//	contextFactory := func(w http.ResponseWriter, r *http.Request, params map[string]string) *AppContext {
//		return &AppContext{
//			Context: router.newContext(w, r, params), // Note: this requires package access
//			DB:      db,
//		}
//	}
//
//	r := router.New[*AppContext](
//		router.WithContextFactory(contextFactory),
//	)
//
// # WebSocket Support
//
// The router supports WebSocket upgrades through the http.Hijacker interface:
//
//	r.Get("/ws", func(ctx *router.Context) handler.Response {
//		return func(w http.ResponseWriter, r *http.Request) error {
//			hijacker, ok := w.(http.Hijacker)
//			if !ok {
//				return errors.New("websocket upgrade not supported")
//			}
//			// Use websocket library to upgrade connection
//			// upgrader.Upgrade(w, r, nil)
//			return nil
//		}
//	})
//
// # Sub-router Mounting
//
// Mount sub-routers for modular application design:
//
//	apiRouter := router.New[*router.Context]()
//	apiRouter.Get("/users", listUsersHandler)
//	apiRouter.Post("/users", createUserHandler)
//
//	mainRouter := router.New[*router.Context]()
//	mainRouter.Mount("/api/v1", apiRouter)
//
// # Performance
//
// The router uses a radix tree for O(k) path matching where k is the key length,
// providing excellent performance even with thousands of routes. The tree supports
// efficient parameter extraction and wildcard matching.
package router
