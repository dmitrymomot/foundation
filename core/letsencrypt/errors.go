package letsencrypt

import "errors"

var (
	// ErrCertificateNotFound is returned when a certificate cannot be found for a domain.
	ErrCertificateNotFound = errors.New("certificate not found")

	// ErrInvalidDomain is returned when the provided domain name is invalid.
	ErrInvalidDomain = errors.New("invalid domain name")

	// ErrCertificateExpired is returned when a certificate has expired.
	ErrCertificateExpired = errors.New("certificate expired")

	// ErrGenerationFailed is returned when certificate generation fails.
	ErrGenerationFailed = errors.New("certificate generation failed")

	// ErrEmailRequired is returned when email is not provided in config.
	ErrEmailRequired = errors.New("email is required for Let's Encrypt account")

	// ErrCertDirRequired is returned when certificate directory is not provided in config.
	ErrCertDirRequired = errors.New("certificate directory is required")
)
