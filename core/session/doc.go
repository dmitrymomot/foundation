// Package session provides secure, generic session management for Go web applications.
//
// This package implements a flexible session system that supports both anonymous
// and authenticated sessions with configurable persistence and transport mechanisms.
// Sessions use cryptographically secure tokens and support automatic expiration,
// token rotation on authentication, and device tracking for analytics.
//
// # Core Components
//
// The package provides four main types:
//
//   - Session[Data]: Generic session container with application-defined data
//   - Manager[Data]: Coordinates session lifecycle operations
//   - Store[Data]: Interface for session persistence (Redis, database, etc.)
//   - Transport: Interface for token transmission (cookies, headers, etc.)
//
// # Basic Usage
//
// Create a session manager with custom data type:
//
//	import "github.com/dmitrymomot/foundation/core/session"
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
//	if err != nil {
//		log.Fatal(err)
//	}
//
// Handle requests with session loading:
//
//	func handler(w http.ResponseWriter, r *http.Request) {
//		sess, err := manager.Load(w, r)
//		if err != nil {
//			http.Error(w, "Session error", http.StatusInternalServerError)
//			return
//		}
//
//		// Work with session
//		if sess.IsAuthenticated() {
//			fmt.Fprintf(w, "Hello user %s", sess.UserID)
//		} else {
//			fmt.Fprintf(w, "Hello anonymous user")
//		}
//
//		// Modify session data
//		sess.Data.Theme = "dark"
//
//		// Save changes
//		if err := manager.Save(w, r, sess); err != nil {
//			log.Printf("Failed to save session: %v", err)
//		}
//	}
//
// # Authentication
//
// Sessions start as anonymous and can be upgraded to authenticated:
//
//	// Login endpoint
//	func login(w http.ResponseWriter, r *http.Request) {
//		userID := authenticateUser(r) // Your authentication logic
//
//		// Upgrade session to authenticated (rotates token for security)
//		if err := manager.Auth(w, r, userID); err != nil {
//			http.Error(w, "Auth failed", http.StatusInternalServerError)
//			return
//		}
//
//		http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
//	}
//
//	// Logout endpoint
//	func logout(w http.ResponseWriter, r *http.Request) {
//		// Option 1: Return to anonymous state (preserves DeviceID and analytics)
//		err := manager.Logout(w, r, session.PreserveData(func(old UserData) UserData {
//			return UserData{
//				Theme:    old.Theme,    // Keep user preferences
//				Language: old.Language,
//				// Other fields are zeroed
//			}
//		}))
//
//		// Option 2: Complete session deletion
//		// err := manager.Delete(w, r)
//
//		if err != nil {
//			log.Printf("Logout error: %v", err)
//		}
//		http.Redirect(w, r, "/", http.StatusSeeOther)
//	}
//
// # Session States and Lifecycle
//
// Sessions have three main states:
//
//   - Anonymous: Created automatically, has DeviceID for analytics
//   - Authenticated: Has UserID after successful login
//   - Expired: Past TTL, automatically replaced with new anonymous session
//
// Each session contains:
//
//   - ID: Stable session identifier (never changes)
//   - Token: Rotatable secure token for authentication
//   - DeviceID: Persistent device/browser identifier
//   - UserID: Set after authentication (uuid.Nil for anonymous)
//   - Data: Application-defined generic data
//   - Timestamps: Creation, update, and expiration times
//
// # Manager Methods
//
// The Manager[Data] type provides the following public methods:
//
//   - Load(w, r) (Session[Data], error): Load existing session or create new anonymous one
//   - Save(w, r, session) error: Persist session changes to store and response
//   - Touch(w, r) error: Extend session expiration on user activity
//   - Auth(w, r, userID) error: Authenticate session with user ID, rotates token
//   - Logout(w, r, ...opts) error: Return session to anonymous state
//   - Delete(w, r) error: Completely remove session from store and client
//
// # Session Methods
//
// The Session[Data] type provides these methods:
//
//   - IsAuthenticated() bool: Returns true if session has valid user ID
//   - IsExpired() bool: Returns true if session has expired
//
// # Security Features
//
// The session system includes several security mechanisms:
//
//   - Cryptographically secure tokens (32 bytes, base64url encoded)
//   - Automatic token rotation on authentication
//   - Configurable session expiration (TTL)
//   - Touch interval throttling to prevent DoS attacks
//   - Separation of concerns between transport and storage
//
// # Configuration Options
//
// Sessions can be configured with:
//
//	session.WithTTL(duration)              // Session lifetime (default: 24h)
//	session.WithTouchInterval(duration)    // Min time between activity updates (default: 5m)
//
// TTL determines how long sessions remain valid. TouchInterval prevents excessive
// storage writes by limiting how frequently session activity updates are recorded.
// Set TouchInterval to 0 to disable auto-touch functionality.
//
// # Error Handling
//
// The package defines comprehensive error types:
//
//   - ErrSessionNotFound: Session doesn't exist in store
//   - ErrSessionExpired: Session has passed expiration time
//   - ErrInvalidToken: Malformed or invalid session token
//   - ErrInvalidUserID: Invalid user ID for authentication
//   - ErrNoTransport: No transport configured
//   - ErrNoStore: No store configured
//   - ErrTokenGeneration: Cryptographic token generation failed
//   - ErrNoToken: No token found in transport
//   - ErrTransportFailed: Transport operation failed
//
// # Implementation Requirements
//
// To use this package, implement the Store and Transport interfaces:
//
//	// Store interface for session persistence
//	type Store[Data any] interface {
//		Get(ctx context.Context, token string) (Session[Data], error)
//		Store(ctx context.Context, session Session[Data]) error
//		Delete(ctx context.Context, id uuid.UUID) error
//	}
//
//	// Transport interface for token transmission
//	type Transport interface {
//		Extract(r *http.Request) (token string, err error)
//		Embed(w http.ResponseWriter, r *http.Request, token string, ttl time.Duration) error
//		Revoke(w http.ResponseWriter, r *http.Request) error
//	}
//
// Common implementations might use:
//   - Store: Redis, PostgreSQL, MongoDB, or in-memory for testing
//   - Transport: HTTP cookies, Authorization headers, or custom schemes
//
// # Thread Safety
//
// The session system uses value semantics throughout to ensure thread safety.
// Sessions are copied when retrieved from storage and when passed to storage,
// preventing race conditions in concurrent environments.
//
// # Performance Considerations
//
// Use TouchInterval to balance between activity tracking accuracy and storage
// performance. A 5-minute interval reduces writes by up to 99% while maintaining
// reasonable session activity tracking.
//
// For high-traffic applications, consider:
//   - Redis-based store for fast session lookups
//   - Cookie-based transport to reduce server-side storage
//   - Appropriate TTL values based on user behavior patterns
package session
