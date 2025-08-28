package middleware

import (
	"net/http"

	"github.com/google/uuid"

	"github.com/dmitrymomot/foundation/core/handler"
)

// requestIDContextKey is used as a key for storing request ID in request context.
type requestIDContextKey struct{}

// RequestIDConfig configures the request ID middleware.
type RequestIDConfig struct {
	// Skip defines a function to skip middleware execution for specific requests
	Skip func(ctx handler.Context) bool
	// Generator creates new request IDs (default: UUID v4)
	Generator func() string
	// HeaderName specifies the header name for the request ID (default: "X-Request-ID")
	HeaderName string
	// UseExisting determines whether to use an existing request ID from the incoming request
	UseExisting bool
}

// RequestID creates a request ID middleware with default configuration.
// It generates a new UUID for each request and includes it in both context and response headers.
//
// Request IDs are essential for distributed systems and debugging. They provide
// a unique identifier for each HTTP request that can be used for:
// - Correlation across microservices
// - Log aggregation and search
// - Error tracking and debugging
// - Performance monitoring
// - Support ticket investigation
//
// Usage:
//
//	// Apply to all routes for request tracking
//	r.Use(middleware.RequestID[*MyContext]())
//
//	// Use request ID in handlers and logging
//	func handleRequest(ctx *MyContext) handler.Response {
//		requestID, ok := middleware.GetRequestID(ctx)
//		if !ok {
//			requestID = "unknown"
//		}
//
//		// Include in structured logging
//		logger := log.WithFields(log.Fields{
//			"request_id": requestID,
//			"user_id":    getUserID(ctx),
//		})
//
//		logger.Info("Processing user request")
//
//		// Pass to downstream services
//		response, err := userService.GetProfile(ctx, requestID)
//		if err != nil {
//			logger.Error("Failed to fetch user profile", "error", err)
//			return response.Error(response.ErrInternalServerError)
//		}
//
//		return response.JSON(response)
//	}
//
// The middleware automatically:
// - Generates a unique UUID v4 for each request
// - Stores the ID in request context for handler access
// - Adds X-Request-ID header to all responses
// - Enables request correlation across service boundaries
func RequestID[C handler.Context]() handler.Middleware[C] {
	return RequestIDWithConfig[C](RequestIDConfig{})
}

// RequestIDWithConfig creates a request ID middleware with custom configuration.
// It assigns a unique identifier to each request for tracing and logging purposes.
// The ID is stored in context and added to response headers.
//
// Advanced Usage Examples:
//
//	// Use existing request ID from incoming requests (for proxied requests)
//	cfg := middleware.RequestIDConfig{
//		UseExisting: true,
//		HeaderName:  "X-Request-ID",
//	}
//	r.Use(middleware.RequestIDWithConfig[*MyContext](cfg))
//
//	// Custom request ID generation (shorter IDs for logging)
//	cfg := middleware.RequestIDConfig{
//		Generator: func() string {
//			return fmt.Sprintf("%d-%s", time.Now().Unix(), randomString(6))
//		},
//		HeaderName: "X-Trace-ID",
//	}
//	r.Use(middleware.RequestIDWithConfig[*MyContext](cfg))
//
//	// Skip request ID generation for health checks
//	cfg := middleware.RequestIDConfig{
//		Skip: func(ctx handler.Context) bool {
//			path := ctx.Request().URL.Path
//			return path == "/health" || path == "/metrics"
//		},
//	}
//
//	// Snowflake-style IDs for high-throughput systems
//	snowflake := NewSnowflakeGenerator(machineID)
//	cfg := middleware.RequestIDConfig{
//		Generator: func() string {
//			return snowflake.Generate().String()
//		},
//		HeaderName: "X-Request-ID",
//	}
//
// Configuration options:
// - Generator: Custom ID generation function (default: UUID v4)
// - HeaderName: Response header name (default: "X-Request-ID")
// - UseExisting: Reuse incoming request ID if present
// - Skip: Skip processing for specific requests
//
// Request ID best practices:
// - Use consistent header names across services
// - Include request IDs in all log messages
// - Forward request IDs to downstream services
// - Store request IDs with user actions for support
func RequestIDWithConfig[C handler.Context](cfg RequestIDConfig) handler.Middleware[C] {
	if cfg.HeaderName == "" {
		cfg.HeaderName = "X-Request-ID"
	}

	if cfg.Generator == nil {
		cfg.Generator = func() string {
			return uuid.New().String()
		}
	}

	return func(next handler.HandlerFunc[C]) handler.HandlerFunc[C] {
		return func(ctx C) handler.Response {
			if cfg.Skip != nil && cfg.Skip(ctx) {
				return next(ctx)
			}

			var requestID string

			// Try to use existing request ID from incoming headers if configured
			if cfg.UseExisting {
				if existingID := ctx.Request().Header.Get(cfg.HeaderName); existingID != "" {
					requestID = existingID
				}
			}

			if requestID == "" {
				requestID = cfg.Generator()
			}

			ctx.SetValue(requestIDContextKey{}, requestID)

			response := next(ctx)

			return func(w http.ResponseWriter, r *http.Request) error {
				w.Header().Set(cfg.HeaderName, requestID)
				return response(w, r)
			}
		}
	}
}

// GetRequestID retrieves the request ID from the request context.
// Returns the request ID and a boolean indicating whether it was found.
//
// Usage in handlers:
//
//	func handleUserUpdate(ctx *MyContext) handler.Response {
//		requestID, ok := middleware.GetRequestID(ctx)
//		if !ok {
//			requestID = "missing"
//		}
//
//		// Create contextual logger with request ID
//		logger := log.WithField("request_id", requestID)
//
//		// Log business operations
//		logger.Info("Starting user update")
//
//		// Pass request ID to services for distributed tracing
//		ctx = context.WithValue(ctx, "request_id", requestID)
//		err := userService.UpdateUser(ctx, userID, updates)
//		if err != nil {
//			logger.Error("User update failed", "error", err, "user_id", userID)
//			return response.Error(response.ErrInternalServerError)
//		}
//
//		logger.Info("User update successful", "user_id", userID)
//
//		// Include request ID in success response for client correlation
//		return response.JSON(map[string]any{
//			"status":     "updated",
//			"user_id":    userID,
//			"request_id": requestID,
//		})
//	}
//
// The request ID enables:
// - Log correlation across multiple services
// - Error tracking and debugging workflows
// - Performance monitoring and APM integration
// - Customer support with specific request investigation
func GetRequestID(ctx handler.Context) (string, bool) {
	id, ok := ctx.Value(requestIDContextKey{}).(string)
	return id, ok
}
