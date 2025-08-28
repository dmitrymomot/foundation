// Package fingerprint generates device fingerprints from HTTP requests for session validation.
//
// This package combines User-Agent, Accept headers, client IP address, and header ordering
// to create unique 32-character hex identifiers. It's designed as an additional security
// layer for detecting potential session hijacking by identifying changes in client
// characteristics between requests.
//
// Basic usage:
//
//	import "github.com/dmitrymomot/foundation/pkg/fingerprint"
//
//	func loginHandler(w http.ResponseWriter, r *http.Request) {
//		// Generate fingerprint for new session
//		fp := fingerprint.Generate(r)
//
//		// Store with session data
//		// (session storage implementation depends on your session store)
//
//		// Later, validate on protected requests
//		if !fingerprint.Validate(r, storedFingerprint) {
//			// Handle potential session hijacking
//		}
//	}
//
// # Fingerprint Components
//
// The fingerprint is built from these HTTP request elements:
//   - User-Agent header
//   - Accept-Language header
//   - Accept-Encoding header
//   - Accept header
//   - Client IP address (using foundation/pkg/clientip)
//   - Set of common browser headers present in the request
//
// # Security Notes
//
// Device fingerprinting has inherent limitations:
//   - IP addresses change (mobile networks, VPN usage)
//   - Browser updates modify User-Agent strings
//   - Users can modify or block headers
//   - Should supplement, not replace, proper session management
//
// Consider implementing graceful handling of fingerprint mismatches rather than
// immediate session termination, as legitimate users may trigger false positives.
package fingerprint
