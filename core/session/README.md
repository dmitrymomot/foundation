# Session Package

A type-safe, pluggable session management framework for Go applications that supports multiple transport mechanisms (cookies, headers, JWT) and storage backends.

## Design Philosophy

This framework is designed for building multiple micro-SaaS applications with consistent session handling:

- **Type-Safe Data**: Generic `Session[Data]` allows each app to define its session structure
- **Transport Agnostic**: Pluggable transports for cookies, headers, and JWT
- **Storage Agnostic**: Interface-based storage (cookie store provided, others implemented per app)
- **UUID-Based**: Consistent use of `uuid.UUID` for all identifiers
- **Analytics-Ready**: Built-in DeviceID for tracking user journeys across auth states
- **Zero Ambiguity**: Compile-time type safety over runtime flexibility

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
│   Cookie    │    Redis     │   Database   │
│   Store     │    Store*    │    Store*    │
│  (provided) │ (implement)  │ (implement)  │
└─────────────┴──────────────┴──────────────┘
```

## Core Components

### 1. Session (Generic)

The session is generic over the Data type, allowing each application to define its own session data structure:

```go
type Session[Data any] struct {
    ID         uuid.UUID  `json:"id"`         // Session token identifier
    DeviceID   uuid.UUID  `json:"device_id"`  // Persistent device/browser ID (survives auth)
    UserID     uuid.UUID  `json:"user_id"`    // Real user ID (uuid.Nil for anonymous)
    Data       Data       `json:"data"`       // Application-defined data structure
    ExpiresAt  time.Time  `json:"expires_at"`
    CreatedAt  time.Time  `json:"created_at"`
    UpdatedAt  time.Time  `json:"updated_at"`
}
```

**Identity Management**:

- `DeviceID`: Generated once per device/browser, persists through login/logout cycles
- `UserID`: `uuid.Nil` for anonymous, populated with user's UUID on authentication
- Enables complete user journey tracking in analytics platforms (Mixpanel pattern)

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

### 3. Store Interface (Generic)

Store interface is generic to match the session type:

```go
type Store[Data any] interface {
    // Get retrieves a session by ID
    Get(ctx context.Context, id uuid.UUID) (*Session[Data], error)

    // Set saves or updates a session
    Set(ctx context.Context, session *Session[Data]) error

    // Delete removes a session
    Delete(ctx context.Context, id uuid.UUID) error

    // DeleteExpired removes all expired sessions (optional)
    DeleteExpired(ctx context.Context) error

    // DeleteByUserID removes all sessions for a user (optional)
    DeleteByUserID(ctx context.Context, userID uuid.UUID) error
}
```

**Important**: The package provides **only a cookie-based store** as the default implementation. All other storage backends (Redis, PostgreSQL, etc.) must be implemented per application. No in-memory store is provided - use cookie store or mocks for testing.

### 4. Manager (Generic)

Manager is generic to support typed session data:

```go
type Manager[Data any] struct {
    store     Store[Data]
    transport Transport
    config    Config
}

// Constructor
func New[Data any](opts ...Option) *Manager[Data] {
    // Returns manager for specific Data type
}
```

### 5. Session Linking (Anonymous → Authenticated)

The package supports linking anonymous sessions to authenticated users:

```go
// Manager methods for session linking
func (m *Manager[Data]) Link(ctx context.Context, w http.ResponseWriter, r *http.Request, userID uuid.UUID) error {
    // Upgrades anonymous session to authenticated
    // - Rotates session ID for security
    // - Preserves DeviceID for analytics continuity
    // - Populates UserID field
    // - Maintains all session data
}

func (m *Manager[Data]) LinkAndMerge(ctx context.Context, w http.ResponseWriter, r *http.Request, userID uuid.UUID, merge MergeFunc[Data]) error {
    // Links and merges with existing user sessions from other devices
}

// MergeFunc for combining session data
type MergeFunc[Data any] func(anonymous, existing *Session[Data]) *Session[Data]
```

**Identity Linking Flow**:

```go
// 1. Anonymous session
Session[Data] {
    ID:       uuid.New(),         // Session token
    DeviceID: uuid.New(),         // Persists through auth
    UserID:   uuid.Nil,           // Empty (anonymous)
    Data:     Data{...},          // App-specific data
}

// 2. After authentication
Session[Data] {
    ID:       uuid.New(),         // Rotated for security
    DeviceID: <same>,             // Unchanged for tracking
    UserID:   <user's UUID>,      // Now populated
    Data:     Data{...},          // Preserved
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

### 1. Define Your Session Data

```go
// Each app defines its session data structure
type MyAppData struct {
    // Required fields for SaaS
    OrgID       uuid.UUID         `json:"org_id,omitempty"`
    Plan        string            `json:"plan,omitempty"`
    Permissions []string          `json:"permissions,omitempty"`

    // User preferences
    Theme       string            `json:"theme,omitempty"`
    Locale      string            `json:"locale,omitempty"`

    // Temporary data
    Flash       map[string]string `json:"flash,omitempty"`
}

// Initialize manager with your data type
var SessionManager = session.New[MyAppData](
    session.WithTransport(session.NewCookieTransport("sid")),
    session.WithTTL(2 * time.Hour),
)
```

### 2. Web Application (Cookie-based)

```go
// Middleware with typed session
func SessionMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        sess, _ := SessionManager.LoadOrCreate(r.Context(), w, r)
        ctx := session.WithSession(r.Context(), sess)
        next.ServeHTTP(w, r.WithContext(ctx))
    })
}

// Handler with typed access
func Handler(w http.ResponseWriter, r *http.Request) {
    sess := session.FromContext[MyAppData](r.Context())

    // Direct typed access - no type assertions!
    sess.Data.Theme = "dark"
    sess.Data.Permissions = []string{"read", "write"}

    SessionManager.Save(r.Context(), w, r, sess)
}
```

### 3. API Server (Token-based)

```go
// API-specific session data
type APISessionData struct {
    Scopes      []string  `json:"scopes"`
    RateLimits  RateLimit `json:"rate_limits"`
    ClientID    uuid.UUID `json:"client_id"`
}

// Initialize with header transport and custom store
var APIManager = session.New[APISessionData](
    session.WithStore(redisStore), // Custom Redis store
    session.WithTransport(session.NewHeaderTransport("Authorization", "Bearer")),
    session.WithTTL(24 * time.Hour),
)

// API middleware
func APIAuthMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        sess, err := APIManager.Load(r.Context(), r)
        if err != nil {
            http.Error(w, "Unauthorized", http.StatusUnauthorized)
            return
        }

        // Check scopes with typed data
        if !hasScope(sess.Data.Scopes, "read:data") {
            http.Error(w, "Forbidden", http.StatusForbidden)
            return
        }

        ctx := session.WithSession(r.Context(), sess)
        next.ServeHTTP(w, r.WithContext(ctx))
    })
}
```

### 4. Hybrid Monolith (Multiple Transports)

```go
// Shared session data structure for both web and API
type AppData struct {
    OrgID       uuid.UUID `json:"org_id,omitempty"`
    Permissions []string  `json:"permissions,omitempty"`
    // ... other fields
}

// Cookie transport for web routes
var WebManager = session.New[AppData](
    session.WithStore(sharedStore),
    session.WithTransport(session.NewCookieTransport("sid")),
)

// Token transport for API routes (same data type)
var APIManager = session.New[AppData](
    session.WithStore(sharedStore), // Same store instance
    session.WithTransport(session.NewHeaderTransport("Authorization", "Bearer")),
)

// Route setup
mux.Handle("/web/", SessionMiddleware(WebManager, webHandler))
mux.Handle("/api/", APIMiddleware(APIManager, apiHandler))
```

### 5. Session Linking (Registration/Login Flow)

```go
func LoginHandler(w http.ResponseWriter, r *http.Request) {
    // Get typed anonymous session
    sess := session.FromContext[MyAppData](r.Context())

    // Track with DeviceID (Mixpanel pattern)
    analytics.Track("", "login_started", map[string]any{
        "$device_id": sess.DeviceID.String(),
    })

    // Authenticate and get user UUID
    userID, err := authenticateUser(r)
    if err != nil {
        return
    }

    // Link anonymous session to user
    err = SessionManager.Link(r.Context(), w, r, userID)
    if err != nil {
        return
    }

    // Create identity cluster in analytics
    analytics.Identify(userID.String(), map[string]any{
        "$device_id": sess.DeviceID.String(),
        "org_id": sess.Data.OrgID.String(),
    })

    // Session state after linking:
    // - UserID: populated with user's UUID
    // - DeviceID: unchanged (analytics continuity)
    // - ID: rotated (security)
    // - Data: preserved (typed and intact)
}

// Merge sessions from multiple devices
func LoginWithMergeHandler(w http.ResponseWriter, r *http.Request) {
    userID, _ := authenticateUser(r)

    // Typed merge function
    mergeFunc := func(anon, existing *Session[MyAppData]) *Session[MyAppData] {
        // Merge with type safety
        existing.Data.Flash = anon.Data.Flash  // Keep flash messages
        if anon.Data.Theme != "" {
            existing.Data.Theme = anon.Data.Theme  // Prefer anonymous choice
        }
        return existing
    }

    SessionManager.LinkAndMerge(r.Context(), w, r, userID, mergeFunc)
}
```

### 5. Logout with Data Preservation

```go
// Simple logout - clears everything except DeviceID
func LogoutHandler(w http.ResponseWriter, r *http.Request) {
    sessionManager.Logout(w, r)
}

// Logout preserving user preferences
func SmartLogoutHandler(w http.ResponseWriter, r *http.Request) {
    sessionManager.Logout(w, r, session.PreserveData(func(old MyAppData) MyAppData {
        return MyAppData{
            Theme:  old.Theme,   // Keep theme preference
            Locale: old.Locale,  // Keep language setting
            // All other fields (UserID, OrgID, Permissions) are zeroed
        }
    }))
}

// Define reusable logout behavior
var PreservePreferences = session.PreserveData(func(old MyAppData) MyAppData {
    return MyAppData{
        Theme:    old.Theme,
        Locale:   old.Locale,
        Timezone: old.Timezone,
    }
})

// Use anywhere in your app
sessionManager.Logout(w, r, PreservePreferences)

// E-commerce example - preserve shopping cart
var PreserveCart = session.PreserveData(func(old ShopData) ShopData {
    return ShopData{
        CartItems: old.CartItems,  // Keep shopping cart
        Currency:  old.Currency,   // Keep currency preference
        // Clear: UserID, PaymentMethods, OrderHistory
    }
})
```

### 6. JWT with Refresh Tokens

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
7. **Session Linking Security**:
    - Always rotate tokens when linking anonymous to authenticated
    - Validate session ownership before linking
    - Consider rate limiting on login attempts
    - Clear sensitive anonymous data after linking
    - Log session linking events for audit trails

## Performance Considerations

1. **Lazy Loading**: Load sessions only when needed
2. **Batch Operations**: Support bulk delete for cleanup
3. **Caching**: Consider caching layer for high-traffic scenarios
4. **Connection Pooling**: Use connection pools for external stores
5. **Minimal Allocations**: Reuse buffers where possible

## Key Design Decisions

1. **Generic Session[Data]**: Each app defines its own typed session structure
2. **UUID Everything**: Consistent `uuid.UUID` for all identifiers (no string conversions)
3. **No Memory Store**: Only cookie store provided, others implemented per app
4. **DeviceID Persistence**: Survives auth cycles for complete analytics tracking
5. **No Background Workers**: Lazy expiration on access
6. **Type Safety First**: Compile-time guarantees over runtime flexibility

## Analytics Integration (Mixpanel Pattern)

The framework follows Mixpanel's identity management pattern for complete user journey tracking:

```go
// Track anonymous user
func TrackAnonymousUser(sess *Session[MyAppData]) {
    analytics.Track("", "page_viewed", map[string]any{
        "$device_id": sess.DeviceID.String(),
        "page": "/products",
    })
}

// After authentication, link identities
func TrackAuthenticatedUser(sess *Session[MyAppData]) {
    // Create identity cluster
    analytics.Identify(sess.UserID.String(), map[string]any{
        "$device_id": sess.DeviceID.String(),
        "org_id": sess.Data.OrgID.String(),
    })

    // Future events include both IDs
    analytics.Track("", "purchase_completed", map[string]any{
        "$device_id": sess.DeviceID.String(),
        "$user_id": sess.UserID.String(),
        "amount": 99.99,
    })
}

// On logout
func HandleLogout(sess *Session[MyAppData]) {
    sess.UserID = uuid.Nil      // Clear user
    sess.ID = uuid.New()         // Rotate token
    // DeviceID persists for next session

    analytics.Track("", "user_logged_out", map[string]any{
        "$device_id": sess.DeviceID.String(),
    })
}
```

### Key Principles (Mixpanel-Compatible)

1. **DeviceID Persistence**: Survives login/logout cycles for complete device tracking
2. **Identity Clustering**: Analytics platforms link DeviceID ↔ UserID automatically
3. **Retroactive Attribution**: Past anonymous events get associated with user
4. **Multi-Device Support**: Each device has its own DeviceID, all linked to same UserID
5. **Privacy Compliant**: DeviceID contains no PII, UserID only after consent

### Implementation Notes

- **Signup**: Call `identify()` to create the identity link
- **Login**: Call `identify()` to restore the identity link
- **Logout**: Keep DeviceID, clear UserID, rotate session token
- **New Device**: Gets new DeviceID, linked to UserID on login

## Testing

Testing approach for typed sessions:

```go
// Test data structure
type TestData struct {
    Value string `json:"value"`
    Count int    `json:"count"`
}

// Using cookie store with httptest
func TestSession(t *testing.T) {
    manager := session.New[TestData](
        session.WithTTL(1 * time.Hour),
    )

    req := httptest.NewRequest("GET", "/", nil)
    rec := httptest.NewRecorder()

    sess, _ := manager.LoadOrCreate(context.Background(), rec, req)
    sess.Data.Value = "test"
    sess.Data.Count = 42

    // Assert typed data
    assert.Equal(t, "test", sess.Data.Value)
}

// Mock store for unit tests
type mockStore[Data any] struct {
    sessions map[uuid.UUID]*Session[Data]
}

func (m *mockStore[Data]) Get(ctx context.Context, id uuid.UUID) (*Session[Data], error) {
    sess, ok := m.sessions[id]
    if !ok {
        return nil, ErrSessionNotFound
    }
    return sess, nil
}

func (m *mockStore[Data]) Set(ctx context.Context, sess *Session[Data]) error {
    m.sessions[sess.ID] = sess
    return nil
}
// ... other methods
```

**Note**: No in-memory store is provided. Use cookie store for integration tests or create mocks for unit tests.

## Extension Points

The framework is designed for extensibility:

1. **Custom Transports**: Implement the Transport interface for new token mechanisms
2. **Custom Stores**: Implement `Store[Data]` for Redis, PostgreSQL, DynamoDB, etc.
3. **Session Events**: Hook into lifecycle (create, authenticate, expire, destroy)
4. **Validation**: Add custom validation for session data
5. **Middleware**: Wrap managers for logging, metrics, rate limiting

Example custom store:

```go
type PostgresStore[Data any] struct {
    db *sql.DB
}

func (s *PostgresStore[Data]) Get(ctx context.Context, id uuid.UUID) (*Session[Data], error) {
    var sess Session[Data]
    var dataJSON []byte

    err := s.db.QueryRowContext(ctx,
        "SELECT id, device_id, user_id, data, expires_at FROM sessions WHERE id = $1",
        id,
    ).Scan(&sess.ID, &sess.DeviceID, &sess.UserID, &dataJSON, &sess.ExpiresAt)

    if err != nil {
        return nil, err
    }

    err = json.Unmarshal(dataJSON, &sess.Data)
    return &sess, err
}
```
