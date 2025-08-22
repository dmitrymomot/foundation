package session

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"net/http"
	"strings"
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

	// Validate required dependencies
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
func (m *Manager[Data]) Load(r *http.Request) (*Session[Data], error) {
	// Try to extract token from request
	token, err := m.transport.Extract(r)
	if err != nil || token == "" {
		// No token found, create new anonymous session
		return m.createNew()
	}

	// Parse token to get session ID
	id, err := m.parseToken(token)
	if err != nil {
		// Invalid token, create new session
		return m.createNew()
	}

	// Load session from store
	session, err := m.store.Get(r.Context(), id)
	if err != nil {
		// Check if it's a "not found" error vs other errors
		if errors.Is(err, ErrSessionNotFound) {
			// Session not found, create new
			return m.createNew()
		}
		// Propagate other errors (network, storage issues)
		return nil, err
	}

	// Check expiration
	if session.IsExpired() {
		// Session expired, create new but preserve DeviceID
		newSession, err := m.createNew()
		if err != nil {
			return nil, err
		}
		newSession.DeviceID = session.DeviceID
		return newSession, nil
	}

	return session, nil
}

// Save persists session changes to store and updates the response.
func (m *Manager[Data]) Save(w http.ResponseWriter, r *http.Request, session *Session[Data]) error {
	// Update timestamp
	session.UpdatedAt = time.Now()

	// Save to store
	if err := m.store.Store(r.Context(), session); err != nil {
		return err
	}

	// Generate token
	token, err := m.generateToken(session.ID)
	if err != nil {
		return err
	}

	// Embed token in response
	ttl := time.Until(session.ExpiresAt)
	return m.transport.Embed(w, token, ttl)
}

// Auth authenticates a session with the given user ID.
// It rotates the session ID for security while preserving DeviceID.
func (m *Manager[Data]) Auth(w http.ResponseWriter, r *http.Request, userID uuid.UUID) error {
	// Get current session
	session, err := m.Load(r)
	if err != nil {
		return err
	}

	// Preserve device tracking
	deviceID := session.DeviceID

	// Rotate session ID for security
	session.ID = uuid.New()
	session.UserID = userID
	session.DeviceID = deviceID
	session.UpdatedAt = time.Now()
	session.ExpiresAt = time.Now().Add(m.config.TTL)

	// Save authenticated session
	return m.Save(w, r, session)
}

// Logout returns the session to anonymous state.
// It rotates the session ID and clears the user ID while preserving DeviceID.
func (m *Manager[Data]) Logout(w http.ResponseWriter, r *http.Request) error {
	// Get current session
	session, err := m.Load(r)
	if err != nil {
		return err
	}

	// Delete the old session from store
	oldID := session.ID
	if err := m.store.Delete(r.Context(), oldID); err != nil {
		// Log error but continue - we still want to create new session
		// In production, you might want to handle this differently
	}

	// Preserve device tracking
	deviceID := session.DeviceID

	// Create new anonymous session with same device
	newSession, err := m.createNew()
	if err != nil {
		return err
	}
	newSession.DeviceID = deviceID

	// Preserve non-sensitive data if needed
	// This is application-specific and could be configured

	// Save anonymous session
	return m.Save(w, r, newSession)
}

// Delete removes a session completely from both store and client.
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
	// Try to extract token from request
	token, err := m.transport.Extract(r)
	if err != nil || token == "" {
		// No token found, nothing to delete
		return nil
	}

	// Parse token to get session ID
	id, err := m.parseToken(token)
	if err != nil {
		// Invalid token, nothing to delete
		return nil
	}

	// Delete from store
	if err := m.store.Delete(r.Context(), id); err != nil {
		// Check if it's a "not found" error
		if errors.Is(err, ErrSessionNotFound) {
			// Already deleted, clear transport
			return m.transport.Clear(w)
		}
		return err
	}

	// Clear from transport
	return m.transport.Clear(w)
}

// createNew creates a new anonymous session.
func (m *Manager[Data]) createNew() (*Session[Data], error) {
	now := time.Now()
	var data Data // Zero value of generic type

	session := &Session[Data]{
		ID:        uuid.New(),
		DeviceID:  uuid.New(),
		UserID:    uuid.Nil, // Anonymous
		Data:      data,
		ExpiresAt: now.Add(m.config.TTL),
		CreatedAt: now,
		UpdatedAt: now,
	}

	return session, nil
}

// generateToken creates a secure token from session ID.
func (m *Manager[Data]) generateToken(id uuid.UUID) (string, error) {
	// Add prefix if configured
	token := id.String()
	if m.config.TokenPrefix != "" {
		token = m.config.TokenPrefix + token
	}

	// Add random suffix for additional entropy if configured
	if m.config.TokenLength > 0 {
		suffix := make([]byte, m.config.TokenLength)
		if _, err := rand.Read(suffix); err != nil {
			return "", err
		}
		token = token + "." + base64.RawURLEncoding.EncodeToString(suffix)
	}

	return token, nil
}

// parseToken extracts session ID from token.
func (m *Manager[Data]) parseToken(token string) (uuid.UUID, error) {
	// Remove prefix if present
	if m.config.TokenPrefix != "" {
		token = strings.TrimPrefix(token, m.config.TokenPrefix)
	}

	// Remove suffix if present
	if idx := strings.Index(token, "."); idx > 0 {
		token = token[:idx]
	}

	// Parse UUID
	return uuid.Parse(token)
}
