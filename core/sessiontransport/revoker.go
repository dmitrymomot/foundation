package sessiontransport

import "context"

// Revoker handles JWT token revocation using session tokens as JWT IDs.
// Implementations can use Redis, databases, or in-memory storage.
type Revoker interface {
	// IsRevoked checks if a session token (used as JWT ID) has been revoked.
	// Returns true if the session token is in the revocation list.
	IsRevoked(ctx context.Context, jti string) (bool, error)

	// Revoke marks a session token (used as JWT ID) as revoked.
	// The session token should remain revoked until the JWT's natural expiration.
	Revoke(ctx context.Context, jti string) error
}

// NoOpRevoker is a no-op implementation that never revokes tokens.
// Use this when token revocation is not required.
type NoOpRevoker struct{}

// IsRevoked always returns false - no JWT IDs are considered revoked.
func (NoOpRevoker) IsRevoked(ctx context.Context, jti string) (bool, error) {
	return false, nil
}

// Revoke is a no-op - does nothing and returns nil.
func (NoOpRevoker) Revoke(ctx context.Context, jti string) error {
	return nil
}
