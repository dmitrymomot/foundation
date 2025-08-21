package middleware

import (
	"net/http"

	"github.com/dmitrymomot/gokit/core/handler"
	"github.com/dmitrymomot/gokit/core/response"
	"github.com/dmitrymomot/gokit/pkg/clientip"
)

// clientIPContextKey is used as a key for storing client IP in request context.
type clientIPContextKey struct{}

// ClientIPConfig configures the client IP extraction middleware.
type ClientIPConfig struct {
	// Skip defines a function to skip middleware execution for specific requests
	Skip func(ctx handler.Context) bool
	// StoreInContext determines whether to store the extracted IP in request context
	StoreInContext bool
	// HeaderName specifies the response header name for the client IP (default: "X-Client-IP")
	HeaderName string
	// StoreInHeader determines whether to include the IP in response headers
	StoreInHeader bool
	// ValidateFunc allows custom validation of the extracted IP address
	ValidateFunc func(ctx handler.Context, ip string) error
}

// ClientIP creates a client IP extraction middleware with default configuration.
// By default, it stores the extracted IP in the request context.
func ClientIP[C handler.Context]() handler.Middleware[C] {
	return ClientIPWithConfig[C](ClientIPConfig{
		StoreInContext: true,
	})
}

// ClientIPWithConfig creates a client IP extraction middleware with custom configuration.
// It extracts the real client IP address from various headers (X-Forwarded-For, X-Real-IP, etc.)
// and optionally stores it in context, validates it, or includes it in response headers.
func ClientIPWithConfig[C handler.Context](cfg ClientIPConfig) handler.Middleware[C] {
	if cfg.HeaderName == "" {
		cfg.HeaderName = "X-Client-IP"
	}

	// Default to storing in context if no other action is configured
	if !cfg.StoreInContext && !cfg.StoreInHeader && cfg.ValidateFunc == nil {
		cfg.StoreInContext = true
	}

	return func(next handler.HandlerFunc[C]) handler.HandlerFunc[C] {
		return func(ctx C) handler.Response {
			if cfg.Skip != nil && cfg.Skip(ctx) {
				return next(ctx)
			}

			ip := clientip.GetIP(ctx.Request())

			if cfg.StoreInContext {
				ctx.SetValue(clientIPContextKey{}, ip)
			}

			if cfg.ValidateFunc != nil {
				if err := cfg.ValidateFunc(ctx, ip); err != nil {
					return response.Error(response.ErrForbidden.WithError(err))
				}
			}

			resp := next(ctx)

			if cfg.StoreInHeader {
				return func(w http.ResponseWriter, r *http.Request) error {
					w.Header().Set(cfg.HeaderName, ip)
					return resp(w, r)
				}
			}

			return resp
		}
	}
}

// GetClientIP retrieves the client IP address from the request context.
// Returns the IP address and a boolean indicating whether it was found.
func GetClientIP(ctx handler.Context) (string, bool) {
	ip, ok := ctx.Value(clientIPContextKey{}).(string)
	return ip, ok
}
