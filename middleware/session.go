package middleware

import (
	"log/slog"

	"github.com/google/uuid"

	"github.com/dmitrymomot/foundation/core/handler"
	"github.com/dmitrymomot/foundation/core/response"
	"github.com/dmitrymomot/foundation/core/session"
)

type sessionKey struct{}

// Session creates middleware that loads session from transport, stores it in context,
// and touches it after request completion.
//
// The middleware:
//   - Loads session from transport (logs errors but continues with empty session)
//   - Stores session in request context
//   - Processes the request
//   - Touches session after request (logs errors but doesn't fail)
//
// Transport must implement Load and Touch methods for session management.
// Logger is used for structured logging of session errors.
func Session[C handler.Context, Data any](
	transport interface {
		Load(handler.Context) (session.Session[Data], error)
		Touch(handler.Context, session.Session[Data]) error
	},
	logger *slog.Logger,
) handler.Middleware[C] {
	return func(next handler.HandlerFunc[C]) handler.HandlerFunc[C] {
		return func(ctx C) handler.Response {
			sess, err := transport.Load(ctx)
			if err != nil {
				// Check if context was cancelled
				if ctxErr := ctx.Err(); ctxErr != nil {
					return response.Error(ctxErr)
				}
				if logger != nil {
					logger.Error("failed to load session", "error", err)
				}
				// Use empty session to allow graceful degradation instead of failing the request
				sess = session.Session[Data]{}
			}

			ctx.SetValue(sessionKey{}, sess)

			resp := next(ctx)

			if err := transport.Touch(ctx, sess); err != nil {
				// Check if context was cancelled during cleanup
				if ctxErr := ctx.Err(); ctxErr != nil {
					if logger != nil {
						logger.Warn("context cancelled during touch")
					}
					return resp
				}
				if logger != nil {
					logger.Error("failed to touch session", "error", err)
				}
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

// RequireAuth creates middleware that requires authenticated user.
// Returns ErrUnauthorized if UserID is uuid.Nil (anonymous session).
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

// RequireGuest creates middleware that requires guest (not authenticated).
// Redirects to /dashboard if UserID is not uuid.Nil (authenticated session).
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
