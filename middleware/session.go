package middleware

import (
	"io"
	"log/slog"

	"github.com/dmitrymomot/foundation/core/handler"
	"github.com/dmitrymomot/foundation/core/response"
	"github.com/dmitrymomot/foundation/core/session"
)

type sessionKey struct{}

// SessionConfig configures the session middleware.
type SessionConfig[C handler.Context, Data any] struct {
	// Skip defines a function to skip middleware execution for specific requests
	Skip func(ctx C) bool
	// Transport implements Load and Store methods for session management
	Transport interface {
		Load(handler.Context) (session.Session[Data], error)
		Store(handler.Context, session.Session[Data]) error
	}
	// Logger for structured logging (default: slog with io.Discard)
	Logger *slog.Logger
	// RequireAuth enforces authenticated user (UserID != uuid.Nil)
	// Returns ErrorHandler response if not authenticated
	RequireAuth bool
	// RequireGuest enforces guest/unauthenticated (UserID == uuid.Nil)
	// Returns ErrorHandler response if authenticated
	RequireGuest bool
	// ErrorHandler defines custom response for auth failures
	// Default: returns response.Error(response.ErrUnauthorized)
	ErrorHandler func(ctx C, err error) handler.Response
}

// Session creates middleware that loads session from transport, stores it in context,
// and stores it after request completion.
//
// The middleware:
//   - Loads session from transport (logs errors but continues with empty session)
//   - Automatically captures client IP and User-Agent from HTTP headers
//   - Stores session in request context
//   - Processes the request
//   - Stores session after request (logs errors and returns error response)
//
// Transport must implement Load and Store methods for session management.
// Transport implementations automatically extract IP via clientip.GetIP() and
// User-Agent from request headers when creating new sessions.
//
// Usage:
//
//	// Apply session middleware to all routes
//	r.Use(middleware.Session[*MyContext, MySessionData](sessionTransport))
//
//	// Use session in handlers
//	func handleDashboard(ctx *MyContext) handler.Response {
//		sess, ok := middleware.GetSession[MySessionData](ctx)
//		if !ok {
//			return response.Error(response.ErrInternalServerError)
//		}
//
//		return response.JSON(map[string]any{
//			"session_id": sess.ID,
//			"user_id":    sess.UserID,
//		})
//	}
//
// The middleware automatically:
// - Creates a default logger that discards output (use SessionWithConfig for custom logger)
// - Allows graceful degradation on session load errors
// - Delegates all store errors to ErrorHandler
func Session[C handler.Context, Data any](
	transport interface {
		Load(handler.Context) (session.Session[Data], error)
		Store(handler.Context, session.Session[Data]) error
	},
) handler.Middleware[C] {
	return SessionWithConfig[C, Data](SessionConfig[C, Data]{
		Transport: transport,
		Logger:    slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
}

// SessionWithConfig creates a session middleware with custom configuration.
// It manages session lifecycle with options for authentication enforcement and custom error handling.
//
// Advanced Usage Examples:
//
//	// Require authentication
//	cfg := middleware.SessionConfig[*MyContext, MySessionData]{
//		Transport:   sessionTransport,
//		RequireAuth: true,
//	}
//	r.Use(middleware.SessionWithConfig(cfg))
//
//	// Require guest (not authenticated) with custom redirect
//	cfg := middleware.SessionConfig[*MyContext, MySessionData]{
//		Transport:    sessionTransport,
//		RequireGuest: true,
//		ErrorHandler: func(ctx *MyContext, err error) handler.Response {
//			return response.Redirect("/dashboard")
//		},
//	}
//	publicRoutes.Use(middleware.SessionWithConfig(cfg))
//
//	// Skip session for health checks
//	cfg := middleware.SessionConfig[*MyContext, MySessionData]{
//		Transport: sessionTransport,
//		Skip: func(ctx *MyContext) bool {
//			path := ctx.Request().URL.Path
//			return path == "/health" || path == "/metrics"
//		},
//	}
//	r.Use(middleware.SessionWithConfig(cfg))
//
//	// Custom error handling for auth failures
//	cfg := middleware.SessionConfig[*MyContext, MySessionData]{
//		Transport:   sessionTransport,
//		RequireAuth: true,
//		Logger:      logger,
//		ErrorHandler: func(ctx *MyContext, err error) handler.Response {
//			logger.Warn("Authentication required", "path", ctx.Request().URL.Path)
//			return response.Redirect("/login?return=" + ctx.Request().URL.Path)
//		},
//	}
//	protectedRoutes.Use(middleware.SessionWithConfig(cfg))
//
//	// Custom logger with context
//	cfg := middleware.SessionConfig[*MyContext, MySessionData]{
//		Transport: sessionTransport,
//		Logger:    slog.New(slog.NewJSONHandler(os.Stdout, nil)),
//	}
//	r.Use(middleware.SessionWithConfig(cfg))
//
// Configuration options:
// - Transport: Session storage backend (required)
// - Logger: Structured logging (default: io.Discard)
// - RequireAuth: Enforce authenticated user
// - RequireGuest: Enforce guest/unauthenticated
// - ErrorHandler: Custom auth failure response
// - Skip: Skip processing for specific requests
//
// Session management best practices:
// - Use RequireAuth for protected routes
// - Use RequireGuest for login/signup pages
// - Customize ErrorHandler for better UX (redirects vs errors)
// - Include session logging in production
func SessionWithConfig[C handler.Context, Data any](cfg SessionConfig[C, Data]) handler.Middleware[C] {
	if cfg.Transport == nil {
		panic("session middleware: transport is required")
	}

	if cfg.RequireAuth && cfg.RequireGuest {
		panic("session middleware: RequireAuth and RequireGuest cannot both be true")
	}

	if cfg.Logger == nil {
		cfg.Logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}

	if cfg.ErrorHandler == nil {
		cfg.ErrorHandler = func(ctx C, err error) handler.Response {
			return response.Error(response.ErrUnauthorized)
		}
	}

	return func(next handler.HandlerFunc[C]) handler.HandlerFunc[C] {
		return func(ctx C) handler.Response {
			if cfg.Skip != nil && cfg.Skip(ctx) {
				return next(ctx)
			}

			sess, err := cfg.Transport.Load(ctx)
			if err != nil {
				if ctxErr := ctx.Err(); ctxErr != nil {
					return response.Error(ctxErr)
				}
				cfg.Logger.ErrorContext(ctx, "session middleware: failed to load session", "error", err)
				// Allow graceful degradation instead of failing the request
				sess = session.Session[Data]{}
			}

			// Check authentication requirements
			if cfg.RequireAuth && !sess.IsAuthenticated() {
				return cfg.ErrorHandler(ctx, response.ErrUnauthorized)
			}

			if cfg.RequireGuest && sess.IsAuthenticated() {
				return cfg.ErrorHandler(ctx, response.ErrForbidden)
			}

			ctx.SetValue(sessionKey{}, sess)

			resp := next(ctx)

			// Get current session (handler may have mutated it)
			currentSess, ok := GetSession[Data](ctx)
			if !ok {
				return resp // Session removed from context
			}

			// ALWAYS store - delegate ALL errors to ErrorHandler
			if err := cfg.Transport.Store(ctx, currentSess); err != nil {
				cfg.Logger.ErrorContext(ctx, "session store failed", "error", err)
				return cfg.ErrorHandler(ctx, err)
			}

			return resp
		}
	}
}

// GetSession retrieves session from context.
// Returns the session and true if found, empty session and false otherwise.
func GetSession[Data any](ctx handler.Context) (session.Session[Data], bool) {
	if ctx == nil {
		return session.Session[Data]{}, false
	}

	if sess, ok := ctx.Value(sessionKey{}).(session.Session[Data]); ok {
		return sess, true
	}

	return session.Session[Data]{}, false
}

// MustGetSession retrieves session from context or panics if not found.
// Use this when session existence is guaranteed by middleware.
func MustGetSession[Data any](ctx handler.Context) session.Session[Data] {
	sess, ok := GetSession[Data](ctx)
	if !ok {
		panic("session not found in context")
	}
	return sess
}

// SetSession updates session in context.
// Use this to store modified session state during request processing.
func SetSession[Data any](ctx handler.Context, sess session.Session[Data]) {
	ctx.SetValue(sessionKey{}, sess)
}
