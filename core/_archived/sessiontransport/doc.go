// Package sessiontransport provides secure transport implementations for the session package.
//
// This package implements the session.Transport interface using different transport mechanisms:
// cookie-based transport for traditional web applications and JWT-based transport for APIs.
// Both transports handle the secure embedding, extraction, and revocation of session tokens.
//
// # Cookie Transport
//
// CookieTransport uses encrypted cookies to store session tokens securely in the client's browser.
// It provides strong security through encryption and is ideal for traditional web applications.
// The default cookie name is "__session" and includes essential cookie attributes.
//
// Basic cookie transport usage:
//
//	import (
//		"time"
//		"github.com/dmitrymomot/foundation/core/cookie"
//		"github.com/dmitrymomot/foundation/core/sessiontransport"
//	)
//
//	// Create cookie manager with 32-byte encryption key
//	manager, err := cookie.New([]string{"your-32-byte-secret-key-here!!"})
//	if err != nil {
//		log.Fatal(err)
//	}
//
//	// Create cookie transport with default settings
//	transport := sessiontransport.NewCookie(manager)
//
//	// Or with custom configuration
//	transport = sessiontransport.NewCookie(manager,
//		sessiontransport.WithCookieName("app_session"),
//		sessiontransport.WithCookieOptions(
//			cookie.WithSecure(true),
//			cookie.WithDomain(".example.com"),
//		),
//	)
//
// # JWT Transport
//
// JWTTransport uses JSON Web Tokens to embed session information that can be verified
// without server-side storage. The session token becomes the JWT ID (jti) claim.
// It supports optional token revocation through the Revoker interface.
//
// Basic JWT transport usage:
//
//	import "github.com/dmitrymomot/foundation/core/sessiontransport"
//
//	// Create JWT transport without revocation
//	transport, err := sessiontransport.NewJWT("your-32-byte-signing-key", nil)
//	if err != nil {
//		log.Fatal(err)
//	}
//
//	// Create with custom configuration
//	transport, err = sessiontransport.NewJWT("your-signing-key", nil,
//		sessiontransport.WithJWTHeaderName("X-Session-Token"),
//		sessiontransport.WithJWTBearerPrefix(false),
//		sessiontransport.WithJWTIssuer("myapp"),
//		sessiontransport.WithJWTAudience("api.example.com"),
//	)
//	if err != nil {
//		log.Fatal(err)
//	}
//
// # JWT Token Revocation
//
// JWT transport supports optional token revocation through the Revoker interface.
// This allows maintaining a blacklist of revoked session tokens:
//
//	import (
//		"context"
//		"time"
//		"github.com/redis/go-redis/v9"
//	)
//
//	type RedisRevoker struct {
//		client *redis.Client
//	}
//
//	func (r *RedisRevoker) IsRevoked(ctx context.Context, sessionToken string) (bool, error) {
//		result := r.client.Exists(ctx, "revoked:"+sessionToken)
//		return result.Val() > 0, result.Err()
//	}
//
//	func (r *RedisRevoker) Revoke(ctx context.Context, sessionToken string) error {
//		// Store revoked token with TTL matching JWT expiration
//		return r.client.Set(ctx, "revoked:"+sessionToken, "1", time.Hour*24).Err()
//	}
//
//	// Use with JWT transport
//	revoker := &RedisRevoker{client: redisClient}
//	transport, err := sessiontransport.NewJWT("signing-key", revoker)
//	if err != nil {
//		log.Fatal(err)
//	}
//
// For testing or when revocation isn't needed, use NoOpRevoker:
//
//	transport, err := sessiontransport.NewJWT("signing-key", sessiontransport.NoOpRevoker{})
//
// # Error Handling
//
// All transport implementations map their specific errors to standard session package errors:
//   - session.ErrNoToken: No token found in the request
//   - session.ErrInvalidToken: Token is malformed, expired, corrupted, or revoked
//   - session.ErrTransportFailed: Infrastructure errors (database, encryption, etc.)
//
// This provides a consistent error interface regardless of the underlying transport mechanism.
//
// # Security Considerations
//
// Cookie Transport:
//   - Uses strong encryption via the cookie package
//   - Supports secure attributes (Secure, HttpOnly, SameSite)
//   - Resistant to tampering and inspection
//   - Limited by browser cookie size (typically 4KB)
//   - Vulnerable to CSRF without proper CSRF protection
//
// JWT Transport:
//   - Tokens are signed but not encrypted (claims are base64-encoded)
//   - Session token is stored as JWT ID claim for revocation support
//   - No server-side storage required for basic validation
//   - Larger payload size compared to encrypted cookies
//   - Token revocation requires external storage (Redis, database)
//
// # Choosing a Transport
//
// Use Cookie Transport for:
//   - Traditional server-rendered web applications
//   - Maximum security with minimal infrastructure
//   - Automatic browser cookie handling
//   - Simple session invalidation
//
// Use JWT Transport for:
//   - REST APIs and stateless services
//   - Mobile applications and SPAs
//   - Cross-domain authentication
//   - Microservices architectures
//   - When you need to include custom claims
package sessiontransport
