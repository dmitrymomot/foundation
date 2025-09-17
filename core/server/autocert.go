package server

import (
	"context"
	"crypto/tls"
	"fmt"
	"html/template"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/dmitrymomot/foundation/core/handler"
	"github.com/dmitrymomot/foundation/core/response"
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

// Handler function types for status pages using generic context
type (
	// ProvisioningHandler handles requests when certificate is being generated
	ProvisioningHandler[C handler.Context] func(ctx C, info *DomainInfo) handler.Response

	// FailedHandler handles requests when certificate generation failed
	FailedHandler[C handler.Context] func(ctx C, info *DomainInfo) handler.Response

	// NotFoundHandler handles requests for unregistered domains
	NotFoundHandler[C handler.Context] func(ctx C) handler.Response
)

// AutoCertConfig holds configuration for automatic HTTPS with certificates.
type AutoCertConfig[C handler.Context] struct {
	// CertManager manages certificate operations
	CertManager CertificateManager

	// DomainStore provides domain registration lookups
	DomainStore DomainStore

	// ProvisioningHandler handles requests during certificate provisioning
	ProvisioningHandler ProvisioningHandler[C]

	// FailedHandler handles requests when certificate generation failed
	FailedHandler FailedHandler[C]

	// NotFoundHandler handles requests for unregistered domains
	NotFoundHandler NotFoundHandler[C]

	// HTTPAddr is the address for the HTTP server (default ":80")
	HTTPAddr string

	// HTTPSAddr is the address for the HTTPS server (default ":443")
	HTTPSAddr string

	// Logger for server operations
	Logger *slog.Logger
}

// AutoCertServer provides automatic HTTPS with certificate management.
type AutoCertServer[C handler.Context] struct {
	mu          sync.RWMutex
	config      *AutoCertConfig[C]
	httpServer  *http.Server
	httpsServer *http.Server
	running     bool
}

// NewAutoCertServer creates a new server with automatic HTTPS support.
func NewAutoCertServer[C handler.Context](cfg *AutoCertConfig[C]) (*AutoCertServer[C], error) {
	if cfg.CertManager == nil {
		return nil, fmt.Errorf("certificate manager is required")
	}
	if cfg.DomainStore == nil {
		return nil, fmt.Errorf("domain store is required")
	}
	if cfg.ProvisioningHandler == nil {
		cfg.ProvisioningHandler = DefaultProvisioningHandler[C]()
	}
	if cfg.FailedHandler == nil {
		cfg.FailedHandler = DefaultFailedHandler[C]()
	}
	if cfg.NotFoundHandler == nil {
		cfg.NotFoundHandler = DefaultNotFoundHandler[C]()
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

	return &AutoCertServer[C]{
		config: cfg,
	}, nil
}

// DefaultProvisioningHandler returns a default provisioning page handler.
func DefaultProvisioningHandler[C handler.Context]() ProvisioningHandler[C] {
	return func(ctx C, info *DomainInfo) handler.Response {
		html := buildProvisioningHTML(info)
		return response.HTMLWithStatus(html, http.StatusAccepted)
	}
}

// DefaultFailedHandler returns a default failed page handler.
func DefaultFailedHandler[C handler.Context]() FailedHandler[C] {
	return func(ctx C, info *DomainInfo) handler.Response {
		html := buildFailedHTML(info)
		return response.HTMLWithStatus(html, http.StatusServiceUnavailable)
	}
}

// DefaultNotFoundHandler returns a default not found page handler.
func DefaultNotFoundHandler[C handler.Context]() NotFoundHandler[C] {
	return func(ctx C) handler.Response {
		return response.HTMLWithStatus(defaultNotFoundHTML, http.StatusNotFound)
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
func (s *AutoCertServer[C]) Run(ctx context.Context, handler http.Handler) error {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return fmt.Errorf("server is already running")
	}
	s.running = true
	s.mu.Unlock()

	s.httpServer = &http.Server{
		Addr:           s.config.HTTPAddr,
		Handler:        s.createHTTPHandler(),
		ReadTimeout:    DefaultReadTimeout,
		WriteTimeout:   DefaultWriteTimeout,
		IdleTimeout:    DefaultIdleTimeout,
		MaxHeaderBytes: http.DefaultMaxHeaderBytes,
	}

	tlsConfig := &tls.Config{
		GetCertificate: s.getCertificate,
		MinVersion:     tls.VersionTLS12,
	}

	s.httpsServer = &http.Server{
		Addr:           s.config.HTTPSAddr,
		Handler:        handler,
		TLSConfig:      tlsConfig,
		ReadTimeout:    DefaultReadTimeout,
		WriteTimeout:   DefaultWriteTimeout,
		IdleTimeout:    DefaultIdleTimeout,
		MaxHeaderBytes: http.DefaultMaxHeaderBytes,
	}

	errCh := make(chan error, 2)

	go func() {
		s.config.Logger.InfoContext(ctx, "starting HTTP server", "addr", s.config.HTTPAddr)
		if err := s.httpServer.ListenAndServe(); err != http.ErrServerClosed {
			errCh <- fmt.Errorf("HTTP server error: %w", err)
		}
	}()

	go func() {
		s.config.Logger.InfoContext(ctx, "starting HTTPS server", "addr", s.config.HTTPSAddr)
		if err := s.httpsServer.ListenAndServeTLS("", ""); err != http.ErrServerClosed {
			errCh <- fmt.Errorf("HTTPS server error: %w", err)
		}
	}()

	select {
	case err := <-errCh:
		if shutdownErr := s.Shutdown(ctx); shutdownErr != nil {
			s.config.Logger.ErrorContext(ctx, "shutdown error during error handling",
				"shutdown_error", shutdownErr,
				"original_error", err)
		}
		return err
	case <-ctx.Done():
		return s.Shutdown(ctx)
	}
}

// createHTTPHandler creates the HTTP handler for port 80.
// It handles ACME challenges and redirects to HTTPS when appropriate.
func (s *AutoCertServer[C]) createHTTPHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.config.CertManager.HandleChallenge(w, r) {
			return
		}

		domain := r.Host
		if idx := strings.LastIndex(domain, ":"); idx != -1 {
			domain = domain[:idx]
		}

		ctx := r.Context()
		info, err := s.config.DomainStore.GetDomain(ctx, domain)
		if err != nil {
			s.config.Logger.ErrorContext(ctx, "domain lookup failed",
				"domain", domain,
				"error", err)
			http.NotFound(w, r)
			return
		}
		if info == nil {
			http.NotFound(w, r)
			return
		}

		if s.config.CertManager.Exists(domain) {
			url := "https://" + r.Host + r.URL.String()
			http.Redirect(w, r, url, http.StatusMovedPermanently)
			return
		}

		switch info.Status {
		case StatusProvisioning:
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
			w.WriteHeader(http.StatusAccepted)
			if _, err := w.Write([]byte(buildProvisioningHTML(info))); err != nil {
				s.config.Logger.ErrorContext(ctx, "failed to write provisioning HTML",
					"domain", domain,
					"error", err)
			}
		case StatusFailed:
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.WriteHeader(http.StatusServiceUnavailable)
			if _, err := w.Write([]byte(buildFailedHTML(info))); err != nil {
				s.config.Logger.ErrorContext(ctx, "failed to write failed HTML",
					"domain", domain,
					"error", err)
			}
		default:
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.WriteHeader(http.StatusAccepted)
			if _, err := w.Write([]byte(buildProvisioningHTML(info))); err != nil {
				s.config.Logger.ErrorContext(ctx, "failed to write default HTML",
					"domain", domain,
					"error", err)
			}
		}
	})
}

// getCertificate retrieves certificates for TLS handshake.
// Only returns certificates that exist, never generates new ones.
func (s *AutoCertServer[C]) getCertificate(hello *tls.ClientHelloInfo) (*tls.Certificate, error) {
	domain := hello.ServerName
	if domain == "" {
		return nil, fmt.Errorf("no server name provided")
	}

	// Use timeout to prevent slow domain lookups from blocking TLS handshake
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	info, err := s.config.DomainStore.GetDomain(ctx, domain)
	if err != nil {
		return nil, fmt.Errorf("domain lookup failed: %w", err)
	}
	if info == nil {
		return nil, fmt.Errorf("domain not registered: %s", domain)
	}

	return s.config.CertManager.GetCertificate(hello)
}

// Shutdown gracefully shuts down both HTTP and HTTPS servers.
func (s *AutoCertServer[C]) Shutdown(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		return nil
	}

	s.config.Logger.InfoContext(ctx, "shutting down servers")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), DefaultShutdownTimeout)
	defer cancel()

	var httpErr, httpsErr error

	if s.httpServer != nil {
		httpErr = s.httpServer.Shutdown(shutdownCtx)
	}

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
