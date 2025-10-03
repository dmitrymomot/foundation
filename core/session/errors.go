package session

import "errors"

var (
	// ErrExpired is returned when a session has expired and is no longer valid.
	ErrExpired = errors.New("session.expired")
	// ErrNotFound is returned when a session cannot be found in the store.
	ErrNotFound = errors.New("session.not_found")
	// ErrNotAuthenticated is returned when authentication fails or no valid token is provided.
	ErrNotAuthenticated = errors.New("session.not_authenticated")
)
