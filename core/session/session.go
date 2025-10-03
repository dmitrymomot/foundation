package session

import (
	"crypto/rand"
	"encoding/base64"
	"time"

	"github.com/google/uuid"
)

// Session represents a user session with generic data storage.
// Data is a type parameter allowing custom session data structures.
type Session[Data any] struct {
	ID        uuid.UUID
	Token     string
	UserID    uuid.UUID
	Data      Data
	ExpiresAt time.Time
	CreatedAt time.Time
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
