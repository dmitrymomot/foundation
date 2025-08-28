// Package server provides HTTP server configuration and lifecycle management
// with graceful shutdown, middleware support, and comprehensive options for
// production deployment. It wraps the standard http.Server with enhanced
// functionality for robust web applications.
//
// # Features
//
//   - Graceful shutdown with proper cleanup
//   - Configurable timeouts and limits
//   - TLS support with automatic certificate management
//   - Middleware integration
//   - Health check endpoints
//   - Request logging and metrics
//   - Signal handling for clean shutdown
//   - Development and production configurations
//
// # Basic Usage
//
// Create and start an HTTP server with default configuration:
//
//	import "github.com/dmitrymomot/foundation/core/server"
//
//	// Create server with router
//	srv := server.New(
//		server.WithAddress(":8080"),
//		server.WithHandler(router),
//		server.WithReadTimeout(30*time.Second),
//		server.WithWriteTimeout(30*time.Second),
//	)
//
//	// Start server
//	if err := srv.ListenAndServe(); err != nil {
//		log.Fatal("Server failed:", err)
//	}
//
// # Server Configuration
//
// Configure server with various options:
//
//	srv := server.New(
//		server.WithAddress("0.0.0.0:8080"),
//		server.WithReadTimeout(15*time.Second),
//		server.WithWriteTimeout(15*time.Second),
//		server.WithIdleTimeout(60*time.Second),
//		server.WithMaxHeaderBytes(1<<20), // 1MB
//		server.WithHandler(myRouter),
//	)
//
// # TLS Configuration
//
// Enable HTTPS with TLS:
//
//	// Basic TLS
//	srv := server.New(
//		server.WithAddress(":8443"),
//		server.WithTLS("cert.pem", "key.pem"),
//		server.WithHandler(router),
//	)
//
//	// TLS with custom config
//	tlsConfig := &tls.Config{
//		MinVersion: tls.VersionTLS12,
//		CipherSuites: []uint16{
//			tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
//			tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
//		},
//	}
//
//	srv := server.New(
//		server.WithAddress(":8443"),
//		server.WithTLSConfig(tlsConfig),
//		server.WithHandler(router),
//	)
//
// # Graceful Shutdown
//
// Handle graceful shutdown with signal management:
//
//	func main() {
//		srv := server.New(
//			server.WithAddress(":8080"),
//			server.WithHandler(router),
//		)
//
//		// Start server in background
//		go func() {
//			if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
//				log.Fatal("Server failed:", err)
//			}
//		}()
//
//		// Wait for shutdown signal
//		quit := make(chan os.Signal, 1)
//		signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
//		<-quit
//
//		log.Println("Shutting down server...")
//
//		// Graceful shutdown with timeout
//		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
//		defer cancel()
//
//		if err := srv.Shutdown(ctx); err != nil {
//			log.Fatal("Server shutdown failed:", err)
//		}
//
//		log.Println("Server stopped")
//	}
//
// # Health Checks
//
// Add health check endpoints:
//
//	srv := server.New(
//		server.WithAddress(":8080"),
//		server.WithHandler(router),
//		server.WithHealthCheck("/health", healthCheckHandler),
//	)
//
//	func healthCheckHandler(w http.ResponseWriter, r *http.Request) {
//		health := map[string]string{
//			"status":    "ok",
//			"timestamp": time.Now().Format(time.RFC3339),
//			"version":   "1.0.0",
//		}
//
//		w.Header().Set("Content-Type", "application/json")
//		json.NewEncoder(w).Encode(health)
//	}
//
// # Middleware Integration
//
// Add middleware for logging, CORS, etc.:
//
//	srv := server.New(
//		server.WithAddress(":8080"),
//		server.WithHandler(router),
//		server.WithMiddleware(
//			loggingMiddleware,
//			corsMiddleware,
//			authMiddleware,
//		),
//	)
//
//	func loggingMiddleware(next http.Handler) http.Handler {
//		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
//			start := time.Now()
//			next.ServeHTTP(w, r)
//			log.Printf("%s %s - %v", r.Method, r.URL.Path, time.Since(start))
//		})
//	}
//
// # Production Configuration
//
// Configure server for production deployment:
//
//	srv := server.New(
//		server.WithAddress(":8080"),
//		server.WithHandler(router),
//		server.WithReadTimeout(10*time.Second),
//		server.WithWriteTimeout(10*time.Second),
//		server.WithIdleTimeout(120*time.Second),
//		server.WithMaxHeaderBytes(1<<20),
//		server.WithTLS("cert.pem", "key.pem"),
//		server.WithMiddleware(
//			securityHeadersMiddleware,
//			rateLimitMiddleware,
//			loggingMiddleware,
//		),
//	)
//
// # Development Configuration
//
// Configure server for development:
//
//	srv := server.New(
//		server.WithAddress("localhost:8080"),
//		server.WithHandler(router),
//		server.WithReadTimeout(30*time.Second),
//		server.WithWriteTimeout(30*time.Second),
//		server.WithMiddleware(
//			corsMiddleware,
//			debugMiddleware,
//		),
//	)
//
// # Custom Error Handling
//
// Handle server errors:
//
//	srv := server.New(
//		server.WithAddress(":8080"),
//		server.WithHandler(router),
//		server.WithErrorHandler(func(err error) {
//			log.Printf("Server error: %v", err)
//			// Send to error tracking service
//			errorTracker.Report(err)
//		}),
//	)
//
// # Best Practices
//
//   - Set appropriate timeouts for your use case
//   - Use TLS for production deployments
//   - Implement proper graceful shutdown
//   - Add health check endpoints for monitoring
//   - Use middleware for cross-cutting concerns
//   - Configure limits to prevent resource exhaustion
//   - Log server events for debugging and monitoring
//   - Handle signals for clean shutdown
//   - Test server configuration under load
//   - Monitor server metrics in production
package server
