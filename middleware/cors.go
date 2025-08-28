package middleware

import (
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"

	"github.com/dmitrymomot/gokit/core/handler"
)

// CORSConfig defines configuration options for CORS middleware.
// It provides fine-grained control over Cross-Origin Resource Sharing policies.
type CORSConfig struct {
	// Skip allows bypassing CORS handling for specific requests
	Skip func(ctx handler.Context) bool

	// AllowOrigins specifies allowed origins. Use "*" for all origins.
	// If empty, defaults to allowing all origins ("*")
	AllowOrigins []string

	// AllowMethods specifies allowed HTTP methods.
	// If empty, defaults to GET, HEAD, PUT, PATCH, POST, DELETE
	AllowMethods []string

	// AllowHeaders specifies allowed request headers.
	// If empty, defaults to common headers including Authorization and Content-Type
	AllowHeaders []string

	// ExposeHeaders specifies which headers are exposed to the client
	ExposeHeaders []string

	// AllowCredentials indicates whether credentials (cookies, authorization headers)
	// are allowed. Cannot be used with wildcard origins for security reasons
	AllowCredentials bool

	// MaxAge specifies how long preflight requests can be cached (in seconds)
	MaxAge int

	// AllowOriginFunc provides custom origin validation logic.
	// Takes precedence over AllowOrigins when set.
	// Returns the allowed origin value and whether the origin is allowed
	AllowOriginFunc func(origin string) (string, bool)
}

// CORS returns a CORS middleware with default configuration.
// Default behavior allows all origins (*), common HTTP methods, and standard headers.
//
// Cross-Origin Resource Sharing (CORS) enables web applications running at one
// domain to access resources from another domain. This middleware handles CORS
// preflight requests and adds appropriate headers to responses.
//
// Usage:
//
//	// Allow all origins (development only)
//	r.Use(middleware.CORS[*MyContext]())
//
//	// Production usage with specific configuration
//	r.Use(middleware.CORSWithConfig[*MyContext](middleware.CORSConfig{
//		AllowOrigins: []string{"https://myapp.com", "https://api.myapp.com"},
//		AllowMethods: []string{"GET", "POST", "PUT", "DELETE"},
//		AllowHeaders: []string{"Content-Type", "Authorization"},
//		AllowCredentials: true,
//		MaxAge: 86400, // 24 hours
//	}))
//
// Default configuration:
// - AllowOrigins: ["*"] (all origins)
// - AllowMethods: ["GET", "HEAD", "PUT", "PATCH", "POST", "DELETE"]
// - AllowHeaders: ["Accept", "Content-Type", "Authorization", ...]
// - AllowCredentials: false (not compatible with wildcard origins)
// - MaxAge: 0 (no caching of preflight requests)
//
// SECURITY NOTE: The default wildcard (*) origin should only be used in
// development. Production applications should specify exact allowed origins.
func CORS[C handler.Context]() handler.Middleware[C] {
	return CORSWithConfig[C](CORSConfig{})
}

// CORSWithConfig returns a CORS middleware with custom configuration.
// Handles both preflight OPTIONS requests and actual CORS requests.
// For security, credentials are only allowed with specific origins (not wildcards).
//
// Advanced Usage Examples:
//
//	// Production API with specific origins and credentials
//	cfg := middleware.CORSConfig{
//		AllowOrigins: []string{
//			"https://myapp.com",
//			"https://admin.myapp.com",
//			"https://mobile.myapp.com",
//		},
//		AllowMethods: []string{"GET", "POST", "PUT", "DELETE"},
//		AllowHeaders: []string{
//			"Content-Type",
//			"Authorization",
//			"X-Requested-With",
//			"X-API-Key",
//		},
//		ExposeHeaders: []string{
//			"X-Total-Count",
//			"X-Request-ID",
//		},
//		AllowCredentials: true,
//		MaxAge:          86400, // Cache preflight for 24 hours
//	}
//	r.Use(middleware.CORSWithConfig[*MyContext](cfg))
//
//	// Dynamic origin validation with custom logic
//	cfg := middleware.CORSConfig{
//		AllowOriginFunc: func(origin string) (string, bool) {
//			// Parse origin URL
//			u, err := url.Parse(origin)
//			if err != nil {
//				return "", false
//			}
//
//			// Allow all subdomains of myapp.com
//			if strings.HasSuffix(u.Host, ".myapp.com") || u.Host == "myapp.com" {
//				return origin, true
//			}
//
//			// Allow localhost for development
//			if strings.HasPrefix(u.Host, "localhost:") {
//				return origin, true
//			}
//
//			return "", false
//		},
//		AllowCredentials: true,
//	}
//
//	// Skip CORS for same-origin requests
//	cfg := middleware.CORSConfig{
//		Skip: func(ctx handler.Context) bool {
//			// Skip if no Origin header (same-origin request)
//			return ctx.Request().Header.Get("Origin") == ""
//		},
//		AllowOrigins: []string{"https://api.myapp.com"},
//	}
//
// Common patterns:
// - Microservices: Allow specific service origins with credentials
// - Public APIs: Use wildcard origins without credentials
// - Development: Allow localhost with any port
// - Mobile apps: Include custom headers for app identification
func CORSWithConfig[C handler.Context](cfg CORSConfig) handler.Middleware[C] {
	if len(cfg.AllowMethods) == 0 {
		cfg.AllowMethods = []string{
			http.MethodGet,
			http.MethodHead,
			http.MethodPut,
			http.MethodPatch,
			http.MethodPost,
			http.MethodDelete,
		}
	}

	if len(cfg.AllowHeaders) == 0 {
		cfg.AllowHeaders = []string{
			"Accept",
			"Accept-Language",
			"Content-Language",
			"Content-Type",
			"Origin",
			"Authorization",
			"X-Request-ID",
		}
	}

	allowMethods := strings.Join(cfg.AllowMethods, ",")
	allowHeaders := strings.Join(cfg.AllowHeaders, ",")
	exposeHeaders := strings.Join(cfg.ExposeHeaders, ",")

	// Pre-build origin lookup map for O(1) validation to avoid O(n) slice search on each request
	allowOriginsMap := make(map[string]bool, len(cfg.AllowOrigins))
	for _, origin := range cfg.AllowOrigins {
		allowOriginsMap[origin] = true
	}

	return func(next handler.HandlerFunc[C]) handler.HandlerFunc[C] {
		return func(ctx C) handler.Response {
			if cfg.Skip != nil && cfg.Skip(ctx) {
				return next(ctx)
			}

			req := ctx.Request()
			origin := req.Header.Get("Origin")

			var allowedOrigin string
			allowed := false

			// Origin validation priority: custom function > wildcard/empty > explicit list
			if cfg.AllowOriginFunc != nil {
				allowedOrigin, allowed = cfg.AllowOriginFunc(origin)
			} else if len(cfg.AllowOrigins) == 0 || allowOriginsMap["*"] {
				allowedOrigin = "*"
				allowed = true
			} else if allowOriginsMap[origin] {
				allowedOrigin = origin
				allowed = true
			}

			// CORS preflight request detection: OPTIONS method + Access-Control-Request-Method header
			isPreflight := req.Method == http.MethodOptions &&
				req.Header.Get("Access-Control-Request-Method") != ""

			if isPreflight {
				requestMethod := req.Header.Get("Access-Control-Request-Method")
				requestHeaders := req.Header.Get("Access-Control-Request-Headers")
				methodAllowed := slices.Contains(cfg.AllowMethods, requestMethod)

				if !allowed || !methodAllowed {
					return func(w http.ResponseWriter, r *http.Request) error {
						w.WriteHeader(http.StatusForbidden)
						return nil
					}
				}

				return func(w http.ResponseWriter, r *http.Request) error {
					headers := w.Header()
					headers.Set("Access-Control-Allow-Origin", allowedOrigin)
					headers.Set("Access-Control-Allow-Methods", allowMethods)

					if requestHeaders != "" {
						headers.Set("Access-Control-Allow-Headers", allowHeaders)
					}

					// Security requirement: credentials must not be allowed with wildcard origins
					// per CORS spec to prevent credential leakage to any origin
					if cfg.AllowCredentials && allowedOrigin != "*" {
						headers.Set("Access-Control-Allow-Credentials", "true")
					}

					if cfg.MaxAge > 0 {
						headers.Set("Access-Control-Max-Age", strconv.Itoa(cfg.MaxAge))
					}

					// Vary headers inform caches that response differs based on these request headers
					headers.Add("Vary", "Origin")
					headers.Add("Vary", "Access-Control-Request-Method")
					headers.Add("Vary", "Access-Control-Request-Headers")

					w.WriteHeader(http.StatusNoContent)
					return nil
				}
			}

			response := next(ctx)

			if !allowed {
				return response
			}

			return func(w http.ResponseWriter, r *http.Request) error {
				headers := w.Header()
				headers.Set("Access-Control-Allow-Origin", allowedOrigin)

				// Security requirement: credentials must not be allowed with wildcard origins
				// per CORS spec to prevent credential leakage to any origin
				if cfg.AllowCredentials && allowedOrigin != "*" {
					headers.Set("Access-Control-Allow-Credentials", "true")
				}

				if exposeHeaders != "" {
					headers.Set("Access-Control-Expose-Headers", exposeHeaders)
				}

				headers.Add("Vary", "Origin")

				return response(w, r)
			}
		}
	}
}

// AllowOriginWildcard returns an AllowOriginFunc that allows any origin except empty strings.
// This provides more flexibility than static wildcard ("*") as it returns the actual origin
// value, enabling proper credential handling while still allowing all origins.
func AllowOriginWildcard() func(origin string) (string, bool) {
	return func(origin string) (string, bool) {
		if origin == "" {
			return "", false
		}
		return origin, true
	}
}

// AllowOriginSubdomain returns an AllowOriginFunc that allows requests from the specified
// domain and all its subdomains. Handles domains with and without ports correctly.
// The domain parameter should be provided without protocol (e.g., "example.com").
// Supports both exact matches and subdomain matches (e.g., "api.example.com").
func AllowOriginSubdomain(domain string) func(origin string) (string, bool) {
	domain = strings.TrimPrefix(domain, "*.")
	domain = strings.TrimPrefix(domain, ".")
	domain = strings.ToLower(domain)
	domainWithDot := "." + domain

	return func(origin string) (string, bool) {
		if origin == "" {
			return "", false
		}

		u, err := url.Parse(origin)
		if err != nil || u.Host == "" {
			return "", false
		}

		host := strings.ToLower(u.Host)

		// Exact domain match (e.g., "example.com")
		if host == domain {
			return origin, true
		}

		// Subdomain match (e.g., "api.example.com" matches ".example.com")
		if strings.HasSuffix(host, domainWithDot) {
			return origin, true
		}

		// Handle hosts with ports (e.g., "example.com:8080" or "api.example.com:3000")
		portIndex := strings.LastIndex(host, ":")
		if portIndex > 0 {
			hostWithoutPort := host[:portIndex]
			if hostWithoutPort == domain || strings.HasSuffix(hostWithoutPort, domainWithDot) {
				return origin, true
			}
		}

		return "", false
	}
}
