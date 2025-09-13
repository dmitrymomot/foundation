package session

import (
	"io"
	"log/slog"
	"time"
)

// Config holds session manager configuration.
type Config struct {
	// Timing
	TTL           time.Duration // Session time-to-live (idle timeout)
	TouchInterval time.Duration // Min time between activity updates (0 = disabled)

	// Logging
	Logger *slog.Logger // Logger for internal operations (nil = no-op logger)
}

func defaultConfig() *Config {
	return &Config{
		TTL:           24 * time.Hour,
		TouchInterval: 5 * time.Minute,
		Logger:        slog.New(slog.NewTextHandler(io.Discard, nil)), // No-op logger by default
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

// WithLogger sets the logger for internal session operations.
// If nil, a no-op logger will be used.
func WithLogger(logger *slog.Logger) Option {
	return func(c *Config) {
		if logger != nil {
			c.Logger = logger
		}
	}
}
