package main

import (
	"context"
	"foundation-basic-example/db/repository"

	"github.com/dmitrymomot/foundation/core/session"
	"github.com/google/uuid"
)

type SessionData struct {
	Name  string
	Email string
}

type sessionStorage struct {
	repo repository.Querier
}

// Get retrieves a session by its token hash.
func (s *sessionStorage) Get(ctx context.Context, tokenHash string) (session.Session[SessionData], error) {
	return session.Session[SessionData]{}, nil
}

// Store saves or updates a session.
func (s *sessionStorage) Store(ctx context.Context, sess session.Session[SessionData]) error {
	return nil
}

// Delete removes a session by its ID.
func (s *sessionStorage) Delete(ctx context.Context, id uuid.UUID) error {
	return nil
}

// Ensure sessionStorage implements required interface
var _ session.Store[SessionData] = (*sessionStorage)(nil)
