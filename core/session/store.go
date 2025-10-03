package session

import (
	"context"

	"github.com/google/uuid"
)

// Store defines the persistence interface for session management.
// Implementations must handle concurrent access safely.
type Store[Data any] interface {
	GetByID(ctx context.Context, id uuid.UUID) (*Session[Data], error)
	GetByToken(ctx context.Context, token string) (*Session[Data], error)
	Save(ctx context.Context, session *Session[Data]) error
	Delete(ctx context.Context, id uuid.UUID) error
	// DeleteExpired removes all expired sessions and returns the count of deleted sessions.
	DeleteExpired(ctx context.Context) (int64, error)
}
