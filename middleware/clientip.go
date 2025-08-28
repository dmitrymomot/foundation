package middleware

import (
	"net/http"

	"github.com/dmitrymomot/foundation/core/handler"
	"github.com/dmitrymomot/foundation/core/response"
	"github.com/dmitrymomot/foundation/pkg/clientip"
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
//
// This middleware automatically extracts the real client IP address from various
// proxy headers (X-Forwarded-For, X-Real-IP, CF-Connecting-IP) and the direct
// connection. Use this for logging, rate limiting, geolocation, or security features.
//
// Usage:
//
//	// Apply to all routes for IP tracking
//	r.Use(middleware.ClientIP[*MyContext]())
//
//	// Use IP in handlers
//	func handleRequest(ctx *MyContext) handler.Response {
//		clientIP, ok := middleware.GetClientIP(ctx)
//		if !ok {
//			// Fallback to request's RemoteAddr
//			clientIP = ctx.Request().RemoteAddr
//		}
//
//		log.Printf("Request from IP: %s", clientIP)
//		return response.JSON(map[string]string{"ip": clientIP})
//	}
//
// The middleware handles these scenarios:
// - Direct connections: Uses connection's remote address
// - Behind reverse proxy: Extracts from X-Forwarded-For, X-Real-IP
// - Behind Cloudflare: Uses CF-Connecting-IP header
// - Multiple proxies: Parses X-Forwarded-For chain to find original client
//
// Common use cases:
// - Rate limiting by IP address
// - Geolocation-based content
// - Security monitoring and logging
// - IP-based access control
func ClientIP[C handler.Context]() handler.Middleware[C] {
	return ClientIPWithConfig[C](ClientIPConfig{
		StoreInContext: true,
	})
}

// ClientIPWithConfig creates a client IP extraction middleware with custom configuration.
// It extracts the real client IP address from various headers (X-Forwarded-For, X-Real-IP, etc.)
// and optionally stores it in context, validates it, or includes it in response headers.
//
// Advanced Usage Examples:
//
//	// Add IP to response headers for debugging
//	cfg := middleware.ClientIPConfig{
//		StoreInContext: true,
//		StoreInHeader:  true,
//		HeaderName:     "X-Client-IP", // Custom header name
//	}
//	r.Use(middleware.ClientIPWithConfig[*MyContext](cfg))
//
//	// IP validation for security (block known bad IPs)
//	blockedIPs := map[string]bool{
//		"192.168.1.100": true,
//		"10.0.0.50":     true,
//	}
//	cfg := middleware.ClientIPConfig{
//		StoreInContext: true,
//		ValidateFunc: func(ctx handler.Context, ip string) error {
//			if blockedIPs[ip] {
//				return fmt.Errorf("IP %s is blocked", ip)
//			}
//			return nil
//		},
//	}
//	r.Use(middleware.ClientIPWithConfig[*MyContext](cfg))
//
//	// Skip IP extraction for health checks
//	cfg := middleware.ClientIPConfig{
//		StoreInContext: true,
//		Skip: func(ctx handler.Context) bool {
//			return ctx.Request().URL.Path == "/health"
//		},
//	}
//
//	// IP geolocation validation (require specific countries)
//	cfg := middleware.ClientIPConfig{
//		StoreInContext: true,
//		ValidateFunc: func(ctx handler.Context, ip string) error {
//			// Use your preferred geolocation service
//			country, err := geoip.GetCountry(ip)
//			if err != nil {
//				return err
//			}
//			allowed := []string{"US", "CA", "GB"}
//			for _, c := range allowed {
//				if country == c {
//					return nil
//				}
//			}
//			return fmt.Errorf("access not allowed from %s", country)
//		},
//	}
//
// Configuration options:
// - StoreInContext: Store IP in request context for handler access
// - StoreInHeader: Include IP in response headers (useful for debugging)
// - ValidateFunc: Custom IP validation (security, geolocation, etc.)
// - Skip: Skip processing for specific requests (health checks, etc.)
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
//
// Usage in handlers:
//
//	func handleLogin(ctx *MyContext) handler.Response {
//		clientIP, ok := middleware.GetClientIP(ctx)
//		if !ok {
//			// Fallback to connection's remote address
//			clientIP = ctx.Request().RemoteAddr
//		}
//
//		// Log security events with IP
//		log.Printf("Login attempt from IP: %s", clientIP)
//
//		// Check rate limiting by IP
//		if rateLimiter.IsBlocked(clientIP) {
//			return response.Error(response.ErrTooManyRequests)
//		}
//
//		// Store IP in audit log
//		auditLog.Record("login", map[string]any{
//			"ip":    clientIP,
//			"user":  userID,
//			"time":  time.Now(),
//		})
//
//		return response.JSON(map[string]string{"status": "ok"})
//	}
//
// The IP address returned is the real client IP, not a proxy IP.
// It's extracted from proxy headers when the application is behind
// load balancers, CDNs, or reverse proxies.
func GetClientIP(ctx handler.Context) (string, bool) {
	ip, ok := ctx.Value(clientIPContextKey{}).(string)
	return ip, ok
}
