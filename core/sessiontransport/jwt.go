package sessiontransport

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/dmitrymomot/foundation/core/session"
	"github.com/dmitrymomot/foundation/pkg/jwt"
)

// JWTTransport implements session.Transport using JWT tokens.
// Supports optional token revocation through the Revoker interface.
type JWTTransport struct {
	service      *jwt.Service
	revoker      Revoker // Can be nil for no revocation
	headerName   string
	bearerPrefix bool
	issuer       string
	audience     string
}

// JWTOption configures the JWT transport.
type JWTOption func(*JWTTransport)

// WithJWTHeaderName sets a custom header name for JWT tokens.
// Default is "Authorization".
func WithJWTHeaderName(name string) JWTOption {
	return func(t *JWTTransport) {
		if name != "" {
			t.headerName = name
		}
	}
}

// WithJWTBearerPrefix controls whether to use "Bearer " prefix.
// Default is true.
func WithJWTBearerPrefix(usePrefix bool) JWTOption {
	return func(t *JWTTransport) {
		t.bearerPrefix = usePrefix
	}
}

// WithJWTIssuer sets the issuer claim for generated tokens.
func WithJWTIssuer(issuer string) JWTOption {
	return func(t *JWTTransport) {
		t.issuer = issuer
	}
}

// WithJWTAudience sets the audience claim for generated tokens.
func WithJWTAudience(audience string) JWTOption {
	return func(t *JWTTransport) {
		t.audience = audience
	}
}

// NewJWT creates a new JWT-based session transport.
//
// # Revoker Parameter
//
// The revoker parameter is OPTIONAL and can be nil. Understanding when to use it:
//
// WITHOUT Revoker (revoker = nil):
//   - Every request validates JWT signature, then calls Store.Get(tokenHash)
//   - Deleting session from Store is SUFFICIENT to invalidate the JWT
//   - JWT becomes useless because Store.Get() returns ErrSessionNotFound
//   - Simple, works fine for most applications
//   - Recommended unless you have specific performance/security needs
//
// WITH Revoker (revoker = implementation):
//   - Provides fast-fail before hitting Store (performance optimization)
//   - Useful when Store is slower than Revoker (e.g., Redis blacklist vs DB lookup)
//   - Explicit distinction between "revoked" vs "session not found"
//   - Additional security layer for sensitive operations
//   - Requires maintaining revocation list with TTL matching JWT expiration
//
// Example without revoker:
//
//	transport, err := sessiontransport.NewJWT(signingKey, nil)
//	// Delete from Store invalidates the JWT - no revoker needed
//	manager.Delete(w, r)
//
// Example with revoker:
//
//	revoker := &RedisRevoker{client: redisClient}
//	transport, err := sessiontransport.NewJWT(signingKey, revoker)
//	// Fast revocation check before Store lookup
func NewJWT(signingKey string, revoker Revoker, opts ...JWTOption) (*JWTTransport, error) {
	service, err := jwt.NewFromString(signingKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create JWT service: %w", err)
	}

	t := &JWTTransport{
		service:      service,
		revoker:      revoker,
		headerName:   "Authorization",
		bearerPrefix: true,
	}

	for _, opt := range opts {
		opt(t)
	}

	return t, nil
}

// Extract retrieves and validates the session token from the JWT in the request header.
func (t *JWTTransport) Extract(r *http.Request) (string, error) {
	// Get token from header
	authHeader := r.Header.Get(t.headerName)
	if authHeader == "" {
		return "", session.ErrNoToken
	}

	// Extract token from header value
	tokenString := authHeader
	if t.bearerPrefix {
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || parts[0] != "Bearer" {
			return "", session.ErrInvalidToken
		}
		tokenString = parts[1]
	}

	if tokenString == "" {
		return "", session.ErrNoToken
	}

	// Parse and validate JWT
	var claims jwt.StandardClaims
	err := t.service.Parse(tokenString, &claims)
	if err != nil {
		// Map JWT errors to session errors
		if errors.Is(err, jwt.ErrExpiredToken) ||
			errors.Is(err, jwt.ErrInvalidToken) ||
			errors.Is(err, jwt.ErrInvalidSignature) ||
			errors.Is(err, jwt.ErrUnexpectedSigningMethod) {
			return "", session.ErrInvalidToken
		}
		return "", session.ErrTransportFailed
	}

	// Check if session token (stored as JWT ID) is revoked
	if t.revoker != nil && claims.ID != "" {
		ctx := r.Context()
		revoked, err := t.revoker.IsRevoked(ctx, claims.ID)
		if err != nil {
			return "", session.ErrTransportFailed
		}
		if revoked {
			return "", session.ErrInvalidToken
		}
	}

	// Return the JWT ID which IS the session token
	if claims.ID == "" {
		return "", session.ErrInvalidToken
	}

	return claims.ID, nil
}

// Embed creates a JWT containing the session token and adds it to the response header.
func (t *JWTTransport) Embed(w http.ResponseWriter, r *http.Request, token string, ttl time.Duration) error {
	// Create claims with session token as JWT ID
	now := time.Now()
	claims := jwt.StandardClaims{
		ID:        token, // Session token IS the JWT ID
		IssuedAt:  now.Unix(),
		ExpiresAt: now.Add(ttl).Unix(),
		Issuer:    t.issuer,
		Audience:  t.audience,
	}

	// Generate JWT
	jwtToken, err := t.service.Generate(claims)
	if err != nil {
		return session.ErrTransportFailed
	}

	// Set header
	headerValue := jwtToken
	if t.bearerPrefix {
		headerValue = "Bearer " + jwtToken
	}
	w.Header().Set(t.headerName, headerValue)

	return nil
}

// Revoke removes the JWT from the response header and revokes the session token if configured.
func (t *JWTTransport) Revoke(w http.ResponseWriter, r *http.Request) error {
	// Remove the header from response
	w.Header().Del(t.headerName)

	// If no revoker configured, we're done
	if t.revoker == nil {
		return nil
	}

	// Get token from request header
	authHeader := r.Header.Get(t.headerName)
	if authHeader == "" {
		return nil // No token to revoke
	}

	// Extract token from header value
	tokenString := authHeader
	if t.bearerPrefix {
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || parts[0] != "Bearer" {
			return nil // Invalid format, skip revocation
		}
		tokenString = parts[1]
	}

	if tokenString == "" {
		return nil // No token to revoke
	}

	// Parse JWT to get the session token (stored as JWT ID)
	var claims jwt.StandardClaims
	err := t.service.Parse(tokenString, &claims)
	if err != nil {
		// Invalid token, skip revocation
		return nil
	}

	// Revoke the session token if present
	if claims.ID != "" {
		ctx := r.Context()
		return t.revoker.Revoke(ctx, claims.ID)
	}

	return nil
}
