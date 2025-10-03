# foundation

A comprehensive toolkit for building secure, scalable web applications in Go. The library implements modern patterns including generics for type safety, functional options for configuration, and interface-based design for flexibility and testability.

[![Ask DeepWiki](https://deepwiki.com/badge.svg)](https://deepwiki.com/dmitrymomot/foundation)
[![GitHub tag (latest SemVer)](https://img.shields.io/github/tag/dmitrymomot/foundation)](https://github.com/dmitrymomot/foundation/tags)
[![Go Reference](https://pkg.go.dev/badge/github.com/dmitrymomot/foundation.svg)](https://pkg.go.dev/github.com/dmitrymomot/foundation)
[![License](https://img.shields.io/github/license/dmitrymomot/foundation)](https://github.com/dmitrymomot/foundation/blob/main/LICENSE)

[![Tests](https://github.com/dmitrymomot/foundation/actions/workflows/tests.yml/badge.svg)](https://github.com/dmitrymomot/foundation/actions/workflows/tests.yml)
[![CodeQL Analysis](https://github.com/dmitrymomot/foundation/actions/workflows/codeql.yml/badge.svg)](https://github.com/dmitrymomot/foundation/actions/workflows/codeql.yml)
[![GolangCI Lint](https://github.com/dmitrymomot/foundation/actions/workflows/golangci-lint.yml/badge.svg)](https://github.com/dmitrymomot/foundation/actions/workflows/golangci-lint.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/dmitrymomot/foundation)](https://goreportcard.com/report/github.com/dmitrymomot/foundation)

## Installation

```bash
go get github.com/dmitrymomot/foundation
```

## Why Foundation Exists

After building multiple Go web applications, I got tired of doing the same work over and over - copy-pasting request handlers between projects, creating huge boilerplate files to work around framework limitations, reimplementing session management for the third time. Each new project meant another week of setup before writing actual business logic.

Foundation is my solution: all the repetitive code I kept rewriting, collected into reusable packages. It's designed to speed up my own project delivery by providing the pieces I always need - type-safe routing with generics, session management, request validation, background jobs - without the ceremony.

No framework lock-in, no magic. Just composable packages that solve common problems so I can focus on building features instead of reinventing infrastructure.

## Quick Start

```go
package main

import (
	"context"
	"database/sql"
	"log"

	"github.com/dmitrymomot/foundation/core/handler"
	"github.com/dmitrymomot/foundation/core/response"
	"github.com/dmitrymomot/foundation/core/router"
	"github.com/dmitrymomot/foundation/core/server"
	"github.com/dmitrymomot/foundation/middleware"
)

// Define your custom context with exactly what you need
type AppContext struct {
	*router.Context
	DB       *sql.DB
	UserID   string
	TenantID string
}

func main() {
	r := router.New[*AppContext]()

	// Add middleware
	r.Use(middleware.CORS[*AppContext]())
	r.Use(middleware.RequestID[*AppContext]())
	r.Use(middleware.Logging[*AppContext]())

	// Type-safe handlers - no casting needed
	r.Get("/", func(ctx *AppContext) handler.Response {
		return response.JSON(map[string]string{
			"status":   "ok",
			"user_id":  ctx.UserID,
			"tenant":   ctx.TenantID,
		})
	})

	r.Get("/users/{id}", func(ctx *AppContext) handler.Response {
		userID := ctx.Param("id")
		// ctx.DB is available with full type safety
		return response.JSON(map[string]string{
			"user_id": userID,
			"message": "User found",
		})
	})

	// Create and run server with graceful shutdown
	ctx := context.Background()
	if err := server.Run(ctx, ":8080", r); err != nil {
		log.Fatal(err)
	}
}
```

## Common Patterns

**Multi-tenant SaaS**: Session management with tenant isolation, rate limiting per tenant, JWT authentication with tenant claims
**Background Processing**: Queue jobs, schedule tasks, process webhooks with retries, CQRS command/event patterns
**Security**: Input sanitization, TOTP 2FA, AES encryption, secure token generation, device fingerprinting
**Observability**: Structured logging with slog, request ID tracking, health check endpoints

## Features

The foundation library is organized into four main categories, providing everything needed to build production-ready web applications:

### Core Framework (22 packages)

**Request & Response**

- HTTP request data binding with validation (`core/binder`)
- Multiple response formats: JSON, HTML, SSE, WebSocket (`core/response`)
- Secure cookie management with encryption (`core/cookie`)
- Type-safe handler abstractions with generics (`core/handler`)

**Routing & Server**

- High-performance HTTP router with middleware support (`core/router`)
- HTTP server with graceful shutdown (`core/server`)
- Static file serving with SPA support (`core/static`)
- Let's Encrypt certificate management (`core/letsencrypt`)

**State Management**

- Generic session system with pluggable transports (`core/session`, `core/sessiontransport`)
- Thread-safe LRU cache implementation (`core/cache`)
- Local filesystem storage with security features (`core/storage`)

**Background Work & Architecture**

- Job queue system with workers and scheduling (`core/queue`)
- CQRS command pattern with handlers and message bus (`core/command`)
- Event-driven architecture with type-safe handlers (`core/event`)

**Security & Validation**

- Input sanitization and data cleaning (`core/sanitizer`)
- Rule-based data validation system (`core/validator`)

**Operations & Configuration**

- Type-safe environment variable loading (`core/config`)
- Structured logging built on slog (`core/logger`)
- Health monitoring endpoints (`core/health`)
- Internationalization with CLDR plural rules (`core/i18n`)
- Email sending interface with template support (`core/email`)

### HTTP Middleware

Pre-built middleware components for common cross-cutting concerns:

- **Security**: CORS, JWT authentication, security headers
- **Observability**: Request logging, request ID tracking
- **Performance**: Rate limiting, request timeout handling
- **Development**: Debug utilities, request/response debugging

### Utilities (16 packages)

Standalone packages providing specific functionality:

- **Security**: JWT tokens (`pkg/jwt`), TOTP authentication (`pkg/totp`), AES encryption (`pkg/secrets`), secure token generation (`pkg/token`)
- **Rate Limiting**: Token bucket implementation with pluggable storage (`pkg/ratelimiter`)
- **Async Programming**: Future pattern utilities (`pkg/async`)
- **Communication**: Pub/sub messaging system (`pkg/broadcast`), webhook delivery with retries (`pkg/webhook`)
- **AI & ML**: Text to vector embeddings using OpenAI and Google AI (`pkg/vectorizer`)
- **Web Utilities**: Client IP extraction (`pkg/clientip`), User-Agent parsing (`pkg/useragent`), device fingerprinting (`pkg/fingerprint`)
- **Content Generation**: QR code generation (`pkg/qrcode`), URL-safe slugs (`pkg/slug`), random name generation (`pkg/randomname`)
- **Feature Management**: Feature flagging with rollout strategies (`pkg/feature`)

### Integrations (7 packages)

Production-ready integrations for databases, email services, and storage:

- **Databases**: PostgreSQL with migrations and connection pooling (`integration/database/pg`), MongoDB with health checking (`integration/database/mongo`), Redis with retry logic (`integration/database/redis`), OpenSearch client (`integration/database/opensearch`)
- **Email Services**: Postmark API integration (`integration/email/postmark`), SMTP sending (`integration/email/smtp`)
- **Storage**: S3-compatible object storage (`integration/storage/s3`)

## Architecture Patterns

The foundation library follows these key architectural patterns:

- **Generics for type safety**: Custom context types eliminate runtime type assertions
- **Functional options**: Flexible configuration without breaking changes
- **Interface-based design**: Pluggable implementations for testing and modularity
- **Security-first approach**: Built-in sanitization, validation, and encryption
- **Multi-tenant considerations**: Tenant isolation patterns throughout the design

## Documentation

For detailed documentation on any package, use the go doc command:

```bash
go doc github.com/dmitrymomot/foundation/core/binder
go doc -all github.com/dmitrymomot/foundation/middleware
```

Each package contains comprehensive documentation with usage examples and detailed API references.

## Requirements

- Go 1.24 or later

## Status

Active development with breaking changes allowed as we work towards v1.0. Production use is at your own discretion - API stability not yet guaranteed.

## License

Licensed under the Apache License, Version 2.0. See [LICENSE](LICENSE) for the full license text.

## Contributing

Contributions are welcome! This is an actively developed library, and breaking changes are allowed as we work towards a stable API.
