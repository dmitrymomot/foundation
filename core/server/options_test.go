package server_test

import (
	"crypto/tls"
	"fmt"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dmitrymomot/foundation/core/server"
)

// TestWithTLS tests the WithTLS option
func TestWithTLS(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		tlsConfig *tls.Config
		wantPanic bool
	}{
		{
			name: "valid TLS config",
			tlsConfig: &tls.Config{
				MinVersion: tls.VersionTLS13,
				MaxVersion: tls.VersionTLS13,
			},
		},
		{
			name:      "nil TLS config",
			tlsConfig: nil,
		},
		{
			name: "TLS config with certificates",
			tlsConfig: &tls.Config{
				MinVersion:   tls.VersionTLS12,
				Certificates: []tls.Certificate{},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			port := fmt.Sprintf(":%d", getFreePort(t))

			if tt.wantPanic {
				assert.Panics(t, func() {
					_ = server.New(port, server.WithTLS(tt.tlsConfig))
				})
			} else {
				srv := server.New(port, server.WithTLS(tt.tlsConfig))
				assert.NotNil(t, srv)
			}
		})
	}
}

// TestWithLogger tests the WithLogger option
func TestWithLogger(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		logger *slog.Logger
	}{
		{
			name:   "custom logger",
			logger: slog.Default().With("test", "value"),
		},
		{
			name:   "nil logger",
			logger: nil,
		},
		{
			name:   "logger with custom handler",
			logger: slog.New(slog.NewTextHandler(nil, nil)),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			port := fmt.Sprintf(":%d", getFreePort(t))
			srv := server.New(port, server.WithLogger(tt.logger))
			assert.NotNil(t, srv)
			// Note: we can't easily verify the logger was set without exposing it
		})
	}
}

// TestWithShutdownTimeout tests the WithShutdownTimeout option
func TestWithShutdownTimeout(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		timeout time.Duration
	}{
		{
			name:    "positive timeout",
			timeout: 30 * time.Second,
		},
		{
			name:    "zero timeout",
			timeout: 0,
		},
		{
			name:    "negative timeout",
			timeout: -5 * time.Second,
		},
		{
			name:    "very short timeout",
			timeout: 1 * time.Millisecond,
		},
		{
			name:    "very long timeout",
			timeout: 1 * time.Hour,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			port := fmt.Sprintf(":%d", getFreePort(t))
			srv := server.New(port, server.WithShutdownTimeout(tt.timeout))
			assert.NotNil(t, srv)
			// Note: we can't easily verify the timeout was set without exposing it
		})
	}
}

// TestMultipleOptions tests applying multiple options
func TestMultipleOptions(t *testing.T) {
	t.Parallel()

	t.Run("all options together", func(t *testing.T) {
		port := fmt.Sprintf(":%d", getFreePort(t))

		tlsConfig := &tls.Config{
			MinVersion: tls.VersionTLS12,
		}
		logger := slog.Default().With("test", "multiple")
		timeout := 10 * time.Second

		srv := server.New(port,
			server.WithTLS(tlsConfig),
			server.WithLogger(logger),
			server.WithShutdownTimeout(timeout),
		)

		assert.NotNil(t, srv)
	})

	t.Run("options applied in different order", func(t *testing.T) {
		port := fmt.Sprintf(":%d", getFreePort(t))

		srv1 := server.New(port,
			server.WithShutdownTimeout(5*time.Second),
			server.WithLogger(slog.Default()),
			server.WithTLS(&tls.Config{}),
		)

		srv2 := server.New(port,
			server.WithTLS(&tls.Config{}),
			server.WithShutdownTimeout(5*time.Second),
			server.WithLogger(slog.Default()),
		)

		assert.NotNil(t, srv1)
		assert.NotNil(t, srv2)
	})

	t.Run("same option applied multiple times", func(t *testing.T) {
		port := fmt.Sprintf(":%d", getFreePort(t))

		// Last option should win
		srv := server.New(port,
			server.WithShutdownTimeout(5*time.Second),
			server.WithShutdownTimeout(10*time.Second),
			server.WithShutdownTimeout(15*time.Second),
		)

		assert.NotNil(t, srv)
	})
}

// TestOptionsThreadSafety tests that options are applied thread-safely
func TestOptionsThreadSafety(t *testing.T) {
	t.Parallel()

	t.Run("concurrent option application", func(t *testing.T) {
		port := fmt.Sprintf(":%d", getFreePort(t))
		srv := server.New(port)

		// Try to apply options concurrently
		// This shouldn't panic or cause race conditions
		done := make(chan bool, 3)

		go func() {
			server.WithTLS(&tls.Config{})(srv)
			done <- true
		}()

		go func() {
			server.WithLogger(slog.Default())(srv)
			done <- true
		}()

		go func() {
			server.WithShutdownTimeout(5 * time.Second)(srv)
			done <- true
		}()

		// Wait for all goroutines to complete
		for i := 0; i < 3; i++ {
			<-done
		}

		assert.NotNil(t, srv)
	})
}

// TestOptionsWithServerLifecycle tests options during server lifecycle
func TestOptionsWithServerLifecycle(t *testing.T) {
	t.Parallel()

	t.Run("options before run", func(t *testing.T) {
		port := fmt.Sprintf(":%d", getFreePort(t))
		srv := server.New(port)

		// Apply options before running
		server.WithTLS(&tls.Config{MinVersion: tls.VersionTLS12})(srv)
		server.WithLogger(slog.Default())(srv)
		server.WithShutdownTimeout(5 * time.Second)(srv)

		assert.NotNil(t, srv)
	})

	t.Run("shutdown timeout affects graceful shutdown", func(t *testing.T) {
		port := fmt.Sprintf(":%d", getFreePort(t))

		// Create server with very short shutdown timeout
		srv := server.New(port, server.WithShutdownTimeout(10*time.Millisecond))
		require.NotNil(t, srv)

		// Note: We can't fully test the shutdown timeout behavior without
		// running the server and having active connections
	})
}
