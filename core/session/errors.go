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

	// ErrInvalidUserID is returned when attempting to authenticate with an invalid user ID.
	ErrInvalidUserID = errors.New("invalid user ID for authentication")

	// Transport errors
	// ErrNoToken is returned when no token is found in the transport (e.g., no cookie, no header).
	ErrNoToken = errors.New("no token in transport")

	// ErrTransportFailed is returned when a transport operation fails due to infrastructure issues.
	ErrTransportFailed = errors.New("transport operation failed")
)
