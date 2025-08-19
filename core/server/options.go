package server

import (
	"crypto/tls"
	"log/slog"
	"time"
)

type Option func(*Server)

func WithTLS(config *tls.Config) Option {
	return func(s *Server) {
		s.mu.Lock()
		defer s.mu.Unlock()
		s.tlsConfig = config
	}
}

func WithLogger(logger *slog.Logger) Option {
	return func(s *Server) {
		s.mu.Lock()
		defer s.mu.Unlock()
		s.logger = logger
	}
}

func WithShutdownTimeout(timeout time.Duration) Option {
	return func(s *Server) {
		s.mu.Lock()
		defer s.mu.Unlock()
		s.shutdown = timeout
	}
}
