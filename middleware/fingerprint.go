package middleware

import (
	"net/http"

	"github.com/dmitrymomot/foundation/core/handler"
	"github.com/dmitrymomot/foundation/core/response"
	"github.com/dmitrymomot/foundation/pkg/fingerprint"
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
	// Options configures fingerprint generation behavior (default uses Cookie mode: excludes IP, includes User-Agent and Accept headers)
	// Use fingerprint.WithIP() for strict mode, fingerprint.WithoutAcceptHeaders() for JWT mode, or combine options
	Options []fingerprint.Option
}

// Fingerprint creates a device fingerprinting middleware with default configuration.
// By default, it stores the generated fingerprint in the request context using Cookie mode
// (excludes IP address to avoid false positives from mobile networks and VPNs).
//
// Device fingerprinting creates a unique identifier based on request characteristics
// like User-Agent, Accept headers, header set patterns, and optionally IP address.
// This helps identify devices for security, analytics, and user experience purposes.
//
// Usage:
//
//	// Apply to all routes for device tracking
//	r.Use(middleware.Fingerprint[*MyContext]())
//
//	// Use fingerprint in handlers
//	func handleSession(ctx *MyContext) handler.Response {
//		fingerprint, ok := middleware.GetFingerprint(ctx)
//		if !ok {
//			return response.Error(response.ErrInternalServerError)
//		}
//
//		// Track device for security
//		log.Printf("Request from device: %s", fingerprint)
//
//		// Check if device is known/trusted
//		if !deviceRepo.IsTrusted(userID, fingerprint) {
//			// Require additional verification for new device
//			return requireTwoFactorAuth(ctx)
//		}
//
//		return response.JSON(map[string]string{"device": fingerprint})
//	}
//
// Common use cases:
// - Fraud detection: Identify suspicious device patterns
// - Session security: Detect session hijacking across devices
// - User experience: Remember device preferences
// - Analytics: Track unique devices/visitors
// - Two-factor authentication: Require 2FA for new devices
func Fingerprint[C handler.Context]() handler.Middleware[C] {
	return FingerprintWithConfig[C](FingerprintConfig{
		StoreInContext: true,
	})
}

// FingerprintWithConfig creates a device fingerprinting middleware with custom configuration.
// It generates a unique fingerprint based on request headers and optional IP address
// to help identify devices and detect suspicious activity.
//
// Advanced Usage Examples:
//
//	// Default mode (Cookie): Excludes IP, includes User-Agent and Accept headers
//	cfg := middleware.FingerprintConfig{
//		StoreInContext: true,
//	}
//	r.Use(middleware.FingerprintWithConfig[*MyContext](cfg))
//
//	// Strict mode: Include IP address for high-security scenarios
//	// WARNING: May cause false positives for mobile users and VPN users
//	cfg := middleware.FingerprintConfig{
//		StoreInContext: true,
//		Options:        []fingerprint.Option{fingerprint.WithIP()},
//	}
//	r.Use(middleware.FingerprintWithConfig[*MyContext](cfg))
//
//	// JWT mode: Minimal fingerprint excluding Accept headers
//	cfg := middleware.FingerprintConfig{
//		StoreInContext: true,
//		Options:        []fingerprint.Option{fingerprint.WithoutAcceptHeaders()},
//	}
//	r.Use(middleware.FingerprintWithConfig[*MyContext](cfg))
//
//	// Include fingerprint in response headers for debugging
//	cfg := middleware.FingerprintConfig{
//		StoreInContext: true,
//		StoreInHeader:  true,
//		HeaderName:     "X-Device-Fingerprint",
//	}
//	r.Use(middleware.FingerprintWithConfig[*MyContext](cfg))
//
//	// Validate device fingerprints for security
//	suspiciousDevices := map[string]bool{
//		"abc123suspicious": true,
//		"def456blocked":    true,
//	}
//	cfg := middleware.FingerprintConfig{
//		StoreInContext: true,
//		ValidateFunc: func(ctx handler.Context, fp string) error {
//			if suspiciousDevices[fp] {
//				return fmt.Errorf("device %s is blocked", fp)
//			}
//			return nil
//		},
//	}
//	r.Use(middleware.FingerprintWithConfig[*MyContext](cfg))
//
//	// Skip fingerprinting for API endpoints
//	cfg := middleware.FingerprintConfig{
//		StoreInContext: true,
//		Skip: func(ctx handler.Context) bool {
//			return strings.HasPrefix(ctx.Request().URL.Path, "/api/")
//		},
//	}
//
//	// Custom device validation with database lookup
//	cfg := middleware.FingerprintConfig{
//		StoreInContext: true,
//		ValidateFunc: func(ctx handler.Context, fp string) error {
//			// Check against known malicious device patterns
//			if deviceRepo.IsBlacklisted(fp) {
//				return fmt.Errorf("device blocked for security")
//			}
//
//			// Rate limit by device fingerprint
//			if rateLimiter.ExceededLimit(fp) {
//				return fmt.Errorf("too many requests from this device")
//			}
//
//			return nil
//		},
//	}
//
// Configuration options:
// - Options: Fingerprint generation options (default: Cookie mode without IP)
// - StoreInContext: Store fingerprint in request context for handler access
// - StoreInHeader: Include fingerprint in response headers (debugging)
// - ValidateFunc: Custom fingerprint validation (security, rate limiting)
// - Skip: Skip processing for specific requests (API endpoints, etc.)
//
// Fingerprint characteristics (default Cookie mode):
// - User-Agent string (browser, OS, device info)
// - Accept headers (language, encoding, content types)
// - Header set fingerprint (which standard headers are present)
// - Client IP address (optional, use WithIP() option)
//
// Available fingerprint modes via Options:
// - Cookie mode (default): fingerprint.WithoutIP() - Recommended for most web apps
// - JWT mode: fingerprint.WithoutAcceptHeaders() - Minimal fingerprint
// - Strict mode: fingerprint.WithIP() - Includes IP, may cause false positives
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

			fp := fingerprint.Generate(ctx.Request(), cfg.Options...)

			if cfg.StoreInContext {
				ctx.SetValue(fingerprintContextKey{}, fp)
			}

			if cfg.ValidateFunc != nil {
				if err := cfg.ValidateFunc(ctx, fp); err != nil {
					return response.Error(response.ErrBadRequest.WithError(err))
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
//
// Usage in handlers:
//
//	func handleUserLogin(ctx *MyContext) handler.Response {
//		fingerprint, ok := middleware.GetFingerprint(ctx)
//		if !ok {
//			return response.Error(response.ErrInternalServerError.WithMessage("Device fingerprinting failed"))
//		}
//
//		// Security: Check if this is a known device for the user
//		userDevices, err := deviceRepo.GetUserDevices(userID)
//		if err != nil {
//			return response.Error(response.ErrInternalServerError)
//		}
//
//		isKnownDevice := false
//		for _, device := range userDevices {
//			if device.Fingerprint == fingerprint {
//				isKnownDevice = true
//				break
//			}
//		}
//
//		if !isKnownDevice {
//			// Save new device and require additional verification
//			deviceRepo.SaveDevice(userID, fingerprint, ctx.Request().UserAgent())
//
//			// Send security notification
//			notificationService.SendNewDeviceAlert(userID, fingerprint)
//
//			// Require 2FA for new device
//			return response.JSON(map[string]any{
//				"status":           "new_device_detected",
//				"requires_2fa":     true,
//				"device_fingerprint": fingerprint,
//			})
//		}
//
//		return response.JSON(map[string]any{
//			"status": "login_successful",
//			"device": fingerprint,
//		})
//	}
//
// The fingerprint is a hash-based identifier that remains consistent
// for the same device/browser combination while being privacy-friendly.
// It cannot be used to identify users across different websites.
func GetFingerprint(ctx handler.Context) (string, bool) {
	fp, ok := ctx.Value(fingerprintContextKey{}).(string)
	return fp, ok
}
