// Package letsencrypt provides Let's Encrypt certificate management with explicit
// control over certificate operations. No automatic generation or background jobs.
//
// The Manager type satisfies the server.CertificateManager interface for integration
// with the foundation server package while maintaining full control over when and how
// certificates are created, renewed, or deleted.
//
// # Types
//
//   - Manager: Certificate operations with explicit control
//   - Config: Manager configuration
//   - Storage: Low-level certificate file operations
//
// # Errors
//
//   - ErrCertificateNotFound: Certificate not found for domain
//   - ErrInvalidDomain: Invalid domain name
//   - ErrCertificateExpired: Certificate expired
//   - ErrGenerationFailed: Certificate generation failed
//
// # Features
//
//   - Explicit operations: Generate, Renew, Delete
//   - ACME HTTP-01 challenge support
//   - Atomic disk storage with caching
//   - Production and staging environments
//   - Retry logic with exponential backoff
//   - Thread-safe operations
//
// # Basic Usage
//
//	import (
//		"context"
//		"fmt"
//		"log"
//
//		"github.com/dmitrymomot/foundation/core/letsencrypt"
//	)
//
//	func main() {
//		manager, err := letsencrypt.NewManager(letsencrypt.Config{
//			Email:   "admin@example.com",
//			CertDir: "/var/cache/certs",
//		})
//		if err != nil {
//			log.Fatal(err)
//		}
//
//		ctx := context.Background()
//
//		// Generate certificate (30-60 seconds)
//		err = manager.Generate(ctx, "example.com")
//		if err != nil {
//			log.Printf("Generation failed: %v", err)
//			return
//		}
//
//		if manager.Exists("example.com") {
//			fmt.Println("Certificate ready")
//		}
//
//		// Renew certificate
//		err = manager.Renew(ctx, "example.com")
//		if err != nil {
//			log.Printf("Renewal failed: %v", err)
//		}
//
//		// Delete certificate
//		err = manager.Delete(ctx, "example.com")
//		if err != nil {
//			log.Printf("Deletion failed: %v", err)
//		}
//	}
//
// # Server Integration
//
// Use with the foundation/core/server package for automatic HTTPS:
//
//	import (
//		"context"
//		"log"
//
//		"github.com/dmitrymomot/foundation/core/handler"
//		"github.com/dmitrymomot/foundation/core/letsencrypt"
//		"github.com/dmitrymomot/foundation/core/response"
//		"github.com/dmitrymomot/foundation/core/router"
//		"github.com/dmitrymomot/foundation/core/server"
//	)
//
//	type AppContext = router.Context
//
//	func main() {
//		certManager, err := letsencrypt.NewManager(letsencrypt.Config{
//			Email:   "admin@example.com",
//			CertDir: "/var/cache/certs",
//		})
//		if err != nil {
//			log.Fatal(err)
//		}
//
//		type MyDomainStore struct{}
//		func (s *MyDomainStore) GetDomain(ctx context.Context, domain string) (*server.DomainInfo, error) {
//			return &server.DomainInfo{
//				Domain:   domain,
//				TenantID: "tenant-123",
//				Status:   server.StatusActive,
//			}, nil
//		}
//
//		r := router.New[*AppContext]()
//		r.Get("/", func(ctx *AppContext) handler.Response {
//			return response.HTML("<h1>Secure App</h1>")
//		})
//
//		config := &server.AutoCertConfig[*AppContext]{
//			CertManager: certManager,
//			DomainStore: &MyDomainStore{},
//			HTTPAddr:    ":80",
//			HTTPSAddr:   ":443",
//		}
//
//		srv, err := server.NewAutoCertServer(config)
//		if err != nil {
//			log.Fatal(err)
//		}
//
//		ctx := context.Background()
//		if err := srv.Run(ctx, r); err != nil {
//			log.Fatal(err)
//		}
//	}
//
// # Storage Operations
//
// The Storage type provides low-level certificate file operations:
//
//	import (
//		"fmt"
//		"log"
//
//		"github.com/dmitrymomot/foundation/core/letsencrypt"
//	)
//
//	func main() {
//		storage, err := letsencrypt.NewStorage("/var/cache/certs")
//		if err != nil {
//			log.Fatal(err)
//		}
//
//		// List certificate domains
//		domains, err := storage.List()
//		if err != nil {
//			log.Printf("List failed: %v", err)
//		}
//
//		if storage.Exists("example.com") {
//			fmt.Println("Certificate found")
//		}
//
//		// Read certificate data
//		certData, err := storage.Read("example.com")
//		if err != nil {
//			log.Printf("Read failed: %v", err)
//		}
//
//		// Copy certificate
//		err = storage.Copy("example.com", "www.example.com")
//		if err != nil {
//			log.Printf("Copy failed: %v", err)
//		}
//
//		err = storage.Delete("example.com")
//		if err != nil {
//			log.Printf("Delete failed: %v", err)
//		}
//	}
//
// # Storage Structure
//
// Certificates stored with atomic writes:
//
//	/var/cache/certs/
//	├── acme_account+key     # ACME account key
//	├── example.com          # Certificate and key
//	├── www.example.com      # Subdomain certificate
//	└── api.example.com      # API subdomain certificate
//
// Each file contains certificate chain and private key in PEM format.
//
// # Manager Methods
//
// server.CertificateManager interface:
//
//   - GetCertificate: TLS handshake certificate retrieval
//   - HandleChallenge: ACME HTTP-01 challenge handling
//   - Exists: Check certificate existence
//
// Management operations:
//
//   - Generate: Create new certificate with retry logic
//   - Renew: Force renewal by regenerating
//   - Delete: Remove certificate from storage
//   - CertDir: Get storage directory path
//
// # ACME Challenge Handling
//
// ACME HTTP-01 challenges require port 80 access for validation:
//
//	import (
//		"net/http"
//
//		"github.com/dmitrymomot/foundation/core/handler"
//		"github.com/dmitrymomot/foundation/core/response"
//		"github.com/dmitrymomot/foundation/core/router"
//	)
//
//	type AppContext = router.Context
//
//	r := router.New[*AppContext]()
//	r.Handle("/*", func(ctx *AppContext) handler.Response {
//		// Handle ACME challenges first
//		if manager.HandleChallenge(ctx.Response(), ctx.Request()) {
//			return nil
//		}
//		return response.Text("Hello, World!")
//	})
//
//	http.ListenAndServe(":80", r)
//
// # Error Handling Patterns
//
// The package provides specific error types for different scenarios:
//
//	import (
//		"errors"
//		"log"
//
//		"github.com/dmitrymomot/foundation/core/letsencrypt"
//	)
//
//	err := manager.Generate(ctx, "example.com")
//	if err != nil {
//		switch {
//		case errors.Is(err, letsencrypt.ErrCertificateNotFound):
//			log.Println("Certificate not found")
//		case errors.Is(err, letsencrypt.ErrInvalidDomain):
//			log.Println("Invalid domain")
//		case errors.Is(err, letsencrypt.ErrGenerationFailed):
//			log.Println("Generation failed")
//		default:
//			log.Printf("Unexpected error: %v", err)
//		}
//	}
//
// # Retry Logic
//
// Built-in retry with exponential backoff for:
//
//   - Network connectivity issues
//   - DNS resolution problems
//   - Rate limiting (429)
//   - Service unavailable (503)
//
// Automatic retries: 3 attempts with 5-second initial backoff.
//
// # Security
//
//   - Explicit operations only (no automatic generation)
//   - Validate domain ownership before generation
//   - Use staging environment for testing
//   - Monitor expiration dates
//   - Rate limit operations
//   - Files stored with 0600 permissions
//   - Atomic writes prevent corruption
//
// # Production Notes
//
//   - Requires ports 80 and 443 available
//   - Domain must point to server IP
//   - Rate limits: 50 certificates per domain per week
//   - Generation time: 30-60 seconds
//   - No wildcard support (needs DNS-01)
//   - Monitor disk space and expiration dates
package letsencrypt
