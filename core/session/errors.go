package session

import "errors"

var (
	// ErrExpired is returned when a session has expired and is no longer valid.
	ErrExpired = errors.New("session has expired")
	// ErrNotFound is returned when a session cannot be found in the store.
	ErrNotFound = errors.New("session not found")
	// ErrNotAuthenticated is returned when authentication fails or no valid token is provided.
	ErrNotAuthenticated = errors.New("authentication failed")
	// ErrMissingIP is returned when creating a session without an IP address.
	ErrMissingIP = errors.New("IP address is required")
	// ErrTokenGeneration is returned when token generation fails.
	ErrTokenGeneration = errors.New("failed to generate token")
	// ErrSaveSession is returned when saving a session to the store fails.
	ErrSaveSession = errors.New("failed to save session")
	// ErrDeleteSession is returned when deleting a session from the store fails.
	ErrDeleteSession = errors.New("failed to delete session")
)
