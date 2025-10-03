# Session Package Refactoring

## Problem Statement

The current session architecture couples cookie-based and JWT-based session management through a shared `Transport` interface. This creates issues for API use cases:

1. **Automatic embedding**: JWT transport automatically sets headers, preventing manual token delivery
2. **Missing token data**: APIs need access/refresh tokens and expiration data in response body
3. **Mixed concerns**: Session Manager handles both business logic and HTTP transport

### Current Architecture

```
session.Manager[Data]
    ├── depends on → Transport interface
    │                 ├── CookieTransport (auto-embeds cookie)
    │                 └── JWTTransport (auto-embeds header)
    └── Store[Data]
```

**Problems:**

- Manager calls `Transport.Embed()` automatically during `Save()`/`Auth()`
- No way to get token data for JSON responses
- Cookie and JWT patterns forced into same abstraction
- Dual-token pattern (access/refresh) not supported

## New Architecture

**Inversion of Control**: Remove Transport dependency from Manager. Transports compose Manager instead.

```
Layer 1: core/session (Pure Business Logic)
    └── Manager[Data] - No HTTP knowledge

Layer 2: core/sessiontransport (HTTP Implementations)
    ├── Cookie[Data] - Uses Manager, auto-embeds cookies
    └── JWT[Data] - Uses Manager, returns token data
```

### Key Changes

1. **Manager becomes HTTP-agnostic**: Only deals with session lifecycle, not HTTP
2. **No Transport interface**: Concrete implementations compose Manager
3. **Transport controls delivery**: Each transport decides how to deliver tokens
4. **Explicit API**: JWT transport returns structured token data
5. **Clean break**: No backward compatibility - build it right from the start

## New Architecture Details

### Layer 1: core/session

```go
// Pure session management - NO HTTP dependencies

type Session[Data any] struct {
    ID        uuid.UUID
    Token     string    // Session identifier (used as refresh token in JWT)
    TokenHash string
    DeviceID  uuid.UUID
    UserID    uuid.UUID
    Data      Data
    ExpiresAt time.Time
    CreatedAt time.Time
    UpdatedAt time.Time
}

type Store[Data any] interface {
    Get(ctx context.Context, tokenHash string) (Session[Data], error)
    Save(ctx context.Context, session Session[Data]) error
    Delete(ctx context.Context, id uuid.UUID) error
}

type Manager[Data any] struct {
    store         Store[Data]
    ttl           time.Duration
    touchInterval time.Duration
    logger        *slog.Logger
}

// Pure functions - no HTTP
func (m *Manager[Data]) New(ctx context.Context) (Session[Data], error)
func (m *Manager[Data]) Load(ctx context.Context, token string) (Session[Data], error)
func (m *Manager[Data]) Save(ctx context.Context, sess Session[Data]) error
func (m *Manager[Data]) Authenticate(ctx context.Context, sess Session[Data], userID uuid.UUID) (Session[Data], error)
func (m *Manager[Data]) Logout(ctx context.Context, sess Session[Data], opts ...LogoutOption[Data]) (Session[Data], error)
func (m *Manager[Data]) Delete(ctx context.Context, sessionID uuid.UUID) error
func (m *Manager[Data]) Touch(ctx context.Context, sess Session[Data]) (Session[Data], bool, error)
```

**Changes from current:**

- Remove `http.ResponseWriter` and `*http.Request` from all methods
- Remove `Transport` dependency
- Return modified sessions instead of mutating via transport
- All methods take `context.Context` instead of deriving from request

### Layer 2: core/sessiontransport

**Transport Interface:**

```go
// Loader defines interface for loading sessions from HTTP requests
type Loader[Data any] interface {
    Load(w http.ResponseWriter, r *http.Request) (session.Session[Data], error)
}

// Saver defines interface for saving sessions to HTTP responses
type Saver[Data any] interface {
    Save(w http.ResponseWriter, r *http.Request, sess session.Session[Data]) error
}

// Transport combines loading and saving
type Transport[Data any] interface {
    Loader[Data]
    Saver[Data]
}
```

#### Cookie Transport

```go
type Cookie[Data any] struct {
    manager   *session.Manager[Data]
    cookieMgr *cookie.Manager
    name      string
}

func NewCookie[Data any](
    manager *session.Manager[Data],
    cookieMgr *cookie.Manager,
    opts ...CookieOption,
) *Cookie[Data]

// Transport interface implementation
func (c *Cookie[Data]) Load(w http.ResponseWriter, r *http.Request) (session.Session[Data], error)
func (c *Cookie[Data]) Save(w http.ResponseWriter, r *http.Request, sess session.Session[Data]) error

// Additional methods for manual control
func (c *Cookie[Data]) Auth(w http.ResponseWriter, r *http.Request, userID uuid.UUID) (session.Session[Data], error)
func (c *Cookie[Data]) Logout(w http.ResponseWriter, r *http.Request, opts ...session.LogoutOption[Data]) error
func (c *Cookie[Data]) Delete(w http.ResponseWriter, r *http.Request) error
```

**Behavior:**

- Implements `Transport[Data]` interface
- Automatically extracts token from cookie
- Automatically sets cookie on save/auth
- Returns session for handler use
- Used as dependency in middleware

#### JWT Transport

```go
type TokenData struct {
    AccessToken  string    `json:"access_token"`
    RefreshToken string    `json:"refresh_token"`
    TokenType    string    `json:"token_type"`
    ExpiresIn    int       `json:"expires_in"`
    ExpiresAt    time.Time `json:"expires_at"`
}

type JWT[Data any] struct {
    manager   *session.Manager[Data]
    signer    *jwt.Signer
    accessTTL time.Duration
    issuer    string
    audience  string
}

func NewJWT[Data any](
    manager *session.Manager[Data],
    signingKey string,
    opts ...JWTOption,
) (*JWT[Data], error)

// Transport interface implementation
func (j *JWT[Data]) Load(w http.ResponseWriter, r *http.Request) (session.Session[Data], error)
func (j *JWT[Data]) Save(w http.ResponseWriter, r *http.Request, sess session.Session[Data]) error

// Additional methods for token generation
func (j *JWT[Data]) Auth(w http.ResponseWriter, r *http.Request, userID uuid.UUID) (session.Session[Data], TokenData, error)
func (j *JWT[Data]) Refresh(w http.ResponseWriter, r *http.Request) (session.Session[Data], TokenData, error)
```

**Behavior:**

- Implements `Transport[Data]` interface
- Extracts JWT from Authorization header
- `Save()` is no-op (JWT tokens are immutable)
- Returns `TokenData` for manual JSON response
- Access token = short-lived JWT (15min-1hr)
- Refresh token = session.Token (long-lived, 24hr+)
- Access token JTI = session token hash (for revocation)

### Layer 3: middleware Package

Session middleware lives in `middleware/` package, following the same pattern as `ClientIP`.

```go
// middleware/session.go

type sessionContextKey struct{}

type SessionConfig[Data any] struct {
    Transport    sessiontransport.Transport[Data]
    Skip         func(ctx handler.Context) bool
    AutoSave     bool
    RequireAuth  bool
    OnError      func(ctx handler.Context, err error) handler.Response
}

func Session[C handler.Context, Data any](transport sessiontransport.Transport[Data]) handler.Middleware[C]
func SessionWithConfig[C handler.Context, Data any](cfg SessionConfig[Data]) handler.Middleware[C]

// Helper functions
func GetSession[Data any](ctx handler.Context) (session.Session[Data], bool)
func MustGetSession[Data any](ctx handler.Context) session.Session[Data]
func GetUserID[Data any](ctx handler.Context) (uuid.UUID, bool)
func SetSession[Data any](ctx handler.Context, sess session.Session[Data])

// Additional middleware
func RequireAuth[C handler.Context, Data any]() handler.Middleware[C]
func RequireGuest[C handler.Context, Data any]() handler.Middleware[C]
```

**Pattern:**

- Middleware defined in `middleware/` package
- Transport passed as dependency (required parameter)
- Config-based customization
- Helper functions for context access
- Follows same pattern as `ClientIP` middleware

## Implementation Plan

**Direct implementation - no phases, no backward compatibility, no tech debt.**

### Step 1: Rewrite core/session Package

**Files to modify:**

- `core/session/session.go` - Keep Session struct, remove Transport interface
- `core/session/manager.go` - Completely rewrite with context-based API
- `core/session/config.go` - Keep as-is
- `core/session/errors.go` - Remove transport-specific errors
- `core/session/doc.go` - Update documentation

**Files to delete:**

- `core/session/transport.go` (if exists) - Remove Transport interface entirely

**New Manager API:**

```go
type Manager[Data any] struct {
    store         Store[Data]
    ttl           time.Duration
    touchInterval time.Duration
    logger        *slog.Logger
}

// Pure functions - NO http.ResponseWriter, NO *http.Request
func (m *Manager[Data]) New(ctx context.Context) (Session[Data], error)
func (m *Manager[Data]) Load(ctx context.Context, token string) (Session[Data], error)
func (m *Manager[Data]) Save(ctx context.Context, sess Session[Data]) error
func (m *Manager[Data]) Authenticate(ctx context.Context, sess Session[Data], userID uuid.UUID) (Session[Data], error)
func (m *Manager[Data]) Logout(ctx context.Context, sess Session[Data], opts ...LogoutOption[Data]) (Session[Data], error)
func (m *Manager[Data]) Delete(ctx context.Context, sessionID uuid.UUID) error
func (m *Manager[Data]) Touch(ctx context.Context, sess Session[Data]) (Session[Data], bool, error)
```

**Implementation checklist:**

- [ ] Remove all HTTP dependencies from manager.go
- [ ] Rewrite all methods to use context.Context
- [ ] Return modified Session structs instead of mutating
- [ ] Remove Transport field from Manager
- [ ] Update all internal helper functions
- [ ] Update tests to not use HTTP

### Step 2: Delete old sessiontransport Package

**Files to delete:**

- Entire `core/sessiontransport/` directory (current implementation)

We'll rebuild it from scratch in the next step.

### Step 3: Create New sessiontransport Package

**Files to create:**

- `core/sessiontransport/doc.go` - Package documentation
- `core/sessiontransport/transport.go` - Transport interface
- `core/sessiontransport/cookie.go` - Cookie transport
- `core/sessiontransport/cookie_test.go` - Cookie tests
- `core/sessiontransport/jwt.go` - JWT transport
- `core/sessiontransport/jwt_test.go` - JWT tests
- `core/sessiontransport/token.go` - TokenData struct

#### Step 3.1: Define Transport Interface

**core/sessiontransport/transport.go:**

```go
// Loader defines interface for loading sessions from HTTP requests
type Loader[Data any] interface {
    Load(w http.ResponseWriter, r *http.Request) (session.Session[Data], error)
}

// Saver defines interface for saving sessions to HTTP responses
type Saver[Data any] interface {
    Save(w http.ResponseWriter, r *http.Request, sess session.Session[Data]) error
}

// Transport combines loading and saving (used by middleware)
type Transport[Data any] interface {
    Loader[Data]
    Saver[Data]
}
```

**Checklist:**

- [ ] Define Loader interface
- [ ] Define Saver interface
- [ ] Define Transport interface (composition)
- [ ] Document interface usage

#### Step 3.2: Implement Cookie Transport

**core/sessiontransport/cookie.go:**

```go
type Cookie[Data any] struct {
    manager   *session.Manager[Data]
    cookieMgr *cookie.Manager
    name      string
}

func NewCookie[Data any](
    manager *session.Manager[Data],
    cookieMgr *cookie.Manager,
    opts ...CookieOption,
) *Cookie[Data]

// Transport interface implementation
func (c *Cookie[Data]) Load(w http.ResponseWriter, r *http.Request) (session.Session[Data], error)
func (c *Cookie[Data]) Save(w http.ResponseWriter, r *http.Request, sess session.Session[Data]) error

// Additional methods for manual control
func (c *Cookie[Data]) Auth(w http.ResponseWriter, r *http.Request, userID uuid.UUID) (session.Session[Data], error)
func (c *Cookie[Data]) Logout(w http.ResponseWriter, r *http.Request, opts ...session.LogoutOption[Data]) error
func (c *Cookie[Data]) Delete(w http.ResponseWriter, r *http.Request) error
```

**Checklist:**

- [ ] Implement Transport interface
- [ ] Cookie extraction from request
- [ ] Cookie setting in response
- [ ] All methods call session.Manager
- [ ] Auth method creates session and sets cookie
- [ ] Logout method clears session and cookie
- [ ] Delete method removes session and cookie
- [ ] Proper error handling
- [ ] Full test coverage

#### Step 3.3: Implement JWT Transport

**core/sessiontransport/token.go:**

```go
type TokenData struct {
    AccessToken  string    `json:"access_token"`
    RefreshToken string    `json:"refresh_token"`
    TokenType    string    `json:"token_type"`
    ExpiresIn    int       `json:"expires_in"`
    ExpiresAt    time.Time `json:"expires_at"`
}
```

**core/sessiontransport/jwt.go:**

```go
type JWT[Data any] struct {
    manager   *session.Manager[Data]
    signer    *jwt.Signer
    accessTTL time.Duration
    issuer    string
    audience  string
}

func NewJWT[Data any](
    manager *session.Manager[Data],
    signingKey string,
    opts ...JWTOption,
) (*JWT[Data], error)

// Transport interface implementation
func (j *JWT[Data]) Load(w http.ResponseWriter, r *http.Request) (session.Session[Data], error)
func (j *JWT[Data]) Save(w http.ResponseWriter, r *http.Request, sess session.Session[Data]) error

// Additional methods for token generation
func (j *JWT[Data]) Auth(w http.ResponseWriter, r *http.Request, userID uuid.UUID) (session.Session[Data], TokenData, error)
func (j *JWT[Data]) Refresh(w http.ResponseWriter, r *http.Request) (session.Session[Data], TokenData, error)
```

**Access Token Claims:**

```go
{
    "jti": "session-token-hash",  // Session token for revocation
    "sub": "user-uuid",
    "iss": "app-issuer",
    "aud": "app-audience",
    "exp": timestamp,
    "iat": timestamp
}
```

**Checklist:**

- [ ] Implement Transport interface
- [ ] Load extracts JWT from Authorization header
- [ ] Load validates JWT and extracts session token from JTI
- [ ] Save is no-op (JWT tokens are immutable)
- [ ] Auth generates access token and returns TokenData
- [ ] Refresh validates refresh token and generates new access token
- [ ] Access token generation with proper claims
- [ ] TokenData struct with JSON tags
- [ ] Proper error handling
- [ ] Full test coverage

### Step 4: Create Middleware

**Files to create:**

- `middleware/session.go` - Session middleware
- `middleware/session_test.go` - Middleware tests

#### Step 4.1: Implement Session Middleware

**middleware/session.go:**

```go
type sessionContextKey struct{}

type SessionConfig[Data any] struct {
    Transport    sessiontransport.Transport[Data]  // Required
    Skip         func(ctx handler.Context) bool
    AutoSave     bool
    RequireAuth  bool
    OnError      func(ctx handler.Context, err error) handler.Response
}

func Session[C handler.Context, Data any](transport sessiontransport.Transport[Data]) handler.Middleware[C]
func SessionWithConfig[C handler.Context, Data any](cfg SessionConfig[Data]) handler.Middleware[C]

// Helper functions
func GetSession[Data any](ctx handler.Context) (session.Session[Data], bool)
func MustGetSession[Data any](ctx handler.Context) session.Session[Data]
func GetUserID[Data any](ctx handler.Context) (uuid.UUID, bool)
func SetSession[Data any](ctx handler.Context, sess session.Session[Data])

// Additional middleware
func RequireAuth[C handler.Context, Data any]() handler.Middleware[C]
func RequireGuest[C handler.Context, Data any]() handler.Middleware[C]
```

**Checklist:**

- [ ] Session middleware with config
- [ ] Transport passed as dependency
- [ ] Skip function support
- [ ] AutoSave support
- [ ] RequireAuth support
- [ ] OnError callback support
- [ ] GetSession helper
- [ ] MustGetSession helper
- [ ] GetUserID helper
- [ ] SetSession helper
- [ ] RequireAuth middleware
- [ ] RequireGuest middleware
- [ ] Full test coverage
- [ ] Documentation following ClientIP pattern

### Step 5: Update Examples

**Files to rewrite:**

- `examples/basic/main.go` - Setup both transports
- `examples/basic/handlers.go` - Implement all handlers

**Handlers to implement:**

**Cookie handlers:**

```go
func handleWebLogin(w http.ResponseWriter, r *http.Request)
func handleWebLogout(w http.ResponseWriter, r *http.Request)
func handleWebDashboard(w http.ResponseWriter, r *http.Request)
```

**API handlers:**

```go
func handleAPILogin(w http.ResponseWriter, r *http.Request)
func handleAPIRefresh(w http.ResponseWriter, r *http.Request)
func handleAPIProtected(w http.ResponseWriter, r *http.Request)
func handleAPILogout(w http.ResponseWriter, r *http.Request)
```

**Checklist:**

- [ ] Both transports working side-by-side
- [ ] Cookie handlers don't touch token data
- [ ] API handlers return JSON with tokens
- [ ] Error handling examples
- [ ] README with curl examples

### Step 6: Update Documentation

**Files to update/create:**

- `core/session/doc.go` - Update with new API
- `core/sessiontransport/doc.go` - Complete package docs with examples
- `README.md` (project root) - Update session section

**Documentation checklist:**

- [ ] Remove all references to old Transport interface
- [ ] Add examples for both Cookie and JWT
- [ ] Explain when to use which transport
- [ ] Document dual-token pattern
- [ ] Show refresh token flow
- [ ] Add security considerations

### Step 7: Update Tests

**Files to update:**

- `core/session/manager_test.go` - Remove HTTP, use context
- `core/session/session_test.go` - Update if needed
- `core/session/race_test.go` - Update if needed

**New test files:**

- `core/sessiontransport/cookie_test.go`
- `core/sessiontransport/jwt_test.go`

**Test checklist:**

- [ ] All session tests pass without HTTP
- [ ] Cookie transport full coverage
- [ ] JWT transport full coverage
- [ ] Integration tests with real stores
- [ ] Race condition tests
- [ ] Example application tests

## Usage Examples

### Cookie-Based Web Application

```go
package main

import (
    "github.com/dmitrymomot/foundation/core/session"
    "github.com/dmitrymomot/foundation/core/sessiontransport"
    "github.com/dmitrymomot/foundation/core/cookie"
    "github.com/dmitrymomot/foundation/middleware"
)

type AppContext struct {
    handler.BaseContext
}

type AppData struct {
    Theme    string `json:"theme"`
    Language string `json:"language"`
}

func main() {
    // Setup
    store := NewRedisStore[AppData](redisClient)
    sessionMgr := session.New(
        session.WithStore(store),
        session.WithTTL(24 * time.Hour),
    )

    cookieMgr, _ := cookie.New([]string{os.Getenv("COOKIE_SECRET")})
    cookieTransport := sessiontransport.NewCookie(
        sessionMgr,
        cookieMgr,
        sessiontransport.WithCookieName("__session"),
    )

    router := router.New[*AppContext]()

    // Apply session middleware globally (with auto-save)
    router.Use(middleware.Session[*AppContext](cookieTransport))

    // Public routes
    router.GET("/", handleHome)

    // Login (guest-only)
    router.POST("/login",
        middleware.RequireGuest[*AppContext, AppData](),
        handleLogin,
    )

    // Protected routes
    router.GET("/dashboard",
        middleware.RequireAuth[*AppContext, AppData](),
        handleDashboard,
    )

    router.POST("/logout", handleLogout)
}

func handleLogin(ctx *AppContext) handler.Response {
    userID := authenticateUser(ctx.Request())

    // Auth creates session and sets cookie
    sess, err := cookieTransport.Auth(ctx.ResponseWriter(), ctx.Request(), userID)
    if err != nil {
        return response.Error(response.ErrInternalServerError)
    }

    return response.Redirect("/dashboard")
}

func handleDashboard(ctx *AppContext) handler.Response {
    sess := middleware.MustGetSession[AppData](ctx)

    // Update session data
    sess.Data.Theme = "dark"
    middleware.SetSession(ctx, sess)  // Auto-saved by middleware

    return response.JSON(map[string]any{
        "user_id": sess.UserID,
        "theme":   sess.Data.Theme,
    })
}

func handleLogout(ctx *AppContext) handler.Response {
    cookieTransport.Logout(ctx.ResponseWriter(), ctx.Request())
    return response.Redirect("/")
}
```

### JWT-Based API

```go
package main

import (
    "github.com/dmitrymomot/foundation/core/session"
    "github.com/dmitrymomot/foundation/core/sessiontransport"
    "github.com/dmitrymomot/foundation/middleware"
    "github.com/dmitrymomot/foundation/pkg/response"
)

type AppContext struct {
    handler.BaseContext
}

type AppData struct {
    Preferences map[string]string `json:"preferences"`
}

func main() {
    // Setup
    store := NewPostgresStore[AppData](db)
    sessionMgr := session.New(
        session.WithStore(store),
        session.WithTTL(24 * time.Hour),
    )

    jwtTransport, _ := sessiontransport.NewJWT(
        sessionMgr,
        os.Getenv("JWT_SECRET"),
        sessiontransport.WithAccessTTL(15 * time.Minute),
        sessiontransport.WithJWTIssuer("myapp"),
    )

    router := router.New[*AppContext]()

    // Public endpoints
    router.POST("/api/login", handleAPILogin)
    router.POST("/api/refresh", handleAPIRefresh)

    // Protected API routes
    apiGroup := router.Group("/api")
    apiGroup.Use(middleware.SessionWithConfig[*AppContext](middleware.SessionConfig[AppData]{
        Transport: jwtTransport,
        AutoSave:  false,  // JWT tokens are immutable
    }))
    apiGroup.GET("/user", handleAPIUser)
    apiGroup.GET("/profile", handleAPIProfile)

    // Admin routes
    adminGroup := router.Group("/api/admin")
    adminGroup.Use(
        middleware.SessionWithConfig[*AppContext](middleware.SessionConfig[AppData]{
            Transport: jwtTransport,
            AutoSave:  false,
        }),
        middleware.RequireAuth[*AppContext, AppData](),
    )
    adminGroup.GET("/users", handleAdminUsers)
}

func handleAPILogin(ctx *AppContext) handler.Response {
    userID := authenticateCredentials(ctx.Request())

    sess, tokens, err := jwtTransport.Auth(ctx.ResponseWriter(), ctx.Request(), userID)
    if err != nil {
        return response.Error(response.ErrUnauthorized)
    }

    // Manual token delivery with custom response
    return response.JSON(map[string]any{
        "tokens": tokens,
        "user": map[string]any{
            "id":   sess.UserID,
            "data": sess.Data,
        },
    })
}

func handleAPIRefresh(ctx *AppContext) handler.Response {
    sess, tokens, err := jwtTransport.Refresh(ctx.ResponseWriter(), ctx.Request())
    if err != nil {
        return response.Error(response.ErrUnauthorized)
    }

    return response.JSON(map[string]any{
        "tokens":  tokens,
        "user_id": sess.UserID,
    })
}

func handleAPIUser(ctx *AppContext) handler.Response {
    sess := middleware.MustGetSession[AppData](ctx)

    return response.JSON(map[string]any{
        "id":   sess.UserID,
        "data": sess.Data,
    })
}
```

## Benefits

1. **Separation of concerns**: Session logic independent of HTTP
2. **Explicit control**: APIs control token delivery
3. **Dual-token support**: Access/refresh pattern built-in
4. **No interface bloat**: Concrete implementations, no forced abstraction
5. **Testability**: Each layer tests independently
6. **Flexibility**: Easy to add new transport types
7. **Type safety**: Full generic support throughout
8. **Clean composition**: Transports compose Manager instead of Manager depending on Transport

## Testing Strategy

### Unit Tests

**session package:**

- Test pure session lifecycle (no HTTP)
- Test token generation/validation
- Test change detection
- Test concurrent access

**sessiontransport/cookie:**

- Test cookie extraction
- Test cookie setting
- Test session operations through transport
- Test error cases

**sessiontransport/jwt:**

- Test JWT generation/validation
- Test access token flow
- Test refresh token flow
- Test TokenData serialization
- Test revocation (if implemented)

### Integration Tests

- Cookie transport with real cookie manager
- JWT transport with real JWT signer
- Both transports with real store (Redis/Postgres)
- Race condition tests

### Example Tests

- Full login/logout flows
- Protected route access
- Token refresh flows
- Error handling

## Design Decisions

### 1. JWT Revocation

**Decision: Optional via interface**

```go
// Optional revocation support
type Revoker interface {
    IsRevoked(ctx context.Context, tokenHash string) (bool, error)
    Revoke(ctx context.Context, tokenHash string, ttl time.Duration) error
}

// Default: no-op (stateless JWTs)
type NoOpRevoker struct{}

// Production: Redis-backed
type RedisRevoker struct {
    client *redis.Client
}
```

**Rationale**: Some apps want pure stateless JWTs, others need revocation. Make it pluggable.

### 2. JWT Audience

**Decision: Single string (can be made slice later if needed)**

```go
type JWT[Data] struct {
    audience string  // Single audience for now
}
```

**Rationale**: YAGNI. Most apps have single audience. Easy to change to `[]string` later.

### 3. Custom JWT Claims

**Decision: Not in v1. Session.Data is enough.**

**Rationale**: Keep it simple. If you need custom claims in JWT, encode Session.Data or use middleware.

### 4. Refresh Token Rotation

**Decision: No rotation by default. Optional via flag.**

```go
sessiontransport.WithRefreshRotation(true)
```

**Rationale**: Rotation improves security but complicates client logic. Make it opt-in.

### 5. Middleware

**Decision: No built-in middleware. Too opinionated.**

**Rationale**: Easy for users to write their own middleware. Transport API is simple enough.

### 6. Session.Token in JWT

**Decision: Store session.Token as JTI claim**

**Rationale**: Allows revocation lookup and links access tokens to session lifecycle.

## Success Criteria

- [ ] Session Manager has no HTTP dependencies
- [ ] Cookie transport works identically to current behavior
- [ ] JWT transport returns structured token data
- [ ] All tests pass
- [ ] Examples demonstrate both transports
- [ ] Documentation is complete
- [ ] Migration guide is clear
- [ ] No breaking changes in Phase 1-4
