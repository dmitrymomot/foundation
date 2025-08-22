# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Documentation

**Every package has comprehensive documentation accessible via `go doc`:**

```bash
go doc github.com/dmitrymomot/gokit/core/session
go doc github.com/dmitrymomot/gokit/middleware
go doc -all github.com/dmitrymomot/gokit/core/binder  # Full docs
```

Each package contains a `doc.go` file with detailed usage examples and robust inline comments.

## Build and Test Commands

```bash
# Run tests with race detection
go test -p 1 -count=1 -race -cover ./...

# Run specific test
go test -run TestName ./path/to/package

# Format and check
go fmt ./...
goimports -w -local github.com/dmitrymomot/gokit .
go vet ./...
```

## Architecture Overview

Go toolkit for building secure web applications with B2B micro-SaaS focus.

- **Core** (`/core`): Framework components using Go generics for type-safe handlers, routers, sessions, request binding, validation, and response handling.
- **Middleware** (`/middleware`): Pre-built HTTP middleware for CORS, JWT auth, rate limiting, security headers, logging.
- **Utilities** (`/pkg`): Standalone packages for JWT, TOTP/2FA, webhooks, async tasks, rate limiting.
- **Integrations** (`/integration`): Database migrations, email (Postmark), S3-compatible storage.

## Key Patterns

- **Generics for type safety**: Custom context types `HandlerFunc[C Context]`
- **Functional options**: `WithStore()`, `WithTransport()` configuration
- **Interface-based**: Pluggable Store, Transport, Router implementations
- **Security-first**: Built-in sanitization, validation, rate limiting

## Testing

- Black-box testing with `package_test` naming convention
- Use `testify` for assertions and mocks

## Stack

- templ + TailwindCSS + HTMX (no JS frameworks)
- PostgreSQL + sqlc
- Multi-tenant B2B SaaS focus
- Active development - breaking changes allowed
