// Package fingerprint generates device fingerprints from HTTP requests for session validation.
//
// This package combines User-Agent, Accept headers, client IP address (optional), and header
// presence to create unique version-prefixed fingerprints. It's designed as an additional
// security layer for detecting potential session hijacking by identifying changes in client
// characteristics between requests.
//
// # Basic Usage
//
//	import (
//		"errors"
//		"github.com/dmitrymomot/foundation/pkg/fingerprint"
//	)
//
//	func loginHandler(w http.ResponseWriter, r *http.Request) {
//		// Generate fingerprint for new session (uses safe defaults)
//		fp := fingerprint.Generate(r)
//
//		// Store with session data
//		// session.SetFingerprint(fp)
//
//		// Later, validate on protected requests
//		if err := fingerprint.Validate(r, storedFingerprint); err != nil {
//			if errors.Is(err, fingerprint.ErrIPMismatch) {
//				// Handle IP change gracefully (user switched networks)
//				log.Warn("IP changed for session", "error", err)
//			} else {
//				// Potential session hijacking - terminate session
//				session.Destroy()
//				http.Error(w, "Session invalid", http.StatusUnauthorized)
//			}
//		}
//	}
//
// # Convenience Functions
//
// Use pre-configured convenience functions for common scenarios:
//
//	// Cookie-based sessions (recommended default, excludes IP)
//	fp := fingerprint.Cookie(r)
//
//	// JWT authentication (minimal fingerprint)
//	fp := fingerprint.JWT(r)
//
//	// High-security (includes IP, may cause false positives)
//	fp := fingerprint.Strict(r)
//
// # Custom Configuration
//
// Use functional options to customize fingerprint generation:
//
//	// Include IP address (high-security mode)
//	fp := fingerprint.Generate(r, fingerprint.WithIP())
//
//	// Exclude Accept headers (JWT authentication)
//	fp := fingerprint.Generate(r, fingerprint.WithoutAcceptHeaders())
//
//	// Combine multiple options
//	fp := fingerprint.Generate(r,
//		fingerprint.WithIP(),
//		fingerprint.WithoutAcceptHeaders(),
//	)
//
// # Error Handling
//
// Validate returns specific errors that can be checked with errors.Is():
//
//	err := fingerprint.Validate(r, storedFP)
//	switch {
//	case err == nil:
//		// Fingerprint matches
//	case errors.Is(err, fingerprint.ErrIPMismatch):
//		// IP address changed (mobile networks, VPN)
//	case errors.Is(err, fingerprint.ErrUserAgentMismatch):
//		// User-Agent changed (browser update, different device)
//	case errors.Is(err, fingerprint.ErrHeadersMismatch):
//		// Accept headers changed (language settings, extensions)
//	case errors.Is(err, fingerprint.ErrHeaderSetMismatch):
//		// Header set changed (different browser/client)
//	case errors.Is(err, fingerprint.ErrInvalidFingerprint):
//		// Stored fingerprint has invalid format
//	}
//
// # Fingerprint Components
//
// The fingerprint can include these HTTP request elements (configurable):
//   - User-Agent header (default: included)
//   - Accept-Language, Accept-Encoding, Accept headers (default: included)
//   - Client IP address via foundation/pkg/clientip (default: excluded)
//   - Presence of common browser headers (default: included)
//
// # Fingerprint Format
//
// Fingerprints are returned in version-prefixed format: "v1:hash"
// The version prefix allows future algorithm changes without breaking existing sessions.
//
// # Security Notes
//
// Device fingerprinting has inherent limitations:
//   - IP addresses change frequently (mobile networks, VPN usage, corporate proxies)
//   - Browser updates modify User-Agent strings
//   - Language settings and extensions can change Accept headers
//   - Users can modify or block headers
//   - Should supplement, not replace, proper session management
//
// Default configuration (IncludeIP: false) balances security and usability by avoiding
// false positives from legitimate IP changes. For high-security scenarios, use Strict()
// but implement graceful handling of IP changes to avoid disrupting mobile users.
package fingerprint
