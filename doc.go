// Package foundation provides a comprehensive toolkit for building secure, scalable web applications
// with a focus on B2B micro-SaaS development. The library implements modern Go patterns including
// generics for type safety, functional options for configuration, and interface-based design for
// flexibility and testability.
//
// # LLM Assistant Note
//
// This file serves as a comprehensive index of all packages in the foundation library,
// designed to help LLMs understand the complete codebase structure and functionality.
// Each package entry includes the full import path and a concise description of its purpose.
//
// # Package Organization
//
// The foundation library is organized into four main categories:
//
//   - Core: Framework components for building web applications
//   - Middleware: HTTP middleware for cross-cutting concerns
//   - Utilities: Standalone packages for common functionality
//   - Integrations: Database, email, and storage service implementations
//
// # Getting Documentation
//
// For detailed documentation on any package, use the go doc command:
//
//	go doc github.com/dmitrymomot/foundation/core/binder
//	go doc -all github.com/dmitrymomot/foundation/middleware
//
// # Core Framework Packages
//
// These packages provide the fundamental building blocks for web applications:
//
//	github.com/dmitrymomot/foundation/core/binder        - HTTP request data binding with validation
//	github.com/dmitrymomot/foundation/core/cache         - Thread-safe LRU cache implementation
//	github.com/dmitrymomot/foundation/core/command       - CQRS command pattern with handlers and message bus
//	github.com/dmitrymomot/foundation/core/config        - Type-safe environment variable loading
//	github.com/dmitrymomot/foundation/core/cookie        - Secure HTTP cookie management with encryption
//	github.com/dmitrymomot/foundation/core/email         - Email sending interface with template support
//	github.com/dmitrymomot/foundation/core/email/templates - Email template rendering using templ
//	github.com/dmitrymomot/foundation/core/email/templates/components - Reusable email template components
//	github.com/dmitrymomot/foundation/core/event         - Event-driven architecture with type-safe handlers
//	github.com/dmitrymomot/foundation/core/handler       - Type-safe HTTP handler abstractions
//	github.com/dmitrymomot/foundation/core/health        - HTTP handlers for service health monitoring
//	github.com/dmitrymomot/foundation/core/i18n          - Internationalization with CLDR plural rules
//	github.com/dmitrymomot/foundation/core/letsencrypt   - Let's Encrypt certificate management with explicit control
//	github.com/dmitrymomot/foundation/core/logger        - Structured logging built on slog
//	github.com/dmitrymomot/foundation/core/queue         - Job queue system with workers and scheduling
//	github.com/dmitrymomot/foundation/core/response      - HTTP response utilities (JSON, HTML, SSE, WebSocket)
//	github.com/dmitrymomot/foundation/core/router        - High-performance HTTP router with middleware
//	github.com/dmitrymomot/foundation/core/sanitizer     - Input sanitization and data cleaning
//	github.com/dmitrymomot/foundation/core/server        - HTTP server with graceful shutdown
//	github.com/dmitrymomot/foundation/core/session       - Generic session management system
//	github.com/dmitrymomot/foundation/core/sessiontransport - Session transport implementations (cookie, JWT)
//	github.com/dmitrymomot/foundation/core/static        - Handlers for serving static files, directories, and SPAs
//	github.com/dmitrymomot/foundation/core/storage       - Local filesystem storage with security features
//	github.com/dmitrymomot/foundation/core/validator     - Rule-based data validation system
//
// # HTTP Middleware Packages
//
// Pre-built middleware components for common cross-cutting concerns:
//
//	github.com/dmitrymomot/foundation/middleware         - CORS, JWT auth, rate limiting, security headers, logging
//
// # Utility Packages
//
// Standalone packages providing specific functionality:
//
//	github.com/dmitrymomot/foundation/pkg/async          - Asynchronous programming utilities with Future pattern
//	github.com/dmitrymomot/foundation/pkg/broadcast      - Generic pub/sub messaging system
//	github.com/dmitrymomot/foundation/pkg/clientip       - Real client IP extraction from HTTP requests
//	github.com/dmitrymomot/foundation/pkg/feature        - Feature flagging system with rollout strategies
//	github.com/dmitrymomot/foundation/pkg/fingerprint    - Device fingerprint generation for session validation
//	github.com/dmitrymomot/foundation/pkg/jwt            - RFC 7519 JSON Web Token implementation
//	github.com/dmitrymomot/foundation/pkg/qrcode         - QR code generation utilities
//	github.com/dmitrymomot/foundation/pkg/randomname     - Human-readable random name generation
//	github.com/dmitrymomot/foundation/pkg/ratelimiter    - Token bucket rate limiting with pluggable storage
//	github.com/dmitrymomot/foundation/pkg/secrets        - AES-256-GCM encryption with compound key derivation
//	github.com/dmitrymomot/foundation/pkg/slug           - URL-safe slug generation with Unicode normalization
//	github.com/dmitrymomot/foundation/pkg/token          - Compact URL-safe token generation with HMAC signatures
//	github.com/dmitrymomot/foundation/pkg/totp           - RFC 6238 TOTP authentication with encrypted secrets
//	github.com/dmitrymomot/foundation/pkg/useragent      - User-Agent parsing for browser and device detection
//	github.com/dmitrymomot/foundation/pkg/vectorizer     - Text to vector embeddings using AI providers (OpenAI, Google AI)
//	github.com/dmitrymomot/foundation/pkg/webhook        - Reliable HTTP webhook delivery with retries
//
// # Integration Packages
//
// Production-ready integrations for databases, email services, and storage:
//
//	github.com/dmitrymomot/foundation/integration/database/mongo      - MongoDB client with health checking
//	github.com/dmitrymomot/foundation/integration/database/opensearch - OpenSearch client initialization
//	github.com/dmitrymomot/foundation/integration/database/pg         - PostgreSQL with migrations and pooling
//	github.com/dmitrymomot/foundation/integration/database/redis      - Redis client with retry logic
//	github.com/dmitrymomot/foundation/integration/email/postmark      - Postmark email service integration
//	github.com/dmitrymomot/foundation/integration/email/smtp          - SMTP email sending implementation
//	github.com/dmitrymomot/foundation/integration/storage/s3          - S3-compatible storage implementation
//
// # Architecture Patterns
//
// The foundation library follows these key architectural patterns:
//
//   - Generics for type safety with custom context types
//   - Functional options for flexible configuration
//   - Interface-based design for testability and modularity
//   - Security-first approach with built-in sanitization and validation
//   - Multi-tenant considerations throughout the design
//
// # Example Usage
//
//	import (
//		"context"
//		"log"
//
//		"github.com/dmitrymomot/foundation/core/handler"
//		"github.com/dmitrymomot/foundation/core/response"
//		"github.com/dmitrymomot/foundation/core/router"
//		"github.com/dmitrymomot/foundation/core/server"
//		"github.com/dmitrymomot/foundation/middleware"
//	)
//
//	func main() {
//		// Create router with router.Context
//		r := router.New[*router.Context]()
//
//		// Add middleware
//		r.Use(middleware.CORS[*router.Context]())
//		r.Use(middleware.RequestID[*router.Context]())
//		r.Use(middleware.Logging[*router.Context]())
//
//		// Define handlers that return Response functions
//		r.Get("/", func(ctx *router.Context) handler.Response {
//			return response.JSON(map[string]string{"status": "ok"})
//		})
//
//		r.Get("/users/{id}", func(ctx *router.Context) handler.Response {
//			userID := ctx.Param("id")
//			return response.JSON(map[string]string{
//				"user_id": userID,
//				"message": "User found",
//			})
//		})
//
//		// Create and run server
//		ctx := context.Background()
//		if err := server.Run(ctx, ":8080", r); err != nil {
//			log.Fatal(err)
//		}
//	}
//
// For complete examples and detailed usage instructions, refer to the individual
// package documentation using the go doc command.
package foundation
