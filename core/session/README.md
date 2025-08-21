# Session Package

A lightweight, pluggable session management package for Go applications that supports multiple transport mechanisms (cookies, headers, JWT) and storage backends.

## Design Philosophy

This package is designed with simplicity and flexibility in mind:

- **Transport Agnostic**: Support cookies for web apps and tokens for APIs
- **Storage Agnostic**: Pluggable storage backends (memory, Redis, database)
- **Minimal Dependencies**: Uses standard library where possible
- **Type Safe**: Leverages Go's type system for compile-time safety
- **Performance Focused**: Zero allocations in hot paths where feasible

## Architecture

```
┌─────────┐      ┌─────────┐      ┌─────────┐
│   Web   │      │   API   │      │   SPA   │
│ Browser │      │ Client  │      │  (JS)   │
└────┬────┘      └────┬────┘      └────┬────┘
     │                │                 │
     │ Cookie         │ Bearer Token   │ Both
     ▼                ▼                 ▼
┌────────────────────────────────────────────┐
│            Transport Layer                 │
├─────────────┬──────────────┬──────────────┤
│   Cookie    │    Header    │     JWT      │
│  Transport  │  Transport   │  Transport   │
└─────────────┴──────────────┴──────────────┘
                     │
                     ▼
┌────────────────────────────────────────────┐
│           Session Manager                  │
│  • Create/Get/Update/Delete               │
│  • Token generation & validation          │
│  • TTL management                         │
└────────────────────────────────────────────┘
                     │
                     ▼
┌────────────────────────────────────────────┐
│            Store Interface                 │
├─────────────┬──────────────┬──────────────┤
│   Memory    │    Redis     │   Database   │
│    Store    │    Store     │    Store     │
└─────────────┴──────────────┴──────────────┘
```

## Core Components

### 1. Session

The session object holds user session data:

```go
type Session struct {
    ID        string                 // Unique session identifier
    UserID    string                 // Associated user (empty for anonymous)
    Data      map[string]interface{} // Session data
    ExpiresAt time.Time              // Expiration timestamp
    CreatedAt time.Time              // Creation timestamp
    UpdatedAt time.Time              // Last update timestamp
}
```

### 2. Transport Interface

Defines how sessions are transmitted between client and server:

```go
type Transport interface {
    // Extract session token from request
    Extract(r *http.Request) (token string, err error)

    // Embed session token in response
    Embed(w http.ResponseWriter, token string, ttl time.Duration) error

    // Clear session token from response
    Clear(w http.ResponseWriter) error
}
```

### 3. Store Interface

Defines session persistence operations:

```go
type Store interface {
    // Get retrieves a session by token
    Get(ctx context.Context, token string) (*Session, error)

    // Set saves or updates a session
    Set(ctx context.Context, session *Session) error

    // Delete removes a session
    Delete(ctx context.Context, token string) error

    // DeleteExpired removes all expired sessions (optional)
    DeleteExpired(ctx context.Context) error
}
```

### 4. Manager

Orchestrates session operations:

```go
type Manager struct {
    store     Store
    transport Transport
    config    Config
}
```

## Transport Implementations

### Cookie Transport

For traditional web applications:

- HttpOnly and Secure flags for security
- SameSite attribute for CSRF protection
- Automatic renewal on activity

### Header Transport

For API clients and mobile apps:

- Bearer token in Authorization header
- Or custom header (e.g., X-Session-Token)
- Stateless operation

### JWT Transport

For distributed systems and microservices:

- Self-contained tokens with claims
- Optional refresh token support
- Signature verification
- No server-side storage needed (stateless)

## Usage Patterns

### 1. Web Application (Cookie-based)

```go
// Initialize with cookie transport
manager := session.New(
    session.WithStore(memoryStore),
    session.WithTransport(session.NewCookieTransport("sid", cookieOptions)),
    session.WithTTL(2 * time.Hour),
)

// HTTP middleware
func SessionMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        sess, _ := manager.LoadOrCreate(r.Context(), w, r)
        ctx := session.NewContext(r.Context(), sess)
        next.ServeHTTP(w, r.WithContext(ctx))
    })
}

// In handler
func Handler(w http.ResponseWriter, r *http.Request) {
    sess := session.FromContext(r.Context())
    sess.Set("key", "value")
    manager.Save(r.Context(), w, r, sess)
}
```

### 2. API Server (Token-based)

```go
// Initialize with header transport
manager := session.New(
    session.WithStore(redisStore),
    session.WithTransport(session.NewHeaderTransport("Authorization", "Bearer")),
    session.WithTTL(24 * time.Hour),
)

// API middleware
func APIAuthMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        sess, err := manager.Load(r.Context(), r)
        if err != nil {
            http.Error(w, "Unauthorized", http.StatusUnauthorized)
            return
        }
        ctx := session.NewContext(r.Context(), sess)
        next.ServeHTTP(w, r.WithContext(ctx))
    })
}
```

### 3. Hybrid Monolith (Multiple Transports)

```go
// Cookie transport for web routes
webManager := session.New(
    session.WithStore(sharedStore),
    session.WithTransport(session.NewCookieTransport("sid", cookieOptions)),
)

// Token transport for API routes
apiManager := session.New(
    session.WithStore(sharedStore), // Same store
    session.WithTransport(session.NewHeaderTransport("Authorization", "Bearer")),
)

// Route setup
mux.Handle("/web/", webMiddleware(webManager, webHandler))
mux.Handle("/api/", apiMiddleware(apiManager, apiHandler))
```

### 4. JWT with Refresh Tokens

```go
// JWT transport with refresh capability
jwtTransport := session.NewJWTTransport(
    session.WithSigningKey([]byte("secret")),
    session.WithRefreshToken(true),
    session.WithAccessTTL(15 * time.Minute),
    session.WithRefreshTTL(7 * 24 * time.Hour),
)

manager := session.New(
    session.WithTransport(jwtTransport),
    // JWT is stateless, but we can use store for refresh tokens
    session.WithStore(refreshTokenStore),
)
```

## Configuration

```go
type Config struct {
    // Token generation
    TokenLength int           // Length of generated tokens (default: 32)
    TokenPrefix string        // Optional prefix for tokens (e.g., "sess_")

    // Timing
    TTL         time.Duration // Session time-to-live
    IdleTimeout time.Duration // Optional idle timeout

    // Security
    Secure      bool          // HTTPS only (for cookies)
    HttpOnly    bool          // No JS access (for cookies)
    SameSite    http.SameSite // CSRF protection (for cookies)

    // Behavior
    AutoRenew   bool          // Renew on activity
    Rolling     bool          // Reset TTL on each request
}
```

## Security Considerations

1. **Token Generation**: Use cryptographically secure random tokens
2. **Transport Security**: Always use HTTPS in production
3. **Cookie Flags**: Set HttpOnly, Secure, and SameSite appropriately
4. **Token Rotation**: Support token rotation on privilege escalation
5. **Expiration**: Implement both idle and absolute timeouts
6. **Storage**: Encrypt sensitive session data at rest

## Performance Considerations

1. **Lazy Loading**: Load sessions only when needed
2. **Batch Operations**: Support bulk delete for cleanup
3. **Caching**: Consider caching layer for high-traffic scenarios
4. **Connection Pooling**: Use connection pools for external stores
5. **Minimal Allocations**: Reuse buffers where possible

## Migration from saaskit/session

Key differences from the saaskit implementation:

1. **Simpler Session Type**: String IDs instead of UUIDs
2. **No Device Fingerprinting**: Can be added via middleware
3. **Single TTL Model**: Simplified from dual idle/max timeouts
4. **No Background Workers**: Lazy expiration instead
5. **Pluggable Transport**: Clean interface for multiple transports
6. **JWT Support**: Built-in JWT transport option

## Extension Points

The package is designed to be extended:

1. **Custom Transports**: Implement the Transport interface
2. **Custom Stores**: Implement the Store interface
3. **Middleware**: Wrap the manager for additional functionality
4. **Session Data**: Store any serializable data
5. **Events**: Hook into session lifecycle events
