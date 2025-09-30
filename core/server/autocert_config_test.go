package server_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dmitrymomot/foundation/core/server"
)

func TestNewAutoCertFromConfig(t *testing.T) {
	t.Run("creates server from config", func(t *testing.T) {
		cfg := server.DefaultAutoCertConfig()
		cfg.ReadTimeout = 30 * time.Second
		cfg.WriteTimeout = 30 * time.Second

		certManager := &MockCertificateManager{}
		domainStore := &MockDomainStore{}

		srv, err := server.NewAutoCertFromConfig(
			cfg,
			certManager,
			domainStore,
		)

		require.NoError(t, err)
		assert.NotNil(t, srv)
	})

	t.Run("allows overriding with options", func(t *testing.T) {
		cfg := server.DefaultAutoCertConfig()

		certManager := &MockCertificateManager{}
		domainStore := &MockDomainStore{}

		customHandler := server.DefaultProvisioningHandler(nil)

		srv, err := server.NewAutoCertFromConfig(
			cfg,
			certManager,
			domainStore,
			server.WithProvisioningHandler(customHandler),
		)

		require.NoError(t, err)
		assert.NotNil(t, srv)
	})

	t.Run("handles custom addresses", func(t *testing.T) {
		cfg := server.AutoCertConfig{
			Config:    server.DefaultConfig(),
			HTTPAddr:  ":8080",
			HTTPSAddr: ":8443",
		}

		certManager := &MockCertificateManager{}
		domainStore := &MockDomainStore{}

		srv, err := server.NewAutoCertFromConfig(
			cfg,
			certManager,
			domainStore,
		)

		require.NoError(t, err)
		assert.NotNil(t, srv)
	})

	t.Run("ignores base Config Addr field", func(t *testing.T) {
		cfg := server.AutoCertConfig{
			Config:    server.DefaultConfig(),
			HTTPAddr:  ":80",
			HTTPSAddr: ":443",
		}
		// Even if Addr is set in base config, it should be ignored
		cfg.Addr = ":9999"

		certManager := &MockCertificateManager{}
		domainStore := &MockDomainStore{}

		srv, err := server.NewAutoCertFromConfig(
			cfg,
			certManager,
			domainStore,
		)

		require.NoError(t, err)
		assert.NotNil(t, srv)
		// The server should use HTTPAddr and HTTPSAddr, not Addr
	})
}

func TestDefaultAutoCertConfig(t *testing.T) {
	cfg := server.DefaultAutoCertConfig()

	// Base Config Addr should be empty
	assert.Empty(t, cfg.Addr)

	// AutoCert addresses should have defaults
	assert.Equal(t, ":80", cfg.HTTPAddr)
	assert.Equal(t, ":443", cfg.HTTPSAddr)

	// Should inherit timeout defaults from base Config
	assert.Equal(t, server.DefaultReadTimeout, cfg.ReadTimeout)
	assert.Equal(t, server.DefaultWriteTimeout, cfg.WriteTimeout)
	assert.Equal(t, server.DefaultIdleTimeout, cfg.IdleTimeout)
	assert.Equal(t, server.DefaultShutdownTimeout, cfg.ShutdownTimeout)
	assert.Equal(t, server.DefaultMaxHeaderBytes, cfg.MaxHeaderBytes)
}
