package server

import (
	"context"
	"crypto/tls"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"
)

// CertificateManager defines the interface for certificate operations
// that the server needs. Implementations could use Let's Encrypt,
// self-signed certs, or any other provider.
type CertificateManager interface {
	// GetCertificate retrieves a certificate for TLS handshake
	GetCertificate(hello *tls.ClientHelloInfo) (*tls.Certificate, error)

	// HandleChallenge handles ACME HTTP-01 challenges
	// Returns true if the request was handled
	HandleChallenge(w http.ResponseWriter, r *http.Request) bool

	// Exists checks if a certificate exists for the domain
	Exists(domain string) bool
}

// DomainStore provides domain registration lookups for the server.
type DomainStore interface {
	// GetDomain returns domain info for routing and tenant identification.
	// Returns nil if the domain is not registered.
	GetDomain(ctx context.Context, domain string) (*DomainInfo, error)
}

// DomainInfo contains domain registration details needed by the server.
type DomainInfo struct {
	// Domain is the fully qualified domain name
	Domain string

	// TenantID is the tenant identifier for multi-tenant domains
	// Empty for main domain and static subdomains
	TenantID string

	// Status indicates the current state of the domain
	Status DomainStatus

	// Error contains the error message if Status is Failed
	Error string

	// CreatedAt is when the domain was registered
	CreatedAt time.Time
}

// DomainStatus represents the current state of a domain.
type DomainStatus string

const (
	// StatusProvisioning indicates certificate is being generated
	StatusProvisioning DomainStatus = "provisioning"

	// StatusActive indicates certificate is ready and valid
	StatusActive DomainStatus = "active"

	// StatusFailed indicates certificate generation failed
	StatusFailed DomainStatus = "failed"
)

// Handler function types for status pages
type (
	// ProvisioningHandler handles requests when certificate is being generated
	ProvisioningHandler func(w http.ResponseWriter, r *http.Request, info *DomainInfo)

	// FailedHandler handles requests when certificate generation failed
	FailedHandler func(w http.ResponseWriter, r *http.Request, info *DomainInfo)

	// NotFoundHandler handles requests for unregistered domains
	NotFoundHandler func(w http.ResponseWriter, r *http.Request)
)

// AutoCertOption is a functional option for configuring AutoCertServer.
type AutoCertOption func(*AutoCertServer)

// AutoCertServer provides automatic HTTPS with certificate management.
type AutoCertServer struct {
	mu          sync.RWMutex
	httpServer  *Server
	httpsServer *Server

	// Core dependencies
	certManager CertificateManager
	domainStore DomainStore

	// Handlers
	provisioningHandler ProvisioningHandler
	failedHandler       FailedHandler
	notFoundHandler     NotFoundHandler

	// State
	running         bool
	shutdownTimeout time.Duration
}

// NewAutoCertServer creates a new server with automatic HTTPS support.
func NewAutoCertServer(opts ...AutoCertOption) (*AutoCertServer, error) {
	// Default configuration
	s := &AutoCertServer{
		httpServer:      New(":80"),       // Default HTTP address
		httpsServer:     New(":443"),      // Default HTTPS address
		shutdownTimeout: 30 * time.Second, // Default shutdown timeout
	}

	for _, opt := range opts {
		opt(s)
	}

	if s.certManager == nil {
		return nil, ErrNoCertManager
	}
	if s.domainStore == nil {
		return nil, ErrNoDomainStore
	}

	if s.provisioningHandler == nil {
		s.provisioningHandler = DefaultProvisioningHandler(s.httpServer.logger)
	}
	if s.failedHandler == nil {
		s.failedHandler = DefaultFailedHandler(s.httpServer.logger)
	}
	if s.notFoundHandler == nil {
		s.notFoundHandler = DefaultNotFoundHandler(s.httpServer.logger)
	}

	return s, nil
}

// DefaultProvisioningHandler returns a default provisioning page handler.
func DefaultProvisioningHandler(logger *slog.Logger) ProvisioningHandler {
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	return func(w http.ResponseWriter, r *http.Request, info *DomainInfo) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
		w.WriteHeader(http.StatusAccepted)
		html := buildProvisioningHTML(info)
		if _, err := w.Write([]byte(html)); err != nil {
			logger.ErrorContext(r.Context(), "failed to write provisioning HTML",
				"domain", info.Domain,
				"error", err)
		}
	}
}

// DefaultFailedHandler returns a default failed page handler.
func DefaultFailedHandler(logger *slog.Logger) FailedHandler {
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	return func(w http.ResponseWriter, r *http.Request, info *DomainInfo) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusServiceUnavailable)
		html := buildFailedHTML(info)
		if _, err := w.Write([]byte(html)); err != nil {
			logger.ErrorContext(r.Context(), "failed to write failed HTML",
				"domain", info.Domain,
				"error", err)
		}
	}
}

// DefaultNotFoundHandler returns a default not found page handler.
func DefaultNotFoundHandler(logger *slog.Logger) NotFoundHandler {
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusNotFound)
		if _, err := w.Write([]byte(defaultNotFoundHTML)); err != nil {
			logger.ErrorContext(r.Context(), "failed to write not found HTML",
				"error", err)
		}
	}
}

// Start starts both HTTP and HTTPS servers with the provided handler.
// The HTTP server handles ACME challenges and redirects to HTTPS.
// The HTTPS server serves the application with TLS.
// This is a blocking operation that returns when the context is canceled or an error occurs.
func (s *AutoCertServer) Start(ctx context.Context, handler http.Handler) error {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return ErrServerAlreadyRunning
	}
	s.running = true
	s.mu.Unlock()

	// Configure TLS for HTTPS server
	s.httpsServer.tlsConfig = &tls.Config{
		GetCertificate: s.getCertificate,
		MinVersion:     tls.VersionTLS12,
	}

	errCh := make(chan error, 2)

	// Start HTTP server with ACME handler
	go func() {
		if err := s.httpServer.Start(ctx, s.createHTTPHandler()); err != nil {
			errCh <- errors.Join(ErrHTTPServer, err)
		}
	}()

	// Start HTTPS server with application handler
	go func() {
		if err := s.httpsServer.Start(ctx, handler); err != nil {
			errCh <- errors.Join(ErrHTTPSServer, err)
		}
	}()

	select {
	case err := <-errCh:
		_ = s.Stop()
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// createHTTPHandler creates the HTTP handler for port 80.
// It handles ACME challenges and redirects to HTTPS when appropriate.
func (s *AutoCertServer) createHTTPHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.certManager.HandleChallenge(w, r) {
			return
		}

		domain := r.Host
		if idx := strings.LastIndex(domain, ":"); idx != -1 {
			domain = domain[:idx]
		}

		ctx := r.Context()
		info, err := s.domainStore.GetDomain(ctx, domain)
		if err != nil {
			s.httpServer.logger.ErrorContext(ctx, "domain lookup failed",
				"domain", domain,
				"error", err)
			http.NotFound(w, r)
			return
		}
		if info == nil {
			s.notFoundHandler(w, r)
			return
		}

		if s.certManager.Exists(domain) {
			url := "https://" + r.Host + r.URL.String()
			http.Redirect(w, r, url, http.StatusMovedPermanently)
			return
		}

		switch info.Status {
		case StatusProvisioning:
			s.provisioningHandler(w, r, info)
		case StatusFailed:
			s.failedHandler(w, r, info)
		default:
			// Treat unknown statuses as provisioning to avoid blocking requests
			s.provisioningHandler(w, r, info)
		}
	})
}

// getCertificate retrieves certificates for TLS handshake.
// Only returns certificates that exist, never generates new ones.
func (s *AutoCertServer) getCertificate(hello *tls.ClientHelloInfo) (*tls.Certificate, error) {
	domain := hello.ServerName
	if domain == "" {
		return nil, ErrNoServerName
	}

	// Use timeout to prevent slow domain lookups from blocking TLS handshake
	ctx, cancel := context.WithTimeout(hello.Context(), DefaultDomainLookupTimeout)
	defer cancel()
	info, err := s.domainStore.GetDomain(ctx, domain)
	if err != nil {
		return nil, errors.Join(ErrDomainLookupFailed, err)
	}
	if info == nil {
		return nil, ErrDomainNotRegistered
	}

	return s.certManager.GetCertificate(hello)
}

// Stop gracefully shuts down both HTTP and HTTPS servers.
func (s *AutoCertServer) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		return nil
	}

	var httpErr, httpsErr error

	if s.httpServer != nil {
		httpErr = s.httpServer.Stop()
	}

	if s.httpsServer != nil {
		httpsErr = s.httpsServer.Stop()
	}

	s.running = false

	if httpErr != nil || httpsErr != nil {
		return errors.Join(httpErr, httpsErr)
	}

	return nil
}

// Run provides errgroup compatibility for coordinated lifecycle management.
// Returns a function that starts the server, monitors context cancellation,
// and performs graceful shutdown when the context is cancelled.
func (s *AutoCertServer) Run(ctx context.Context, handler http.Handler) func() error {
	return func() error {
		errCh := make(chan error, 1)
		go func() {
			errCh <- s.Start(ctx, handler)
		}()

		select {
		case <-ctx.Done():
			if stopErr := s.Stop(); stopErr != nil {
				s.httpServer.logger.Error("failed to stop autocert server during context cancellation", "error", stopErr)
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
