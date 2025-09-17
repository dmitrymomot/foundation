package server

import (
	"crypto/tls"
	"log/slog"
	"time"
)

// Option configures server behavior.
type Option func(*Server)

// WithTLS configures TLS settings for HTTPS.
func WithTLS(config *tls.Config) Option {
	return func(s *Server) {
		s.mu.Lock()
		defer s.mu.Unlock()
		s.tlsConfig = config
	}
}

// WithLogger sets a custom logger for server operations.
func WithLogger(logger *slog.Logger) Option {
	return func(s *Server) {
		s.mu.Lock()
		defer s.mu.Unlock()
		s.logger = logger
	}
}

// WithShutdownTimeout sets the maximum time to wait for graceful shutdown.
func WithShutdownTimeout(timeout time.Duration) Option {
	return func(s *Server) {
		s.mu.Lock()
		defer s.mu.Unlock()
		s.shutdown = timeout
	}
}
