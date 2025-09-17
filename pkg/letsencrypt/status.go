package letsencrypt

import "net/http"

// StatusPageHandler defines the interface for handling domain status pages.
// Implementations can provide custom HTML, JSON responses, or any other format.
type StatusPageHandler interface {
	// ServeProvisioning handles requests when a certificate is being generated.
	// This should inform the user that SSL/TLS is being configured.
	ServeProvisioning(w http.ResponseWriter, r *http.Request, info *DomainInfo)

	// ServeFailed handles requests when certificate generation failed.
	// This should explain the error and provide guidance.
	ServeFailed(w http.ResponseWriter, r *http.Request, info *DomainInfo)

	// ServeNotFound handles requests for unregistered domains.
	// This typically returns a 404 error.
	ServeNotFound(w http.ResponseWriter, r *http.Request)
}
