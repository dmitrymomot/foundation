package session

import "errors"

var (
	// ErrSessionNotFound is returned when a session cannot be found in the store.
	ErrSessionNotFound = errors.New("session not found")

	// ErrSessionExpired is returned when a session has passed its expiration time.
	ErrSessionExpired = errors.New("session expired")

	// ErrInvalidToken is returned when a session token is malformed or invalid.
	ErrInvalidToken = errors.New("invalid session token")

	// ErrNoTransport is returned when no transport is configured.
	ErrNoTransport = errors.New("no transport configured")

	// ErrNoStore is returned when no store is configured.
	ErrNoStore = errors.New("no store configured")

	// ErrTokenGeneration is returned when token generation fails.
	ErrTokenGeneration = errors.New("failed to generate token")
)
