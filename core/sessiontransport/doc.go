// Package sessiontransport provides secure transport implementations for the session package.
//
// This package implements the session.Transport interface using different transport mechanisms:
// Cookie-based transport for traditional web applications and JWT-based transport for APIs.
// Each transport handles the secure embedding, extraction, and revocation of session tokens.
//
// # Cookie Transport
//
// CookieTransport uses encrypted cookies to store session tokens securely in the client's browser.
// It provides strong security through encryption and is ideal for traditional web applications.
//
// Basic cookie transport usage:
//
//	import (
//		"github.com/dmitrymomot/foundation/core/cookie"
//		"github.com/dmitrymomot/foundation/core/sessiontransport"
//	)
//
//	func main() {
//		// Create cookie manager with encryption keys
//		manager, err := cookie.New([]string{"your-32-byte-secret-key-here!!"})
//		if err != nil {
//			panic(err)
//		}
//
//		// Create cookie transport with default settings
//		transport := sessiontransport.NewCookie(manager)
//
//		// Or with custom options
//		transport = sessiontransport.NewCookie(manager,
//			sessiontransport.WithCookieName("my_session"),
//			sessiontransport.WithCookieOptions(
//				cookie.WithSecure(true),
//				cookie.WithDomain(".example.com"),
//			),
//		)
//	}
//
// # JWT Transport
//
// JWTTransport uses JSON Web Tokens to embed session information that can be verified
// without server-side storage. It supports optional token revocation through the Revoker interface.
//
// Basic JWT transport usage:
//
//	import "github.com/dmitrymomot/foundation/core/sessiontransport"
//
//	func main() {
//		// Create JWT transport without revocation
//		transport, err := sessiontransport.NewJWT("your-signing-key", nil)
//		if err != nil {
//			panic(err)
//		}
//
//		// Or with custom options
//		transport, err = sessiontransport.NewJWT("your-signing-key", nil,
//			sessiontransport.WithJWTHeaderName("X-Auth-Token"),
//			sessiontransport.WithJWTBearerPrefix(false),
//			sessiontransport.WithJWTIssuer("myapp"),
//			sessiontransport.WithJWTAudience("api.example.com"),
//		)
//		if err != nil {
//			panic(err)
//		}
//	}
//
// # JWT with Revocation
//
// For additional security, JWT transport can be configured with a revoker to maintain
// a blacklist of revoked tokens:
//
//	type RedisRevoker struct {
//		client *redis.Client
//	}
//
//	func (r *RedisRevoker) IsRevoked(ctx context.Context, jti string) (bool, error) {
//		result := r.client.Exists(ctx, "revoked:"+jti)
//		return result.Val() > 0, result.Err()
//	}
//
//	func (r *RedisRevoker) Revoke(ctx context.Context, jti string) error {
//		return r.client.Set(ctx, "revoked:"+jti, "1", time.Hour*24).Err()
//	}
//
//	func main() {
//		revoker := &RedisRevoker{client: redisClient}
//		transport, err := sessiontransport.NewJWT("signing-key", revoker)
//		if err != nil {
//			panic(err)
//		}
//	}
//
// # Security Considerations
//
// Cookie Transport Security:
//   - Uses strong encryption via the cookie package
//   - Supports secure cookie attributes (Secure, HttpOnly, SameSite)
//   - Resistant to tampering and inspection
//   - Vulnerable to CSRF if not properly configured
//   - Cookie size limits apply (typically 4KB)
//
// JWT Transport Security:
//   - Tokens are signed but not encrypted (claims are visible)
//   - No server-side storage required for basic validation
//   - Supports standard JWT security practices
//   - Token revocation requires additional infrastructure
//   - Larger payload size compared to cookies
//
// # Choosing a Transport
//
// Use Cookie Transport when:
//   - Building traditional web applications
//   - Need maximum security with minimal setup
//   - Working with browsers that reliably support cookies
//   - Want server-side session invalidation without additional infrastructure
//
// Use JWT Transport when:
//   - Building APIs or stateless services
//   - Need to include claims in the token itself
//   - Working with mobile apps or SPA clients
//   - Require cross-domain authentication
//   - Need to scale horizontally without shared session storage
//
// # Error Handling
//
// All transport implementations map their specific errors to standard session errors:
//   - session.ErrNoToken: No token found in the request
//   - session.ErrInvalidToken: Token is malformed, expired, or revoked
//   - session.ErrTransportFailed: Infrastructure or encoding/decoding errors
//
// This consistent error interface allows session managers to handle transport
// failures uniformly regardless of the underlying transport mechanism.
package sessiontransport
