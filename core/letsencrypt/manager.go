package letsencrypt

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/acme/autocert"
)

// Manager handles Let's Encrypt certificate operations.
// All operations are explicit with no automatic generation or renewal.
// This implementation satisfies the server.CertificateManager interface
// without explicitly depending on it.
type Manager struct {
	mu      sync.RWMutex
	email   string
	certDir string
	acme    *autocert.Manager
	cache   autocert.DirCache
}

// Config holds configuration for the certificate manager.
type Config struct {
	// Email is the contact email for Let's Encrypt account.
	Email string

	// CertDir is the directory to store certificates.
	CertDir string
}

// NewManager creates a new certificate manager.
func NewManager(cfg Config) (*Manager, error) {
	if cfg.Email == "" {
		return nil, fmt.Errorf("email is required for Let's Encrypt account")
	}
	if cfg.CertDir == "" {
		return nil, fmt.Errorf("certificate directory is required")
	}

	cache := autocert.DirCache(cfg.CertDir)

	m := &Manager{
		email:   cfg.Email,
		certDir: cfg.CertDir,
		cache:   cache,
	}

	// Configure autocert manager
	m.acme = &autocert.Manager{
		Cache:      cache,
		Prompt:     autocert.AcceptTOS,
		Email:      cfg.Email,
		HostPolicy: m.hostPolicy,
	}

	return m, nil
}

// hostPolicy is used internally to control which domains can get certificates.
// Since we manage certificates explicitly, this just allows all requests.
func (m *Manager) hostPolicy(ctx context.Context, host string) error {
	// Remove port if present
	if idx := strings.LastIndex(host, ":"); idx != -1 {
		host = host[:idx]
	}
	// We allow all hosts here since certificate generation is explicit
	return nil
}

// Generate creates a new certificate for the domain.
// This is a blocking operation that may take 30-60 seconds.
// It includes retry logic with exponential backoff for transient failures.
func (m *Manager) Generate(ctx context.Context, domain string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Force new certificate generation by getting it
	hello := &tls.ClientHelloInfo{
		ServerName: domain,
	}

	// Retry logic with exponential backoff
	const maxRetries = 3
	backoff := time.Second * 5

	var lastErr error
	for attempt := 1; attempt <= maxRetries; attempt++ {
		_, err := m.acme.GetCertificate(hello)
		if err == nil {
			return nil
		}

		lastErr = err

		// Check if error is retryable (network errors, rate limits)
		if attempt < maxRetries && isRetryableError(err) {
			select {
			case <-ctx.Done():
				return fmt.Errorf("context canceled during certificate generation for %s: %w", domain, ctx.Err())
			case <-time.After(backoff):
				backoff *= 2 // Exponential backoff
				continue
			}
		}
		break
	}

	return fmt.Errorf("failed to generate certificate for %s after %d attempts: %w", domain, maxRetries, lastErr)
}

// isRetryableError checks if an error is retryable (network errors, rate limits)
func isRetryableError(err error) bool {
	if err == nil {
		return false
	}

	// Check for common retryable error patterns
	errStr := err.Error()
	retryablePatterns := []string{
		"connection refused",
		"network is unreachable",
		"no such host",
		"timeout",
		"rate limit",
		"429", // Too Many Requests
		"503", // Service Unavailable
		"temporary failure",
	}

	for _, pattern := range retryablePatterns {
		if strings.Contains(strings.ToLower(errStr), pattern) {
			return true
		}
	}

	return false
}

// Renew forces renewal of an existing certificate.
// This is a blocking operation that may take 30-60 seconds.
func (m *Manager) Renew(ctx context.Context, domain string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Delete the existing certificate to force renewal
	if err := m.cache.Delete(ctx, domain); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete existing certificate: %w", err)
	}

	// Generate new certificate
	hello := &tls.ClientHelloInfo{
		ServerName: domain,
	}

	_, err := m.acme.GetCertificate(hello)
	if err != nil {
		return fmt.Errorf("failed to renew certificate for %s: %w", domain, err)
	}

	return nil
}

// Delete removes the certificate for a domain.
func (m *Manager) Delete(domain string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	ctx := context.Background()
	if err := m.cache.Delete(ctx, domain); err != nil {
		return fmt.Errorf("failed to delete certificate for %s: %w", domain, err)
	}

	return nil
}

// Exists checks if a certificate exists for the domain.
func (m *Manager) Exists(domain string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	ctx := context.Background()
	_, err := m.cache.Get(ctx, domain)
	return err == nil
}

// GetCertificate retrieves a certificate for TLS handshake.
// Returns an error if the certificate doesn't exist.
// This method is used by the TLS configuration.
func (m *Manager) GetCertificate(hello *tls.ClientHelloInfo) (*tls.Certificate, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	domain := hello.ServerName
	if domain == "" {
		return nil, fmt.Errorf("no server name provided")
	}

	// Only return existing certificates, don't generate new ones
	ctx := context.Background()
	certBytes, err := m.cache.Get(ctx, domain)
	if err != nil {
		return nil, fmt.Errorf("certificate not found for %s: %w", domain, err)
	}

	cert, err := tls.X509KeyPair(certBytes, certBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to load certificate for %s: %w", domain, err)
	}

	return &cert, nil
}

// HandleChallenge handles ACME HTTP-01 challenges.
// Returns true if the request was an ACME challenge.
func (m *Manager) HandleChallenge(w http.ResponseWriter, r *http.Request) bool {
	// Check if this is an ACME challenge request
	if !strings.HasPrefix(r.URL.Path, "/.well-known/acme-challenge/") {
		return false
	}

	// Use autocert's HTTP handler for challenges
	m.acme.HTTPHandler(nil).ServeHTTP(w, r)
	return true
}

// CertDir returns the directory where certificates are stored.
func (m *Manager) CertDir() string {
	return m.certDir
}

// certPath returns the file path for a domain's certificate.
func (m *Manager) certPath(domain string) string {
	// autocert.DirCache uses domain name as filename
	return filepath.Join(m.certDir, domain)
}
