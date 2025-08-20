package middleware

import (
	"net/http"

	"github.com/dmitrymomot/gokit/core/handler"
	"github.com/google/uuid"
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
func RequestID[C handler.Context]() handler.Middleware[C] {
	return RequestIDWithConfig[C](RequestIDConfig{})
}

// RequestIDWithConfig creates a request ID middleware with custom configuration.
// It assigns a unique identifier to each request for tracing and logging purposes.
// The ID is stored in context and added to response headers.
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
func GetRequestID(ctx handler.Context) (string, bool) {
	id, ok := ctx.Value(requestIDContextKey{}).(string)
	return id, ok
}
