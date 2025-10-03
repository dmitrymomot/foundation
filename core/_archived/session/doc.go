// Package session provides secure, generic session management for Go web applications.
//
// This package supports both anonymous and authenticated sessions with configurable
// persistence and transport. Sessions use cryptographically secure tokens with
// automatic expiration, token rotation on authentication, and device tracking.
//
// # Basic Usage
//
// Create a session manager with your custom data type:
//
//	type UserData struct {
//		Theme    string `json:"theme"`
//		Language string `json:"language"`
//	}
//
//	manager, err := session.New[UserData](
//		session.WithStore(myStore),
//		session.WithTransport(myTransport),
//		session.WithConfig(
//			session.WithTTL(24 * time.Hour),
//			session.WithTouchInterval(5 * time.Minute),
//		),
//	)
//
// Handle sessions in HTTP handlers:
//
//	func handler(w http.ResponseWriter, r *http.Request) {
//		sess, err := manager.Load(w, r)
//		if err != nil {
//			http.Error(w, "Session error", http.StatusInternalServerError)
//			return
//		}
//
//		if sess.IsAuthenticated() {
//			fmt.Fprintf(w, "Hello user %s", sess.UserID)
//		}
//
//		sess.Data.Theme = "dark"
//		manager.Save(w, r, sess)
//	}
//
// # Authentication Flow
//
// Sessions start anonymous and upgrade to authenticated:
//
//	// Login
//	func login(w http.ResponseWriter, r *http.Request) {
//		userID := authenticateUser(r)
//		manager.Auth(w, r, userID)  // Rotates token for security
//		http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
//	}
//
//	// Logout - preserves DeviceID for analytics
//	func logout(w http.ResponseWriter, r *http.Request) {
//		manager.Logout(w, r, session.PreserveData(func(old UserData) UserData {
//			return UserData{Theme: old.Theme}  // Keep preferences
//		}))
//		http.Redirect(w, r, "/", http.StatusSeeOther)
//	}
//
// # Configuration
//
// Load from environment variables:
//
//	config := session.DefaultConfig()
//	env.Parse(&config)  // SESSION_TTL=24h, SESSION_TOUCH_INTERVAL=5m
//
//	manager, err := session.NewFromConfig[UserData](
//		config,
//		session.WithStore(myStore),
//		session.WithTransport(myTransport),
//	)
//
// # Core Components
//
//   - Session[Data]: Generic session container with application-defined data
//   - Manager[Data]: Coordinates session lifecycle (Load, Save, Auth, Logout, Delete)
//   - Store[Data]: Interface for persistence (Redis, database, etc.)
//   - Transport: Interface for token transmission (cookies, headers, etc.)
//
// # Session Structure
//
// Each session contains:
//   - ID: Stable identifier
//   - Token/TokenHash: Secure authentication token
//   - DeviceID: Persistent device identifier for analytics
//   - UserID: Set after authentication (uuid.Nil for anonymous)
//   - Data: Application-defined generic data
//   - Timestamps: CreatedAt, UpdatedAt, ExpiresAt
//
// # Security Features
//
//   - Cryptographically secure tokens (32 bytes)
//   - Token rotation on authentication
//   - Configurable TTL expiration
//   - Touch interval throttling (prevents DoS)
//   - Thread-safe value semantics
//
// # Implementation Requirements
//
// Implement Store and Transport interfaces:
//
//	type Store[Data any] interface {
//		Get(ctx context.Context, tokenHash string) (Session[Data], error)
//		Store(ctx context.Context, session Session[Data]) error
//		Delete(ctx context.Context, id uuid.UUID) error
//	}
//
//	type Transport interface {
//		Extract(r *http.Request) (token string, err error)
//		Embed(w http.ResponseWriter, r *http.Request, token string, ttl time.Duration) error
//		Revoke(w http.ResponseWriter, r *http.Request) error
//	}
//
// Common implementations: Redis/PostgreSQL stores, cookie/header transports.
//
// # Performance
//
// TouchInterval balances activity tracking with storage performance. A 5-minute
// interval reduces writes significantly while maintaining reasonable tracking.
// Change detection automatically skips unnecessary store writes.
package session
