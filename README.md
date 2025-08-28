# foundation

A comprehensive toolkit for building secure, scalable web applications with a focus on B2B micro-SaaS development. The library implements modern Go patterns including generics for type safety, functional options for configuration, and interface-based design for flexibility and testability.

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

## Features

The foundation library is organized into four main categories, providing everything needed to build production-ready web applications:

### Core Framework (18 packages)

Essential building blocks for web applications:

- **Request Handling**: HTTP request data binding with validation (`core/binder`)
- **Routing**: High-performance HTTP router with middleware support (`core/router`)
- **Type-Safe Handlers**: Generic handler abstractions for better type safety (`core/handler`)
- **Response Utilities**: JSON, HTML, SSE, and WebSocket response helpers (`core/response`)
- **Server Management**: HTTP server with graceful shutdown (`core/server`)
- **Session Management**: Generic session system with pluggable transports (`core/session`, `core/sessiontransport`)
- **Configuration**: Type-safe environment variable loading (`core/config`)
- **Security**: Input sanitization, secure cookies, data validation (`core/sanitizer`, `core/cookie`, `core/validator`)
- **Storage & Caching**: Local filesystem storage and thread-safe LRU cache (`core/storage`, `core/cache`)
- **Background Processing**: Job queue system with workers and scheduling (`core/queue`)
- **Utilities**: Structured logging, internationalization, email interface (`core/logger`, `core/i18n`, `core/email`)

### HTTP Middleware

Pre-built middleware components for common cross-cutting concerns:

- **Security**: CORS, JWT authentication, security headers
- **Observability**: Request logging, request ID tracking
- **Performance**: Rate limiting, request timeout handling
- **Development**: Debug utilities, request/response debugging

### Utilities (15 packages)

Standalone packages providing specific functionality:

- **Security**: JWT tokens (`pkg/jwt`), TOTP authentication (`pkg/totp`), AES encryption (`pkg/secrets`)
- **Rate Limiting**: Token bucket implementation with pluggable storage (`pkg/ratelimiter`)
- **Async Programming**: Future pattern utilities (`pkg/async`)
- **Communication**: Pub/sub messaging system (`pkg/broadcast`), webhook delivery (`pkg/webhook`)
- **Utilities**: QR code generation (`pkg/qrcode`), URL-safe slugs (`pkg/slug`), random names (`pkg/randomname`)
- **Web Features**: Client IP extraction (`pkg/clientip`), User-Agent parsing (`pkg/useragent`)
- **Development**: Device fingerprinting (`pkg/fingerprint`), feature flags (`pkg/feature`), secure tokens (`pkg/token`)

### Integrations (7 packages)

Production-ready integrations for databases, email services, and storage:

- **Databases**: PostgreSQL with migrations (`integration/database/pg`), MongoDB (`integration/database/mongo`), Redis (`integration/database/redis`), OpenSearch (`integration/database/opensearch`)
- **Email Services**: Postmark integration (`integration/email/postmark`), SMTP support (`integration/email/smtp`)
- **Storage**: S3-compatible storage implementation (`integration/storage/s3`)

## Quick Start

```go
package main

import (
	"context"
	"log"

	"github.com/dmitrymomot/foundation/core/handler"
	"github.com/dmitrymomot/foundation/core/response"
	"github.com/dmitrymomot/foundation/core/router"
	"github.com/dmitrymomot/foundation/core/server"
	"github.com/dmitrymomot/foundation/middleware"
)

func main() {
	// Create router with router.Context
	r := router.New[*router.Context]()

	// Add middleware
	r.Use(middleware.CORS[*router.Context]())
	r.Use(middleware.RequestID[*router.Context]())
	r.Use(middleware.Logging[*router.Context]())

	// Define handlers that return Response functions
	r.Get("/", func(ctx *router.Context) handler.Response {
		return response.JSON(map[string]string{"status": "ok"})
	})

	r.Get("/users/{id}", func(ctx *router.Context) handler.Response {
		userID := ctx.Param("id")
		return response.JSON(map[string]string{
			"user_id": userID,
			"message": "User found",
		})
	})

	// Create and run server
	ctx := context.Background()
	if err := server.Run(ctx, ":8080", r); err != nil {
		log.Fatal(err)
	}
}
```

## Architecture Patterns

The foundation library follows these key architectural patterns:

- **Generics for type safety** with custom context types
- **Functional options** for flexible configuration
- **Interface-based design** for testability and modularity
- **Security-first approach** with built-in sanitization and validation
- **Multi-tenant considerations** throughout the design

## Documentation

For detailed documentation on any package, use the go doc command:

```bash
go doc github.com/dmitrymomot/foundation/core/binder
go doc -all github.com/dmitrymomot/foundation/middleware
```

Each package contains comprehensive documentation with usage examples and detailed API references.

## Requirements

- Go 1.24 or later

## License

Licensed under the Apache License, Version 2.0. See [LICENSE](LICENSE) for the full license text.

## Contributing

Contributions are welcome! This is an actively developed library, and breaking changes are allowed as we work towards a stable API.