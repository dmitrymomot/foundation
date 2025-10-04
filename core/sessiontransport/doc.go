// Package sessiontransport provides HTTP transport implementations for session management.
//
// This package bridges the gap between the transport-agnostic core/session package
// and HTTP, providing Cookie-based and JWT-based transports. Both transports use
// Session.Token for authentication, ensuring a unified security model.
//
// # Automatic IP and User-Agent Extraction
//
// All transports automatically extract and store client information when creating sessions:
//   - IP Address: Extracted via clientip.GetIP() with proxy header support
//   - User-Agent: Extracted from "User-Agent" HTTP header
//   - Fingerprint: Generated via fingerprint package (transport-specific)
//
// This extraction happens in the Load() method when creating new anonymous sessions.
//
// # Cookie Transport
//
// Cookie transport stores Session.Token as a signed cookie value. It provides
// graceful degradation - automatically creating anonymous sessions when cookies
// are missing or invalid.
//
// Example usage:
//
//	import (
//		"github.com/dmitrymomot/foundation/core/session"
//		"github.com/dmitrymomot/foundation/core/cookie"
//		"github.com/dmitrymomot/foundation/core/sessiontransport"
//	)
//
//	// Setup
//	store := NewYourStoreImplementation()
//	sessionMgr := session.NewManager[SessionData](store, 24*time.Hour, 5*time.Minute)
//	cookieMgr, _ := cookie.New([]string{"your-secret-key"})
//	transport := sessiontransport.NewCookie(sessionMgr, cookieMgr, "session")
//
//	// Load session (creates anonymous if missing/invalid)
//	// IP and User-Agent are automatically extracted from request
//	sess, err := transport.Load(ctx, r)
//
//	// Access tracked information
//	clientIP := sess.IP                    // e.g., "203.0.113.42"
//	device := sess.Device()                // e.g., "Chrome/120.0 (Windows, desktop)"
//	userAgent := sess.UserAgent            // Raw User-Agent string
//
//	// Authenticate user (preserves IP and UserAgent)
//	sess, err = transport.Authenticate(ctx, w, r, userID)
//
//	// Logout user (preserves IP and UserAgent)
//	sess, err = transport.Logout(ctx, w, r)
//
//	// Delete session
//	err = transport.Delete(ctx, w, r)
//
// # JWT Transport
//
// JWT transport stores Session.Token in the JTI (JWT ID) claim. It returns
// token pairs (access + refresh) and handles token rotation on refresh.
//
// Example usage:
//
//	import (
//		"github.com/dmitrymomot/foundation/core/session"
//		"github.com/dmitrymomot/foundation/core/sessiontransport"
//	)
//
//	// Setup
//	store := NewYourStoreImplementation()
//	sessionMgr := session.NewManager[SessionData](store, 24*time.Hour, 5*time.Minute)
//	transport, _ := sessiontransport.NewJWT(
//		sessionMgr,
//		"your-jwt-secret",
//		15*time.Minute, // access token TTL
//		"your-app",
//	)
//
//	// Load session (returns ErrNoToken if no bearer token)
//	// IP and User-Agent are automatically extracted from request
//	sess, err := transport.Load(ctx, r)
//	if err == sessiontransport.ErrNoToken {
//		// No token present
//	}
//
//	// Access tracked information (same as Cookie transport)
//	clientIP := sess.IP                    // e.g., "203.0.113.42"
//	device := sess.Device()                // e.g., "Safari/17.0 (iOS, mobile)"
//
//	// Authenticate user (returns token pair, preserves IP/UserAgent)
//	sess, tokens, err := transport.Authenticate(ctx, w, r, userID)
//	// tokens.AccessToken, tokens.RefreshToken, tokens.ExpiresIn
//
//	// Refresh tokens (rotates both)
//	sess, tokens, err = transport.Refresh(ctx, oldRefreshToken)
//
//	// Logout user
//	err = transport.Logout(ctx, w, r)
//
// # Proxy Header Support
//
// The clientip.GetIP() function supports standard proxy headers in order of priority:
//  1. X-Forwarded-For (first trusted IP)
//  2. X-Real-IP
//  3. CF-Connecting-IP (Cloudflare)
//  4. True-Client-IP (Akamai, Cloudflare)
//  5. X-Client-IP
//  6. Forwarded (RFC 7239)
//
// Falls back to RemoteAddr if no proxy headers present. IP addresses are validated
// for correct IPv4/IPv6 format before being stored in sessions.
//
// # Unified Security Model
//
// Both transports use Session.Token for authentication:
//   - Cookie: Session.Token is the signed cookie value
//   - JWT: Session.Token is stored in the JTI claim
//
// This unified approach ensures:
//   - Consistent token rotation on authentication/logout
//   - Single source of truth for session authentication
//   - Easy migration between transports
//   - Simplified session validation logic
//   - Automatic IP and User-Agent tracking for security
//
// # Token Rotation
//
// Both transports implement automatic token rotation:
//   - Authenticate: New Session.Token on login
//   - Logout: New Session.Token on logout (prevents session fixation)
//   - Refresh (JWT): New Session.Token on refresh (prevents token reuse)
//
// # Touch Mechanism
//
// The Touch method extends session expiration if touchInterval has elapsed.
// Note that GetByID/GetByToken already handle touch internally, so explicit
// Touch calls are only needed when you want to extend session lifetime outside
// of normal request flows.
//
// # Error Handling
//
// The package defines specific errors:
//   - ErrNoToken: No authentication token present (JWT only)
//   - ErrInvalidToken: Token format or signature invalid
//
// Session errors from core/session package are also propagated:
//   - session.ErrExpired: Session has expired
//   - session.ErrNotFound: Session not found in store
//   - session.ErrNotAuthenticated: Authentication failed
package sessiontransport
