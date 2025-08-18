# gokit

[![Ask DeepWiki](https://deepwiki.com/badge.svg)](https://deepwiki.com/dmitrymomot/gokit)
[![GitHub tag (latest SemVer)](https://img.shields.io/github/tag/dmitrymomot/gokit)](https://github.com/dmitrymomot/gokit/tags)
[![Go Reference](https://pkg.go.dev/badge/github.com/dmitrymomot/gokit.svg)](https://pkg.go.dev/github.com/dmitrymomot/gokit)
[![License](https://img.shields.io/github/license/dmitrymomot/gokit)](https://github.com/dmitrymomot/gokit/blob/main/LICENSE)

[![Tests](https://github.com/dmitrymomot/gokit/actions/workflows/tests.yml/badge.svg)](https://github.com/dmitrymomot/gokit/actions/workflows/tests.yml)
[![CodeQL Analysis](https://github.com/dmitrymomot/gokit/actions/workflows/codeql.yml/badge.svg)](https://github.com/dmitrymomot/gokit/actions/workflows/codeql.yml)
[![GolangCI Lint](https://github.com/dmitrymomot/gokit/actions/workflows/golangci-lint.yml/badge.svg)](https://github.com/dmitrymomot/gokit/actions/workflows/golangci-lint.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/dmitrymomot/gokit)](https://goreportcard.com/report/github.com/dmitrymomot/gokit)

## Architecture & Design Decisions

### Package Organization

The framework follows a **modular single-package design** with logical separation:

```
gokit/
├── handler/             # Handler interfaces & utilities
│   ├── context.go       # Generic context interfaces
│   └── handler.go       # Core handler definitions
│
├── router/              # Routing logic
│   ├── mux.go           # Router implementation
│   ├── chain.go         # Middleware chaining
│   └── context.go       # Router-specific context
│
├── httpserver/          # Server management
│   ├── server.go        # HTTP server configuration
│   └── options.go       # Server initialization options
│
├── response/            # Response implementations
│   ├── base.go         # Basic responses
│   ├── json.go         # JSON responses
│   ├── sse.go         # Server-sent events
│   └── ...            # Other response types
│
└── (root)              # Core package
    ├── gokit.go        # Package-level helpers
    └── errors.go       # Global error types
```

### Key Design Principles

1. **Type-Safe Generic Design**: Everything revolves around `[C contexter]` type parameter for compile-time safety
2. **Response Pattern**: Handlers return `Response` interface, not `error` - this separates business logic from HTTP details
3. **Tight Integration**: Router, context, and responses are deliberately coupled for type safety
4. **No Import Cycles**: Response package defines its own interface, compatible with core
5. **Pragmatic Over Pure**: Single package when it makes sense, avoiding over-engineering

### Why Not Fully Modular?

Unlike Fiber's plugin architecture, GoKit maintains a **tightly integrated core** because:
- The generic type system `[C contexter]` creates natural coupling
- The `Error` type implements both `error` and `Response` interfaces
- Internal router is optimized for the specific handler signature
- Simpler imports and better performance with single package

### Response Interface Philosophy

```go
// Instead of traditional:
func handler(w http.ResponseWriter, r *http.Request) {
    json.NewEncoder(w).Encode(data) // Handler knows HTTP details
}

// GoKit pattern:
func handler(ctx *Context) Response {
    return response.JSON(data) // Handler returns WHAT, framework handles HOW
}
```

This provides:
- **Testability**: Easy to test Response objects without HTTP
- **Composability**: Can wrap/decorate responses
- **Clarity**: Handler returns business result, not HTTP details
- **Type safety**: Compiler ensures valid responses

## To-Do

- [x] Rework go-chi to change handler interface from `func(http.ResponseWriter, *http.Request)` => `func(Context) Response`
- [x] ability to set custom context type via generics
- [x] extend default context with helpers for different default types of responses and request parsers
- [x] router new function must be configurable via options pattern
- [ ] robust default configuration, especially for my personal purpose
- [ ] default middlewares with new interface
- [ ] default http server with graceful shutdown
- [ ] default handler for files uploading with customizable storage provider
- [ ] default SSE and WebSocket support
- [ ] add useful helper methods to default Context implementation
- [ ] add ability to setup binder

## Additional Response Types (Planned)

### Template Responses

- `Template(name string, data any)` - Render HTML templates
- `TemplateString(tmpl string, data any)` - Inline template rendering

### Partial Content

- `PartialContent(content []byte, start, end, total int64)` - For range requests (206 status)

### WebSocket Upgrade

- `WebSocketUpgrade(handler func(conn *websocket.Conn))` - Handle WebSocket connections
