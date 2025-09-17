package server

import (
	"context"
	"crypto/tls"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/dmitrymomot/foundation/pkg/letsencrypt"
)

// AutoCertConfig holds configuration for automatic HTTPS with Let's Encrypt.
type AutoCertConfig struct {
	// CertManager manages certificate operations.
	CertManager *letsencrypt.Manager

	// DomainStore provides domain registration lookups.
	DomainStore letsencrypt.DomainStore

	// StatusHandler handles status pages for provisioning states.
	StatusHandler letsencrypt.StatusPageHandler

	// HTTPAddr is the address for the HTTP server (default ":80").
	HTTPAddr string

	// HTTPSAddr is the address for the HTTPS server (default ":443").
	HTTPSAddr string

	// Logger for server operations.
	Logger *slog.Logger
}

// AutoCertServer wraps Server to provide automatic HTTPS with Let's Encrypt.
type AutoCertServer struct {
	mu          sync.RWMutex
	config      *AutoCertConfig
	httpServer  *http.Server
	httpsServer *http.Server
	running     bool
}

// NewAutoCertServer creates a new server with automatic HTTPS support.
func NewAutoCertServer(cfg *AutoCertConfig) (*AutoCertServer, error) {
	if cfg.CertManager == nil {
		return nil, fmt.Errorf("certificate manager is required")
	}
	if cfg.DomainStore == nil {
		return nil, fmt.Errorf("domain store is required")
	}
	if cfg.StatusHandler == nil {
		cfg.StatusHandler = letsencrypt.NewDefaultStatusPages()
	}
	if cfg.HTTPAddr == "" {
		cfg.HTTPAddr = ":80"
	}
	if cfg.HTTPSAddr == "" {
		cfg.HTTPSAddr = ":443"
	}
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}

	return &AutoCertServer{
		config: cfg,
	}, nil
}

// Run starts both HTTP and HTTPS servers with the provided handler.
// The HTTP server handles ACME challenges and redirects to HTTPS.
// The HTTPS server serves the application with TLS.
func (s *AutoCertServer) Run(ctx context.Context, handler http.Handler) error {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return fmt.Errorf("server is already running")
	}
	s.running = true
	s.mu.Unlock()

	// Create HTTP server for challenges and redirects
	httpHandler := s.domainMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Default behavior for HTTP when certificate exists
		domain := r.Host
		if s.config.CertManager.Exists(domain) {
			// Redirect to HTTPS
			url := "https://" + r.Host + r.URL.String()
			http.Redirect(w, r, url, http.StatusMovedPermanently)
		} else {
			// Show provisioning page or error
			ctx := r.Context()
			info, err := s.config.DomainStore.GetDomain(ctx, domain)
			if err != nil || info == nil {
				s.config.StatusHandler.ServeNotFound(w, r)
			} else {
				switch info.Status {
				case letsencrypt.StatusProvisioning:
					s.config.StatusHandler.ServeProvisioning(w, r, info)
				case letsencrypt.StatusFailed:
					s.config.StatusHandler.ServeFailed(w, r, info)
				default:
					s.config.StatusHandler.ServeProvisioning(w, r, info)
				}
			}
		}
	}))

	s.httpServer = &http.Server{
		Addr:           s.config.HTTPAddr,
		Handler:        httpHandler,
		ReadTimeout:    15 * time.Second,
		WriteTimeout:   15 * time.Second,
		IdleTimeout:    60 * time.Second,
		MaxHeaderBytes: http.DefaultMaxHeaderBytes,
	}

	// Create HTTPS server with TLS config
	tlsConfig := &tls.Config{
		GetCertificate: s.getCertificate,
		MinVersion:     tls.VersionTLS12,
	}

	httpsHandler := s.domainMiddleware(handler)
	s.httpsServer = &http.Server{
		Addr:           s.config.HTTPSAddr,
		Handler:        httpsHandler,
		TLSConfig:      tlsConfig,
		ReadTimeout:    15 * time.Second,
		WriteTimeout:   15 * time.Second,
		IdleTimeout:    60 * time.Second,
		MaxHeaderBytes: http.DefaultMaxHeaderBytes,
	}

	// Start servers
	errCh := make(chan error, 2)

	// Start HTTP server
	go func() {
		s.config.Logger.InfoContext(ctx, "starting HTTP server", "addr", s.config.HTTPAddr)
		if err := s.httpServer.ListenAndServe(); err != http.ErrServerClosed {
			errCh <- fmt.Errorf("HTTP server error: %w", err)
		}
	}()

	// Start HTTPS server
	go func() {
		s.config.Logger.InfoContext(ctx, "starting HTTPS server", "addr", s.config.HTTPSAddr)
		if err := s.httpsServer.ListenAndServeTLS("", ""); err != http.ErrServerClosed {
			errCh <- fmt.Errorf("HTTPS server error: %w", err)
		}
	}()

	// Wait for context cancellation or error
	select {
	case err := <-errCh:
		s.Shutdown(ctx)
		return err
	case <-ctx.Done():
		return s.Shutdown(ctx)
	}
}

// getCertificate retrieves certificates for TLS handshake.
// Only returns certificates that exist, never generates new ones.
func (s *AutoCertServer) getCertificate(hello *tls.ClientHelloInfo) (*tls.Certificate, error) {
	domain := hello.ServerName
	if domain == "" {
		return nil, fmt.Errorf("no server name provided")
	}

	// Check if domain is registered
	ctx := context.Background()
	info, err := s.config.DomainStore.GetDomain(ctx, domain)
	if err != nil {
		return nil, fmt.Errorf("domain lookup failed: %w", err)
	}
	if info == nil {
		return nil, fmt.Errorf("domain not registered: %s", domain)
	}

	// Only return certificate if it exists
	return s.config.CertManager.GetCertificate(hello)
}

// domainMiddleware validates domains and adds tenant headers.
func (s *AutoCertServer) domainMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Handle ACME challenges first
		if s.config.CertManager.HandleChallenge(w, r) {
			return
		}

		// Extract domain from host
		domain := r.Host
		if idx := strings.LastIndex(domain, ":"); idx != -1 {
			domain = domain[:idx]
		}

		// Check domain registration
		ctx := r.Context()
		info, err := s.config.DomainStore.GetDomain(ctx, domain)
		if err != nil {
			s.config.Logger.ErrorContext(ctx, "domain lookup failed",
				"domain", domain,
				"error", err)
			s.config.StatusHandler.ServeNotFound(w, r)
			return
		}
		if info == nil {
			s.config.StatusHandler.ServeNotFound(w, r)
			return
		}

		// Add tenant header if applicable
		if info.TenantID != "" {
			r.Header.Set("X-Tenant-ID", info.TenantID)
		}

		next.ServeHTTP(w, r)
	})
}

// Shutdown gracefully shuts down both HTTP and HTTPS servers.
func (s *AutoCertServer) Shutdown(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		return nil
	}

	s.config.Logger.InfoContext(ctx, "shutting down servers")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var httpErr, httpsErr error

	// Shutdown HTTP server
	if s.httpServer != nil {
		httpErr = s.httpServer.Shutdown(shutdownCtx)
	}

	// Shutdown HTTPS server
	if s.httpsServer != nil {
		httpsErr = s.httpsServer.Shutdown(shutdownCtx)
	}

	s.running = false

	if httpErr != nil {
		return fmt.Errorf("HTTP shutdown error: %w", httpErr)
	}
	if httpsErr != nil {
		return fmt.Errorf("HTTPS shutdown error: %w", httpsErr)
	}

	s.config.Logger.InfoContext(ctx, "servers shutdown complete")
	return nil
}
