package server_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dmitrymomot/foundation/core/server"
)

func TestNewFromConfig(t *testing.T) {
	t.Run("creates server from config with defaults", func(t *testing.T) {
		cfg := server.DefaultConfig()
		srv, err := server.NewFromConfig(cfg)

		require.NoError(t, err)
		assert.NotNil(t, srv)
	})

	t.Run("applies custom config values", func(t *testing.T) {
		cfg := server.Config{
			Addr:            ":9000",
			ReadTimeout:     10 * time.Second,
			WriteTimeout:    20 * time.Second,
			IdleTimeout:     30 * time.Second,
			ShutdownTimeout: 5 * time.Second,
			MaxHeaderBytes:  2 << 20, // 2MB
		}

		srv, err := server.NewFromConfig(cfg)

		require.NoError(t, err)
		assert.NotNil(t, srv)
	})

	t.Run("allows overriding config values with options", func(t *testing.T) {
		cfg := server.Config{
			Addr:            ":8080",
			ReadTimeout:     15 * time.Second,
			ShutdownTimeout: 30 * time.Second,
		}

		// Override shutdown timeout with option
		srv, err := server.NewFromConfig(
			cfg,
			server.WithShutdownTimeout(10*time.Second),
		)

		require.NoError(t, err)
		assert.NotNil(t, srv)
	})

	t.Run("fails without address", func(t *testing.T) {
		cfg := server.Config{
			ReadTimeout: 10 * time.Second,
			// Address is empty
		}

		srv, err := server.NewFromConfig(cfg)

		assert.Error(t, err)
		assert.Nil(t, srv)
		assert.Contains(t, err.Error(), "address is required")
	})

	t.Run("handles zero values in config", func(t *testing.T) {
		cfg := server.Config{
			Addr: ":8080",
			// All other values are zero
		}

		srv, err := server.NewFromConfig(cfg)

		require.NoError(t, err)
		assert.NotNil(t, srv)
	})

	t.Run("skips TLS if cert or key missing", func(t *testing.T) {
		cfg := server.Config{
			Addr:        ":8080",
			TLSCertFile: "cert.pem",
			// TLSKeyFile is empty - TLS should not be configured
		}

		srv, err := server.NewFromConfig(cfg)

		require.NoError(t, err)
		assert.NotNil(t, srv)
	})

	t.Run("fails with invalid TLS files", func(t *testing.T) {
		cfg := server.Config{
			Addr:        ":8080",
			TLSCertFile: "/nonexistent/cert.pem",
			TLSKeyFile:  "/nonexistent/key.pem",
		}

		srv, err := server.NewFromConfig(cfg)

		assert.Error(t, err)
		assert.Nil(t, srv)
	})
}

func TestDefaultConfig(t *testing.T) {
	cfg := server.DefaultConfig()

	assert.Equal(t, ":8080", cfg.Addr)
	assert.Equal(t, server.DefaultReadTimeout, cfg.ReadTimeout)
	assert.Equal(t, server.DefaultWriteTimeout, cfg.WriteTimeout)
	assert.Equal(t, server.DefaultIdleTimeout, cfg.IdleTimeout)
	assert.Equal(t, server.DefaultShutdownTimeout, cfg.ShutdownTimeout)
	assert.Equal(t, server.DefaultMaxHeaderBytes, cfg.MaxHeaderBytes)
	assert.Empty(t, cfg.TLSCertFile)
	assert.Empty(t, cfg.TLSKeyFile)
}
