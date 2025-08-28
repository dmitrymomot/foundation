// Package handler provides type-safe HTTP handler abstractions with support
// for custom context types, middleware composition, and clean error handling.
//
// The package defines core types that enable building HTTP handlers with
// Go generics for compile-time type safety and clean separation between
// business logic and HTTP concerns.
//
// # Basic Usage
//
// Define handlers that return Response functions:
//
//	import (
//		"net/http"
//		"github.com/dmitrymomot/foundation/core/handler"
//		"github.com/dmitrymomot/foundation/core/response"
//	)
//
//	func greetHandler(ctx handler.Context) handler.Response {
//		name := ctx.Param("name")
//		if name == "" {
//			name = "World"
//		}
//		return response.Text("Hello, " + name + "!")
//	}
//
// # Context Interface
//
// The Context interface extends standard context.Context with HTTP methods:
//
//	type Context interface {
//		context.Context                    // Standard context methods
//		Request() *http.Request           // Access to HTTP request
//		ResponseWriter() http.ResponseWriter // Access to response writer
//		Param(key string) string          // Get path parameters
//		SetValue(key, val any)           // Store request-scoped values
//	}
//
// # Core Types
//
//	// Response renders HTTP responses and returns any rendering errors
//	type Response func(w http.ResponseWriter, r *http.Request) error
//
//	// HandlerFunc is a type-safe handler with custom context support
//	type HandlerFunc[C Context] func(ctx C) Response
//
//	// ErrorHandler processes errors from handler or response execution
//	type ErrorHandler[C Context] func(ctx C, err error)
//
//	// Middleware wraps handlers for cross-cutting concerns
//	type Middleware[C Context] func(next HandlerFunc[C]) HandlerFunc[C]
//
// # Middleware Usage
//
// Use existing middleware from the foundation/middleware package:
//
//	import (
//		"github.com/dmitrymomot/foundation/core/router"
//		"github.com/dmitrymomot/foundation/middleware"
//	)
//
//	r := router.New[*router.Context]()
//	r.Use(middleware.Logging[*router.Context]())
//	r.Use(middleware.CORS[*router.Context]())
//
//	r.Get("/hello/{name}", greetHandler)
//
// # Integration
//
// This package is typically used with github.com/dmitrymomot/foundation/core/router
// and github.com/dmitrymomot/foundation/core/response packages for complete
// HTTP handling functionality.
package handler
