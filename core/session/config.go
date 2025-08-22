package session

import (
	"time"
)

// Config holds session manager configuration.
type Config struct {
	// Token generation
	TokenLength int    // Length of random suffix for tokens (0 = UUID only)
	TokenPrefix string // Optional prefix for tokens (e.g., "sess_")

	// Timing
	TTL time.Duration // Session time-to-live
}

// defaultConfig returns default configuration.
func defaultConfig() *Config {
	return &Config{
		TokenLength: 0,  // UUID only by default
		TokenPrefix: "", // No prefix by default
		TTL:         24 * time.Hour,
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

// WithTokenLength sets the length of random suffix for tokens.
// Set to 0 to use UUID only (default).
func WithTokenLength(length int) Option {
	return func(c *Config) {
		c.TokenLength = length
	}
}

// WithTokenPrefix sets the prefix for session tokens.
// Useful for identifying token types (e.g., "sess_", "sid_").
func WithTokenPrefix(prefix string) Option {
	return func(c *Config) {
		c.TokenPrefix = prefix
	}
}
