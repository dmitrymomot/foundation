package server

import "errors"

var (
	// Certificate and domain errors
	ErrNoCertManager       = errors.New("certificate manager is required")
	ErrNoDomainStore       = errors.New("domain store is required")
	ErrNoServerName        = errors.New("no server name provided")
	ErrDomainNotRegistered = errors.New("domain not registered")
	ErrDomainLookupFailed  = errors.New("domain lookup failed")

	// TLS configuration errors
	ErrEmptyCertPath         = errors.New("certificate or key file path cannot be empty")
	ErrEmptyServerName       = errors.New("server name cannot be empty")
	ErrInvalidTLSVersion     = errors.New("invalid TLS version")
	ErrInvalidClientAuthType = errors.New("invalid client auth type")
	ErrTLSVersionMismatch    = errors.New("TLS version mismatch")
	ErrFailedLoadCert        = errors.New("failed to load certificate")

	// Server lifecycle errors
	ErrServerAlreadyRunning = errors.New("server is already running")
	ErrHTTPServer           = errors.New("HTTP server error")
	ErrHTTPSServer          = errors.New("HTTPS server error")
	ErrHTTPShutdown         = errors.New("HTTP shutdown error")
	ErrHTTPSShutdown        = errors.New("HTTPS shutdown error")
)
