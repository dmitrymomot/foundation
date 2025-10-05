package session

import "time"

// Config provides environment-based configuration for session management.
// It allows control over session duration and touch behavior through environment variables.
type Config struct {
	TTL time.Duration `env:"SESSION_TTL" envDefault:"24h"`

	// TouchInterval reduces database writes by only updating sessions after this interval
	TouchInterval time.Duration `env:"SESSION_TOUCH_INTERVAL" envDefault:"15m"`
}

// DefaultConfig returns a Config with sensible defaults.
func DefaultConfig() Config {
	return Config{
		TTL:           24 * time.Hour,
		TouchInterval: 15 * time.Minute,
	}
}

// NewFromConfig creates a session Manager from configuration.
// The store parameter is required and must be provided by the caller.
func NewFromConfig[Data any](cfg Config, store Store[Data]) *Manager[Data] {
	return NewManager[Data](store, cfg.TTL, cfg.TouchInterval)
}
