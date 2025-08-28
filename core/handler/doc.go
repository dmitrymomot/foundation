// Package handler provides types and interfaces for HTTP request processing
// with type-safe context handling and middleware support. It defines the core
// abstractions for building HTTP handlers with custom context types and
// composable middleware chains.
//
// # Features
//
//   - Type-safe HTTP handlers using Go generics
//   - Custom context interface for request-specific data
//   - Composable middleware system with type safety
//   - Clean separation between response generation and rendering
//   - Error handling abstraction for consistent error processing
//   - Compatible with standard HTTP routing libraries
//
// # Core Types
//
// The package defines several key types that work together to provide
// a cohesive request processing framework:
//
//	import "github.com/dmitrymomot/foundation/core/handler"
//
//	// Response function renders HTTP responses
//	type Response func(w http.ResponseWriter, r *http.Request) error
//
//	// Type-safe handler with custom context
//	type HandlerFunc[C Context] func(ctx C) Response
//
//	// Error handling function
//	type ErrorHandler[C Context] func(ctx C, err error)
//
//	// Middleware function for handler composition
//	type Middleware[C Context] func(next HandlerFunc[C]) HandlerFunc[C]
//
// # Context Interface
//
// The Context interface extends Go's standard context.Context with HTTP-specific methods:
//
//	type Context interface {
//		context.Context                    // Standard context methods
//		Request() *http.Request           // Access to HTTP request
//		ResponseWriter() http.ResponseWriter // Access to response writer
//		Param(key string) string          // Get path parameters
//		SetValue(key, val any)           // Store request-scoped values
//	}
//
// # Basic Handler Implementation
//
// Create HTTP handlers using the type-safe HandlerFunc:
//
//	import (
//		"net/http"
//		"github.com/dmitrymomot/foundation/core/handler"
//	)
//
//	// Define a simple handler
//	func helloHandler(ctx handler.Context) handler.Response {
//		name := ctx.Param("name")
//		if name == "" {
//			name = "World"
//		}
//
//		return func(w http.ResponseWriter, r *http.Request) error {
//			w.Header().Set("Content-Type", "text/plain")
//			w.WriteHeader(http.StatusOK)
//			_, err := w.Write([]byte("Hello, " + name + "!"))
//			return err
//		}
//	}
//
// # Custom Context Implementation
//
// Implement the Context interface for your specific application needs:
//
//	import (
//		"context"
//		"net/http"
//	)
//
//	type AppContext struct {
//		context.Context
//		request  *http.Request
//		response http.ResponseWriter
//		params   map[string]string
//		values   map[any]any
//		userID   string // Application-specific field
//	}
//
//	func (c *AppContext) Request() *http.Request {
//		return c.request
//	}
//
//	func (c *AppContext) ResponseWriter() http.ResponseWriter {
//		return c.response
//	}
//
//	func (c *AppContext) Param(key string) string {
//		return c.params[key]
//	}
//
//	func (c *AppContext) SetValue(key, val any) {
//		if c.values == nil {
//			c.values = make(map[any]any)
//		}
//		c.values[key] = val
//	}
//
//	func (c *AppContext) Value(key any) any {
//		if val, ok := c.values[key]; ok {
//			return val
//		}
//		return c.Context.Value(key)
//	}
//
//	// Application-specific method
//	func (c *AppContext) UserID() string {
//		return c.userID
//	}
//
// # Handler with Custom Context
//
// Use your custom context in handlers:
//
//	func userProfileHandler(ctx *AppContext) handler.Response {
//		userID := ctx.UserID()
//		if userID == "" {
//			return unauthorizedResponse()
//		}
//
//		user, err := getUserByID(userID)
//		if err != nil {
//			return errorResponse(err)
//		}
//
//		return jsonResponse(user)
//	}
//
//	// Response helper functions
//	func jsonResponse(data any) handler.Response {
//		return func(w http.ResponseWriter, r *http.Request) error {
//			w.Header().Set("Content-Type", "application/json")
//			w.WriteHeader(http.StatusOK)
//			return json.NewEncoder(w).Encode(data)
//		}
//	}
//
//	func unauthorizedResponse() handler.Response {
//		return func(w http.ResponseWriter, r *http.Request) error {
//			w.WriteHeader(http.StatusUnauthorized)
//			return nil
//		}
//	}
//
// # Middleware Implementation
//
// Create reusable middleware that can be applied to handlers:
//
//	// Logging middleware
//	func LoggingMiddleware[C handler.Context]() handler.Middleware[C] {
//		return func(next handler.HandlerFunc[C]) handler.HandlerFunc[C] {
//			return func(ctx C) handler.Response {
//				start := time.Now()
//				req := ctx.Request()
//
//				log.Printf("Started %s %s", req.Method, req.URL.Path)
//
//				response := next(ctx)
//
//				return func(w http.ResponseWriter, r *http.Request) error {
//					err := response(w, r)
//					duration := time.Since(start)
//					log.Printf("Completed %s %s in %v", req.Method, req.URL.Path, duration)
//					return err
//				}
//			}
//		}
//	}
//
//	// Authentication middleware
//	func AuthMiddleware[C *AppContext]() handler.Middleware[C] {
//		return func(next handler.HandlerFunc[C]) handler.HandlerFunc[C] {
//			return func(ctx C) handler.Response {
//				token := ctx.Request().Header.Get("Authorization")
//				if token == "" {
//					return unauthorizedResponse()
//				}
//
//				userID, err := validateToken(token)
//				if err != nil {
//					return unauthorizedResponse()
//				}
//
//				ctx.userID = userID
//				return next(ctx)
//			}
//		}
//	}
//
// # Middleware Composition
//
// Compose multiple middleware functions together:
//
//	func applyMiddleware[C handler.Context](
//		h handler.HandlerFunc[C],
//		middlewares ...handler.Middleware[C],
//	) handler.HandlerFunc[C] {
//		// Apply middleware in reverse order
//		for i := len(middlewares) - 1; i >= 0; i-- {
//			h = middlewares[i](h)
//		}
//		return h
//	}
//
//	// Usage
//	finalHandler := applyMiddleware(
//		userProfileHandler,
//		LoggingMiddleware[*AppContext](),
//		AuthMiddleware[*AppContext](),
//	)
//
// # Error Handling
//
// Implement error handlers for consistent error processing:
//
//	func defaultErrorHandler(ctx *AppContext, err error) {
//		log.Printf("Handler error: %v", err)
//
//		w := ctx.ResponseWriter()
//		switch {
//		case errors.Is(err, ErrNotFound):
//			w.WriteHeader(http.StatusNotFound)
//			json.NewEncoder(w).Encode(map[string]string{
//				"error": "Resource not found",
//			})
//		case errors.Is(err, ErrUnauthorized):
//			w.WriteHeader(http.StatusUnauthorized)
//			json.NewEncoder(w).Encode(map[string]string{
//				"error": "Unauthorized access",
//			})
//		default:
//			w.WriteHeader(http.StatusInternalServerError)
//			json.NewEncoder(w).Encode(map[string]string{
//				"error": "Internal server error",
//			})
//		}
//	}
//
// # Router Integration
//
// Integrate with HTTP routers by converting handler functions:
//
//	import "github.com/gorilla/mux"
//
//	func wrapHandler[C handler.Context](
//		h handler.HandlerFunc[C],
//		createContext func(*http.Request, http.ResponseWriter) C,
//		errorHandler handler.ErrorHandler[C],
//	) http.HandlerFunc {
//		return func(w http.ResponseWriter, r *http.Request) {
//			ctx := createContext(r, w)
//			response := h(ctx)
//			if err := response(w, r); err != nil {
//				errorHandler(ctx, err)
//			}
//		}
//	}
//
//	// Usage with gorilla/mux
//	func setupRoutes() *mux.Router {
//		r := mux.NewRouter()
//
//		createAppContext := func(req *http.Request, w http.ResponseWriter) *AppContext {
//			return &AppContext{
//				Context:  req.Context(),
//				request:  req,
//				response: w,
//				params:   mux.Vars(req),
//			}
//		}
//
//		r.HandleFunc("/users/{id}",
//			wrapHandler(userProfileHandler, createAppContext, defaultErrorHandler),
//		).Methods("GET")
//
//		return r
//	}
//
// # Advanced Response Patterns
//
// Implement sophisticated response patterns:
//
//	// Conditional response based on Accept header
//	func adaptiveResponse(data any) handler.Response {
//		return func(w http.ResponseWriter, r *http.Request) error {
//			accept := r.Header.Get("Accept")
//			switch {
//			case strings.Contains(accept, "application/json"):
//				w.Header().Set("Content-Type", "application/json")
//				return json.NewEncoder(w).Encode(data)
//			case strings.Contains(accept, "text/html"):
//				w.Header().Set("Content-Type", "text/html")
//				return renderHTML(w, data)
//			default:
//				w.Header().Set("Content-Type", "application/json")
//				return json.NewEncoder(w).Encode(data)
//			}
//		}
//	}
//
//	// Streaming response
//	func streamingResponse(data <-chan []byte) handler.Response {
//		return func(w http.ResponseWriter, r *http.Request) error {
//			w.Header().Set("Content-Type", "text/plain")
//			w.Header().Set("Transfer-Encoding", "chunked")
//			w.WriteHeader(http.StatusOK)
//
//			flusher := w.(http.Flusher)
//			for chunk := range data {
//				if _, err := w.Write(chunk); err != nil {
//					return err
//				}
//				flusher.Flush()
//			}
//			return nil
//		}
//	}
//
// # Testing Handlers
//
// The clean separation makes handlers easy to test:
//
//	import (
//		"net/http"
//		"net/http/httptest"
//		"testing"
//	)
//
//	func TestUserProfileHandler(t *testing.T) {
//		// Create test context
//		req := httptest.NewRequest("GET", "/users/123", nil)
//		w := httptest.NewRecorder()
//
//		ctx := &AppContext{
//			Context:  req.Context(),
//			request:  req,
//			response: w,
//			params:   map[string]string{"id": "123"},
//			userID:   "123",
//		}
//
//		// Execute handler
//		response := userProfileHandler(ctx)
//		err := response(w, req)
//
//		// Assert results
//		assert.NoError(t, err)
//		assert.Equal(t, http.StatusOK, w.Code)
//		assert.Contains(t, w.Body.String(), "user_id")
//	}
//
// # Best Practices
//
//   - Keep handlers focused on business logic, not HTTP concerns
//   - Use middleware for cross-cutting concerns like logging, auth, CORS
//   - Implement custom context types for application-specific data
//   - Handle errors consistently using error handlers
//   - Compose middleware in logical order (logging first, auth second, etc.)
//   - Test handlers in isolation using mock contexts
//   - Use response helper functions for common response patterns
//   - Keep context interfaces minimal and focused
//   - Prefer composition over inheritance for middleware functionality
package handler
