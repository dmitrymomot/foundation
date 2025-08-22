package session

import (
	"crypto/rand"
	"encoding/base64"
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
func New[Data any](opts ...ManagerOption[Data]) *Manager[Data] {
	m := &Manager[Data]{
		config: defaultConfig(),
	}

	for _, opt := range opts {
		opt(m)
	}

	return m
}

// Get retrieves an existing session or creates a new anonymous one.
// New sessions automatically get a DeviceID and have UserID set to uuid.Nil.
func (m *Manager[Data]) Get(r *http.Request) (*Session[Data], error) {
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
		// Session not found, create new
		return m.createNew()
	}

	// Check expiration
	if session.IsExpired() {
		// Session expired, create new but preserve DeviceID
		newSession, _ := m.createNew()
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
	token := m.generateToken(session.ID)

	// Embed token in response
	ttl := time.Until(session.ExpiresAt)
	return m.transport.Embed(w, token, ttl)
}

// Auth authenticates a session with the given user ID.
// It rotates the session ID for security while preserving DeviceID.
func (m *Manager[Data]) Auth(w http.ResponseWriter, r *http.Request, userID uuid.UUID) error {
	// Get current session
	session, err := m.Get(r)
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
	session, err := m.Get(r)
	if err != nil {
		return err
	}

	// Preserve device tracking
	deviceID := session.DeviceID

	// Create new anonymous session with same device
	newSession, _ := m.createNew()
	newSession.DeviceID = deviceID

	// Preserve non-sensitive data if needed
	// This is application-specific and could be configured

	// Save anonymous session
	return m.Save(w, r, newSession)
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
func (m *Manager[Data]) generateToken(id uuid.UUID) string {
	// Add prefix if configured
	token := id.String()
	if m.config.TokenPrefix != "" {
		token = m.config.TokenPrefix + token
	}

	// Add random suffix for additional entropy if configured
	if m.config.TokenLength > 0 {
		suffix := make([]byte, m.config.TokenLength)
		rand.Read(suffix)
		token = token + "." + base64.RawURLEncoding.EncodeToString(suffix)
	}

	return token
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
