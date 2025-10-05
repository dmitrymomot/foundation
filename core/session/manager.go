package session

import (
	"context"
	"errors"
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

// GetByID retrieves a session by ID and validates expiration.
func (m *Manager[Data]) GetByID(ctx context.Context, id uuid.UUID) (Session[Data], error) {
	session, err := m.store.GetByID(ctx, id)
	if err != nil {
		return Session[Data]{}, err
	}

	if session.IsExpired() {
		return Session[Data]{}, ErrExpired
	}

	return *session, nil
}

// GetByToken retrieves a session by token and validates expiration.
func (m *Manager[Data]) GetByToken(ctx context.Context, token string) (Session[Data], error) {
	session, err := m.store.GetByToken(ctx, token)
	if err != nil {
		return Session[Data]{}, err
	}

	if session.IsExpired() {
		return Session[Data]{}, ErrExpired
	}

	return *session, nil
}

// Store handles all session persistence based on session state.
// When a session is deleted, returns ErrNotAuthenticated to signal Transport for cookie/token cleanup.
func (m *Manager[Data]) Store(ctx context.Context, sess Session[Data]) error {
	if sess.IsDeleted() {
		if err := m.store.Delete(ctx, sess.ID); err != nil && !errors.Is(err, ErrNotFound) {
			return errors.Join(ErrDeleteSession, err)
		}
		return ErrNotAuthenticated
	}

	sess.Touch(m.ttl, m.touchInterval)

	if sess.IsModified() {
		return m.store.Save(ctx, &sess)
	}

	return nil
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
