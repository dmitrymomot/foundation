// Package middleware provides HTTP middleware components for common cross-cutting
// concerns in web applications. It offers type-safe, composable middleware that
// integrates with the foundation framework's handler.Context interface.
//
// # Available Middleware
//
// This package includes the following middleware:
//
//   - BodyLimit: Restricts request body size to prevent resource exhaustion
//   - ClientIP: Extracts real client IP addresses from proxy headers
//   - CORS: Handles Cross-Origin Resource Sharing headers and preflight requests
//   - Fingerprint: Generates device fingerprints for security and analytics
//   - I18n: Provides internationalization support with automatic language detection
//   - JWT: Validates JWT tokens and extracts claims for authentication
//   - Logging: Logs HTTP request and response details with structured logging
//   - RateLimit: Implements request rate limiting with token bucket algorithm
//   - RequestID: Generates unique request identifiers for tracing
//   - SecureHeaders: Sets security-focused HTTP response headers
//   - Session: Manages user sessions with automatic IP/UserAgent tracking and touch mechanism
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
// # Session Middleware
//
// The Session middleware automatically tracks client IP and User-Agent information:
//
//	// Session middleware with automatic tracking
//	app.Use(middleware.Session(transport))
//
//	// Sessions automatically include:
//	// - Client IP (via clientip.GetIP with proxy header support)
//	// - User-Agent string
//	// - Device identifier (via sess.Device() method)
//
//	// Access session information in handlers
//	func handler(ctx *YourContext) handler.Response {
//		sess := middleware.GetSession[SessionData](ctx)
//		log.Info("Request from",
//			"ip", sess.IP,
//			"device", sess.Device(),
//			"user_id", sess.UserID,
//		)
//		return response.JSON(map[string]any{"status": "ok"})
//	}
//
// For session hijacking detection and security monitoring examples,
// see the core/session package documentation.
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
