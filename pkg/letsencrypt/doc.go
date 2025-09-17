// Package letsencrypt provides Let's Encrypt certificate management for multi-tenant SaaS applications.
//
// This package offers explicit control over SSL/TLS certificates without any automatic generation
// or background jobs. It's designed for production environments where you need full control over
// when and how certificates are created, renewed, or deleted.
//
// # Features
//
//   - Explicit certificate operations (Generate, Renew, Delete)
//   - No automatic certificate generation or renewal
//   - Adapter-based domain registration lookups
//   - Configurable status pages for provisioning states
//   - Support for multi-tenant architectures
//   - Let's Encrypt staging environment support
//
// # Basic Usage
//
//	// Create certificate manager
//	manager, err := letsencrypt.NewManager(letsencrypt.Config{
//	    Email:   "admin@example.com",
//	    CertDir: "/var/cache/certs",
//	    Staging: false, // Use production Let's Encrypt
//	})
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	// Generate a certificate (blocking operation, ~30-60 seconds)
//	err = manager.Generate(ctx, "example.com")
//	if err != nil {
//	    log.Printf("Certificate generation failed: %v", err)
//	}
//
//	// Check if certificate exists
//	if manager.Exists("example.com") {
//	    fmt.Println("Certificate ready")
//	}
//
//	// Renew a certificate
//	err = manager.Renew(ctx, "example.com")
//
//	// Delete a certificate
//	err = manager.Delete("example.com")
//
// # Domain Store Integration
//
// Implement the DomainStore interface to integrate with your domain registration system:
//
//	type MyDomainStore struct {
//	    db *sql.DB
//	}
//
//	func (s *MyDomainStore) GetDomain(ctx context.Context, domain string) (*letsencrypt.DomainInfo, error) {
//	    var info letsencrypt.DomainInfo
//	    err := s.db.QueryRow(`
//	        SELECT domain, tenant_id, status, error, created_at
//	        FROM domains WHERE domain = $1
//	    `, domain).Scan(&info.Domain, &info.TenantID, &info.Status, &info.Error, &info.CreatedAt)
//	    if err == sql.ErrNoRows {
//	        return nil, nil // Domain not registered
//	    }
//	    return &info, err
//	}
//
// # Custom Status Pages
//
// Implement the StatusPageHandler interface for custom provisioning pages:
//
//	type MyStatusHandler struct {
//	    templates *template.Template
//	}
//
//	func (h *MyStatusHandler) ServeProvisioning(w http.ResponseWriter, r *http.Request, info *DomainInfo) {
//	    w.Header().Set("Content-Type", "text/html")
//	    w.WriteHeader(http.StatusAccepted)
//	    h.templates.ExecuteTemplate(w, "provisioning.html", info)
//	}
//
//	func (h *MyStatusHandler) ServeFailed(w http.ResponseWriter, r *http.Request, info *DomainInfo) {
//	    w.Header().Set("Content-Type", "text/html")
//	    w.WriteHeader(http.StatusServiceUnavailable)
//	    h.templates.ExecuteTemplate(w, "failed.html", info)
//	}
//
//	func (h *MyStatusHandler) ServeNotFound(w http.ResponseWriter, r *http.Request) {
//	    http.NotFound(w, r)
//	}
//
// # Server Integration
//
// Use with the foundation/core/server package for automatic HTTPS:
//
//	import "github.com/dmitrymomot/foundation/core/server"
//
//	// Configure server with AutoCert
//	srv := server.New(
//	    server.WithAutoCert(&server.AutoCertConfig{
//	        CertManager:   manager,
//	        DomainStore:   myDomainStore,
//	        StatusHandler: letsencrypt.NewDefaultStatusPages(),
//	        HTTPAddr:      ":80",
//	        HTTPSAddr:     ":443",
//	    }),
//	)
//
//	// Run server (handles both HTTP and HTTPS)
//	srv.Run(ctx, myApp)
//
// # Certificate Storage
//
// Certificates are stored on disk in the specified directory:
//
//	/var/cache/certs/
//	├── acme_account+key     # ACME account key
//	└── example.com          # Certificate and key for example.com
//	└── tenant.example.com   # Certificate and key for tenant subdomain
//
// # Domain Status Flow
//
//  1. Domain registered → Status: provisioning
//  2. Certificate generated → Status: active
//  3. Certificate failed → Status: failed
//  4. Domain removed → Certificate deleted
//
// # Security Considerations
//
//   - Never serve application content over HTTP
//   - Always validate domain ownership before generating certificates
//   - Use staging environment for development and testing
//   - Monitor certificate expiration dates
//   - Implement rate limiting for certificate operations
//
// # Limitations
//
//   - Requires ports 80 and 443 to be available
//   - Domain must point to server IP before certificate generation
//   - Let's Encrypt rate limits apply (50 certificates per domain per week)
//   - No wildcard certificate support (requires DNS-01 challenge)
package letsencrypt
