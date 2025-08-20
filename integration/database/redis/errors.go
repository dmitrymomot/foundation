package redis

import "errors"

// Domain-specific Redis errors for consistent error handling across the application.
// Use errors.Is() to check error types for retry logic and user-facing messages.
var (
	ErrFailedToParseRedisConnString = errors.New("failed to parse redis connection string")
	ErrRedisNotReady                = errors.New("redis did not become ready within the given time period")
	ErrEmptyConnectionURL           = errors.New("empty redis connection URL")
	ErrHealthcheckFailed            = errors.New("redis healthcheck failed")
)
