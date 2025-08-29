package middleware

import (
	"maps"
	"net/http"

	"github.com/dmitrymomot/foundation/core/handler"
)

// SecurityHeadersConfig configures the security headers middleware.
// It provides fine-grained control over HTTP security headers.
type SecurityHeadersConfig struct {
	// Skip defines a function to skip middleware execution for specific requests
	Skip func(ctx handler.Context) bool

	// ContentTypeOptions controls X-Content-Type-Options header
	ContentTypeOptions string

	// FrameOptions controls X-Frame-Options header
	FrameOptions string

	// XSSProtection controls X-XSS-Protection header
	XSSProtection string

	// StrictTransportSecurity controls Strict-Transport-Security header
	StrictTransportSecurity string

	// ContentSecurityPolicy controls Content-Security-Policy header
	ContentSecurityPolicy string

	// ReferrerPolicy controls Referrer-Policy header
	ReferrerPolicy string

	// PermissionsPolicy controls Permissions-Policy header
	PermissionsPolicy string

	// CrossOriginOpenerPolicy controls Cross-Origin-Opener-Policy header
	CrossOriginOpenerPolicy string

	// CrossOriginEmbedderPolicy controls Cross-Origin-Embedder-Policy header
	CrossOriginEmbedderPolicy string

	// CrossOriginResourcePolicy controls Cross-Origin-Resource-Policy header
	CrossOriginResourcePolicy string

	// CustomHeaders allows adding additional custom security headers
	CustomHeaders map[string]string

	// IsDevelopment disables HSTS and relaxes some policies for development
	IsDevelopment bool
}

// Predefined robust security configurations
var (
	// StrictSecurity provides maximum security with strict policies.
	// Use this for applications requiring highest security standards.
	StrictSecurity = SecurityHeadersConfig{
		ContentTypeOptions:        "nosniff",
		FrameOptions:              "DENY",
		XSSProtection:             "1; mode=block",
		StrictTransportSecurity:   "max-age=63072000; includeSubDomains; preload",
		ContentSecurityPolicy:     "default-src 'none'; script-src 'self'; style-src 'self'; img-src 'self'; font-src 'self'; connect-src 'self'; frame-ancestors 'none'; base-uri 'self'; form-action 'self'",
		ReferrerPolicy:            "no-referrer",
		PermissionsPolicy:         "accelerometer=(), camera=(), geolocation=(), gyroscope=(), magnetometer=(), microphone=(), payment=(), usb=()",
		CrossOriginOpenerPolicy:   "same-origin",
		CrossOriginEmbedderPolicy: "require-corp",
		CrossOriginResourcePolicy: "same-origin",
	}

	// BalancedSecurity provides good security with compatibility.
	// Use this for most web applications.
	BalancedSecurity = SecurityHeadersConfig{
		ContentTypeOptions:        "nosniff",
		FrameOptions:              "SAMEORIGIN",
		XSSProtection:             "1; mode=block",
		StrictTransportSecurity:   "max-age=31536000; includeSubDomains",
		ContentSecurityPolicy:     "default-src 'self'; script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline'; img-src 'self' data: https:; font-src 'self' data:",
		ReferrerPolicy:            "strict-origin-when-cross-origin",
		PermissionsPolicy:         "geolocation=(), microphone=(), camera=()",
		CrossOriginOpenerPolicy:   "same-origin-allow-popups",
		CrossOriginEmbedderPolicy: "",
		CrossOriginResourcePolicy: "cross-origin",
	}

	// RelaxedSecurity provides basic security for maximum compatibility.
	// Use this only when strict policies break functionality.
	RelaxedSecurity = SecurityHeadersConfig{
		ContentTypeOptions:        "nosniff",
		FrameOptions:              "",
		XSSProtection:             "1; mode=block",
		StrictTransportSecurity:   "",
		ContentSecurityPolicy:     "",
		ReferrerPolicy:            "strict-origin-when-cross-origin",
		PermissionsPolicy:         "",
		CrossOriginOpenerPolicy:   "",
		CrossOriginEmbedderPolicy: "",
		CrossOriginResourcePolicy: "",
	}

	// DevelopmentSecurity provides minimal security for local development.
	// WARNING: Never use in production.
	DevelopmentSecurity = SecurityHeadersConfig{
		ContentTypeOptions: "nosniff",
		XSSProtection:      "1; mode=block",
		ReferrerPolicy:     "strict-origin-when-cross-origin",
		IsDevelopment:      true,
	}
)

// SecurityHeaders creates a security headers middleware with balanced configuration.
// It adds comprehensive security headers to protect against common web vulnerabilities.
//
// This middleware adds essential HTTP security headers to protect against:
// - Cross-Site Scripting (XSS) attacks
// - Clickjacking and frame injection
// - MIME type confusion attacks
// - Protocol downgrade attacks
// - Information leakage via referrer headers
//
// Usage:
//
//	// Apply to all routes with balanced security (recommended for most apps)
//	r.Use(middleware.SecurityHeaders[*MyContext]())
//
//	// Or apply to specific route groups
//	api := r.Group("/api")
//	api.Use(middleware.SecurityHeaders[*MyContext]())
//
// The balanced configuration provides good security while maintaining compatibility
// with common web application patterns:
// - Allows same-origin iframe embedding
// - Permits inline styles and scripts (with CSP)
// - Enables HTTPS with reasonable HSTS duration
// - Allows essential browser APIs while blocking dangerous ones
//
// Headers included:
// - X-Content-Type-Options: nosniff
// - X-Frame-Options: SAMEORIGIN
// - X-XSS-Protection: 1; mode=block
// - Strict-Transport-Security: max-age=31536000; includeSubDomains
// - Content-Security-Policy: Balanced policy allowing self and inline content
// - Referrer-Policy: strict-origin-when-cross-origin
// - And more modern security headers
func SecurityHeaders[C handler.Context]() handler.Middleware[C] {
	return SecurityHeadersWithConfig[C](BalancedSecurity)
}

// SecurityHeadersStrict creates a security headers middleware with strict configuration.
// Use this for applications requiring maximum security.
//
// This configuration provides the highest level of protection but may break some
// functionality that relies on inline scripts, external resources, or iframe embedding.
//
// Usage:
//
//	// For high-security applications (banking, government, etc.)
//	r.Use(middleware.SecurityHeadersStrict[*MyContext]())
//
//	// Skip strict headers for specific routes that need relaxed policies
//	cfg := middleware.StrictSecurity
//	cfg.Skip = func(ctx handler.Context) bool {
//		return strings.HasPrefix(ctx.Request().URL.Path, "/embed/")
//	}
//	r.Use(middleware.SecurityHeadersWithConfig[*MyContext](cfg))
//
// Strict security features:
// - X-Frame-Options: DENY (no iframe embedding allowed)
// - Content-Security-Policy: Blocks all external resources and inline content
// - HSTS with preload directive for maximum transport security
// - Cross-Origin policies that isolate the application completely
// - Permissions-Policy that blocks all dangerous browser APIs
//
// IMPORTANT: Test thoroughly before deploying strict headers as they may break:
// - Third-party widgets (maps, payment processors, analytics)
// - Content management systems with inline editing
// - Applications using WebRTC, geolocation, or camera APIs
// - Legacy code that relies on inline scripts or styles
func SecurityHeadersStrict[C handler.Context]() handler.Middleware[C] {
	return SecurityHeadersWithConfig[C](StrictSecurity)
}

// SecurityHeadersRelaxed creates a security headers middleware with relaxed configuration.
// Use this when strict policies break required functionality.
//
// This configuration provides basic security while allowing more flexibility for
// applications that integrate with many third-party services or legacy systems.
//
// Usage:
//
//	// For development or applications with many integrations
//	r.Use(middleware.SecurityHeadersRelaxed[*MyContext]())
//
//	// Development mode (disables HSTS)
//	cfg := middleware.RelaxedSecurity
//	cfg.IsDevelopment = true
//	r.Use(middleware.SecurityHeadersWithConfig[*MyContext](cfg))
//
// Relaxed security features:
// - Allows iframe embedding from any origin
// - Permissive Content-Security-Policy that allows most content
// - Shorter HSTS duration
// - Less restrictive cross-origin policies
// - More permissive browser API access
//
// Use cases:
// - Development environments where strict policies interfere with tooling
// - Legacy applications that cannot be easily updated
// - Applications heavily relying on third-party content and widgets
// - Gradual security implementation (start relaxed, then tighten)
//
// WARNING: Relaxed headers provide minimal protection. Use only when necessary
// and work toward implementing stricter policies over time.
func SecurityHeadersRelaxed[C handler.Context]() handler.Middleware[C] {
	return SecurityHeadersWithConfig[C](RelaxedSecurity)
}

// SecurityHeadersWithConfig creates a security headers middleware with custom configuration.
// For most cases, use the predefined configurations (StrictSecurity, BalancedSecurity, etc.)
// instead of creating custom configs.
//
// Advanced Usage Examples:
//
//	// Custom CSP for applications using external CDNs
//	cfg := middleware.BalancedSecurity
//	cfg.ContentSecurityPolicy = "default-src 'self'; " +
//		"script-src 'self' https://cdn.jsdelivr.net https://unpkg.com; " +
//		"style-src 'self' 'unsafe-inline' https://fonts.googleapis.com; " +
//		"font-src 'self' https://fonts.gstatic.com; " +
//		"img-src 'self' data: https:;"
//	r.Use(middleware.SecurityHeadersWithConfig[*MyContext](cfg))
//
//	// Add custom security headers
//	cfg := middleware.BalancedSecurity
//	cfg.CustomHeaders = map[string]string{
//		"X-Application-Version": version.Get(),
//		"X-Rate-Limit-Policy":   "standard",
//		"X-API-Deprecation":     "v1 sunset 2024-12-31",
//	}
//	r.Use(middleware.SecurityHeadersWithConfig[*MyContext](cfg))
//
//	// Environment-specific configuration
//	cfg := middleware.BalancedSecurity
//	if os.Getenv("ENV") == "development" {
//		cfg.IsDevelopment = true // Disables HSTS
//		cfg.ContentSecurityPolicy = "" // Disable CSP in dev
//	}
//	r.Use(middleware.SecurityHeadersWithConfig[*MyContext](cfg))
//
//	// Skip security headers for specific paths
//	cfg := middleware.StrictSecurity
//	cfg.Skip = func(ctx handler.Context) bool {
//		path := ctx.Request().URL.Path
//		// Skip for health checks and metrics
//		return path == "/health" || path == "/metrics" ||
//			// Skip for webhook endpoints that need permissive policies
//			strings.HasPrefix(path, "/webhooks/")
//	}
//
// Configuration tips:
// - Start with BalancedSecurity and customize as needed
// - Test CSP policies thoroughly with browser developer tools
// - Use report-only CSP initially: 'Content-Security-Policy-Report-Only'
// - Monitor security headers with online scanners (securityheaders.com)
// - Set IsDevelopment=true in local environments to avoid HSTS issues
func SecurityHeadersWithConfig[C handler.Context](cfg SecurityHeadersConfig) handler.Middleware[C] {
	// Handle development mode - disable HSTS
	if cfg.IsDevelopment {
		cfg.StrictTransportSecurity = ""
	}

	// Pre-build headers map to avoid repeated checks
	headers := make(map[string]string)
	if cfg.ContentTypeOptions != "" {
		headers["X-Content-Type-Options"] = cfg.ContentTypeOptions
	}
	if cfg.FrameOptions != "" {
		headers["X-Frame-Options"] = cfg.FrameOptions
	}
	if cfg.XSSProtection != "" {
		headers["X-XSS-Protection"] = cfg.XSSProtection
	}
	if cfg.StrictTransportSecurity != "" {
		headers["Strict-Transport-Security"] = cfg.StrictTransportSecurity
	}
	if cfg.ContentSecurityPolicy != "" {
		headers["Content-Security-Policy"] = cfg.ContentSecurityPolicy
	}
	if cfg.ReferrerPolicy != "" {
		headers["Referrer-Policy"] = cfg.ReferrerPolicy
	}
	if cfg.PermissionsPolicy != "" {
		headers["Permissions-Policy"] = cfg.PermissionsPolicy
	}
	if cfg.CrossOriginOpenerPolicy != "" {
		headers["Cross-Origin-Opener-Policy"] = cfg.CrossOriginOpenerPolicy
	}
	if cfg.CrossOriginEmbedderPolicy != "" {
		headers["Cross-Origin-Embedder-Policy"] = cfg.CrossOriginEmbedderPolicy
	}
	if cfg.CrossOriginResourcePolicy != "" {
		headers["Cross-Origin-Resource-Policy"] = cfg.CrossOriginResourcePolicy
	}

	// Add custom headers
	maps.Copy(headers, cfg.CustomHeaders)

	return func(next handler.HandlerFunc[C]) handler.HandlerFunc[C] {
		return func(ctx C) handler.Response {
			if cfg.Skip != nil && cfg.Skip(ctx) {
				return next(ctx)
			}

			response := next(ctx)

			return func(w http.ResponseWriter, r *http.Request) error {
				// Apply all headers at once
				for key, value := range headers {
					w.Header().Set(key, value)
				}
				return response(w, r)
			}
		}
	}
}
