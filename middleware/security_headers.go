package middleware

import (
	"net/http"

	"github.com/dmitrymomot/gokit/core/handler"
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
func SecurityHeaders[C handler.Context]() handler.Middleware[C] {
	return SecurityHeadersWithConfig[C](BalancedSecurity)
}

// SecurityHeadersStrict creates a security headers middleware with strict configuration.
// Use this for applications requiring maximum security.
func SecurityHeadersStrict[C handler.Context]() handler.Middleware[C] {
	return SecurityHeadersWithConfig[C](StrictSecurity)
}

// SecurityHeadersRelaxed creates a security headers middleware with relaxed configuration.
// Use this when strict policies break required functionality.
func SecurityHeadersRelaxed[C handler.Context]() handler.Middleware[C] {
	return SecurityHeadersWithConfig[C](RelaxedSecurity)
}

// SecurityHeadersWithConfig creates a security headers middleware with custom configuration.
// For most cases, use the predefined configurations (StrictSecurity, BalancedSecurity, etc.)
// instead of creating custom configs.
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
	for k, v := range cfg.CustomHeaders {
		headers[k] = v
	}

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
