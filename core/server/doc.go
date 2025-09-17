// Package server provides production-ready HTTP/HTTPS servers with graceful shutdown,
// configurable timeouts, and automatic Let's Encrypt certificate management.
//
// Two server types are available: Server for basic HTTP/HTTPS needs and AutoCertServer
// for automatic certificate management with multi-tenant domain support.
//
// # Features
//
//   - Production-ready timeouts and graceful shutdown
//   - Thread-safe concurrent operations
//   - Structured logging with slog
//   - Functional options pattern
//   - Automatic Let's Encrypt certificates (AutoCertServer)
//   - Multi-tenant domain support
//   - ACME HTTP-01 challenge handling
//
// # Basic HTTP Server Usage
//
// Create and run a server with default configuration:
//
//	import (
//		"context"
//		"log"
//
//		"github.com/dmitrymomot/foundation/core/handler"
//		"github.com/dmitrymomot/foundation/core/response"
//		"github.com/dmitrymomot/foundation/core/router"
//		"github.com/dmitrymomot/foundation/core/server"
//	)
//
//	type AppContext = router.Context
//
//	func main() {
//		r := router.New[*AppContext]()
//
//		r.Get("/", func(ctx *AppContext) handler.Response {
//			return response.Text("Hello, World!")
//		})
//
//		ctx := context.Background()
//		if err := server.Run(ctx, ":8080", r); err != nil {
//			log.Fatal(err)
//		}
//	}
//
// # Server Configuration
//
// Configure server with custom options using functional options pattern:
//
//	import (
//		"log/slog"
//		"os"
//		"time"
//	)
//
//	type AppContext = router.Context
//
//	r := router.New[*AppContext]()
//	r.Get("/health", func(ctx *AppContext) handler.Response {
//		return response.JSON(map[string]string{"status": "ok"})
//	})
//
//	srv := server.New(":8080",
//		server.WithShutdownTimeout(60*time.Second),
//		server.WithLogger(slog.New(slog.NewJSONHandler(os.Stdout, nil))),
//	)
//
//	ctx := context.Background()
//	if err := srv.Run(ctx, r); err != nil {
//		log.Fatal(err)
//	}
//
// # HTTPS with TLS Configuration
//
// Use built-in TLS presets for secure HTTPS:
//
//	type AppContext = router.Context
//
//	r := router.New[*AppContext]()
//	r.Get("/", func(ctx *AppContext) handler.Response {
//		return response.HTML("<h1>Secure Server</h1>")
//	})
//
//	// Option 1: Use default secure configuration
//	srv := server.New(":8443",
//		server.WithTLS(server.DefaultTLSConfig()),
//		server.WithLogger(logger),
//	)
//
//	// Option 2: Use strict configuration for high security
//	srv := server.New(":8443",
//		server.WithTLS(server.StrictTLSConfig()),
//		server.WithLogger(logger),
//	)
//
//	// Option 3: Customize with options
//	tlsConfig := server.NewTLSConfig(
//		server.WithTLSCertificate("cert.pem", "key.pem"),
//		server.WithTLSMinVersion(tls.VersionTLS13),
//	)
//	srv := server.New(":8443",
//		server.WithTLS(tlsConfig),
//		server.WithLogger(logger),
//	)
//
//	if err := srv.Run(ctx, r); err != nil {
//		log.Fatal(err)
//	}
//
// # Automatic HTTPS with Let's Encrypt
//
// Use AutoCertServer for automatic certificate management:
//
//	import (
//		"github.com/dmitrymomot/foundation/core/handler"
//		"github.com/dmitrymomot/foundation/core/letsencrypt"
//		"github.com/dmitrymomot/foundation/core/response"
//		"github.com/dmitrymomot/foundation/core/router"
//		"github.com/dmitrymomot/foundation/core/server"
//	)
//
//	type AppContext = router.Context
//
//	// Create certificate manager
//	certManager, err := letsencrypt.NewManager(letsencrypt.Config{
//		Email:   "admin@example.com",
//		CertDir: "/var/cache/certs",
//		Staging: false, // Use true for testing
//	})
//	if err != nil {
//		log.Fatal(err)
//	}
//
//	// Implement domain store
//	type MyDomainStore struct{}
//	func (s *MyDomainStore) GetDomain(ctx context.Context, domain string) (*server.DomainInfo, error) {
//		return &server.DomainInfo{
//			Domain:   domain,
//			TenantID: "tenant-123",
//			Status:   server.StatusActive,
//		}, nil
//	}
//
//	r := router.New[*AppContext]()
//	r.Get("/", func(ctx *AppContext) handler.Response {
//		return response.HTML("<h1>Secure App</h1>")
//	})
//
//	config := &server.AutoCertConfig[*AppContext]{
//		CertManager: certManager,
//		DomainStore: &MyDomainStore{},
//		HTTPAddr:    ":80",
//		HTTPSAddr:   ":443",
//	}
//
//	srv, err := server.NewAutoCertServer(config)
//	if err != nil {
//		log.Fatal(err)
//	}
//
//	if err := srv.Run(ctx, r); err != nil {
//		log.Fatal(err)
//	}
//
// # Custom Status Handlers
//
// Customize status pages for certificate provisioning and failures:
//
//	import "net/http"
//
//	type AppContext = router.Context
//
//	config := &server.AutoCertConfig[*AppContext]{
//		CertManager: certManager,
//		DomainStore: domainStore,
//
//		ProvisioningHandler: func(ctx *AppContext, info *server.DomainInfo) handler.Response {
//			return response.HTML("<h1>Setting up SSL for " + info.Domain + "</h1>")
//		},
//
//		FailedHandler: func(ctx *AppContext, info *server.DomainInfo) handler.Response {
//			return response.HTMLWithStatus(
//				"<h1>Certificate Error</h1><p>" + info.Error + "</p>",
//				http.StatusServiceUnavailable,
//			)
//		},
//
//		NotFoundHandler: func(ctx *AppContext) handler.Response {
//			return response.HTMLWithStatus(
//				"<h1>Domain Not Found</h1>",
//				http.StatusNotFound,
//			)
//		},
//	}
//
// # Graceful Shutdown
//
// Handle graceful shutdown with signal management:
//
//	import (
//		"os"
//		"os/signal"
//		"syscall"
//	)
//
//	type AppContext = router.Context
//
//	func main() {
//		srv := server.New(":8080")
//
//		ctx, cancel := context.WithCancel(context.Background())
//
//		// Handle shutdown signals
//		sigCh := make(chan os.Signal, 1)
//		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
//		go func() {
//			<-sigCh
//			log.Println("Shutdown signal received")
//			cancel()
//		}()
//
//		r := router.New[*AppContext]()
//		r.Get("/", func(ctx *AppContext) handler.Response {
//			return response.Text("Server running")
//		})
//
//		if err := srv.Run(ctx, r); err != nil {
//			log.Printf("Server error: %v", err)
//		}
//		log.Println("Server shutdown complete")
//	}
//
// # Domain Status Management
//
// The AutoCertServer supports three domain statuses for certificate lifecycle management:
//
//   - StatusProvisioning: Certificate is being generated (shows progress page)
//   - StatusActive: Certificate is ready and valid (serves application)
//   - StatusFailed: Certificate generation failed (shows error page)
//
// Domain status is managed through the DomainStore interface:
//
//	type DomainInfo struct {
//		Domain    string        // Fully qualified domain name
//		TenantID  string        // Tenant identifier for multi-tenant domains
//		Status    DomainStatus  // Current status
//		Error     string        // Error message if status is Failed
//		CreatedAt time.Time     // When domain was registered
//	}
//
// # Production Configuration
//
// Example production server configuration with comprehensive settings:
//
//	type AppContext = router.Context
//
//	func main() {
//		logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
//			Level: slog.LevelInfo,
//		}))
//
//		// Use intermediate TLS config for production (broader compatibility)
//		srv := server.New(":8443",
//			server.WithTLS(server.IntermediateTLSConfig()),
//			server.WithLogger(logger),
//			server.WithShutdownTimeout(30*time.Second),
//		)
//
//		r := router.New[*AppContext]()
//		r.Get("/health", func(ctx *AppContext) handler.Response {
//			return response.JSON(map[string]string{"status": "healthy"})
//		})
//
//		ctx, cancel := context.WithCancel(context.Background())
//		defer cancel()
//
//		if err := srv.Run(ctx, r); err != nil {
//			logger.Error("Server failed", "error", err)
//			os.Exit(1)
//		}
//	}
//
// # Defaults
//
// Production-ready defaults for both server types:
//
//   - Read/Write timeout: 15 seconds
//   - Idle timeout: 60 seconds
//   - Shutdown timeout: 30 seconds
//   - Logger: slog.Default()
//   - AutoCertServer addresses: :80 (HTTP), :443 (HTTPS)
//
// # Interfaces
//
// AutoCertServer requires:
//
//   - CertificateManager: Certificate operations and ACME challenges
//   - DomainStore: Domain lookups and validation
//
// Both servers are thread-safe and handle errors gracefully including
// port conflicts, invalid configuration, TLS errors, and shutdown timeouts.
package server
