package letsencrypt

import (
	"context"
	"time"
)

// DomainStatus represents the current state of a domain's certificate.
type DomainStatus string

const (
	// StatusProvisioning indicates certificate is being generated.
	StatusProvisioning DomainStatus = "provisioning"

	// StatusActive indicates certificate is ready and valid.
	StatusActive DomainStatus = "active"

	// StatusFailed indicates certificate generation failed.
	StatusFailed DomainStatus = "failed"
)

// DomainInfo contains information about a registered domain.
type DomainInfo struct {
	// Domain is the fully qualified domain name.
	Domain string

	// TenantID is the tenant identifier for multi-tenant domains.
	// Empty for main domain and static subdomains.
	TenantID string

	// Status indicates the current certificate state.
	Status DomainStatus

	// Error contains the error message if Status is Failed.
	Error string

	// CreatedAt is when the domain was registered.
	CreatedAt time.Time
}

// DomainStore is the interface for domain registration lookups.
// Implementations should be thread-safe.
type DomainStore interface {
	// GetDomain retrieves information about a registered domain.
	// Returns nil if the domain is not registered.
	GetDomain(ctx context.Context, domain string) (*DomainInfo, error)
}
