// Package server provides a robust HTTP server implementation with graceful shutdown,
// configurable options, and production-ready defaults. It wraps the standard http.Server
// with enhanced functionality for reliable web applications.
//
// # Key Features
//
//   - Graceful shutdown with configurable timeout
//   - TLS/HTTPS support with custom configuration
//   - Thread-safe concurrent access protection
//   - Structured logging integration
//   - Production-ready default timeouts
//   - Simple configuration via functional options
//
// # Basic Usage
//
// Create and run a server with default configuration:
//
//	import (
//		"context"
//		"net/http"
//		"github.com/dmitrymomot/foundation/core/server"
//	)
//
//	func main() {
//		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
//			w.Write([]byte("Hello, World!"))
//		})
//
//		ctx := context.Background()
//		if err := server.Run(ctx, ":8080", handler); err != nil {
//			log.Fatal(err)
//		}
//	}
//
// # Server Configuration
//
// Configure server with custom options:
//
//	srv := server.New(":8080",
//		server.WithShutdownTimeout(60*time.Second),
//		server.WithLogger(slog.New(slog.NewJSONHandler(os.Stdout, nil))),
//	)
//
//	ctx := context.Background()
//	if err := srv.Run(ctx, handler); err != nil {
//		log.Fatal(err)
//	}
//
// # TLS/HTTPS Configuration
//
// Enable HTTPS with custom TLS configuration:
//
//	tlsConfig := &tls.Config{
//		MinVersion: tls.VersionTLS12,
//		CipherSuites: []uint16{
//			tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
//			tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,
//		},
//	}
//
//	srv := server.New(":8443",
//		server.WithTLS(tlsConfig),
//		server.WithLogger(logger),
//	)
//
//	if err := srv.Run(ctx, handler); err != nil {
//		log.Fatal(err)
//	}
//
// # Graceful Shutdown
//
// Handle graceful shutdown with signal management:
//
//	func main() {
//		srv := server.New(":8080")
//
//		// Create cancellable context
//		ctx, cancel := context.WithCancel(context.Background())
//
//		// Handle shutdown signals
//		sigCh := make(chan os.Signal, 1)
//		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
//
//		go func() {
//			<-sigCh
//			log.Println("Shutdown signal received")
//			cancel() // Triggers graceful shutdown
//		}()
//
//		// Run server - blocks until context is canceled
//		if err := srv.Run(ctx, handler); err != nil {
//			log.Printf("Server error: %v", err)
//		}
//
//		log.Println("Server shutdown complete")
//	}
//
// # Production Configuration
//
// Example production server configuration:
//
//	import (
//		"crypto/tls"
//		"log/slog"
//		"os"
//		"time"
//	)
//
//	func main() {
//		// Production logger
//		logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
//			Level: slog.LevelInfo,
//		}))
//
//		// TLS configuration
//		tlsConfig := &tls.Config{
//			MinVersion:               tls.VersionTLS12,
//			PreferServerCipherSuites: true,
//		}
//
//		srv := server.New(":8443",
//			server.WithTLS(tlsConfig),
//			server.WithLogger(logger),
//			server.WithShutdownTimeout(30*time.Second),
//		)
//
//		ctx, cancel := context.WithCancel(context.Background())
//		defer cancel()
//
//		// Setup signal handling for graceful shutdown
//		setupGracefulShutdown(cancel)
//
//		if err := srv.Run(ctx, myHandler); err != nil {
//			logger.Error("Server failed", "error", err)
//			os.Exit(1)
//		}
//	}
//
// # Server Defaults
//
// The server includes production-ready defaults:
//
//   - ReadTimeout: 15 seconds
//   - WriteTimeout: 15 seconds
//   - IdleTimeout: 60 seconds
//   - MaxHeaderBytes: http.DefaultMaxHeaderBytes (1MB)
//   - Graceful shutdown timeout: 30 seconds
//   - Logger: slog.Default()
//
// # Thread Safety
//
// The Server type is safe for concurrent use. All methods properly synchronize
// access to internal state using read-write mutexes.
//
// # Error Handling
//
// The server handles various error conditions:
//
//   - Port already in use
//   - Invalid addresses
//   - TLS certificate errors
//   - Graceful shutdown timeouts
//   - Multiple Run() calls on the same server instance
//
// All errors are properly logged and returned to the caller for handling.
package server
