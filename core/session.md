# Session System Design

**Principles:**

- Clean separation: Manager → Transport → Middleware
- Simple, understandable code
- No tricks, no over-engineering
- Leverage existing packages (core/cookie, pkg/fingerprint)

## Architecture

```
┌─────────────────┐
│   Middleware    │  HTTP layer - manages session lifecycle
└────────┬────────┘
         │ uses
┌────────▼────────┐
│   Transport     │  HTTP ↔ Session bridge
└────────┬────────┘
         │ uses
┌────────▼────────┐
│    Manager      │  Pure business logic - no HTTP
└────────┬────────┘
         │ uses
┌────────▼────────┐
│     Store       │  Persistence
└─────────────────┘
```

**Each layer has ONE job:**

- **Store**: Save/load sessions from database
- **Manager**: Session lifecycle (create, validate, expire)
- **Transport**: Extract/embed sessions in HTTP (cookies, JWT)
- **Middleware**: HTTP integration (load, save, auth checks)

## Layer 1: Store (Persistence)

**Single responsibility: Persist and retrieve sessions**

```go
package session

type Session[Data any] struct {
    ID        uuid.UUID
    Token     string    // Credential (rotates on auth)
    UserID    uuid.UUID // nil = anonymous
    Data      Data      // Custom app data
    ExpiresAt time.Time
    CreatedAt time.Time
    UpdatedAt time.Time
}

type Store[Data any] interface {
    // Two lookup methods - different use cases
    GetByID(ctx context.Context, id uuid.UUID) (Session[Data], error)
    GetByToken(ctx context.Context, token string) (Session[Data], error)

    Save(ctx context.Context, sess Session[Data]) error
    Delete(ctx context.Context, id uuid.UUID) error
    DeleteExpired(ctx context.Context) error // Cleanup for periodic task
}
```

**Why two Get methods:**

- `GetByID`: Admin operations (revoke all user sessions, etc.)
- `GetByToken`: Both Cookie and JWT use this (unified approach)

**Why separate ID and Token:**

- **ID** is stable (foreign key for cart_items, etc.)
- **Token** rotates on authentication (security - session fixation prevention)
- **Token** is the credential in both Cookie and JWT (JTI = Session.Token)

**Token Generation:**

```go
func generateToken() string {
    b := make([]byte, 32) // 256 bits for strong entropy
    if _, err := rand.Read(b); err != nil {
        panic(err) // crypto/rand should never fail
    }
    return base64.RawURLEncoding.EncodeToString(b) // base64url without padding
}
```

## Layer 2: Manager (Business Logic)

**Single responsibility: Session lifecycle, no HTTP knowledge**

```go
package session

type Manager[Data any] struct {
    store         Store[Data]
    ttl           time.Duration
    touchInterval time.Duration
}

// Create new anonymous session
func (m *Manager[Data]) New(ctx context.Context) (Session[Data], error) {
    sess := Session[Data]{
        ID:        uuid.New(),
        Token:     generateToken(), // crypto/rand
        ExpiresAt: time.Now().Add(m.ttl),
        CreatedAt: time.Now(),
        UpdatedAt: time.Now(),
    }
    return sess, m.store.Save(ctx, sess)
}

// Load by ID (for admin operations)
func (m *Manager[Data]) GetByID(ctx context.Context, id uuid.UUID) (Session[Data], error) {
    sess, err := m.store.GetByID(ctx, id)
    if err != nil {
        return Session[Data]{}, err
    }

    if sess.ExpiresAt.Before(time.Now()) {
        m.store.Delete(ctx, sess.ID) // Cleanup
        return Session[Data]{}, ErrExpired
    }

    return sess, nil
}

// Load by token (used by both Cookie and JWT transports)
func (m *Manager[Data]) GetByToken(ctx context.Context, token string) (Session[Data], error) {
    sess, err := m.store.GetByToken(ctx, token)
    if err != nil {
        return Session[Data]{}, err
    }

    if sess.ExpiresAt.Before(time.Now()) {
        m.store.Delete(ctx, sess.ID)
        return Session[Data]{}, ErrExpired
    }

    return sess, nil
}

// Save session
func (m *Manager[Data]) Save(ctx context.Context, sess Session[Data]) error {
    sess.UpdatedAt = time.Now()
    return m.store.Save(ctx, sess)
}

// Authenticate - converts anonymous → authenticated
func (m *Manager[Data]) Authenticate(ctx context.Context, sess Session[Data], userID uuid.UUID) (Session[Data], error) {
    sess.UserID = userID
    sess.Token = generateToken() // Rotate token (security)
    sess.ExpiresAt = time.Now().Add(m.ttl)
    sess.UpdatedAt = time.Now()

    return sess, m.store.Save(ctx, sess)
}

// Logout - converts authenticated → anonymous
func (m *Manager[Data]) Logout(ctx context.Context, sess Session[Data]) (Session[Data], error) {
    sess.UserID = uuid.Nil
    sess.Token = generateToken() // Rotate token
    sess.Data = *new(Data)        // Clear data
    sess.UpdatedAt = time.Now()

    return sess, m.store.Save(ctx, sess)
}

// Delete session entirely
func (m *Manager[Data]) Delete(ctx context.Context, id uuid.UUID) error {
    return m.store.Delete(ctx, id)
}

// shouldTouch checks if session should be touched
func (m *Manager[Data]) shouldTouch(sess Session[Data]) bool {
    return time.Since(sess.UpdatedAt) >= m.touchInterval
}

// touch updates session expiration (internal helper)
func (m *Manager[Data]) touch(sess Session[Data]) Session[Data] {
    sess.ExpiresAt = time.Now().Add(m.ttl)
    sess.UpdatedAt = time.Now()
    return sess
}

// CleanupExpired removes all expired sessions (periodic task)
// Implements PeriodicTaskHandlerFunc interface from core/queue
func (m *Manager[Data]) CleanupExpired(ctx context.Context) error {
    // Implementation depends on Store having a cleanup method
    // Store interface will need: DeleteExpired(ctx context.Context) error
    return m.store.DeleteExpired(ctx)
}
```

**That's it. No HTTP. No cookies. No JWT. Just session logic.**

## Layer 3: Transport (HTTP Bridge)

**Single responsibility: Extract sessions from requests, embed sessions in responses**

### Cookie Transport

```go
package sessiontransport

type Cookie[Data any] struct {
    manager   *session.Manager[Data]
    cookieMgr *cookie.Manager  // core/cookie package
    name      string
}

func NewCookie[Data any](mgr *session.Manager[Data], cookieMgr *cookie.Manager, name string) *Cookie[Data] {
    return &Cookie[Data]{
        manager:   mgr,
        cookieMgr: cookieMgr,
        name:      name,
    }
}

// Load session from cookie
func (c *Cookie[Data]) Load(ctx context.Context, r *http.Request) (session.Session[Data], error) {
    token, err := c.cookieMgr.Get(r, c.name)
    if err != nil {
        // No cookie - create new anonymous session
        return c.manager.New(ctx)
    }

    sess, err := c.manager.GetByToken(ctx, token)
    if err != nil {
        // Invalid/expired - create new
        return c.manager.New(ctx)
    }

    return sess, nil
}

// Save session to cookie
func (c *Cookie[Data]) Save(ctx context.Context, w http.ResponseWriter, sess session.Session[Data]) error {
    c.cookieMgr.Set(w, c.name, sess.Token)
    return c.manager.Save(ctx, sess)
}

// Authenticate user
func (c *Cookie[Data]) Authenticate(ctx context.Context, w http.ResponseWriter, r *http.Request, userID uuid.UUID) (session.Session[Data], error) {
    sess, _ := c.Load(ctx, r)
    sess, err := c.manager.Authenticate(ctx, sess, userID)
    if err != nil {
        return session.Session[Data]{}, err
    }

    c.cookieMgr.Set(w, c.name, sess.Token) // New token
    return sess, nil
}

// Logout user
func (c *Cookie[Data]) Logout(ctx context.Context, w http.ResponseWriter, r *http.Request) (session.Session[Data], error) {
    sess, _ := c.Load(ctx, r)
    sess, err := c.manager.Logout(ctx, sess)
    if err != nil {
        return session.Session[Data]{}, err
    }

    c.cookieMgr.Set(w, c.name, sess.Token) // New token
    return sess, nil
}

// Delete session
func (c *Cookie[Data]) Delete(ctx context.Context, w http.ResponseWriter, r *http.Request) error {
    sess, _ := c.Load(ctx, r)
    c.cookieMgr.Delete(w, c.name)
    return c.manager.Delete(ctx, sess.ID)
}

// Touch updates session expiration if interval passed
func (c *Cookie[Data]) Touch(ctx context.Context, w http.ResponseWriter, sess session.Session[Data]) error {
    if !c.manager.shouldTouch(sess) {
        return nil // Too soon, skip
    }

    sess = c.manager.touch(sess)
    c.cookieMgr.Set(w, c.name, sess.Token)
    return c.manager.Save(ctx, sess)
}
```

### JWT Transport

```go
package sessiontransport

type TokenPair struct {
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
}

func NewJWT[Data any](mgr *session.Manager[Data], secretKey string, accessTTL time.Duration, issuer string) (*JWT[Data], error) {
    signer, err := jwt.NewSigner(secretKey)
    if err != nil {
        return nil, err
    }

    return &JWT[Data]{
        manager:   mgr,
        signer:    signer,
        accessTTL: accessTTL,
        issuer:    issuer,
    }, nil
}

// Load session from JWT
func (j *JWT[Data]) Load(ctx context.Context, r *http.Request) (session.Session[Data], error) {
    tokenStr := extractBearerToken(r)
    if tokenStr == "" {
        return session.Session[Data]{}, ErrNoToken
    }

    // Validate JWT
    claims, err := j.signer.Verify(tokenStr)
    if err != nil {
        return session.Session[Data]{}, err
    }

    // Extract Session.Token from JTI
    sessionToken := claims.JTI
    if sessionToken == "" {
        return session.Session[Data]{}, ErrInvalidToken
    }

    // Same lookup as Cookie transport (unified)
    return j.manager.GetByToken(ctx, sessionToken)
}

// Save is no-op for JWT (tokens are immutable)
func (j *JWT[Data]) Save(ctx context.Context, w http.ResponseWriter, sess session.Session[Data]) error {
    return nil // JWT tokens cannot be updated
}

// Authenticate user - returns token pair
func (j *JWT[Data]) Authenticate(ctx context.Context, w http.ResponseWriter, r *http.Request, userID uuid.UUID) (session.Session[Data], TokenPair, error) {
    // Load existing session or create new
    sess, err := j.Load(ctx, r)
    if err != nil {
        sess, err = j.manager.New(ctx)
        if err != nil {
            return session.Session[Data]{}, TokenPair{}, err
        }
    }

    // Authenticate (rotates token)
    sess, err = j.manager.Authenticate(ctx, sess, userID)
    if err != nil {
        return session.Session[Data]{}, TokenPair{}, err
    }

    // Generate JWT with Session.Token in JTI (unified with Cookie)
    accessToken, err := j.signer.Sign(jwt.Claims{
        JTI:       sess.Token,          // Session token (same as refresh token)
        Subject:   sess.UserID.String(),
        Issuer:    j.issuer,
        IssuedAt:  time.Now(),
        ExpiresAt: time.Now().Add(j.accessTTL),
    })
    if err != nil {
        return session.Session[Data]{}, TokenPair{}, err
    }

    return sess, TokenPair{
        AccessToken:  accessToken,
        RefreshToken: sess.Token, // Same token in both JTI and refresh_token
        TokenType:    "Bearer",
        ExpiresIn:    int(j.accessTTL.Seconds()),
        ExpiresAt:    time.Now().Add(j.accessTTL),
    }, nil
}

// Refresh - generate new access token and rotate refresh token
func (j *JWT[Data]) Refresh(ctx context.Context, refreshToken string) (session.Session[Data], TokenPair, error) {
    // Load by refresh token
    sess, err := j.manager.GetByToken(ctx, refreshToken)
    if err != nil {
        return session.Session[Data]{}, TokenPair{}, err
    }

    if sess.UserID == uuid.Nil {
        return session.Session[Data]{}, TokenPair{}, ErrNotAuthenticated
    }

    // Rotate session token (security - refresh token rotation)
    sess.Token = generateToken()
    sess.ExpiresAt = time.Now().Add(j.manager.ttl)
    sess.UpdatedAt = time.Now()
    if err := j.manager.Save(ctx, sess); err != nil {
        return session.Session[Data]{}, TokenPair{}, err
    }

    // Generate new access token with new session token
    accessToken, err := j.signer.Sign(jwt.Claims{
        JTI:       sess.Token,            // New session token
        Subject:   sess.UserID.String(),
        Issuer:    j.issuer,
        IssuedAt:  time.Now(),
        ExpiresAt: time.Now().Add(j.accessTTL),
    })
    if err != nil {
        return session.Session[Data]{}, TokenPair{}, err
    }

    return sess, TokenPair{
        AccessToken:  accessToken,
        RefreshToken: sess.Token, // New rotated refresh token
        TokenType:    "Bearer",
        ExpiresIn:    int(j.accessTTL.Seconds()),
        ExpiresAt:    time.Now().Add(j.accessTTL),
    }, nil
}

// Logout
func (j *JWT[Data]) Logout(ctx context.Context, w http.ResponseWriter, r *http.Request) error {
    sess, err := j.Load(ctx, r)
    if err != nil {
        return err
    }

    _, err = j.manager.Logout(ctx, sess)
    return err
}

// Delete
func (j *JWT[Data]) Delete(ctx context.Context, w http.ResponseWriter, r *http.Request) error {
    sess, err := j.Load(ctx, r)
    if err != nil {
        return err
    }

    return j.manager.Delete(ctx, sess.ID)
}

// Touch updates session expiration if interval passed
func (j *JWT[Data]) Touch(ctx context.Context, w http.ResponseWriter, sess session.Session[Data]) error {
    if !j.manager.shouldTouch(sess) {
        return nil // Too soon, skip
    }

    sess = j.manager.touch(sess)
    // JWT is immutable - no header update, but save to DB to extend expiration
    return j.manager.Save(ctx, sess)
}

func extractBearerToken(r *http.Request) string {
    auth := r.Header.Get("Authorization")
    if strings.HasPrefix(auth, "Bearer ") {
        return strings.TrimPrefix(auth, "Bearer ")
    }
    return ""
}
```

## Layer 4: Middleware (HTTP Integration)

**Single responsibility: HTTP lifecycle management**

```go
package middleware

type sessionKey struct{}

// Simple session middleware with auto-touch
func Session[C handler.Context, Data any](
    transport interface {
        Load(context.Context, *http.Request) (session.Session[Data], error)
        Save(context.Context, http.ResponseWriter, session.Session[Data]) error
        Touch(context.Context, http.ResponseWriter, session.Session[Data]) error
    },
) handler.Middleware[C] {
    return func(next handler.HandlerFunc[C]) handler.HandlerFunc[C] {
        return func(ctx C) handler.Response {
            // Load session
            sess, err := transport.Load(ctx.Context(), ctx.Request())
            if err != nil {
                // Log error but continue with empty session
                sess = session.Session[Data]{}
            }

            // Store in context
            r := ctx.Request().WithContext(context.WithValue(ctx.Context(), sessionKey{}, sess))
            ctx.SetRequest(r)

            // Process request
            resp := next(ctx)

            // Touch session (extends expiration if interval passed)
            // Failures are logged but don't fail the request
            sess, ok := GetSession[Data](ctx)
            if ok {
                if err := transport.Touch(ctx.Context(), ctx.ResponseWriter(), sess); err != nil {
                    // TODO: Use proper logger instead of log package
                    log.Printf("WARN: failed to touch session: %v", err)
                }
            }

            return resp
        }
    }
}

// Get session from context
func GetSession[Data any](ctx handler.Context) (session.Session[Data], bool) {
    sess, ok := ctx.Request().Context().Value(sessionKey{}).(session.Session[Data])
    return sess, ok
}

// Must get session (panics if not found)
func MustGetSession[Data any](ctx handler.Context) session.Session[Data] {
    sess, ok := GetSession[Data](ctx)
    if !ok {
        panic("session not found in context")
    }
    return sess
}

// Update session in context
func SetSession[Data any](ctx handler.Context, sess session.Session[Data]) {
    r := ctx.Request().WithContext(context.WithValue(ctx.Context(), sessionKey{}, sess))
    ctx.SetRequest(r)
}

// Require authenticated user
func RequireAuth[C handler.Context, Data any]() handler.Middleware[C] {
    return func(next handler.HandlerFunc[C]) handler.HandlerFunc[C] {
        return func(ctx C) handler.Response {
            sess, ok := GetSession[Data](ctx)
            if !ok || sess.UserID == uuid.Nil {
                return response.Error(response.ErrUnauthorized)
            }
            return next(ctx)
        }
    }
}

// Require guest (not authenticated)
func RequireGuest[C handler.Context, Data any]() handler.Middleware[C] {
    return func(next handler.HandlerFunc[C]) handler.HandlerFunc[C] {
        return func(ctx C) handler.Response {
            sess, ok := GetSession[Data](ctx)
            if ok && sess.UserID != uuid.Nil {
                return response.Redirect("/dashboard")
            }
            return next(ctx)
        }
    }
}
```

## Usage Examples

### Periodic Cleanup Task

```go
// Setup periodic cleanup (runs every hour to remove expired sessions)
sessionMgr := session.NewManager(store, 24*time.Hour, 1*time.Hour)

// Register as periodic task with queue system
cleanupHandler := queue.NewPeriodicTaskHandler(
    "session_cleanup",
    sessionMgr.CleanupExpired,
)

// Schedule to run every hour
scheduler.Register(cleanupHandler, 1*time.Hour)
```

### Cookie-Based Web App

```go
type AppData struct {
    Theme string `json:"theme"`
}

func main() {
    // Setup
    store := NewPostgresStore[AppData](db)
    sessionMgr := session.NewManager(
        store,
        24*time.Hour,      // TTL
        1*time.Hour,       // Touch interval
    )

    cookieMgr, _ := cookie.New([]string{os.Getenv("SECRET")})
    cookieTransport := sessiontransport.NewCookie(sessionMgr, cookieMgr, "sid")

    router := router.New[*AppContext]()
    router.Use(middleware.Session[*AppContext, AppData](cookieTransport))

    // Routes
    router.POST("/login", middleware.RequireGuest[*AppContext, AppData](), handleLogin)
    router.GET("/dashboard", middleware.RequireAuth[*AppContext, AppData](), handleDashboard)
    router.POST("/logout", handleLogout)
}

func handleLogin(ctx *AppContext) handler.Response {
    userID := authenticateUser(ctx.Request())

    // Authenticate creates new session and sets cookie
    sess, err := cookieTransport.Authenticate(
        ctx.Context(),
        ctx.ResponseWriter(),
        ctx.Request(),
        userID,
    )
    if err != nil {
        return response.Error(response.ErrInternalServerError)
    }

    return response.Redirect("/dashboard")
}

func handleDashboard(ctx *AppContext) handler.Response {
    sess := middleware.MustGetSession[AppData](ctx)

    // Update session data
    sess.Data.Theme = "dark"
    middleware.SetSession(ctx, sess) // Auto-saved by middleware

    return response.JSON(map[string]any{
        "user_id": sess.UserID,
        "theme":   sess.Data.Theme,
    })
}

func handleLogout(ctx *AppContext) handler.Response {
    cookieTransport.Logout(ctx.Context(), ctx.ResponseWriter(), ctx.Request())
    return response.Redirect("/")
}
```

### JWT-Based API

```go
type AppData struct {
    Preferences map[string]string `json:"preferences"`
}

func main() {
    // Setup
    store := NewPostgresStore[AppData](db)
    sessionMgr := session.NewManager(
        store,
        24*time.Hour,      // Session TTL
        1*time.Hour,       // Touch interval
    )

    jwtTransport, _ := sessiontransport.NewJWT(
        sessionMgr,
        os.Getenv("JWT_SECRET"),
        15*time.Minute, // Access token TTL
        "myapp",
    )

    router := router.New[*AppContext]()

    // Public endpoints
    router.POST("/api/login", handleAPILogin)
    router.POST("/api/refresh", handleAPIRefresh)

    // Protected endpoints
    api := router.Group("/api")
    api.Use(middleware.Session[*AppContext, AppData](jwtTransport))
    api.Use(middleware.RequireAuth[*AppContext, AppData]())
    api.GET("/profile", handleProfile)
}

func handleAPILogin(ctx *AppContext) handler.Response {
    userID := authenticateCredentials(ctx.Request())

    sess, tokens, err := jwtTransport.Authenticate(
        ctx.Context(),
        ctx.ResponseWriter(),
        ctx.Request(),
        userID,
    )
    if err != nil {
        return response.Error(response.ErrUnauthorized)
    }

    return response.JSON(map[string]any{
        "tokens": tokens,
        "user":   sess.UserID,
    })
}

func handleAPIRefresh(ctx *AppContext) handler.Response {
    var req struct {
        RefreshToken string `json:"refresh_token"`
    }
    if err := binder.Bind(ctx.Request(), &req); err != nil {
        return response.Error(response.ErrBadRequest)
    }

    sess, tokens, err := jwtTransport.Refresh(ctx.Context(), req.RefreshToken)
    if err != nil {
        return response.Error(response.ErrUnauthorized)
    }

    return response.JSON(map[string]any{
        "tokens": tokens,
        "user":   sess.UserID,
    })
}

func handleProfile(ctx *AppContext) handler.Response {
    sess := middleware.MustGetSession[AppData](ctx)

    return response.JSON(map[string]any{
        "user_id": sess.UserID,
        "data":    sess.Data,
    })
}
```

## Security Features

### Already Handled (via core/cookie)

- HttpOnly, Secure, SameSite cookies
- Signed cookies (prevents tampering)

### Built-in

- Token rotation on authentication (session fixation prevention)
- Refresh token rotation on JWT refresh (prevents token replay attacks)
- Short TTLs with touch intervals (extend active sessions)
- Automatic expiration checking
- Separate ID/Token (preserves cart while rotating credentials)
- Unified token approach (both Cookie and JWT use Session.Token)
- Periodic cleanup of expired sessions

### Optional (Add as Needed)

- Device fingerprinting via pkg/fingerprint
- IP logging
- Anomaly detection

## Testing Strategy

**Unit tests:**

- Manager: Pure logic, no HTTP
- Transport: Mock HTTP requests/responses
- Middleware: Mock handlers

**Integration tests:**

- Real store (Postgres/Redis)
- Full request/response cycle

## Implementation Checklist

- [ ] core/session/session.go - Session struct and token generation
- [ ] core/session/manager.go - Manager implementation (including CleanupExpired)
- [ ] core/session/store.go - Store interface (including DeleteExpired)
- [ ] core/session/errors.go - Error types
- [ ] core/sessiontransport/cookie.go - Cookie transport
- [ ] core/sessiontransport/jwt.go - JWT transport (with refresh token rotation)
- [ ] middleware/session.go - Session middleware (with touch error handling)
- [ ] Tests for all components
- [ ] Example application (including cleanup task setup)

## Design Decisions

### Why no interface for Transport?

Different transports have different APIs:

- Cookie: Simple load/save
- JWT: Returns token pairs on auth/refresh

**Don't force abstraction where behavior differs.**

### Why auto-touch in middleware?

Active sessions auto-extend without handler code. Transport decides how to handle touch (Cookie updates cookie, JWT only updates DB).

### Why separate Authenticate/Logout methods?

Explicit operations. Handler clearly shows intent.

### Why Session.Data generic?

Type-safe custom data. No type assertions in handlers.

### Why Token in JTI instead of Session.ID?

**Unified approach** - both Cookie and JWT use Session.Token:

- Cookie: Looks up by token from cookie
- JWT: Looks up by token from JTI claim
- Same code path: `manager.GetByToken()`
- Simpler, more consistent

### Why touch interval?

Extend active sessions without forcing re-authentication:

- User browsing site → touch every hour → session stays alive
- User inactive → no touch → expires after TTL
- Reduces database writes (only touch after interval passes)

### Why Touch on Transport instead of Manager in middleware?

**Clean layering** - Middleware depends only on Transport:
- Transport already has Manager internally
- Transport decides how to handle touch (Cookie updates cookie, JWT only DB)
- No redundant dependencies in middleware
- Proper encapsulation: `Manager → Transport → Middleware`

### Concurrent Touch Handling

**Race condition is acceptable** - when two requests touch the same session simultaneously:
- Both extend the session by the same TTL
- Last write wins for timestamps (harmless)
- No data corruption possible

**Optional DB-level prevention** (if needed):
```sql
UPDATE sessions
SET updated_at = $1, expires_at = $2
WHERE id = $3
  AND updated_at < $1 - touch_interval
```

Only updates if the interval has actually passed, preventing redundant writes.

### Touch Failure Handling

**Non-critical operation** - touch failures should not break requests:
- Log as WARNING and continue
- Session still valid, just not extended
- User can continue their request normally
- Next request will retry the touch

---

**Keep it simple. Each layer does ONE thing. No magic.**
