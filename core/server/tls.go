package server

import (
	"crypto/tls"
	"errors"
	"fmt"
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
type TLSConfigOption func(*tls.Config) error

// WithTLSCertificate adds a certificate to the TLS configuration.
func WithTLSCertificate(certFile, keyFile string) TLSConfigOption {
	return func(cfg *tls.Config) error {
		if certFile == "" || keyFile == "" {
			return ErrEmptyCertPath
		}
		cert, err := tls.LoadX509KeyPair(certFile, keyFile)
		if err != nil {
			return errors.Join(ErrFailedLoadCert, err)
		}
		cfg.Certificates = append(cfg.Certificates, cert)
		return nil
	}
}

// WithTLSClientAuth configures client certificate authentication.
func WithTLSClientAuth(clientAuthType tls.ClientAuthType) TLSConfigOption {
	return func(cfg *tls.Config) error {
		// Validate client auth type is within valid range (0-4)
		switch clientAuthType {
		case tls.NoClientCert,
			tls.RequestClientCert,
			tls.RequireAnyClientCert,
			tls.VerifyClientCertIfGiven,
			tls.RequireAndVerifyClientCert:
			cfg.ClientAuth = clientAuthType
			return nil
		default:
			return errors.Join(ErrInvalidClientAuthType, fmt.Errorf("%d", clientAuthType))
		}
	}
}

// WithTLSMinVersion sets the minimum TLS version.
func WithTLSMinVersion(version uint16) TLSConfigOption {
	return func(cfg *tls.Config) error {
		// Validate TLS version is within acceptable range
		if !isValidTLSVersion(version) {
			return errors.Join(ErrInvalidTLSVersion, fmt.Errorf("0x%04x", version))
		}

		// Ensure MinVersion <= MaxVersion if MaxVersion is set
		if cfg.MaxVersion > 0 && version > cfg.MaxVersion {
			return ErrTLSVersionMismatch
		}

		cfg.MinVersion = version
		return nil
	}
}

// WithTLSMaxVersion sets the maximum TLS version.
func WithTLSMaxVersion(version uint16) TLSConfigOption {
	return func(cfg *tls.Config) error {
		// Validate TLS version is within acceptable range
		if !isValidTLSVersion(version) {
			return errors.Join(ErrInvalidTLSVersion, fmt.Errorf("0x%04x", version))
		}

		// Ensure MinVersion <= MaxVersion if MinVersion is set
		if cfg.MinVersion > 0 && version < cfg.MinVersion {
			return ErrTLSVersionMismatch
		}

		cfg.MaxVersion = version
		return nil
	}
}

// isValidTLSVersion checks if a TLS version is valid and supported.
func isValidTLSVersion(version uint16) bool {
	switch version {
	case tls.VersionTLS10, tls.VersionTLS11, tls.VersionTLS12, tls.VersionTLS13:
		return true
	default:
		return false
	}
}

// WithTLSServerName sets the expected server name for verification.
func WithTLSServerName(serverName string) TLSConfigOption {
	return func(cfg *tls.Config) error {
		if serverName == "" {
			return ErrEmptyServerName
		}
		cfg.ServerName = serverName
		return nil
	}
}

// WithTLSInsecureSkipVerify disables certificate verification.
// WARNING: Use only for testing. Never use in production.
func WithTLSInsecureSkipVerify() TLSConfigOption {
	return func(cfg *tls.Config) error {
		cfg.InsecureSkipVerify = true
		return nil
	}
}

// NewTLSConfig creates a new TLS configuration with the given options,
// starting from a secure default configuration.
func NewTLSConfig(opts ...TLSConfigOption) (*tls.Config, error) {
	cfg := DefaultTLSConfig()
	for _, opt := range opts {
		if err := opt(cfg); err != nil {
			return nil, err
		}
	}
	return cfg, nil
}
