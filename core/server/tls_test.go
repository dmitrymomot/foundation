package server_test

import (
	"crypto/tls"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dmitrymomot/foundation/core/server"
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
		cfg, err := server.NewTLSConfig()
		require.NoError(t, err)
		assert.NotNil(t, cfg)
		assert.Equal(t, uint16(tls.VersionTLS12), cfg.MinVersion)
	})

	t.Run("with min version", func(t *testing.T) {
		cfg, err := server.NewTLSConfig(
			server.WithTLSMinVersion(tls.VersionTLS13),
		)
		require.NoError(t, err)
		assert.Equal(t, uint16(tls.VersionTLS13), cfg.MinVersion)
	})

	t.Run("with server name", func(t *testing.T) {
		cfg, err := server.NewTLSConfig(
			server.WithTLSServerName("example.com"),
		)
		require.NoError(t, err)
		assert.Equal(t, "example.com", cfg.ServerName)
	})

	t.Run("with client auth", func(t *testing.T) {
		cfg, err := server.NewTLSConfig(
			server.WithTLSClientAuth(tls.RequireAndVerifyClientCert),
		)
		require.NoError(t, err)
		assert.Equal(t, tls.RequireAndVerifyClientCert, cfg.ClientAuth)
	})

	t.Run("with insecure skip verify", func(t *testing.T) {
		cfg, err := server.NewTLSConfig(
			server.WithTLSInsecureSkipVerify(),
		)
		require.NoError(t, err)
		assert.True(t, cfg.InsecureSkipVerify)
	})

	t.Run("multiple options", func(t *testing.T) {
		cfg, err := server.NewTLSConfig(
			server.WithTLSMinVersion(tls.VersionTLS13),
			server.WithTLSServerName("example.com"),
			server.WithTLSClientAuth(tls.RequestClientCert),
		)
		require.NoError(t, err)
		assert.Equal(t, uint16(tls.VersionTLS13), cfg.MinVersion)
		assert.Equal(t, "example.com", cfg.ServerName)
		assert.Equal(t, tls.RequestClientCert, cfg.ClientAuth)
	})
}

func TestWithTLSCertificate(t *testing.T) {
	t.Run("nonexistent files return error", func(t *testing.T) {
		// This test ensures the option returns an error for nonexistent files
		cfg, err := server.NewTLSConfig(
			server.WithTLSCertificate("nonexistent.pem", "nonexistent.key"),
		)
		require.Error(t, err)
		assert.Nil(t, cfg)
		assert.Contains(t, err.Error(), "failed to load certificate")
	})

	t.Run("empty cert path returns error", func(t *testing.T) {
		cfg, err := server.NewTLSConfig(
			server.WithTLSCertificate("", "key.pem"),
		)
		require.Error(t, err)
		assert.Nil(t, cfg)
		assert.ErrorIs(t, err, server.ErrEmptyCertPath)
	})

	t.Run("empty key path returns error", func(t *testing.T) {
		cfg, err := server.NewTLSConfig(
			server.WithTLSCertificate("cert.pem", ""),
		)
		require.Error(t, err)
		assert.Nil(t, cfg)
		assert.ErrorIs(t, err, server.ErrEmptyCertPath)
	})

	t.Run("both paths empty returns error", func(t *testing.T) {
		cfg, err := server.NewTLSConfig(
			server.WithTLSCertificate("", ""),
		)
		require.Error(t, err)
		assert.Nil(t, cfg)
		assert.ErrorIs(t, err, server.ErrEmptyCertPath)
	})
}

func TestTLSOptionValidation(t *testing.T) {
	t.Run("invalid TLS version returns error", func(t *testing.T) {
		cfg, err := server.NewTLSConfig(
			server.WithTLSMinVersion(0x0300), // Invalid version (SSL 3.0)
		)
		require.Error(t, err)
		assert.Nil(t, cfg)
		assert.ErrorIs(t, err, server.ErrInvalidTLSVersion)
	})

	t.Run("valid TLS versions accepted", func(t *testing.T) {
		versions := []uint16{
			tls.VersionTLS10,
			tls.VersionTLS11,
			tls.VersionTLS12,
			tls.VersionTLS13,
		}

		for _, version := range versions {
			cfg, err := server.NewTLSConfig(
				server.WithTLSMinVersion(version),
			)
			require.NoError(t, err)
			assert.Equal(t, version, cfg.MinVersion)
		}
	})

	t.Run("empty server name returns error", func(t *testing.T) {
		cfg, err := server.NewTLSConfig(
			server.WithTLSServerName(""),
		)
		require.Error(t, err)
		assert.Nil(t, cfg)
		assert.ErrorIs(t, err, server.ErrEmptyServerName)
	})

	t.Run("invalid client auth type returns error", func(t *testing.T) {
		cfg, err := server.NewTLSConfig(
			server.WithTLSClientAuth(tls.ClientAuthType(99)), // Invalid type
		)
		require.Error(t, err)
		assert.Nil(t, cfg)
		assert.ErrorIs(t, err, server.ErrInvalidClientAuthType)
	})

	t.Run("valid client auth types accepted", func(t *testing.T) {
		authTypes := []tls.ClientAuthType{
			tls.NoClientCert,
			tls.RequestClientCert,
			tls.RequireAnyClientCert,
			tls.VerifyClientCertIfGiven,
			tls.RequireAndVerifyClientCert,
		}

		for _, authType := range authTypes {
			cfg, err := server.NewTLSConfig(
				server.WithTLSClientAuth(authType),
			)
			require.NoError(t, err)
			assert.Equal(t, authType, cfg.ClientAuth)
		}
	})
}
