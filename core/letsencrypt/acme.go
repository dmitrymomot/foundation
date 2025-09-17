package letsencrypt

import (
	"crypto/tls"
	"net/http"
)

// ACMEProvider defines the interface for ACME certificate operations.
// This abstraction allows for testing without real ACME requests and
// enables swapping providers (e.g., for staging environments).
type ACMEProvider interface {
	// GetCertificate obtains a certificate from the ACME provider.
	// It's called during TLS handshake to get or generate certificates.
	GetCertificate(hello *tls.ClientHelloInfo) (*tls.Certificate, error)

	// HTTPHandler returns the HTTP handler for ACME HTTP-01 challenges.
	// The handler processes ACME challenge requests at /.well-known/acme-challenge/.
	HTTPHandler(fallback http.Handler) http.Handler
}
