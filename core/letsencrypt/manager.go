package letsencrypt

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"syscall"
	"time"

	"golang.org/x/crypto/acme"
	"golang.org/x/crypto/acme/autocert"
)

// Manager handles Let's Encrypt certificate operations.
// All operations are explicit with no automatic generation or renewal.
// This implementation satisfies the server.CertificateManager interface
// without explicitly depending on it.
type Manager struct {
	mu           sync.RWMutex
	email        string
	certDir      string
	acme         ACMEProvider
	cache        autocert.Cache
	maxRetries   int
	retryBackoff time.Duration
}

// Config holds configuration for the certificate manager.
type Config struct {
	// Email is the contact email for Let's Encrypt account.
	Email string

	// CertDir is the directory to store certificates.
	CertDir string
}

// NewManager creates a new certificate manager.
func NewManager(cfg Config, opts ...ManagerOption) (*Manager, error) {
	if cfg.Email == "" {
		return nil, ErrEmailRequired
	}
	if cfg.CertDir == "" {
		return nil, ErrCertDirRequired
	}

	// Create manager with defaults
	m := &Manager{
		email:        cfg.Email,
		certDir:      cfg.CertDir,
		cache:        autocert.DirCache(cfg.CertDir),
		maxRetries:   3,
		retryBackoff: 5 * time.Second,
	}

	// Apply options first
	for _, opt := range opts {
		opt(m)
	}

	// Create default ACME provider if none was injected
	if m.acme == nil {
		m.acme = &autocert.Manager{
			Cache:      m.cache,
			Prompt:     autocert.AcceptTOS,
			Email:      cfg.Email,
			HostPolicy: m.hostPolicy,
		}
	}

	return m, nil
}

// hostPolicy is used internally to control which domains can get certificates.
// Since we manage certificates explicitly, this just allows all requests.
func (m *Manager) hostPolicy(ctx context.Context, host string) error {
	// Remove port if present (validation only, host not used further)
	if idx := strings.LastIndex(host, ":"); idx != -1 {
		// Validate host has content before port
		if idx == 0 {
			return fmt.Errorf("invalid host: %s", host)
		}
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

	backoff := m.retryBackoff

	var lastErr error
	for attempt := 1; attempt <= m.maxRetries; attempt++ {
		_, err := m.acme.GetCertificate(hello)
		if err == nil {
			return nil
		}

		lastErr = err

		if attempt < m.maxRetries && IsRetryableError(err) {
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

	return errors.Join(ErrGenerationFailed, fmt.Errorf("failed after %d attempts for domain %s", m.maxRetries, domain), lastErr)
}

// IsRetryableError determines if an error indicates a transient failure worth retrying.
// Returns true for network timeouts, connection failures, and rate limiting responses.
// This function checks for specific error types and patterns, handling wrapped errors properly.
func IsRetryableError(err error) bool {
	if err == nil {
		return false
	}

	// Check for specific error types using errors.As
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		// Network timeouts are retryable
		return true
	}

	// Check for specific ACME/Let's Encrypt errors
	var acmeErr *acme.Error
	if errors.As(err, &acmeErr) {
		// Rate limiting (429) or service unavailable (503)
		if acmeErr.StatusCode == http.StatusTooManyRequests ||
			acmeErr.StatusCode == http.StatusServiceUnavailable {
			return true
		}
	}

	// Check for syscall errors
	var syscallErr *os.SyscallError
	if errors.As(err, &syscallErr) {
		// Connection refused, network unreachable, etc.
		if syscallErr.Err == syscall.ECONNREFUSED ||
			syscallErr.Err == syscall.ENETUNREACH ||
			syscallErr.Err == syscall.ETIMEDOUT {
			return true
		}
	}

	// Fallback to string matching for other error types
	errStr := strings.ToLower(err.Error())
	retryablePatterns := []string{
		"connection refused",
		"network is unreachable",
		"no such host",
		"timeout",
		"rate limit",
		"too many requests",
		"service unavailable",
		"temporary failure",
		"i/o timeout",
		"connection reset",
	}

	for _, pattern := range retryablePatterns {
		if strings.Contains(errStr, pattern) {
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
func (m *Manager) Delete(ctx context.Context, domain string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if err := m.cache.Delete(ctx, domain); err != nil {
		return errors.Join(errors.New("delete failed"), fmt.Errorf("domain %s", domain), err)
	}

	return nil
}

// Exists checks if a certificate exists for the domain.
func (m *Manager) Exists(domain string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Use context.Background() as this is a non-blocking check
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

	// Use context.TODO() as this implements tls.Config.GetCertificate interface
	// which doesn't provide a context parameter
	ctx := context.TODO()
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
