package session

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/dmitrymomot/foundation/pkg/useragent"
)

// Session represents a user session with generic data storage.
// The Data type parameter allows custom session data structures specific to your application.
type Session[Data any] struct {
	// ID is the stable unique session identifier that never changes during the session lifecycle
	ID uuid.UUID

	// Token is the cryptographically secure session token (32 bytes base64url).
	// Used as cookie value or JWT JTI claim.
	Token string

	// UserID identifies the authenticated user (uuid.Nil for anonymous sessions)
	UserID uuid.UUID

	// Fingerprint is the device fingerprint for security validation (format: v1:hash, 35 chars)
	Fingerprint string

	IP        string
	UserAgent string

	// Data holds custom application-specific session information.
	// Examples: shopping cart, UI preferences, A/B test variants.
	Data Data

	ExpiresAt time.Time
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt time.Time

	// isModified tracks if the session needs saving
	isModified bool
}

// Device returns a human-readable device identifier based on the User-Agent string.
// Returns "Unknown device" if UserAgent is empty or parsing fails.
// Examples: "Chrome/120.0 (Windows, desktop)", "Safari/17.0 (iOS, mobile)", "Bot: Googlebot"
func (s Session[Data]) Device() string {
	if s.UserAgent == "" {
		return "Unknown device"
	}

	ua, err := useragent.Parse(s.UserAgent)
	if err != nil {
		return "Unknown device"
	}

	return ua.GetShortIdentifier()
}

// NewSessionParams contains parameters for creating a new session.
type NewSessionParams struct {
	Fingerprint string
	IP          string
	UserAgent   string
}

// New creates a new anonymous session with generated token and ID.
// The session is marked as modified and ready to be saved.
func New[Data any](params NewSessionParams, ttl time.Duration) (Session[Data], error) {
	if params.IP == "" {
		return Session[Data]{}, ErrMissingIP
	}

	token, err := generateToken()
	if err != nil {
		return Session[Data]{}, errors.Join(ErrTokenGeneration, err)
	}

	now := time.Now()
	return Session[Data]{
		ID:          uuid.New(),
		Token:       token,
		UserID:      uuid.Nil,
		Fingerprint: params.Fingerprint,
		IP:          params.IP,
		UserAgent:   params.UserAgent,
		Data:        *new(Data),
		ExpiresAt:   now.Add(ttl),
		CreatedAt:   now,
		UpdatedAt:   now,
		isModified:  true,
	}, nil
}

// Authenticate marks the session for authentication with the given userID.
// Rotates the session token but preserves the session ID for security.
// Optional data parameter sets session data.
func (s *Session[Data]) Authenticate(userID uuid.UUID, data ...Data) error {
	if err := s.rotateToken(); err != nil {
		return err
	}
	s.UserID = userID
	if len(data) > 0 {
		s.Data = data[0]
	}
	s.UpdatedAt = time.Now()
	s.isModified = true
	return nil
}

// Refresh rotates the session token without changing authentication state or session ID.
// Useful for periodic token rotation for security.
func (s *Session[Data]) Refresh() error {
	if err := s.rotateToken(); err != nil {
		return err
	}
	s.UpdatedAt = time.Now()
	s.isModified = true
	return nil
}

// Logout marks the session for deletion by setting DeletedAt timestamp.
func (s *Session[Data]) Logout() {
	s.DeletedAt = time.Now()
	s.isModified = true
}

// SetData updates the session's custom data.
func (s *Session[Data]) SetData(data Data) {
	s.Data = data
	s.UpdatedAt = time.Now()
	s.isModified = true
}

// Touch extends the session expiration if the touch interval has elapsed.
// This reduces write operations by only updating when sufficient time has passed.
func (s *Session[Data]) Touch(ttl, touchInterval time.Duration) {
	if time.Since(s.UpdatedAt) >= touchInterval {
		s.ExpiresAt = time.Now().Add(ttl)
		s.UpdatedAt = time.Now()
		s.isModified = true
	}
}

// IsAuthenticated returns true if the session has a valid user ID.
func (s Session[Data]) IsAuthenticated() bool {
	return s.UserID != uuid.Nil && s.Token != ""
}

// IsDeleted returns true if the session is marked for deletion.
func (s Session[Data]) IsDeleted() bool {
	return !s.DeletedAt.IsZero()
}

// IsModified returns true if the session has been modified and needs saving.
func (s Session[Data]) IsModified() bool {
	return s.isModified
}

// IsExpired returns true if the session has expired.
func (s Session[Data]) IsExpired() bool {
	return time.Now().After(s.ExpiresAt)
}

// rotateToken generates a new token while preserving the session ID.
func (s *Session[Data]) rotateToken() error {
	newToken, err := generateToken()
	if err != nil {
		return errors.Join(ErrTokenGeneration, err)
	}
	s.Token = newToken
	s.isModified = true
	return nil
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
