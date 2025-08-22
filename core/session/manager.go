package session

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"net/http"
	"time"

	"github.com/google/uuid"
)

// Manager handles session lifecycle operations.
// It coordinates between Store and Transport implementations.
type Manager[Data any] struct {
	store     Store[Data]
	transport Transport
	config    *Config
}

// ManagerOption is a functional option for configuring the session manager.
type ManagerOption[Data any] func(*Manager[Data])

// WithStore sets the session store.
func WithStore[Data any](store Store[Data]) ManagerOption[Data] {
	return func(m *Manager[Data]) {
		m.store = store
	}
}

// WithTransport sets the session transport.
func WithTransport[Data any](transport Transport) ManagerOption[Data] {
	return func(m *Manager[Data]) {
		m.transport = transport
	}
}

// WithConfig sets configuration options.
func WithConfig[Data any](opts ...Option) ManagerOption[Data] {
	return func(m *Manager[Data]) {
		for _, opt := range opts {
			opt(m.config)
		}
	}
}

// New creates a new session manager with the given options.
func New[Data any](opts ...ManagerOption[Data]) (*Manager[Data], error) {
	m := &Manager[Data]{
		config: defaultConfig(),
	}

	for _, opt := range opts {
		opt(m)
	}

	if m.store == nil {
		return nil, ErrNoStore
	}
	if m.transport == nil {
		return nil, ErrNoTransport
	}

	return m, nil
}

// Load retrieves an existing session or creates a new anonymous one.
// New sessions automatically get a DeviceID and have UserID set to uuid.Nil.
// If TouchInterval > 0, sessions are automatically extended on activity.
func (m *Manager[Data]) Load(w http.ResponseWriter, r *http.Request) (Session[Data], error) {
	token, err := m.transport.Extract(r)
	if err != nil || token == "" {
		return m.createNew()
	}

	session, err := m.store.Get(r.Context(), token)
	if err != nil {
		if errors.Is(err, ErrSessionNotFound) {
			return m.createNew()
		}
		return Session[Data]{}, err
	}

	if session.IsExpired() {
		// Create new but preserve DeviceID for analytics continuity
		newSession, err := m.createNew()
		if err != nil {
			return Session[Data]{}, err
		}
		newSession.DeviceID = session.DeviceID
		return newSession, nil
	}

	// Auto-touch if enabled
	if m.config.TouchInterval > 0 {
		_ = m.touch(w, r, session) // Best effort, ignore errors
	}

	return session, nil
}

// Save persists session changes to store and updates the response.
func (m *Manager[Data]) Save(w http.ResponseWriter, r *http.Request, session Session[Data]) error {
	session.UpdatedAt = time.Now()

	if err := m.store.Store(r.Context(), session); err != nil {
		return err
	}

	// Embed token in response (use existing token, don't regenerate)
	ttl := time.Until(session.ExpiresAt)
	return m.transport.Embed(w, r, session.Token, ttl)
}

// Touch extends session expiration on user activity.
// Safe to call frequently - internally throttled by TouchInterval.
func (m *Manager[Data]) Touch(w http.ResponseWriter, r *http.Request) error {
	// Extract token to get existing session (without auto-touch from Load)
	token, err := m.transport.Extract(r)
	if err != nil || token == "" {
		return nil // No session to touch
	}

	// Get session directly
	session, err := m.store.Get(r.Context(), token)
	if err != nil {
		return nil // Best effort - don't fail
	}

	return m.touch(w, r, session)
}

// touch is the internal implementation for extending session expiration.
// It updates both storage and transport to keep them in sync.
func (m *Manager[Data]) touch(w http.ResponseWriter, r *http.Request, session Session[Data]) error {
	now := time.Now()

	// Check throttling - prevent excessive updates
	if m.config.TouchInterval > 0 && now.Sub(session.UpdatedAt) < m.config.TouchInterval {
		return nil // Too soon, skip
	}

	session.UpdatedAt = now
	session.ExpiresAt = now.Add(m.config.TTL)

	// Update storage
	if err := m.store.Store(r.Context(), session); err != nil {
		// Could log error but continue - best effort
		return nil
	}

	// Update transport (refreshes cookie MaxAge) - use existing token
	ttl := time.Until(session.ExpiresAt)
	return m.transport.Embed(w, r, session.Token, ttl)
}

// Auth authenticates a session with the given user ID.
// It rotates the token for security while preserving session ID and DeviceID.
func (m *Manager[Data]) Auth(w http.ResponseWriter, r *http.Request, userID uuid.UUID) error {
	if userID == uuid.Nil {
		return ErrInvalidUserID
	}

	// Get current session (Load will auto-touch if configured, but we'll update anyway)
	session, err := m.Load(w, r)
	if err != nil {
		return err
	}

	// Rotate token for security
	newToken, err := generateToken()
	if err != nil {
		return err
	}

	session.Token = newToken
	session.UserID = userID
	session.UpdatedAt = time.Now()
	session.ExpiresAt = time.Now().Add(m.config.TTL)

	// Save authenticated session
	return m.Save(w, r, session)
}

// LogoutOption is a functional option for logout behavior.
type LogoutOption[Data any] func(*logoutConfig[Data])

type logoutConfig[Data any] struct {
	preserveData func(old Data) Data
}

// PreserveData allows preserving non-sensitive data during logout.
// The function receives the old session data and returns data to preserve.
// Example:
//
//	manager.Logout(w, r, session.PreserveData(func(old MyData) MyData {
//	    return MyData{
//	        Theme:  old.Theme,
//	        Locale: old.Locale,
//	        // UserID, Permissions, etc. are zeroed out
//	    }
//	}))
func PreserveData[Data any](fn func(old Data) Data) LogoutOption[Data] {
	return func(c *logoutConfig[Data]) {
		c.preserveData = fn
	}
}

// Logout returns the session to anonymous state.
// It creates a new session with new ID and token while preserving DeviceID.
// Optionally preserves non-sensitive data using PreserveData option.
func (m *Manager[Data]) Logout(w http.ResponseWriter, r *http.Request, opts ...LogoutOption[Data]) error {
	// Get current session (don't need auto-touch since we're logging out)
	session, err := m.Load(w, r)
	if err != nil {
		return err
	}

	// Process options
	cfg := &logoutConfig[Data]{}
	for _, opt := range opts {
		opt(cfg)
	}

	_ = m.store.Delete(r.Context(), session.ID)

	// Create new anonymous session
	newSession, err := m.createNew()
	if err != nil {
		return err
	}

	// Always preserve DeviceID for analytics continuity
	newSession.DeviceID = session.DeviceID

	// Preserve custom data if specified
	if cfg.preserveData != nil {
		newSession.Data = cfg.preserveData(session.Data)
	}

	// Save anonymous session
	return m.Save(w, r, newSession)
}

// Delete removes a session completely from both store and client.
// This operation is idempotent - it will not return an error if the session
// is already deleted or doesn't exist.
//
// Use this when you want to:
//   - Completely terminate a session without creating a new one
//   - Clean up sessions on security events (e.g., password change)
//   - Remove sessions when a user account is deleted
//
// This is different from Logout() which:
//   - Creates a new anonymous session with the same DeviceID
//   - Maintains analytics continuity
//   - Keeps the user on the site but unauthenticated
func (m *Manager[Data]) Delete(w http.ResponseWriter, r *http.Request) error {
	token, err := m.transport.Extract(r)
	if err != nil || token == "" {
		return nil
	}

	session, err := m.store.Get(r.Context(), token)
	if err != nil {
		if errors.Is(err, ErrSessionNotFound) {
			// Already deleted, clear transport
			return m.transport.Revoke(w, r)
		}
		// Other error, still try to clear transport
		_ = m.transport.Revoke(w, r)
		return err
	}

	if err := m.store.Delete(r.Context(), session.ID); err != nil {
		if !errors.Is(err, ErrSessionNotFound) {
			return err
		}
	}

	return m.transport.Revoke(w, r)
}

// createNew creates a new anonymous session.
func (m *Manager[Data]) createNew() (Session[Data], error) {
	now := time.Now()
	var data Data // Zero value of generic type

	// Generate secure token
	token, err := generateToken()
	if err != nil {
		return Session[Data]{}, err
	}

	session := Session[Data]{
		ID:        uuid.New(),
		Token:     token,
		DeviceID:  uuid.New(),
		UserID:    uuid.Nil, // Anonymous
		Data:      data,
		ExpiresAt: now.Add(m.config.TTL),
		CreatedAt: now,
		UpdatedAt: now,
	}

	return session, nil
}

// generateToken creates a cryptographically secure token.
// Uses 32 bytes (256 bits) for strong entropy, encoded as base64url without padding.
func generateToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", ErrTokenGeneration
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
