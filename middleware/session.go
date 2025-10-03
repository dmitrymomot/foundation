package middleware

import (
	"log/slog"
	"net/http"

	"github.com/dmitrymomot/foundation/core/handler"
	"github.com/dmitrymomot/foundation/core/response"
	"github.com/dmitrymomot/foundation/core/session"
)

// sessionContextKey is used as a key for storing session in request context.
type sessionContextKey struct{}

// SessionConfig configures the session middleware.
type SessionConfig[Data any] struct {
	// Skip defines a function to skip middleware execution for specific requests
	Skip func(ctx handler.Context) bool

	// Manager is the session manager instance used for loading and saving sessions (required)
	Manager *session.Manager[Data]

	// ErrorHandler defines how to handle session errors (default: returns 500 Internal Server Error)
	// This is called when session loading fails. Session save errors are logged but don't fail the request.
	ErrorHandler func(ctx handler.Context, err error) handler.Response

	// AutoSave enables automatic session saving after handler execution (default: true)
	// When disabled, handlers must manually call manager.Save() to persist changes
	AutoSave bool

	// Logger for session middleware operations (default: slog.Default())
	Logger *slog.Logger
}

// Session creates a session middleware with a session manager.
// It automatically loads sessions before handlers and saves them after.
// Sessions are stored in the request context and can be accessed using GetSession.
//
// This is the most common way to add session management to your application.
//
// Usage:
//
//	// Create session manager
//	manager, err := session.New[UserData](
//		session.WithStore(myStore),
//		session.WithTransport(myTransport),
//	)
//	if err != nil {
//		log.Fatal(err)
//	}
//
//	// Apply session middleware globally
//	r.Use(middleware.Session[*MyContext, UserData](manager))
//
//	// Access session in handlers
//	func handleProfile(ctx *MyContext) handler.Response {
//		sess, ok := middleware.GetSession[UserData](ctx)
//		if !ok {
//			return response.Error(response.ErrInternalServer)
//		}
//
//		// Modify session data
//		sess.Data.Theme = "dark"
//		middleware.SetSession(ctx, sess)
//
//		return response.JSON(map[string]any{
//			"user_id": sess.UserID.String(),
//			"theme":   sess.Data.Theme,
//		})
//	}
//
// The middleware automatically:
// - Loads existing session or creates new anonymous session
// - Stores session in request context for handler access
// - Saves session changes after handler completes
// - Returns 500 Internal Server Error if session loading fails
func Session[C handler.Context, Data any](manager *session.Manager[Data]) handler.Middleware[C] {
	return SessionWithConfig[C, Data](SessionConfig[Data]{
		Manager:  manager,
		AutoSave: true,
	})
}

// SessionWithConfig creates a session middleware with custom configuration.
// It provides fine-grained control over session loading, saving, and error handling.
//
// Advanced Usage Examples:
//
//	// Skip session for public endpoints
//	cfg := middleware.SessionConfig[UserData]{
//		Manager: manager,
//		Skip: func(ctx handler.Context) bool {
//			path := ctx.Request().URL.Path
//			return path == "/health" || strings.HasPrefix(path, "/public/")
//		},
//	}
//	r.Use(middleware.SessionWithConfig[*MyContext, UserData](cfg))
//
//	// Custom error handling
//	cfg := middleware.SessionConfig[UserData]{
//		Manager: manager,
//		ErrorHandler: func(ctx handler.Context, err error) handler.Response {
//			log.Printf("Session error: %v", err)
//			return response.Error(response.ErrInternalServer.WithMessage("Session unavailable"))
//		},
//	}
//
//	// Manual session saving (disable auto-save)
//	cfg := middleware.SessionConfig[UserData]{
//		Manager:  manager,
//		AutoSave: false, // Handlers must call manager.Save() manually
//	}
//	r.Use(middleware.SessionWithConfig[*MyContext, UserData](cfg))
//
//	// With custom logger
//	cfg := middleware.SessionConfig[UserData]{
//		Manager: manager,
//		Logger:  slog.New(slog.NewJSONHandler(os.Stdout, nil)),
//	}
func SessionWithConfig[C handler.Context, Data any](cfg SessionConfig[Data]) handler.Middleware[C] {
	if cfg.Manager == nil {
		panic("session middleware: manager is required")
	}

	if cfg.ErrorHandler == nil {
		cfg.ErrorHandler = func(ctx handler.Context, err error) handler.Response {
			httpErr := response.ErrInternalServerError
			if err != nil {
				httpErr = httpErr.WithMessage("Session error: " + err.Error())
			}
			return response.Error(httpErr)
		}
	}

	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}

	return func(next handler.HandlerFunc[C]) handler.HandlerFunc[C] {
		return func(ctx C) handler.Response {
			if cfg.Skip != nil && cfg.Skip(ctx) {
				return next(ctx)
			}

			// Load session (creates new anonymous session if none exists)
			sess, err := cfg.Manager.Load(ctx.ResponseWriter(), ctx.Request())
			if err != nil {
				cfg.Logger.Error("failed to load session",
					slog.String("path", ctx.Request().URL.Path),
					slog.String("error", err.Error()))
				return cfg.ErrorHandler(ctx, err)
			}

			// Store session in context
			ctx.SetValue(sessionContextKey{}, sess)

			// Execute handler
			resp := next(ctx)

			// If auto-save is disabled, return response as-is
			if !cfg.AutoSave {
				return resp
			}

			// Wrap response to save session after handler completes
			return func(w http.ResponseWriter, r *http.Request) error {
				// Retrieve potentially modified session from context
				currentSess, ok := GetSession[Data](ctx)
				if !ok {
					// Session was removed from context, just execute response
					cfg.Logger.Warn("session not found in context during save",
						slog.String("path", r.URL.Path))
					return resp(w, r)
				}

				// Execute the handler's response first
				err := resp(w, r)

				// Save session after response (best effort)
				if saveErr := cfg.Manager.Save(w, r, currentSess); saveErr != nil {
					// Log but don't fail the request - response already sent
					cfg.Logger.Error("failed to save session",
						slog.String("path", r.URL.Path),
						slog.String("session_id", currentSess.ID.String()),
						slog.String("error", saveErr.Error()))
				}

				return err
			}
		}
	}
}

// GetSession retrieves the session from the request context.
// Returns the session and a boolean indicating whether it was found.
//
// Use this in handlers to access the current session:
//
//	func handleProfile(ctx *MyContext) handler.Response {
//		sess, ok := middleware.GetSession[UserData](ctx)
//		if !ok {
//			return response.Error(response.ErrInternalServer.WithMessage("Session not available"))
//		}
//
//		if !sess.IsAuthenticated() {
//			return response.Error(response.ErrUnauthorized)
//		}
//
//		// Access session data
//		userID := sess.UserID
//		theme := sess.Data.Theme
//
//		return response.JSON(map[string]any{
//			"user_id": userID.String(),
//			"theme":   theme,
//		})
//	}
func GetSession[Data any](ctx handler.Context) (session.Session[Data], bool) {
	sess, ok := ctx.Value(sessionContextKey{}).(session.Session[Data])
	if !ok {
		return session.Session[Data]{}, false
	}
	return sess, true
}

// SetSession updates the session in the request context.
// Use this after modifying session data or metadata to ensure changes are persisted.
//
// Note: With auto-save enabled (default), the middleware automatically saves
// the session from context after the handler completes. You only need to call
// SetSession if you modified the session and want those changes persisted.
//
// Usage:
//
//	func handleUpdatePreferences(ctx *MyContext) handler.Response {
//		sess, ok := middleware.GetSession[UserData](ctx)
//		if !ok {
//			return response.Error(response.ErrInternalServer)
//		}
//
//		// Modify session data
//		sess.Data.Theme = "dark"
//		sess.Data.Language = "en"
//
//		// Store updated session back to context
//		middleware.SetSession(ctx, sess)
//
//		return response.JSON(map[string]any{"status": "updated"})
//	}
func SetSession[Data any](ctx handler.Context, sess session.Session[Data]) {
	ctx.SetValue(sessionContextKey{}, sess)
}
