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

// WithReadTimeout sets the maximum duration for reading requests.
func WithReadTimeout(timeout time.Duration) Option {
	return func(s *Server) {
		s.mu.Lock()
		defer s.mu.Unlock()
		s.readTimeout = timeout
	}
}

// WithWriteTimeout sets the maximum duration for writing responses.
func WithWriteTimeout(timeout time.Duration) Option {
	return func(s *Server) {
		s.mu.Lock()
		defer s.mu.Unlock()
		s.writeTimeout = timeout
	}
}

// WithIdleTimeout sets the maximum duration for idle connections.
func WithIdleTimeout(timeout time.Duration) Option {
	return func(s *Server) {
		s.mu.Lock()
		defer s.mu.Unlock()
		s.idleTimeout = timeout
	}
}

// WithMaxHeaderBytes sets the maximum size of request headers.
func WithMaxHeaderBytes(max int) Option {
	return func(s *Server) {
		s.mu.Lock()
		defer s.mu.Unlock()
		s.maxHeaderBytes = max
	}
}

// WithServerShutdownTimeout sets the maximum time to wait for graceful shutdown of AutoCertServer.
func WithServerShutdownTimeout(timeout time.Duration) AutoCertOption {
	return func(s *AutoCertServer) {
		if timeout > 0 {
			s.shutdownTimeout = timeout
		}
	}
}
