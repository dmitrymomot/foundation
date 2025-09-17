package server_test

import (
	"crypto/tls"
	"testing"

	"github.com/dmitrymomot/foundation/core/server"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultTLSConfig(t *testing.T) {
	cfg := server.DefaultTLSConfig()

	assert.NotNil(t, cfg)
	assert.Equal(t, uint16(tls.VersionTLS12), cfg.MinVersion)
	assert.NotEmpty(t, cfg.CipherSuites)
	assert.Contains(t, cfg.CipherSuites, uint16(tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256))
	assert.Contains(t, cfg.CipherSuites, uint16(tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305_SHA256))
	assert.Contains(t, cfg.CurvePreferences, tls.X25519)
	assert.Contains(t, cfg.CurvePreferences, tls.CurveP256)
}

func TestModernTLSConfig(t *testing.T) {
	cfg := server.ModernTLSConfig()

	assert.NotNil(t, cfg)
	assert.Equal(t, uint16(tls.VersionTLS13), cfg.MinVersion)
	assert.Empty(t, cfg.CipherSuites) // TLS 1.3 auto-selects cipher suites
	assert.Contains(t, cfg.CurvePreferences, tls.X25519)
	assert.Contains(t, cfg.CurvePreferences, tls.CurveP256)
}

func TestIntermediateTLSConfig(t *testing.T) {
	cfg := server.IntermediateTLSConfig()

	assert.NotNil(t, cfg)
	assert.Equal(t, uint16(tls.VersionTLS12), cfg.MinVersion)
	assert.NotEmpty(t, cfg.CipherSuites)
	// Should include ECDHE ciphers for forward secrecy
	assert.Contains(t, cfg.CipherSuites, uint16(tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256))
	assert.Len(t, cfg.CurvePreferences, 3) // X25519, P256, P384
}

func TestStrictTLSConfig(t *testing.T) {
	cfg := server.StrictTLSConfig()

	assert.NotNil(t, cfg)
	assert.Equal(t, uint16(tls.VersionTLS13), cfg.MinVersion)
	assert.True(t, cfg.SessionTicketsDisabled)
	assert.Equal(t, tls.RenegotiateNever, cfg.Renegotiation)
	assert.False(t, cfg.PreferServerCipherSuites)
}

func TestNewTLSConfig(t *testing.T) {
	t.Run("default config", func(t *testing.T) {
		cfg := server.NewTLSConfig()
		assert.NotNil(t, cfg)
		assert.Equal(t, uint16(tls.VersionTLS12), cfg.MinVersion)
	})

	t.Run("with min version", func(t *testing.T) {
		cfg := server.NewTLSConfig(
			server.WithTLSMinVersion(tls.VersionTLS13),
		)
		assert.Equal(t, uint16(tls.VersionTLS13), cfg.MinVersion)
	})

	t.Run("with server name", func(t *testing.T) {
		cfg := server.NewTLSConfig(
			server.WithTLSServerName("example.com"),
		)
		assert.Equal(t, "example.com", cfg.ServerName)
	})

	t.Run("with client auth", func(t *testing.T) {
		cfg := server.NewTLSConfig(
			server.WithTLSClientAuth(tls.RequireAndVerifyClientCert),
		)
		assert.Equal(t, tls.RequireAndVerifyClientCert, cfg.ClientAuth)
	})

	t.Run("with insecure skip verify", func(t *testing.T) {
		cfg := server.NewTLSConfig(
			server.WithTLSInsecureSkipVerify(),
		)
		assert.True(t, cfg.InsecureSkipVerify)
	})

	t.Run("multiple options", func(t *testing.T) {
		cfg := server.NewTLSConfig(
			server.WithTLSMinVersion(tls.VersionTLS13),
			server.WithTLSServerName("example.com"),
			server.WithTLSClientAuth(tls.RequestClientCert),
		)
		assert.Equal(t, uint16(tls.VersionTLS13), cfg.MinVersion)
		assert.Equal(t, "example.com", cfg.ServerName)
		assert.Equal(t, tls.RequestClientCert, cfg.ClientAuth)
	})
}

func TestWithTLSCertificate(t *testing.T) {
	// This test just ensures the option doesn't panic
	// Loading actual cert files would require fixtures
	cfg := server.NewTLSConfig(
		server.WithTLSCertificate("nonexistent.pem", "nonexistent.key"),
	)
	require.NotNil(t, cfg)
	assert.Empty(t, cfg.Certificates) // Should be empty since files don't exist
}
