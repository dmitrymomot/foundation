package middleware

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/dmitrymomot/gokit/core/handler"
)

// SecurityHeadersConfig configures the security headers middleware.
// It provides fine-grained control over HTTP security headers.
type SecurityHeadersConfig struct {
	// Skip defines a function to skip middleware execution for specific requests
	Skip func(ctx handler.Context) bool

	// ContentTypeOptions controls X-Content-Type-Options header (default: "nosniff")
	ContentTypeOptions string

	// FrameOptions controls X-Frame-Options header (default: "DENY")
	FrameOptions string

	// XSSProtection controls X-XSS-Protection header (default: "1; mode=block")
	XSSProtection string

	// StrictTransportSecurity controls Strict-Transport-Security header
	// (default: "max-age=31536000; includeSubDomains")
	StrictTransportSecurity string

	// ContentSecurityPolicy controls Content-Security-Policy header
	// (default: "default-src 'self'")
	ContentSecurityPolicy string

	// ReferrerPolicy controls Referrer-Policy header (default: "strict-origin-when-cross-origin")
	ReferrerPolicy string

	// PermissionsPolicy controls Permissions-Policy header
	// (default: "geolocation=(), microphone=(), camera=()")
	PermissionsPolicy string

	// CrossOriginOpenerPolicy controls Cross-Origin-Opener-Policy header
	// (default: "same-origin")
	CrossOriginOpenerPolicy string

	// CrossOriginEmbedderPolicy controls Cross-Origin-Embedder-Policy header
	// (default: "require-corp")
	CrossOriginEmbedderPolicy string

	// CrossOriginResourcePolicy controls Cross-Origin-Resource-Policy header
	// (default: "same-origin")
	CrossOriginResourcePolicy string

	// CustomHeaders allows adding additional custom security headers
	CustomHeaders map[string]string

	// IsDevelopment disables HSTS and relaxes some policies for development
	IsDevelopment bool
}

// defaultSecurityConfig returns the default security headers configuration
func defaultSecurityConfig() SecurityHeadersConfig {
	return SecurityHeadersConfig{
		ContentTypeOptions:        "nosniff",
		FrameOptions:              "DENY",
		XSSProtection:             "1; mode=block",
		StrictTransportSecurity:   "max-age=31536000; includeSubDomains",
		ContentSecurityPolicy:     "default-src 'self'",
		ReferrerPolicy:            "strict-origin-when-cross-origin",
		PermissionsPolicy:         "geolocation=(), microphone=(), camera=()",
		CrossOriginOpenerPolicy:   "same-origin",
		CrossOriginEmbedderPolicy: "require-corp",
		CrossOriginResourcePolicy: "same-origin",
	}
}

// SecurityHeaders creates a security headers middleware with default configuration.
// It adds comprehensive security headers to protect against common web vulnerabilities.
func SecurityHeaders[C handler.Context]() handler.Middleware[C] {
	return SecurityHeadersWithConfig[C](defaultSecurityConfig())
}

// SecurityHeadersWithConfig creates a security headers middleware with custom configuration.
// It adds various HTTP security headers to responses to enhance application security.
func SecurityHeadersWithConfig[C handler.Context](cfg SecurityHeadersConfig) handler.Middleware[C] {
	// Handle development mode for HSTS
	if cfg.IsDevelopment && cfg.StrictTransportSecurity == "max-age=31536000; includeSubDomains" {
		cfg.StrictTransportSecurity = ""
	}

	return func(next handler.HandlerFunc[C]) handler.HandlerFunc[C] {
		return func(ctx C) handler.Response {
			if cfg.Skip != nil && cfg.Skip(ctx) {
				return next(ctx)
			}

			response := next(ctx)

			return func(w http.ResponseWriter, r *http.Request) error {
				// Add security headers only if not empty
				if cfg.ContentTypeOptions != "" {
					w.Header().Set("X-Content-Type-Options", cfg.ContentTypeOptions)
				}
				if cfg.FrameOptions != "" {
					w.Header().Set("X-Frame-Options", cfg.FrameOptions)
				}
				if cfg.XSSProtection != "" {
					w.Header().Set("X-XSS-Protection", cfg.XSSProtection)
				}
				if cfg.StrictTransportSecurity != "" {
					w.Header().Set("Strict-Transport-Security", cfg.StrictTransportSecurity)
				}
				if cfg.ContentSecurityPolicy != "" {
					w.Header().Set("Content-Security-Policy", cfg.ContentSecurityPolicy)
				}
				if cfg.ReferrerPolicy != "" {
					w.Header().Set("Referrer-Policy", cfg.ReferrerPolicy)
				}
				if cfg.PermissionsPolicy != "" {
					w.Header().Set("Permissions-Policy", cfg.PermissionsPolicy)
				}
				if cfg.CrossOriginOpenerPolicy != "" {
					w.Header().Set("Cross-Origin-Opener-Policy", cfg.CrossOriginOpenerPolicy)
				}
				if cfg.CrossOriginEmbedderPolicy != "" {
					w.Header().Set("Cross-Origin-Embedder-Policy", cfg.CrossOriginEmbedderPolicy)
				}
				if cfg.CrossOriginResourcePolicy != "" {
					w.Header().Set("Cross-Origin-Resource-Policy", cfg.CrossOriginResourcePolicy)
				}

				// Add custom headers
				for key, value := range cfg.CustomHeaders {
					w.Header().Set(key, value)
				}

				return response(w, r)
			}
		}
	}
}

// SecurityHeadersPreset represents a pre-configured set of security headers
type SecurityHeadersPreset string

const (
	// SecurityPresetStrict provides maximum security with strict policies
	SecurityPresetStrict SecurityHeadersPreset = "strict"
	// SecurityPresetBalanced provides good security with compatibility
	SecurityPresetBalanced SecurityHeadersPreset = "balanced"
	// SecurityPresetRelaxed provides basic security for maximum compatibility
	SecurityPresetRelaxed SecurityHeadersPreset = "relaxed"
)

// SecurityHeadersWithPreset creates a security headers middleware with a preset configuration.
// Presets provide common security configurations for different use cases.
func SecurityHeadersWithPreset[C handler.Context](preset SecurityHeadersPreset) handler.Middleware[C] {
	var cfg SecurityHeadersConfig

	switch preset {
	case SecurityPresetStrict:
		cfg = SecurityHeadersConfig{
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
	case SecurityPresetBalanced:
		cfg = SecurityHeadersConfig{
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
	case SecurityPresetRelaxed:
		cfg = SecurityHeadersConfig{
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
	default:
		cfg = SecurityHeadersConfig{}
	}

	return SecurityHeadersWithConfig[C](cfg)
}

// CSPDirective represents a Content Security Policy directive
type CSPDirective struct {
	Name   string
	Values []string
}

// CSPBuilder helps build Content Security Policy headers
type CSPBuilder struct {
	directives []CSPDirective
}

// NewCSPBuilder creates a new CSP builder
func NewCSPBuilder() *CSPBuilder {
	return &CSPBuilder{
		directives: make([]CSPDirective, 0),
	}
}

// DefaultSrc sets the default-src directive
func (b *CSPBuilder) DefaultSrc(sources ...string) *CSPBuilder {
	b.directives = append(b.directives, CSPDirective{Name: "default-src", Values: sources})
	return b
}

// ScriptSrc sets the script-src directive
func (b *CSPBuilder) ScriptSrc(sources ...string) *CSPBuilder {
	b.directives = append(b.directives, CSPDirective{Name: "script-src", Values: sources})
	return b
}

// StyleSrc sets the style-src directive
func (b *CSPBuilder) StyleSrc(sources ...string) *CSPBuilder {
	b.directives = append(b.directives, CSPDirective{Name: "style-src", Values: sources})
	return b
}

// ImgSrc sets the img-src directive
func (b *CSPBuilder) ImgSrc(sources ...string) *CSPBuilder {
	b.directives = append(b.directives, CSPDirective{Name: "img-src", Values: sources})
	return b
}

// FontSrc sets the font-src directive
func (b *CSPBuilder) FontSrc(sources ...string) *CSPBuilder {
	b.directives = append(b.directives, CSPDirective{Name: "font-src", Values: sources})
	return b
}

// ConnectSrc sets the connect-src directive
func (b *CSPBuilder) ConnectSrc(sources ...string) *CSPBuilder {
	b.directives = append(b.directives, CSPDirective{Name: "connect-src", Values: sources})
	return b
}

// FrameAncestors sets the frame-ancestors directive
func (b *CSPBuilder) FrameAncestors(sources ...string) *CSPBuilder {
	b.directives = append(b.directives, CSPDirective{Name: "frame-ancestors", Values: sources})
	return b
}

// BaseURI sets the base-uri directive
func (b *CSPBuilder) BaseURI(sources ...string) *CSPBuilder {
	b.directives = append(b.directives, CSPDirective{Name: "base-uri", Values: sources})
	return b
}

// FormAction sets the form-action directive
func (b *CSPBuilder) FormAction(sources ...string) *CSPBuilder {
	b.directives = append(b.directives, CSPDirective{Name: "form-action", Values: sources})
	return b
}

// Build constructs the final CSP header value
func (b *CSPBuilder) Build() string {
	var parts []string
	for _, directive := range b.directives {
		if len(directive.Values) > 0 {
			parts = append(parts, fmt.Sprintf("%s %s", directive.Name, strings.Join(directive.Values, " ")))
		}
	}
	return strings.Join(parts, "; ")
}
