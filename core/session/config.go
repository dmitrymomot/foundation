package session

import "time"

// Config provides environment-based configuration for session management.
// It allows control over session duration and touch behavior through environment variables.
type Config struct {
	// TTL is the session time-to-live duration in seconds
	TTL int `env:"SESSION_TTL" envDefault:"86400"` // 24 hours

	// TouchInterval determines how often sessions are updated on access (in seconds).
	// This reduces database write operations while keeping sessions alive.
	TouchInterval int `env:"SESSION_TOUCH_INTERVAL" envDefault:"900"` // 15 minutes
}

// DefaultConfig returns a Config with sensible defaults.
func DefaultConfig() Config {
	return Config{
		TTL:           86400, // 24 hours
		TouchInterval: 900,   // 15 minutes
	}
}

// NewFromConfig creates a session Manager from configuration.
// The store parameter is required and must be provided by the caller.
func NewFromConfig[Data any](cfg Config, store Store[Data]) *Manager[Data] {
	ttl := time.Duration(cfg.TTL) * time.Second
	touchInterval := time.Duration(cfg.TouchInterval) * time.Second

	return NewManager[Data](store, ttl, touchInterval)
}
