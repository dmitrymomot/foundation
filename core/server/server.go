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

type Server struct {
	mu        sync.RWMutex
	addr      string
	server    *http.Server
	logger    *slog.Logger
	shutdown  time.Duration
	tlsConfig *tls.Config
	running   bool
}

func New(addr string, opts ...Option) *Server {
	s := &Server{
		addr:     addr,
		logger:   slog.Default(),
		shutdown: 30 * time.Second,
	}

	for _, opt := range opts {
		opt(s)
	}

	return s
}

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

	// Capture TLS config while holding the lock to avoid race
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

func Run(ctx context.Context, addr string, handler http.Handler) error {
	server := New(addr)
	return server.Run(ctx, handler)
}
