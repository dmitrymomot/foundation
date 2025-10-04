// Package session provides pure business logic for managing user sessions with generic data storage.
//
// This package handles session lifecycle without any HTTP knowledge - HTTP integration
// is handled separately by the sessiontransport package. Sessions can be anonymous
// (guest users) or authenticated (logged-in users), with seamless conversion between states.
//
// # Key Features
//
// - Generic Data type for custom session data structures
// - Anonymous sessions with UserID = uuid.Nil
// - Seamless conversion from anonymous to authenticated sessions
// - Token rotation on authentication/logout (prevents session fixation attacks)
// - Touch mechanism to extend active sessions without forcing updates on every request
// - Separate ID and Token: ID is stable (for foreign keys), Token rotates (security)
// - Clean separation of concerns following session system design principles
//
// # Session Structure
//
// A Session contains:
//   - ID: Stable identifier for database foreign keys, rotates on auth/logout
//   - Token: Cryptographically secure token for authentication, rotates on auth/logout
//   - UserID: uuid.Nil for anonymous, actual UUID for authenticated sessions
//   - Fingerprint: Device fingerprint for security validation ("v1:hash" format, 35 chars)
//   - IP: Client IP address (required, supports IPv4/IPv6, extracted from proxy headers)
//   - UserAgent: Raw User-Agent string from HTTP request (optional)
//   - Data: Generic type parameter for custom session data
//   - ExpiresAt: Session expiration timestamp
//   - CreatedAt: Session creation timestamp
//   - UpdatedAt: Last modification timestamp
//
// The Device() method parses UserAgent to return a human-readable device identifier.
//
// # Manager Operations
//
// The Manager handles session lifecycle:
//   - New: Create anonymous session (requires NewSessionParams with IP, Fingerprint, UserAgent)
//   - GetByID: Retrieve session by stable ID
//   - GetByToken: Retrieve session by authentication token
//   - Save: Update session data
//   - Authenticate: Convert anonymous session to authenticated (rotates token, preserves IP/UserAgent)
//   - Logout: Convert authenticated session to anonymous (rotates token, preserves IP/UserAgent)
//   - Delete: Remove session by ID
//   - CleanupExpired: Remove expired sessions (run periodically)
//   - Refresh: Rotate token and extend expiration (keeps same session ID)
//
// # Store Interface
//
// The Store interface defines persistence requirements:
//   - GetByID: Retrieve by session ID
//   - GetByToken: Retrieve by authentication token
//   - Save: Persist session (upsert)
//   - Delete: Remove session by ID
//   - DeleteExpired: Cleanup expired sessions
//
// Implementations must handle concurrent access safely.
//
// # Security Considerations
//
// Token Rotation: Both Authenticate and Logout rotate the session token by:
//  1. Generating a new cryptographically secure token
//  2. Deleting the old session
//  3. Creating a new session with new ID and token
//
// This prevents session fixation attacks where an attacker sets a known session
// identifier before authentication.
//
// Touch Mechanism: Sessions are extended only when touchInterval has elapsed since
// the last update, reducing write operations while maintaining session activity.
//
// IP and UserAgent Tracking: Sessions track client IP and User-Agent for:
//   - Session hijacking detection (IP change alerts)
//   - Security audit logs (device information via Device() method)
//   - Anomaly detection (unusual device or location changes)
//
// IP addresses are extracted by sessiontransport using clientip.GetIP(), which:
//   - Supports standard proxy headers (X-Forwarded-For, X-Real-IP, etc.)
//   - Validates IP format (IPv4/IPv6)
//   - Required field - session creation fails without valid IP
//
// UserAgent is optional but recommended for security monitoring via Device() method.
//
// # Basic Usage
//
//	import (
//		"context"
//		"time"
//
//		"github.com/dmitrymomot/foundation/core/session"
//		"github.com/google/uuid"
//	)
//
//	// Define custom session data
//	type SessionData struct {
//		Theme      string
//		Language   string
//		ShoppingCart []string
//	}
//
//	// Create manager with a store implementation
//	store := NewYourStoreImplementation()
//	mgr := session.NewManager[SessionData](
//		store,
//		24*time.Hour,     // TTL: 24 hours
//		5*time.Minute,    // Touch interval: extend only if >5min since last update
//	)
//
//	ctx := context.Background()
//
//	// Create anonymous session (for guest users)
//	// In practice, sessiontransport automatically extracts these values from HTTP request
//	params := session.NewSessionParams{
//		Fingerprint: "v1:abc123...",        // from fingerprint package
//		IP:          "203.0.113.42",        // from clientip.GetIP(r)
//		UserAgent:   "Mozilla/5.0...",      // from r.Header.Get("User-Agent")
//	}
//	sess, err := mgr.New(ctx, params)
//	if err != nil {
//		// handle error
//	}
//	// Note: IP is required, session creation fails without it
//	// UserAgent is optional but recommended for security monitoring
//	// Fingerprint format is "v1:hash" (35 characters total)
//	// All values are preserved through Authenticate/Logout/Refresh operations
//
//	// Access IP and device information
//	clientIP := sess.IP                    // "203.0.113.42"
//	device := sess.Device()                // "Chrome/120.0 (Windows, desktop)"
//	userAgent := sess.UserAgent            // Raw User-Agent string
//
//	// Update session data
//	sess.Data.Theme = "dark"
//	sess.Data.Language = "en"
//	if err := mgr.Save(ctx, &sess); err != nil {
//		// handle error
//	}
//
//	// Retrieve session by token (e.g., from cookie)
//	retrieved, err := mgr.GetByToken(ctx, sess.Token)
//	if err != nil {
//		// handle error
//	}
//
//	// Authenticate session (user logs in)
//	userID := uuid.New()
//	authenticated, err := mgr.Authenticate(ctx, sess, userID)
//	if err != nil {
//		// handle error
//	}
//	// authenticated.Token is different (rotated for security)
//	// authenticated.ID is different (new session)
//	// authenticated.UserID == userID
//	// authenticated.Data preserved from anonymous session
//
//	// Logout session (user logs out)
//	anonymous, err := mgr.Logout(ctx, authenticated)
//	if err != nil {
//		// handle error
//	}
//	// anonymous.Token is different (rotated for security)
//	// anonymous.ID is different (new session)
//	// anonymous.UserID == uuid.Nil
//	// anonymous.Data is cleared
//
//	// Delete session
//	if err := mgr.Delete(ctx, sess.ID); err != nil {
//		// handle error
//	}
//
// # Periodic Cleanup
//
// Run periodic cleanup to prevent session table growth:
//
//	import "time"
//
//	// Run cleanup every hour
//	ticker := time.NewTicker(1 * time.Hour)
//	defer ticker.Stop()
//
//	for range ticker.C {
//		if err := mgr.CleanupExpired(ctx); err != nil {
//			// handle error
//		}
//	}
//
// # Error Handling
//
// The package defines standard errors:
//   - ErrExpired: Session has expired
//   - ErrNotFound: Session not found in store
//   - ErrNotAuthenticated: Authentication failed
//
// Example error handling:
//
//	sess, err := mgr.GetByToken(ctx, token)
//	if err != nil {
//		switch {
//		case errors.Is(err, session.ErrExpired):
//			// Session expired, create new anonymous session
//		case errors.Is(err, session.ErrNotFound):
//			// Session not found, create new anonymous session
//		default:
//			// Handle other errors
//		}
//	}
//
// # Session Hijacking Detection
//
// Use IP and Device tracking to detect suspicious session activity:
//
//	// On each request, check if IP changed
//	currentSess, err := mgr.GetByToken(ctx, token)
//	if err != nil {
//		// handle error
//	}
//
//	// Get current request IP
//	currentIP := clientip.GetIP(r)
//
//	// Alert on IP change (potential session hijacking)
//	if currentSess.IP != currentIP {
//		// Log security event
//		logger.Warn("IP changed for session",
//			"session_id", currentSess.ID,
//			"user_id", currentSess.UserID,
//			"old_ip", currentSess.IP,
//			"new_ip", currentIP,
//			"old_device", currentSess.Device(),
//		)
//
//		// Take action: force re-authentication, send alert email, etc.
//		// Or update IP if expected (e.g., mobile networks)
//		currentSess.IP = currentIP
//		mgr.Save(ctx, &currentSess)
//	}
//
// Note: IP changes are common with:
//   - Mobile networks (cellular towers)
//   - VPNs (server switching)
//   - Corporate proxies (load balancing)
//
// Consider your application's security requirements when implementing IP validation.
//
// # Design Principles
//
// This package follows clean separation principles:
//   - No HTTP knowledge (use sessiontransport for HTTP integration)
//   - Simple, straightforward code without tricks
//   - Clear responsibilities: business logic only
//   - Type-safe generic data storage
//   - Security-first approach with token rotation
package session
