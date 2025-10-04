package session

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// Manager handles session lifecycle including creation, retrieval, and expiration.
// The touchInterval determines how often sessions are automatically extended on access,
// reducing write operations to the store.
type Manager[Data any] struct {
	store         Store[Data]
	ttl           time.Duration
	touchInterval time.Duration
}

// NewManager creates a session manager with the specified store, time-to-live duration,
// and touch interval. The touchInterval prevents updating session expiration on every access,
// reducing write operations to the store.
func NewManager[Data any](store Store[Data], ttl, touchInterval time.Duration) *Manager[Data] {
	return &Manager[Data]{
		store:         store,
		ttl:           ttl,
		touchInterval: touchInterval,
	}
}

// New creates and persists a new anonymous session with empty data.
func (m *Manager[Data]) New(ctx context.Context, fingerprint string) (Session[Data], error) {
	token, err := generateToken()
	if err != nil {
		return Session[Data]{}, fmt.Errorf("failed to generate token: %w", err)
	}

	now := time.Now()
	session := Session[Data]{
		ID:          uuid.New(),
		Token:       token,
		UserID:      uuid.Nil,
		Fingerprint: fingerprint,
		Data:        *new(Data),
		ExpiresAt:   now.Add(m.ttl),
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	if err := m.store.Save(ctx, &session); err != nil {
		return Session[Data]{}, fmt.Errorf("failed to save session: %w", err)
	}

	return session, nil
}

// GetByID retrieves a session by ID, extending its expiration if the touch interval has elapsed.
func (m *Manager[Data]) GetByID(ctx context.Context, id uuid.UUID) (Session[Data], error) {
	now := time.Now()
	session, err := m.store.GetByID(ctx, id)
	if err != nil {
		return Session[Data]{}, err
	}

	if now.After(session.ExpiresAt) {
		return Session[Data]{}, ErrExpired
	}

	if m.shouldTouch(session) {
		touched, err := m.touch(ctx, *session)
		if err != nil {
			return Session[Data]{}, err
		}
		return touched, nil
	}

	return *session, nil
}

// GetByToken retrieves a session by token, extending its expiration if the touch interval has elapsed.
func (m *Manager[Data]) GetByToken(ctx context.Context, token string) (Session[Data], error) {
	now := time.Now()
	session, err := m.store.GetByToken(ctx, token)
	if err != nil {
		return Session[Data]{}, err
	}

	if now.After(session.ExpiresAt) {
		return Session[Data]{}, ErrExpired
	}

	if m.shouldTouch(session) {
		touched, err := m.touch(ctx, *session)
		if err != nil {
			return Session[Data]{}, err
		}
		return touched, nil
	}

	return *session, nil
}

// Save updates an existing session in the store.
func (m *Manager[Data]) Save(ctx context.Context, sess *Session[Data]) error {
	now := time.Now()
	sess.UpdatedAt = now
	return m.store.Save(ctx, sess)
}

// Authenticate converts an anonymous session to an authenticated session.
// Rotates the session token for security, preserves session data, and extends expiration.
func (m *Manager[Data]) Authenticate(ctx context.Context, sess Session[Data], userID uuid.UUID) (Session[Data], error) {
	newToken, err := generateToken()
	if err != nil {
		return Session[Data]{}, fmt.Errorf("failed to generate token: %w", err)
	}

	if err := m.store.Delete(ctx, sess.ID); err != nil {
		return Session[Data]{}, fmt.Errorf("failed to delete old session: %w", err)
	}

	now := time.Now()
	authenticated := Session[Data]{
		ID:          uuid.New(),
		Token:       newToken,
		UserID:      userID,
		Fingerprint: sess.Fingerprint,
		Data:        sess.Data,
		ExpiresAt:   now.Add(m.ttl),
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	if err := m.store.Save(ctx, &authenticated); err != nil {
		return Session[Data]{}, fmt.Errorf("failed to save authenticated session: %w", err)
	}

	return authenticated, nil
}

// Logout converts an authenticated session to an anonymous session.
// Rotates the session token for security, clears user ID and data, and extends expiration.
func (m *Manager[Data]) Logout(ctx context.Context, sess Session[Data]) (Session[Data], error) {
	newToken, err := generateToken()
	if err != nil {
		return Session[Data]{}, fmt.Errorf("failed to generate token: %w", err)
	}

	if err := m.store.Delete(ctx, sess.ID); err != nil {
		return Session[Data]{}, fmt.Errorf("failed to delete old session: %w", err)
	}

	now := time.Now()
	anonymous := Session[Data]{
		ID:          uuid.New(),
		Token:       newToken,
		UserID:      uuid.Nil,
		Fingerprint: sess.Fingerprint,
		Data:        *new(Data),
		ExpiresAt:   now.Add(m.ttl),
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	if err := m.store.Save(ctx, &anonymous); err != nil {
		return Session[Data]{}, fmt.Errorf("failed to save anonymous session: %w", err)
	}

	return anonymous, nil
}

// Delete removes a session by ID.
func (m *Manager[Data]) Delete(ctx context.Context, id uuid.UUID) error {
	return m.store.Delete(ctx, id)
}

// shouldTouch returns true if enough time has passed since the last update
// to warrant extending the session expiration.
func (m *Manager[Data]) shouldTouch(session *Session[Data]) bool {
	return time.Since(session.UpdatedAt) >= m.touchInterval
}

// touch extends the session expiration and updates the timestamp.
// This reduces write operations by only updating when touchInterval has elapsed.
func (m *Manager[Data]) touch(ctx context.Context, sess Session[Data]) (Session[Data], error) {
	if err := ctx.Err(); err != nil {
		return Session[Data]{}, err
	}

	now := time.Now()
	sess.ExpiresAt = now.Add(m.ttl)
	sess.UpdatedAt = now
	if err := m.store.Save(ctx, &sess); err != nil {
		return Session[Data]{}, err
	}
	return sess, nil
}

// CleanupExpired removes all expired sessions from the store.
// Should be called periodically to prevent session table growth.
func (m *Manager[Data]) CleanupExpired(ctx context.Context) error {
	return m.store.DeleteExpired(ctx)
}

// GetTTL returns the session time-to-live duration.
func (m *Manager[Data]) GetTTL() time.Duration {
	return m.ttl
}

// Refresh rotates the session token and extends expiration.
// Unlike Authenticate/Logout, this keeps the same session ID (critical for audit logs).
func (m *Manager[Data]) Refresh(ctx context.Context, sess Session[Data]) (Session[Data], error) {
	if sess.UserID == uuid.Nil {
		return Session[Data]{}, ErrNotAuthenticated
	}

	newToken, err := generateToken()
	if err != nil {
		return Session[Data]{}, fmt.Errorf("failed to generate token: %w", err)
	}

	now := time.Now()
	sess.Token = newToken
	sess.ExpiresAt = now.Add(m.ttl)
	sess.UpdatedAt = now

	if err := m.store.Save(ctx, &sess); err != nil {
		return Session[Data]{}, fmt.Errorf("failed to save refreshed session: %w", err)
	}

	return sess, nil
}
