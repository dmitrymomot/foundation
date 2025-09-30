package server

import (
	"context"
	"crypto/tls"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"sync"
	"time"
)

// Server wraps http.Server with graceful shutdown and configuration options.
// Safe for concurrent use.
type Server struct {
	mu             sync.RWMutex
	addr           string
	server         *http.Server
	logger         *slog.Logger
	shutdown       time.Duration
	readTimeout    time.Duration
	writeTimeout   time.Duration
	idleTimeout    time.Duration
	maxHeaderBytes int
	tlsConfig      *tls.Config
	running        bool
}

// New creates a new Server with the given address and options.
// Defaults to 30-second graceful shutdown timeout and a no-op logger.
func New(addr string, opts ...Option) *Server {
	s := &Server{
		addr:           addr,
		logger:         slog.New(slog.NewTextHandler(io.Discard, nil)),
		shutdown:       DefaultShutdownTimeout,
		readTimeout:    DefaultReadTimeout,
		writeTimeout:   DefaultWriteTimeout,
		idleTimeout:    DefaultIdleTimeout,
		maxHeaderBytes: DefaultMaxHeaderBytes,
	}

	for _, opt := range opts {
		opt(s)
	}

	return s
}

// Start starts the server and blocks until the context is canceled or an error occurs.
// Returns context.Err() when the context is canceled.
// Use Stop() for graceful shutdown.
func (s *Server) Start(ctx context.Context, handler http.Handler) error {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return ErrServerAlreadyRunning
	}
	s.running = true

	s.server = &http.Server{
		Addr:           s.addr,
		Handler:        handler,
		ReadTimeout:    s.readTimeout,
		WriteTimeout:   s.writeTimeout,
		IdleTimeout:    s.idleTimeout,
		MaxHeaderBytes: s.maxHeaderBytes,
		TLSConfig:      s.tlsConfig,
	}

	// Check TLS before unlocking to avoid race when accessing tlsConfig during server start
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
		return ctx.Err()
	}
}

// Stop gracefully shuts down the server using the configured timeout.
// Returns immediately if the server is not running.
func (s *Server) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running || s.server == nil {
		return nil
	}

	s.logger.Info("shutting down server gracefully", "timeout", s.shutdown)

	shutdownCtx, cancel := context.WithTimeout(context.Background(), s.shutdown)
	defer cancel()

	err := s.server.Shutdown(shutdownCtx)
	s.running = false

	if err != nil {
		s.logger.Error("server shutdown error", "error", err)
		return err
	}

	s.logger.Info("server shutdown complete")
	return nil
}

// Run provides errgroup compatibility for coordinated lifecycle management.
// Returns a function that starts the server, monitors context cancellation,
// and performs graceful shutdown when the context is cancelled.
func (s *Server) Run(ctx context.Context, handler http.Handler) func() error {
	return func() error {
		errCh := make(chan error, 1)
		go func() {
			errCh <- s.Start(ctx, handler)
		}()

		select {
		case <-ctx.Done():
			if stopErr := s.Stop(); stopErr != nil {
				s.logger.Error("failed to stop server during context cancellation", "error", stopErr)
			}
			<-errCh
			return nil
		case err := <-errCh:
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return nil
			}
			return err
		}
	}
}

// Run is a convenience function that creates and runs a server with default settings.
func Run(ctx context.Context, addr string, handler http.Handler) error {
	server := New(addr)
	return server.Start(ctx, handler)
}
