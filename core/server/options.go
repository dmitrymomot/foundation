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

// WithAutoCert is deprecated. Use NewAutoCertServer directly instead.
// This is kept for documentation purposes only.
//
// Usage:
//
//	config := &server.AutoCertConfig[MyContext]{
//	    CertManager: certManager,
//	    DomainStore: domainStore,
//	    HTTPAddr:    ":80",
//	    HTTPSAddr:   ":443",
//	}
//	autoCertServer, err := server.NewAutoCertServer(config)
func WithAutoCert() Option {
	// This is a placeholder for documentation.
	// AutoCertServer should be created directly using NewAutoCertServer.
	return func(s *Server) {
		// No-op: AutoCertServer is a separate type
	}
}
