package letsencrypt

import (
	"time"

	"golang.org/x/crypto/acme/autocert"
)

// ManagerOption configures a Manager during initialization.
type ManagerOption func(*Manager)

// WithACMEProvider sets a custom ACME provider implementation.
// This is primarily useful for testing with mock providers,
// but can also be used to swap providers (e.g., staging vs production).
func WithACMEProvider(provider ACMEProvider) ManagerOption {
	return func(m *Manager) {
		m.acme = provider
	}
}

// WithCache sets a custom cache implementation.
// By default, autocert.DirCache is used with the configured certificate directory.
func WithCache(cache autocert.Cache) ManagerOption {
	return func(m *Manager) {
		m.cache = cache
	}
}

// WithRetryConfig sets custom retry configuration for certificate generation.
// This is primarily useful for testing to avoid long delays.
func WithRetryConfig(maxRetries int, backoff time.Duration) ManagerOption {
	return func(m *Manager) {
		m.maxRetries = maxRetries
		m.retryBackoff = backoff
	}
}
