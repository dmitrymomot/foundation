// Package middleware provides HTTP middleware components for common cross-cutting
// concerns in web applications. It offers type-safe, composable middleware that
// integrates with the foundation framework's handler.Context interface.
//
// # Available Middleware
//
// This package includes the following middleware:
//
//   - ClientIP: Extracts real client IP addresses from proxy headers
//   - CORS: Handles Cross-Origin Resource Sharing headers and preflight requests
//   - Fingerprint: Generates device fingerprints for security and analytics
//   - I18n: Provides internationalization support with automatic language detection
//   - JWT: Validates JWT tokens and extracts claims for authentication
//   - RateLimit: Implements request rate limiting with token bucket algorithm
//   - RequestID: Generates unique request identifiers for tracing
//   - SecureHeaders: Sets security-focused HTTP response headers
//
// # Common Patterns
//
// All middleware follow consistent patterns:
//
//   - Generic functions with handler.Context type parameters
//   - Basic constructor functions (e.g., JWT[C](), ClientIP[C]())
//   - Advanced WithConfig constructors for custom configuration
//   - Context helper functions for retrieving and storing values (e.g., GetClientIP(), SetTranslator())
//   - Optional skip conditions, validation functions, and error handlers
//
// # Basic Usage
//
// Most middleware can be used with minimal configuration:
//
//	import "github.com/dmitrymomot/foundation/middleware"
//
//	// Basic usage with default configuration
//	app.Use(middleware.RequestID[*YourContext]())
//	app.Use(middleware.ClientIP[*YourContext]())
//	app.Use(middleware.JWT[*YourContext]("your-secret-key"))
//
//	// Retrieve values in handlers
//	func handler(ctx *YourContext) handler.Response {
//		if requestID, ok := middleware.GetRequestID(ctx); ok {
//			// Use request ID for logging
//		}
//		if claims, ok := middleware.GetStandardClaims(ctx); ok {
//			// Use JWT claims for authorization
//		}
//		return response.JSON(map[string]any{"status": "ok"})
//	}
//
// # Advanced Configuration
//
// Use WithConfig constructors for advanced customization:
//
//	app.Use(middleware.ClientIPWithConfig[*YourContext](middleware.ClientIPConfig{
//		StoreInContext: true,
//		StoreInHeader:  true,
//		HeaderName:     "X-Client-IP",
//		Skip: func(ctx handler.Context) bool {
//			return strings.HasPrefix(ctx.Request().URL.Path, "/health")
//		},
//	}))
//
// # Documentation
//
// For detailed configuration options, examples, and API documentation for each
// middleware, see the individual middleware source files or use go doc:
//
//	go doc github.com/dmitrymomot/foundation/middleware.JWT
//	go doc github.com/dmitrymomot/foundation/middleware.RateLimit
//	go doc -all github.com/dmitrymomot/foundation/middleware
package middleware
