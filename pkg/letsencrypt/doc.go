// Package letsencrypt provides utilities for provisioning TLS certificates from
// Let's Encrypt (or any ACME-compatible CA) and persisting them to disk.
//
// The package currently focuses on performing an HTTP-01 challenge using the
// lego ACME client, making it convenient to bootstrap certificates for domains
// that can temporarily expose port 80 during issuance.
package letsencrypt
