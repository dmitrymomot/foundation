// Package sessiontransport provides HTTP transport implementations for session management.
//
// This package bridges the gap between the transport-agnostic core/session package
// and HTTP, providing Cookie-based and JWT-based transports. Both transports use
// Session.Token for authentication, ensuring a unified security model.
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
//	sess, err := transport.Load(ctx, r)
//
//	// Authenticate user
//	sess, err = transport.Authenticate(ctx, w, r, userID)
//
//	// Logout user
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
//	sess, err := transport.Load(ctx, r)
//	if err == sessiontransport.ErrNoToken {
//		// No token present
//	}
//
//	// Authenticate user (returns token pair)
//	sess, tokens, err := transport.Authenticate(ctx, w, r, userID)
//	// tokens.AccessToken, tokens.RefreshToken, tokens.ExpiresIn
//
//	// Refresh tokens (rotates both)
//	sess, tokens, err = transport.Refresh(ctx, oldRefreshToken)
//
//	// Logout user
//	err = transport.Logout(ctx, w, r)
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
