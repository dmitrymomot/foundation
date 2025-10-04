package session

import (
	"crypto/rand"
	"encoding/base64"
	"time"

	"github.com/google/uuid"
)

// Session represents a user session with generic data storage.
// The Data type parameter allows custom session data structures specific to your application.
type Session[Data any] struct {
	// ID is the unique session identifier, rotated on authentication/logout
	ID uuid.UUID

	// Token is the cryptographically secure session token (32 bytes base64url).
	// Used as cookie value or JWT JTI claim.
	Token string

	// UserID identifies the authenticated user (uuid.Nil for anonymous sessions)
	UserID uuid.UUID

	// Fingerprint is the device fingerprint for security validation (format: v1:hash, 35 chars)
	Fingerprint string

	// Data holds custom application-specific session information.
	// Examples: shopping cart, UI preferences, A/B test variants.
	Data Data

	// ExpiresAt is when the session becomes invalid
	ExpiresAt time.Time

	// CreatedAt records initial session creation time
	CreatedAt time.Time

	// UpdatedAt tracks last session modification or touch
	UpdatedAt time.Time
}

// generateToken creates a cryptographically secure random token using 32 bytes (256 bits)
// encoded as base64 URL-safe string without padding.
func generateToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
