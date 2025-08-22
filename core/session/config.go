package session

import (
	"time"
)

// Config holds session manager configuration.
type Config struct {
	// Timing
	TTL           time.Duration // Session time-to-live (idle timeout)
	TouchInterval time.Duration // Min time between activity updates (0 = disabled)
}

func defaultConfig() *Config {
	return &Config{
		TTL:           24 * time.Hour,
		TouchInterval: 5 * time.Minute,
	}
}

// Option is a functional option for configuring the session manager.
type Option func(*Config)

// WithTTL sets the session time-to-live.
func WithTTL(ttl time.Duration) Option {
	return func(c *Config) {
		c.TTL = ttl
	}
}

// WithTouchInterval sets the minimum time between session activity updates.
// This prevents excessive storage writes (DDoS protection).
// Set to 0 to disable auto-touch functionality.
func WithTouchInterval(interval time.Duration) Option {
	return func(c *Config) {
		c.TouchInterval = interval
	}
}
