// Package letsencrypt provides a Let's Encrypt certificate manager implementation.
//
// This package offers explicit control over SSL/TLS certificates without any automatic generation
// or background jobs. It's designed for production environments where you need full control over
// when and how certificates are created, renewed, or deleted.
//
// The Manager type satisfies the server.CertificateManager interface without explicitly
// depending on it, following the dependency inversion principle.
//
// # Features
//
//   - Explicit certificate operations (Generate, Renew, Delete)
//   - No automatic certificate generation or renewal
//   - ACME HTTP-01 challenge support
//   - Certificate caching on disk
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
// # Server Integration
//
// Use with the foundation/core/server package for automatic HTTPS:
//
//	import "github.com/dmitrymomot/foundation/core/server"
//
//	// Create certificate manager
//	certManager := letsencrypt.NewManager(letsencrypt.Config{
//	    Email:   "admin@example.com",
//	    CertDir: "/var/cache/certs",
//	})
//
//	// Configure server with AutoCert
//	config := &server.AutoCertConfig[MyContext]{
//	    CertManager:         certManager,  // Satisfies CertificateManager interface
//	    DomainStore:         myDomainStore, // Your DomainStore implementation
//	    ProvisioningHandler: myProvisioningHandler,
//	    HTTPAddr:            ":80",
//	    HTTPSAddr:           ":443",
//	}
//
//	srv, err := server.NewAutoCertServer(config)
//	if err != nil {
//	    log.Fatal(err)
//	}
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
// # Certificate Methods
//
// The Manager provides these methods that satisfy server.CertificateManager:
//
//   - GetCertificate: Retrieves certificate for TLS handshake
//   - HandleChallenge: Handles ACME HTTP-01 challenges
//   - Exists: Checks if certificate exists
//
// Additional management methods:
//
//   - Generate: Explicitly create new certificate
//   - Renew: Force certificate renewal
//   - Delete: Remove certificate from disk
//
// # Security Considerations
//
//   - Never generate certificates automatically
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
