package session

import (
	"io"
	"log/slog"
	"time"
)

// Config holds session manager configuration.
type Config struct {
	// Timing
	TTL           time.Duration `env:"SESSION_TTL" envDefault:"24h"`           // Session time-to-live (idle timeout)
	TouchInterval time.Duration `env:"SESSION_TOUCH_INTERVAL" envDefault:"5m"` // Min time between activity updates (0 = disabled)
}

// DefaultConfig returns a Config with secure defaults.
func DefaultConfig() Config {
	return Config{
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

// NewFromConfig creates a Manager from configuration.
// Store and Transport must be provided via options.
// Additional options can override config values.
func NewFromConfig[Data any](cfg Config, opts ...ManagerOption[Data]) (*Manager[Data], error) {
	// Create a new manager with default config first
	m := &Manager[Data]{
		config: DefaultConfig(),
	}

	// Apply config values directly
	if cfg.TTL > 0 {
		m.config.TTL = cfg.TTL
	}
	if cfg.TouchInterval >= 0 {
		m.config.TouchInterval = cfg.TouchInterval
	}

	// Apply user-provided options (including Store, Transport, and any overrides)
	for _, opt := range opts {
		opt(m)
	}

	// Validate required dependencies
	if m.store == nil {
		return nil, ErrNoStore
	}
	if m.transport == nil {
		return nil, ErrNoTransport
	}

	// Set no-op logger if not provided via options
	if m.logger == nil {
		m.logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}

	return m, nil
}
