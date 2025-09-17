package letsencrypt

import (
	"context"
	"crypto/tls"
	"errors"
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
	cache   autocert.Cache
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
		return nil, ErrEmailRequired
	}
	if cfg.CertDir == "" {
		return nil, ErrCertDirRequired
	}

	cache := autocert.DirCache(cfg.CertDir)
	acmeManager := &autocert.Manager{
		Cache:      cache,
		Prompt:     autocert.AcceptTOS,
		Email:      cfg.Email,
		HostPolicy: autocert.HostWhitelist(), // Default policy
	}

	return newManager(cfg.Email, cfg.CertDir, acmeManager, cache), nil
}

// newManager creates a manager with injected dependencies (for testing).
func newManager(email, certDir string, acme *autocert.Manager, cache autocert.Cache) *Manager {
	m := &Manager{
		email:   email,
		certDir: certDir,
		acme:    acme,
		cache:   cache,
	}

	acme.HostPolicy = m.hostPolicy
	return m
}

// hostPolicy is used internally to control which domains can get certificates.
// Since we manage certificates explicitly, this just allows all requests.
func (m *Manager) hostPolicy(ctx context.Context, host string) error {
	// Remove port if present
	if idx := strings.LastIndex(host, ":"); idx != -1 {
		host = host[:idx]
	}
	// Allow all hosts since certificate generation is explicit
	return nil
}

// Generate creates a new certificate for the domain.
// This is a blocking operation that may take 30-60 seconds.
// It includes retry logic with exponential backoff for transient failures.
func (m *Manager) Generate(ctx context.Context, domain string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	hello := &tls.ClientHelloInfo{
		ServerName: domain,
	}

	const maxRetries = 3
	backoff := time.Second * 5

	var lastErr error
	for attempt := 1; attempt <= maxRetries; attempt++ {
		_, err := m.acme.GetCertificate(hello)
		if err == nil {
			return nil
		}

		lastErr = err

		if attempt < maxRetries && IsRetryableError(err) {
			select {
			case <-ctx.Done():
				return errors.Join(ErrGenerationFailed, fmt.Errorf("context canceled for domain %s", domain), ctx.Err())
			case <-time.After(backoff):
				backoff *= 2
				continue
			}
		}
		break
	}

	return errors.Join(ErrGenerationFailed, fmt.Errorf("failed after %d attempts for domain %s", maxRetries, domain), lastErr)
}

// IsRetryableError determines if an error indicates a transient failure worth retrying.
// Returns true for network timeouts, connection failures, and rate limiting responses.
func IsRetryableError(err error) bool {
	if err == nil {
		return false
	}

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

	if err := m.cache.Delete(ctx, domain); err != nil && !os.IsNotExist(err) {
		return errors.Join(errors.New("failed to delete existing certificate"), err)
	}

	hello := &tls.ClientHelloInfo{
		ServerName: domain,
	}

	_, err := m.acme.GetCertificate(hello)
	if err != nil {
		return errors.Join(ErrGenerationFailed, fmt.Errorf("renewal failed for domain %s", domain), err)
	}

	return nil
}

// Delete removes the certificate for a domain.
func (m *Manager) Delete(domain string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	ctx := context.Background()
	if err := m.cache.Delete(ctx, domain); err != nil {
		return errors.Join(errors.New("delete failed"), fmt.Errorf("domain %s", domain), err)
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
		return nil, ErrInvalidDomain
	}

	ctx := context.Background()
	certBytes, err := m.cache.Get(ctx, domain)
	if err != nil {
		return nil, errors.Join(ErrCertificateNotFound, fmt.Errorf("domain %s", domain), err)
	}

	cert, err := tls.X509KeyPair(certBytes, certBytes)
	if err != nil {
		return nil, errors.Join(errors.New("failed to load certificate"), fmt.Errorf("domain %s", domain), err)
	}

	return &cert, nil
}

// HandleChallenge handles ACME HTTP-01 challenges.
// Returns true if the request was an ACME challenge.
func (m *Manager) HandleChallenge(w http.ResponseWriter, r *http.Request) bool {
	if !strings.HasPrefix(r.URL.Path, "/.well-known/acme-challenge/") {
		return false
	}

	m.acme.HTTPHandler(nil).ServeHTTP(w, r)
	return true
}

// CertDir returns the directory where certificates are stored.
func (m *Manager) CertDir() string {
	return m.certDir
}

// certPath returns the file path for a domain's certificate.
// Uses autocert.DirCache naming convention where domain is the filename.
func (m *Manager) certPath(domain string) string {
	return filepath.Join(m.certDir, domain)
}
