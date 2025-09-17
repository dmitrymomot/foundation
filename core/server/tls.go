package server

import (
	"crypto/tls"
)

// DefaultTLSConfig returns a secure default TLS configuration following
// Mozilla's Modern compatibility recommendations.
// Supports TLS 1.2+ with strong cipher suites.
func DefaultTLSConfig() *tls.Config {
	return &tls.Config{
		MinVersion: tls.VersionTLS12,
		CipherSuites: []uint16{
			// TLS 1.3 cipher suites (auto-selected when TLS 1.3 is negotiated)
			// TLS_AES_128_GCM_SHA256
			// TLS_AES_256_GCM_SHA384
			// TLS_CHACHA20_POLY1305_SHA256

			// TLS 1.2 cipher suites (ECDHE only for forward secrecy)
			tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305_SHA256,
			tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305_SHA256,
		},
		CurvePreferences: []tls.CurveID{
			tls.X25519,
			tls.CurveP256,
		},
	}
}

// ModernTLSConfig returns a TLS configuration following Mozilla's Modern
// compatibility guidelines. Requires TLS 1.3 only with the strongest cipher suites.
// Use this for internal services or when you control all clients.
func ModernTLSConfig() *tls.Config {
	return &tls.Config{
		MinVersion: tls.VersionTLS13,
		// TLS 1.3 cipher suites are auto-selected
		CurvePreferences: []tls.CurveID{
			tls.X25519,
			tls.CurveP256,
		},
	}
}

// IntermediateTLSConfig returns a TLS configuration following Mozilla's
// Intermediate compatibility guidelines. Supports TLS 1.2+ with a broader
// range of cipher suites for compatibility with older clients.
func IntermediateTLSConfig() *tls.Config {
	return &tls.Config{
		MinVersion: tls.VersionTLS12,
		CipherSuites: []uint16{
			// ECDHE with AEAD ciphers (preferred)
			tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305_SHA256,
			tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305_SHA256,
		},
		CurvePreferences: []tls.CurveID{
			tls.X25519,
			tls.CurveP256,
			tls.CurveP384,
		},
	}
}

// StrictTLSConfig returns a highly secure TLS configuration with additional
// hardening options enabled. Use this for high-security environments.
func StrictTLSConfig() *tls.Config {
	return &tls.Config{
		MinVersion: tls.VersionTLS13,
		// TLS 1.3 only with auto-selected cipher suites
		CurvePreferences: []tls.CurveID{
			tls.X25519, // Preferred for performance and security
			tls.CurveP256,
		},
		// Additional security settings
		SessionTicketsDisabled:      true,                 // Disable session tickets for forward secrecy
		DynamicRecordSizingDisabled: false,                // Keep enabled for performance
		Renegotiation:               tls.RenegotiateNever, // Prevent renegotiation attacks
		PreferServerCipherSuites:    false,                // Let Go select the best cipher (Go 1.17+)
	}
}

// TLSConfigOption represents a functional option for customizing TLS configuration.
type TLSConfigOption func(*tls.Config)

// WithTLSCertificate adds a certificate to the TLS configuration.
func WithTLSCertificate(certFile, keyFile string) TLSConfigOption {
	return func(cfg *tls.Config) {
		cert, err := tls.LoadX509KeyPair(certFile, keyFile)
		if err != nil {
			return // Silently ignore errors in options
		}
		cfg.Certificates = append(cfg.Certificates, cert)
	}
}

// WithTLSClientAuth configures client certificate authentication.
func WithTLSClientAuth(clientAuthType tls.ClientAuthType) TLSConfigOption {
	return func(cfg *tls.Config) {
		cfg.ClientAuth = clientAuthType
	}
}

// WithTLSMinVersion sets the minimum TLS version.
func WithTLSMinVersion(version uint16) TLSConfigOption {
	return func(cfg *tls.Config) {
		cfg.MinVersion = version
	}
}

// WithTLSServerName sets the expected server name for verification.
func WithTLSServerName(serverName string) TLSConfigOption {
	return func(cfg *tls.Config) {
		cfg.ServerName = serverName
	}
}

// WithTLSInsecureSkipVerify disables certificate verification.
// WARNING: Use only for testing. Never use in production.
func WithTLSInsecureSkipVerify() TLSConfigOption {
	return func(cfg *tls.Config) {
		cfg.InsecureSkipVerify = true
	}
}

// NewTLSConfig creates a new TLS configuration with the given options,
// starting from a secure default configuration.
func NewTLSConfig(opts ...TLSConfigOption) *tls.Config {
	cfg := DefaultTLSConfig()
	for _, opt := range opts {
		opt(cfg)
	}
	return cfg
}
