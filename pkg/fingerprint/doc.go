// Package fingerprint generates device fingerprints from HTTP requests for session validation.
//
// This package combines User-Agent, Accept headers, client IP, and header ordering patterns
// to create unique 32-character hex identifiers for devices and browsers. It's useful for
// detecting session hijacking, implementing additional security measures, and tracking
// device consistency across requests.
//
// # Fingerprinting Components
//
// The fingerprint is generated from these HTTP request components:
//   - User-Agent header (browser and OS information)
//   - Accept-Language header (locale preferences)
//   - Accept-Encoding header (compression preferences)
//   - Accept header (content type preferences)
//   - Client IP address (extracted using clientip package)
//   - Header ordering pattern (browser-specific header order)
//
// # Usage
//
// Basic fingerprint generation:
//
//	func loginHandler(w http.ResponseWriter, r *http.Request) {
//		// Generate fingerprint for new session
//		fp := fingerprint.Generate(r)
//
//		// Store fingerprint in session
//		session, _ := store.Get(r, "session")
//		session.Values["fingerprint"] = fp
//		session.Save(r, w)
//
//		// Continue with authentication...
//	}
//
// Session validation:
//
//	func protectedHandler(w http.ResponseWriter, r *http.Request) {
//		session, _ := store.Get(r, "session")
//		storedFingerprint, ok := session.Values["fingerprint"].(string)
//		if !ok {
//			http.Error(w, "Invalid session", http.StatusUnauthorized)
//			return
//		}
//
//		if !fingerprint.Validate(r, storedFingerprint) {
//			// Fingerprint mismatch - possible session hijacking
//			log.Printf("Fingerprint mismatch for session %s", session.ID)
//			http.Error(w, "Session validation failed", http.StatusUnauthorized)
//			return
//		}
//
//		// Continue with authorized request...
//	}
//
// # Security Considerations
//
// Device fingerprinting provides an additional layer of security but has limitations:
//   - IP addresses can change (mobile networks, VPNs)
//   - Browser updates may modify User-Agent strings
//   - Users may disable or modify certain headers
//   - Should be combined with other security measures, not used alone
//
// Best practices:
//   - Use alongside traditional session management
//   - Implement graceful degradation for fingerprint mismatches
//   - Consider IP address changes for mobile users
//   - Log fingerprint mismatches for security monitoring
//
// # Fingerprint Stability
//
// Components ranked by stability (most to least stable):
//  1. Header ordering (very stable per browser)
//  2. Accept headers (stable for same browser/OS)
//  3. User-Agent (stable until browser updates)
//  4. Client IP (can change frequently on mobile)
//
// # Performance Characteristics
//
// - Fingerprint generation: ~50-100 µs per request
//   - SHA256 hashing is the primary computational cost
//   - Header enumeration is O(n) where n is number of headers
//   - String operations are minimal and optimized
//
// # Hash Properties
//
// The generated fingerprint has these properties:
//   - 32-character hex string (128-bit entropy)
//   - Uniform distribution across the hex space
//   - Collision resistance provided by SHA256
//   - Deterministic for identical input components
//
// # Integration Examples
//
// Middleware for automatic fingerprinting:
//
//	func FingerprintMiddleware(next http.Handler) http.Handler {
//		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
//			fp := fingerprint.Generate(r)
//			ctx := context.WithValue(r.Context(), "fingerprint", fp)
//			next.ServeHTTP(w, r.WithContext(ctx))
//		})
//	}
//
// Redis-based fingerprint storage:
//
//	func storeFingerprint(sessionID, fingerprint string) error {
//		key := fmt.Sprintf("session:fp:%s", sessionID)
//		return redis.Set(key, fingerprint, 24*time.Hour).Err()
//	}
//
//	func validateFingerprint(sessionID string, r *http.Request) (bool, error) {
//		key := fmt.Sprintf("session:fp:%s", sessionID)
//		stored, err := redis.Get(key).Result()
//		if err != nil {
//			return false, err
//		}
//		return fingerprint.Validate(r, stored), nil
//	}
package fingerprint
