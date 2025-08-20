// Package middleware provides HTTP middleware components for common cross-cutting concerns
// in web applications. It offers a comprehensive suite of middleware for client IP extraction,
// device fingerprinting, JWT authentication, rate limiting, and request ID generation.
//
// The middleware package is designed to work with the handler.Context interface from the
// gokit framework, providing type-safe, composable middleware that can be easily chained
// together to build robust HTTP services.
//
// # Architecture
//
// All middleware functions follow a consistent pattern:
//   - Generic functions that accept a handler.Context type parameter
//   - Configuration structs for customization
//   - Default constructors for common use cases
//   - WithConfig constructors for advanced configuration
//   - Context helpers for retrieving stored values
//
// Each middleware can be configured to:
//   - Skip execution based on custom logic
//   - Store extracted data in request context
//   - Include information in response headers
//   - Validate or transform data before processing
//
// # Client IP Middleware
//
// The ClientIP middleware extracts the real client IP address from various headers,
// handling proxy forwarding scenarios correctly.
//
//	import "github.com/dmitrymomot/gokit/middleware"
//
//	// Basic usage - stores IP in context
//	app.Use(middleware.ClientIP[*YourContext]())
//
//	// Advanced configuration
//	app.Use(middleware.ClientIPWithConfig[*YourContext](middleware.ClientIPConfig{
//		StoreInContext: true,
//		StoreInHeader:  true,
//		HeaderName:     "X-Client-IP",
//		ValidateFunc: func(ctx handler.Context, ip string) error {
//			if isBlocked(ip) {
//				return errors.New("IP address blocked")
//			}
//			return nil
//		},
//	}))
//
//	// Retrieve client IP in handlers
//	func handler(ctx *YourContext) handler.Response {
//		if ip, ok := middleware.GetClientIP(ctx); ok {
//			// Use the client IP
//		}
//		return response.JSON(map[string]any{"status": "ok"})
//	}
//
// # Device Fingerprinting Middleware
//
// The Fingerprint middleware generates unique device fingerprints based on request
// characteristics to help identify devices and detect suspicious activity.
//
//	// Basic usage
//	app.Use(middleware.Fingerprint[*YourContext]())
//
//	// Custom configuration
//	app.Use(middleware.FingerprintWithConfig[*YourContext](middleware.FingerprintConfig{
//		StoreInContext: true,
//		StoreInHeader:  true,
//		HeaderName:     "X-Device-Fingerprint",
//		ValidateFunc: func(ctx handler.Context, fingerprint string) error {
//			if isSuspicious(fingerprint) {
//				return errors.New("suspicious device detected")
//			}
//			return nil
//		},
//	}))
//
//	// Retrieve fingerprint in handlers
//	func handler(ctx *YourContext) handler.Response {
//		if fp, ok := middleware.GetFingerprint(ctx); ok {
//			// Use the device fingerprint
//		}
//		return response.JSON(map[string]any{"status": "ok"})
//	}
//
// # JWT Authentication Middleware
//
// The JWT middleware provides comprehensive JWT token validation with support for
// custom claims, multiple token extraction methods, and configurable error handling.
//
//	import "github.com/dmitrymomot/gokit/pkg/jwt"
//
//	// Basic usage with signing key
//	app.Use(middleware.JWT[*YourContext]("your-secret-key"))
//
//	// Advanced configuration with custom claims
//	type CustomClaims struct {
//		jwt.StandardClaims
//		UserID string `json:"user_id"`
//		Role   string `json:"role"`
//	}
//
//	jwtService, _ := jwt.NewFromString("your-secret-key")
//	app.Use(middleware.JWTWithConfig[*YourContext](middleware.JWTConfig{
//		Service:        jwtService,
//		StoreInContext: true,
//		ClaimsFactory: func() any {
//			return &CustomClaims{}
//		},
//		TokenExtractor: middleware.JWTFromMultiple(
//			middleware.JWTFromAuthHeader(),
//			middleware.JWTFromCookie("token"),
//			middleware.JWTFromQuery("token"),
//		),
//		ErrorHandler: func(ctx handler.Context, err error) handler.Response {
//			return response.JSONWithStatus(response.ErrUnauthorized, 401)
//		},
//	}))
//
//	// Retrieve JWT claims in handlers
//	func protectedHandler(ctx *YourContext) handler.Response {
//		// Get standard claims
//		if claims, ok := middleware.GetStandardClaims(ctx); ok {
//			userID := claims.Subject
//			// Use standard claims
//		}
//
//		// Get custom claims
//		if claims, ok := middleware.GetJWTClaims[*CustomClaims](ctx); ok {
//			userID := claims.UserID
//			role := claims.Role
//			// Use custom claims
//		}
//
//		return response.JSON(map[string]any{"status": "authenticated"})
//	}
//
// # Token Extraction Strategies
//
// The JWT middleware supports various token extraction methods:
//
//	// From Authorization header (Bearer scheme)
//	middleware.JWTFromAuthHeader()
//
//	// From custom header
//	middleware.JWTFromHeader("X-Auth-Token")
//
//	// From URL query parameter
//	middleware.JWTFromQuery("token")
//
//	// From HTTP cookie
//	middleware.JWTFromCookie("jwt_token")
//
//	// From form data
//	middleware.JWTFromForm("access_token")
//
//	// Multiple strategies (first match wins)
//	middleware.JWTFromMultiple(
//		middleware.JWTFromAuthHeader(),
//		middleware.JWTFromCookie("token"),
//		middleware.JWTFromQuery("token"),
//	)
//
// # Rate Limiting Middleware
//
// The RateLimit middleware provides configurable request rate limiting using
// token bucket algorithm with pluggable storage backends.
//
//	import "github.com/dmitrymomot/gokit/pkg/ratelimiter"
//
//	// Create rate limiter with memory storage
//	store := ratelimiter.NewMemoryStore()
//	limiter, _ := ratelimiter.NewBucket(store, ratelimiter.Config{
//		Capacity:     100,    // 100 requests
//		RefillRate:   10,     // 10 requests per second
//		RefillPeriod: time.Second,
//	})
//
//	// Basic usage
//	app.Use(middleware.RateLimit[*YourContext](middleware.RateLimitConfig{
//		Limiter:    limiter,
//		SetHeaders: true, // Include rate limit headers in response
//	}))
//
//	// Custom key extraction and error handling
//	app.Use(middleware.RateLimit[*YourContext](middleware.RateLimitConfig{
//		Limiter:    limiter,
//		SetHeaders: true,
//		KeyExtractor: func(ctx handler.Context) string {
//			// Rate limit by user ID instead of IP
//			if userID := getUserID(ctx); userID != "" {
//				return "user:" + userID
//			}
//			return "anonymous"
//		},
//		ErrorHandler: func(ctx handler.Context, result *ratelimiter.Result) handler.Response {
//			return response.JSONWithStatus(map[string]any{
//				"error":       "rate limit exceeded",
//				"retry_after": result.RetryAfter().Seconds(),
//			}, 429)
//		},
//	}))
//
// When rate limiting is active, the following headers are included in responses:
//   - X-RateLimit-Limit: Maximum number of requests allowed
//   - X-RateLimit-Remaining: Number of requests remaining in current window
//   - X-RateLimit-Reset: Unix timestamp when the rate limit resets
//   - Retry-After: Seconds to wait before retrying (when limit exceeded)
//
// # Request ID Middleware
//
// The RequestID middleware generates unique identifiers for each request to
// facilitate request tracing and correlation across distributed systems.
//
//	// Basic usage - generates UUID v4
//	app.Use(middleware.RequestID[*YourContext]())
//
//	// Custom configuration
//	app.Use(middleware.RequestIDWithConfig[*YourContext](middleware.RequestIDConfig{
//		HeaderName:  "X-Trace-ID",
//		UseExisting: true, // Use existing header if present
//		Generator: func() string {
//			// Custom ID generation
//			return "req_" + generateCustomID()
//		},
//	}))
//
//	// Retrieve request ID in handlers
//	func handler(ctx *YourContext) handler.Response {
//		if requestID, ok := middleware.GetRequestID(ctx); ok {
//			log.Printf("Processing request: %s", requestID)
//		}
//		return response.JSON(map[string]any{"status": "ok"})
//	}
//
// # Configuration Patterns
//
// All middleware support common configuration patterns:
//
//	// Skip middleware for specific requests
//	Skip: func(ctx handler.Context) bool {
//		return strings.HasPrefix(ctx.Request().URL.Path, "/health")
//	},
//
//	// Store data in request context
//	StoreInContext: true,
//
//	// Include data in response headers
//	StoreInHeader: true,
//
//	// Custom validation logic
//	ValidateFunc: func(ctx handler.Context, data string) error {
//		return validateData(data)
//	},
//
// # Error Handling
//
// The middleware package integrates with the response package error system:
//   - JWT middleware returns 401 Unauthorized for authentication failures
//   - Rate limiting returns 429 Too Many Requests when limits are exceeded
//   - Client IP validation returns 403 Forbidden for blocked IPs
//   - Fingerprint validation returns 400 Bad Request for suspicious devices
//
// Custom error handlers can be provided for fine-grained control over error responses.
//
// # Performance Considerations
//
// - Client IP extraction is optimized for common proxy configurations
// - Fingerprinting uses fast hashing algorithms for device identification
// - JWT validation uses constant-time comparison for security
// - Rate limiting supports efficient memory and Redis-based storage
// - Request ID generation uses cryptographically secure random sources
//
// # Best Practices
//
// 1. Order middleware appropriately:
//   - RequestID first for tracing
//   - ClientIP early for accurate IP extraction
//   - RateLimit before authentication to prevent abuse
//   - JWT authentication before business logic
//   - Fingerprint for additional security context
//
// 2. Configure appropriate skip conditions for health checks and static assets
// 3. Use secure signing keys for JWT middleware in production
// 4. Choose appropriate rate limiting strategies based on your application needs
// 5. Store sensitive data only in context, not in response headers
// 6. Implement proper logging and monitoring for middleware failures
//
//	// Example middleware chain
//	app.Use(middleware.RequestID[*YourContext]())
//	app.Use(middleware.ClientIP[*YourContext]())
//	app.Use(middleware.RateLimit[*YourContext](rateLimitConfig))
//	app.Use(middleware.JWT[*YourContext]("secret"))
//	app.Use(middleware.Fingerprint[*YourContext]())
package middleware
