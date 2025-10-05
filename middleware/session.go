package middleware

import (
	"errors"
	"io"
	"log/slog"

	"github.com/dmitrymomot/foundation/core/handler"
	"github.com/dmitrymomot/foundation/core/response"
	"github.com/dmitrymomot/foundation/core/session"
)

type sessionKey struct{}

// SessionConfig configures the session middleware.
type SessionConfig[C handler.Context, Data any] struct {
	// Transport implements Load and Store methods for session management (required)
	Transport interface {
		Load(handler.Context) (session.Session[Data], error)
		Store(handler.Context, session.Session[Data]) error
	}
	// Logger for structured logging (default: slog with io.Discard)
	Logger *slog.Logger
	// Skip defines a function to skip middleware execution for specific requests
	Skip func(ctx C) bool
	// RequireAuth enforces authenticated user (UserID != uuid.Nil)
	// Returns ErrorHandler response if not authenticated
	RequireAuth bool
	// RequireGuest enforces guest/unauthenticated (UserID == uuid.Nil)
	// Returns ErrorHandler response if authenticated
	RequireGuest bool
	// ErrorHandler defines custom response for transport and auth failures
	// Default: returns response.Error(response.ErrUnauthorized)
	ErrorHandler func(ctx C, err error) handler.Response
}

// Session creates middleware that manages session lifecycle with strict error handling.
//
// The middleware follows this flow:
//  1. Load session from Transport → if error, delegate to ErrorHandler
//  2. Check auth requirements (RequireAuth/RequireGuest)
//  3. Store session in context
//  4. Execute handler
//  5. Get session from context
//  6. Store session via Transport → if error, delegate to ErrorHandler
//
// ALL Transport errors (Load and Store) are delegated to ErrorHandler.
// There is no graceful degradation - errors stop request processing.
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
//		sess := middleware.MustGetSession[MySessionData](ctx)
//		return response.JSON(map[string]any{
//			"session_id": sess.ID,
//			"user_id":    sess.UserID,
//		})
//	}
//
// The middleware automatically:
// - Creates a default logger that discards output (use SessionWithConfig for custom logger)
// - Delegates all transport errors to ErrorHandler (no silent failures)
// - Enforces strict error handling throughout the session lifecycle
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
// It manages session lifecycle with strict error handling and options for authentication enforcement.
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
//	// Custom logger with graceful degradation strategy
//	cfg := middleware.SessionConfig[*MyContext, MySessionData]{
//		Transport: sessionTransport,
//		Logger:    slog.New(slog.NewJSONHandler(os.Stdout, nil)),
//		ErrorHandler: func(ctx *MyContext, err error) handler.Response {
//			// Custom strategy: log and continue with empty session
//			logger.Error("session error", "error", err)
//			middleware.SetSession(ctx, session.Session[MySessionData]{})
//			return nil // Continue processing (ErrorHandler can return nil to continue)
//		},
//	}
//	r.Use(middleware.SessionWithConfig(cfg))
//
// Configuration options:
// - Transport: Session storage backend (required, panics if nil)
// - Logger: Structured logging (default: io.Discard)
// - RequireAuth: Enforce authenticated user
// - RequireGuest: Enforce guest/unauthenticated (panics if both RequireAuth and RequireGuest are true)
// - ErrorHandler: Custom response for transport and auth failures (default: response.ErrUnauthorized)
// - Skip: Skip processing for specific requests
//
// Session management best practices:
// - Use RequireAuth for protected routes
// - Use RequireGuest for login/signup pages
// - Customize ErrorHandler for better UX (redirects vs errors)
// - Include session logging in production
// - ErrorHandler decides error handling strategy (fail fast vs graceful degradation)
func SessionWithConfig[C handler.Context, Data any](cfg SessionConfig[C, Data]) handler.Middleware[C] {
	// Validate configuration
	if cfg.Transport == nil {
		panic("session middleware: transport is required")
	}

	if cfg.RequireAuth && cfg.RequireGuest {
		panic("session middleware: RequireAuth and RequireGuest cannot both be true")
	}

	// Set defaults
	if cfg.Logger == nil {
		cfg.Logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}

	if cfg.ErrorHandler == nil {
		cfg.ErrorHandler = func(ctx C, err error) handler.Response {
			return response.Error(err)
		}
	}

	return func(next handler.HandlerFunc[C]) handler.HandlerFunc[C] {
		return func(ctx C) handler.Response {
			// Skip middleware if configured
			if cfg.Skip != nil && cfg.Skip(ctx) {
				return next(ctx)
			}

			// 1. Load session from Transport
			sess, err := cfg.Transport.Load(ctx)
			if err != nil {
				if !errors.Is(err, session.ErrNotAuthenticated) {
					cfg.Logger.ErrorContext(ctx, "session load failed", "error", err)
				}
				return cfg.ErrorHandler(ctx, err)
			}

			// 2. Check auth requirements
			if cfg.RequireAuth && !sess.IsAuthenticated() {
				cfg.Logger.WarnContext(ctx, "authentication required but session not authenticated")
				return cfg.ErrorHandler(ctx, response.ErrUnauthorized)
			}

			if cfg.RequireGuest && sess.IsAuthenticated() {
				cfg.Logger.WarnContext(ctx, "guest required but session is authenticated")
				return cfg.ErrorHandler(ctx, response.ErrForbidden)
			}

			// 3. Store session in context
			ctx.SetValue(sessionKey{}, sess)

			// 4. Execute handler
			resp := next(ctx)

			// 5. Get session from context (handler may have mutated it)
			currentSess, ok := GetSession[Data](ctx)
			if !ok {
				// Session was removed from context, skip storage
				return resp
			}

			// 6. Store session via Transport
			if err := cfg.Transport.Store(ctx, currentSess); err != nil {
				if !errors.Is(err, session.ErrNotAuthenticated) {
					cfg.Logger.ErrorContext(ctx, "session store failed", "error", err)
				}
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
