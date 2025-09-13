package session

import (
	"context"
	"net/http"
	"time"

	"github.com/google/uuid"
)

// Session represents a user session with generic data type.
// Supports both anonymous and authenticated states.
type Session[Data any] struct {
	ID        uuid.UUID `json:"id"`         // Stable session identifier
	Token     string    `json:"-"`          // Raw secure token (not exposed in JSON)
	TokenHash string    `json:"-"`          // SHA-256 hash of the secure token (not exposed)
	DeviceID  uuid.UUID `json:"device_id"`  // Persistent device/browser ID
	UserID    uuid.UUID `json:"user_id"`    // User ID (uuid.Nil for anonymous)
	Data      Data      `json:"data"`       // Application-defined data
	ExpiresAt time.Time `json:"expires_at"` // Session expiration time
	CreatedAt time.Time `json:"created_at"` // Session creation time
	UpdatedAt time.Time `json:"updated_at"` // Last update time
}

// IsAuthenticated returns true if the session has a valid user ID.
func (s Session[Data]) IsAuthenticated() bool {
	return s.UserID != uuid.Nil
}

// IsExpired returns true if the session has expired.
func (s Session[Data]) IsExpired() bool {
	return time.Now().After(s.ExpiresAt)
}

// Store defines the interface for session persistence.
// Implementations can use cookies, Redis, databases, etc.
// All methods use value semantics to ensure thread safety.
type Store[Data any] interface {
	// Get retrieves a session by its token hash.
	// Returns a copy of the session to ensure thread safety.
	Get(ctx context.Context, tokenHash string) (Session[Data], error)

	// Store saves or updates a session.
	// Takes a copy of the session to ensure thread safety.
	Store(ctx context.Context, session Session[Data]) error

	// Delete removes a session by its ID.
	Delete(ctx context.Context, id uuid.UUID) error
}

// Transport defines how sessions are transmitted between client and server.
// Implementations can use cookies, headers, or JWT tokens.
type Transport interface {
	// Extract retrieves the session token from the request.
	Extract(r *http.Request) (token string, err error)

	// Embed adds the session token to the response.
	// The request is provided for implementations that need it (e.g., cookie consent checking).
	Embed(w http.ResponseWriter, r *http.Request, token string, ttl time.Duration) error

	// Revoke removes the session token from the response and invalidates it.
	// The request is provided to allow implementations to extract the token for revocation.
	Revoke(w http.ResponseWriter, r *http.Request) error
}
