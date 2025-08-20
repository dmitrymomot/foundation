package middleware

import (
	"net/http"

	"github.com/dmitrymomot/gokit/core/handler"
	"github.com/dmitrymomot/gokit/core/response"
	"github.com/dmitrymomot/gokit/pkg/fingerprint"
)

// fingerprintContextKey is used as a key for storing device fingerprint in request context.
type fingerprintContextKey struct{}

// FingerprintConfig configures the device fingerprinting middleware.
type FingerprintConfig struct {
	// Skip defines a function to skip middleware execution for specific requests
	Skip func(ctx handler.Context) bool
	// HeaderName specifies the response header name for the fingerprint (default: "X-Device-Fingerprint")
	HeaderName string
	// StoreInContext determines whether to store the generated fingerprint in request context
	StoreInContext bool
	// StoreInHeader determines whether to include the fingerprint in response headers
	StoreInHeader bool
	// ValidateFunc allows custom validation of the generated fingerprint
	ValidateFunc func(ctx handler.Context, fingerprint string) error
}

// Fingerprint creates a device fingerprinting middleware with default configuration.
// By default, it stores the generated fingerprint in the request context.
func Fingerprint[C handler.Context]() handler.Middleware[C] {
	return FingerprintWithConfig[C](FingerprintConfig{
		StoreInContext: true,
	})
}

// FingerprintWithConfig creates a device fingerprinting middleware with custom configuration.
// It generates a unique fingerprint based on request headers, IP address, and other characteristics
// to help identify devices and detect suspicious activity.
func FingerprintWithConfig[C handler.Context](cfg FingerprintConfig) handler.Middleware[C] {
	if cfg.HeaderName == "" {
		cfg.HeaderName = "X-Device-Fingerprint"
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

			fp := fingerprint.Generate(ctx.Request())

			if cfg.StoreInContext {
				ctx.SetValue(fingerprintContextKey{}, fp)
			}

			if cfg.ValidateFunc != nil {
				if err := cfg.ValidateFunc(ctx, fp); err != nil {
					return response.JSONWithStatus(response.ErrBadRequest.WithError(err), response.ErrBadRequest.Status)
				}
			}

			response := next(ctx)

			if cfg.StoreInHeader {
				return func(w http.ResponseWriter, r *http.Request) error {
					w.Header().Set(cfg.HeaderName, fp)
					return response(w, r)
				}
			}

			return response
		}
	}
}

// GetFingerprint retrieves the device fingerprint from the request context.
// Returns the fingerprint and a boolean indicating whether it was found.
func GetFingerprint(ctx handler.Context) (string, bool) {
	fp, ok := ctx.Value(fingerprintContextKey{}).(string)
	return fp, ok
}
