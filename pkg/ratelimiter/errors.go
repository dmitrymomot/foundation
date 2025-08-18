package ratelimiter

import "errors"

// Package-level error definitions for rate limiter operations.
var (
	ErrInvalidConfig     = errors.New("invalid configuration")
	ErrInvalidTokenCount = errors.New("invalid token count")
	ErrContextCancelled  = errors.New("context cancelled")
	ErrStoreUnavailable  = errors.New("store unavailable")
	ErrRateLimitExceeded = errors.New("rate limit exceeded")
)
