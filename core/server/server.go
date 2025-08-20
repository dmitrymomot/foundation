package server

import (
	"context"
	"crypto/tls"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"
)

// Server wraps http.Server with graceful shutdown and configuration options.
// Safe for concurrent use.
type Server struct {
	mu        sync.RWMutex  // Protects all fields
	addr      string        // Listen address
	server    *http.Server  // Underlying HTTP server
	logger    *slog.Logger  // Server logger
	shutdown  time.Duration // Graceful shutdown timeout
	tlsConfig *tls.Config   // TLS configuration
	running   bool          // Server state
}

// New creates a new Server with the given address and options.
// Defaults to 30-second graceful shutdown timeout and the default logger.
func New(addr string, opts ...Option) *Server {
	s := &Server{
		addr:     addr,
		logger:   slog.Default(),
		shutdown: 30 * time.Second, // Reasonable default for graceful shutdown
	}

	for _, opt := range opts {
		opt(s)
	}

	return s
}

// Run starts the server and blocks until the context is canceled or an error occurs.
// Automatically handles graceful shutdown when context is canceled.
func (s *Server) Run(ctx context.Context, handler http.Handler) error {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return fmt.Errorf("server is already running")
	}
	s.running = true

	s.server = &http.Server{
		Addr:           s.addr,
		Handler:        handler,
		ReadTimeout:    15 * time.Second,
		WriteTimeout:   15 * time.Second,
		IdleTimeout:    60 * time.Second,
		MaxHeaderBytes: http.DefaultMaxHeaderBytes,
		TLSConfig:      s.tlsConfig,
	}

	// Capture TLS config while holding the lock to avoid race conditions
	hasTLS := s.tlsConfig != nil
	s.mu.Unlock()

	errCh := make(chan error, 1)
	go func() {
		s.logger.InfoContext(ctx, "starting server", "addr", s.addr)

		var err error
		if hasTLS {
			err = s.server.ListenAndServeTLS("", "")
		} else {
			err = s.server.ListenAndServe()
		}

		if err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()

	select {
	case err := <-errCh:
		s.mu.Lock()
		s.running = false
		s.mu.Unlock()
		return err
	case <-ctx.Done():
		return s.Shutdown(ctx)
	}
}

// Shutdown gracefully shuts down the server using the configured timeout.
// Returns immediately if the server is not running.
func (s *Server) Shutdown(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running || s.server == nil {
		return nil
	}

	s.logger.InfoContext(ctx, "shutting down server gracefully", "timeout", s.shutdown)

	shutdownCtx, cancel := context.WithTimeout(context.Background(), s.shutdown)
	defer cancel()

	err := s.server.Shutdown(shutdownCtx)
	s.running = false

	if err != nil {
		s.logger.ErrorContext(ctx, "server shutdown error", "error", err)
		return err
	}

	s.logger.InfoContext(ctx, "server shutdown complete")
	return nil
}

// Run is a convenience function that creates and runs a server with default settings.
func Run(ctx context.Context, addr string, handler http.Handler) error {
	server := New(addr)
	return server.Run(ctx, handler)
}
