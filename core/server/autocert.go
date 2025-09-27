package server

import (
	"context"
	"crypto/tls"
	"errors"
	"html/template"
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
	running bool
}

// NewAutoCertServer creates a new server with automatic HTTPS support.
func NewAutoCertServer(opts ...AutoCertOption) (*AutoCertServer, error) {
	// Default configuration
	s := &AutoCertServer{
		httpServer:  New(":80"),  // Default HTTP address
		httpsServer: New(":443"), // Default HTTPS address
	}

	// Apply all options
	for _, opt := range opts {
		opt(s)
	}

	// Validation - ensure required dependencies are set
	if s.certManager == nil {
		return nil, ErrNoCertManager
	}
	if s.domainStore == nil {
		return nil, ErrNoDomainStore
	}

	// Set default handlers if not provided
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

// buildProvisioningHTML generates the provisioning status page
func buildProvisioningHTML(info *DomainInfo) string {
	tmpl := `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width, initial-scale=1">
    <meta http-equiv="refresh" content="10">
    <title>Securing Connection</title>
    <style>
        body {
            font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif;
            background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
            min-height: 100vh;
            display: flex;
            align-items: center;
            justify-content: center;
            margin: 0;
            padding: 20px;
        }
        .container {
            background: white;
            border-radius: 12px;
            box-shadow: 0 20px 60px rgba(0,0,0,0.15);
            padding: 40px;
            max-width: 500px;
            width: 100%;
            text-align: center;
        }
        h1 { color: #333; margin: 0 0 20px; font-size: 28px; }
        .lock-icon { font-size: 48px; margin-bottom: 20px; }
        .domain { color: #667eea; font-weight: 600; font-size: 18px; }
        p { color: #666; line-height: 1.6; margin: 15px 0; }
        .progress {
            width: 100%;
            height: 6px;
            background: #f0f0f0;
            border-radius: 3px;
            overflow: hidden;
            margin: 30px 0;
        }
        .progress-bar {
            height: 100%;
            background: linear-gradient(90deg, #667eea, #764ba2);
            animation: progress 2s ease-in-out infinite;
        }
        @keyframes progress {
            0% { transform: translateX(-100%); }
            50% { transform: translateX(0); }
            100% { transform: translateX(100%); }
        }
    </style>
</head>
<body>
    <div class="container">
        <div class="lock-icon">🔒</div>
        <h1>Setting up secure connection</h1>
        <p>We're configuring SSL/TLS for</p>
        <p class="domain">{{.Domain}}</p>
        <div class="progress"><div class="progress-bar"></div></div>
        <p>This typically takes 30-60 seconds.</p>
        <p style="font-size: 14px; color: #999;">This page will refresh automatically</p>
    </div>
</body>
</html>`

	result := strings.ReplaceAll(tmpl, "{{.Domain}}", template.HTMLEscapeString(info.Domain))
	return result
}

func buildFailedHTML(info *DomainInfo) string {
	tmpl := `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width, initial-scale=1">
    <title>Configuration Required</title>
    <style>
        body {
            font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif;
            background: #f5f5f5;
            min-height: 100vh;
            display: flex;
            align-items: center;
            justify-content: center;
            margin: 0;
            padding: 20px;
        }
        .container {
            background: white;
            border-radius: 12px;
            box-shadow: 0 4px 20px rgba(0,0,0,0.1);
            padding: 40px;
            max-width: 600px;
            width: 100%;
        }
        h1 { color: #d93025; margin: 0 0 20px; font-size: 28px; }
        .domain {
            color: #333;
            font-weight: 600;
            background: #f8f9fa;
            padding: 8px 12px;
            border-radius: 6px;
            display: inline-block;
            margin: 10px 0;
        }
        .error-box {
            background: #fef2f2;
            border: 1px solid #fecaca;
            border-radius: 8px;
            padding: 16px;
            margin: 20px 0;
        }
        .error-message {
            color: #b91c1c;
            font-family: 'Courier New', monospace;
            font-size: 14px;
            word-break: break-all;
        }
        h3 { color: #333; margin: 30px 0 15px; }
        ul { color: #666; line-height: 1.8; }
        li { margin: 8px 0; }
    </style>
</head>
<body>
    <div class="container">
        <h1>⚠️ Domain Configuration Required</h1>
        <p>Unable to set up SSL/TLS certificate for:</p>
        <div class="domain">{{.Domain}}</div>
        <div class="error-box">
            <strong>Error Details:</strong>
            <div class="error-message">{{.Error}}</div>
        </div>
        <h3>Common causes:</h3>
        <ul>
            <li>DNS records not properly configured</li>
            <li>Domain not pointing to our servers</li>
            <li>CAA records blocking Let's Encrypt</li>
            <li>Rate limits exceeded</li>
        </ul>
        <p>Please verify your DNS settings and contact support if the issue persists.</p>
    </div>
</body>
</html>`

	result := strings.ReplaceAll(tmpl, "{{.Domain}}", template.HTMLEscapeString(info.Domain))
	result = strings.ReplaceAll(result, "{{.Error}}", template.HTMLEscapeString(info.Error))
	return result
}

const defaultNotFoundHTML = `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width, initial-scale=1">
    <title>404 - Domain Not Found</title>
    <style>
        body {
            font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif;
            background: #f5f5f5;
            min-height: 100vh;
            display: flex;
            align-items: center;
            justify-content: center;
            margin: 0;
        }
        .container { text-align: center; }
        h1 { font-size: 120px; color: #e0e0e0; margin: 0; font-weight: 700; }
        h2 { color: #333; margin: 20px 0; font-size: 28px; }
        p { color: #666; font-size: 18px; }
    </style>
</head>
<body>
    <div class="container">
        <h1>404</h1>
        <h2>Domain Not Found</h2>
        <p>The requested domain is not configured on this server.</p>
    </div>
</body>
</html>`

// Run starts both HTTP and HTTPS servers with the provided handler.
// The HTTP server handles ACME challenges and redirects to HTTPS.
// The HTTPS server serves the application with TLS.
func (s *AutoCertServer) Run(ctx context.Context, handler http.Handler) error {
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

	// Run HTTP server with ACME handler
	go func() {
		if err := s.httpServer.Run(ctx, s.createHTTPHandler()); err != nil {
			errCh <- errors.Join(ErrHTTPServer, err)
		}
	}()

	// Run HTTPS server with application handler
	go func() {
		if err := s.httpsServer.Run(ctx, handler); err != nil {
			errCh <- errors.Join(ErrHTTPSServer, err)
		}
	}()

	select {
	case err := <-errCh:
		_ = s.Shutdown(ctx)
		return err
	case <-ctx.Done():
		return s.Shutdown(ctx)
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
			// Default to provisioning state for any other status
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
	ctx, cancel := context.WithTimeout(hello.Context(), 10*time.Second)
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

// Shutdown gracefully shuts down both HTTP and HTTPS servers.
func (s *AutoCertServer) Shutdown(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		return nil
	}

	var httpErr, httpsErr error

	if s.httpServer != nil {
		httpErr = s.httpServer.Shutdown(ctx)
	}

	if s.httpsServer != nil {
		httpsErr = s.httpsServer.Shutdown(ctx)
	}

	s.running = false

	if httpErr != nil || httpsErr != nil {
		return errors.Join(httpErr, httpsErr)
	}

	return nil
}
