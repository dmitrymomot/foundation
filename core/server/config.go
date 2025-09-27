package server

import (
	"crypto/tls"
	"errors"
	"fmt"
	"time"
)

// ErrMissingAddress is returned when server address is not provided.
var ErrMissingAddress = errors.New("server address is required")

// Config holds server configuration with environment variable support.
type Config struct {
	// Server address
	Addr string `env:"SERVER_ADDR" envDefault:":8080"`

	// Timeouts
	ReadTimeout     time.Duration `env:"SERVER_READ_TIMEOUT" envDefault:"15s"`
	WriteTimeout    time.Duration `env:"SERVER_WRITE_TIMEOUT" envDefault:"15s"`
	IdleTimeout     time.Duration `env:"SERVER_IDLE_TIMEOUT" envDefault:"60s"`
	ShutdownTimeout time.Duration `env:"SERVER_SHUTDOWN_TIMEOUT" envDefault:"30s"`

	// Header limits
	MaxHeaderBytes int `env:"SERVER_MAX_HEADER_BYTES" envDefault:"1048576"` // 1MB

	// TLS Configuration (optional)
	TLSCertFile string `env:"SERVER_TLS_CERT_FILE" envDefault:""`
	TLSKeyFile  string `env:"SERVER_TLS_KEY_FILE" envDefault:""`
}

// DefaultConfig returns a Config with sensible defaults.
func DefaultConfig() Config {
	return Config{
		Addr:            ":8080",
		ReadTimeout:     DefaultReadTimeout,
		WriteTimeout:    DefaultWriteTimeout,
		IdleTimeout:     DefaultIdleTimeout,
		ShutdownTimeout: DefaultShutdownTimeout,
		MaxHeaderBytes:  DefaultMaxHeaderBytes,
	}
}

// NewFromConfig creates a Server from configuration.
// Additional options can override config values.
func NewFromConfig(cfg Config, opts ...Option) (*Server, error) {
	// Validate address
	if cfg.Addr == "" {
		return nil, ErrMissingAddress
	}

	// Build options from config
	configOpts := make([]Option, 0)

	// Apply timeout configurations
	if cfg.ReadTimeout > 0 {
		configOpts = append(configOpts, WithReadTimeout(cfg.ReadTimeout))
	}
	if cfg.WriteTimeout > 0 {
		configOpts = append(configOpts, WithWriteTimeout(cfg.WriteTimeout))
	}
	if cfg.IdleTimeout > 0 {
		configOpts = append(configOpts, WithIdleTimeout(cfg.IdleTimeout))
	}
	if cfg.ShutdownTimeout > 0 {
		configOpts = append(configOpts, WithShutdownTimeout(cfg.ShutdownTimeout))
	}
	if cfg.MaxHeaderBytes > 0 {
		configOpts = append(configOpts, WithMaxHeaderBytes(cfg.MaxHeaderBytes))
	}

	// Handle TLS if configured
	if cfg.TLSCertFile != "" && cfg.TLSKeyFile != "" {
		tlsConfig, err := loadTLSFromFiles(cfg.TLSCertFile, cfg.TLSKeyFile)
		if err != nil {
			return nil, fmt.Errorf("failed to load TLS configuration from files %s, %s: %w",
				cfg.TLSCertFile, cfg.TLSKeyFile, err)
		}
		configOpts = append(configOpts, WithTLS(tlsConfig))
	}

	// Append user-provided options to override config if needed
	configOpts = append(configOpts, opts...)

	// Use existing New constructor with combined options
	return New(cfg.Addr, configOpts...), nil
}

// loadTLSFromFiles creates a TLS config from certificate and key files.
func loadTLSFromFiles(certFile, keyFile string) (*tls.Config, error) {
	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return nil, err
	}

	return &tls.Config{
		Certificates: []tls.Certificate{cert},
		MinVersion:   tls.VersionTLS12, // Enforce TLS 1.2 minimum
	}, nil
}
