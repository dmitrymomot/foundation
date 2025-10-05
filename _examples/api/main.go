package main

import (
	"context"
	"errors"
	"os"
	"os/signal"
	"syscall"

	"github.com/dmitrymomot/foundation/_examples/api/db/repository"
	"github.com/dmitrymomot/foundation/core/config"
	"github.com/dmitrymomot/foundation/core/handler"
	"github.com/dmitrymomot/foundation/core/health"
	"github.com/dmitrymomot/foundation/core/logger"
	"github.com/dmitrymomot/foundation/core/response"
	"github.com/dmitrymomot/foundation/core/router"
	"github.com/dmitrymomot/foundation/core/server"
	"github.com/dmitrymomot/foundation/core/session"
	"github.com/dmitrymomot/foundation/core/sessiontransport"
	"github.com/dmitrymomot/foundation/integration/database/pg"
	"github.com/dmitrymomot/foundation/middleware"
	"github.com/google/uuid"
	"golang.org/x/sync/errgroup"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	var cfg Config
	config.MustLoad(&cfg) // panic on error

	log := logger.New(logger.WithDevelopment(cfg.AppName))

	// Init postgres connection, it handles auto-retry and ping inside function
	db, err := pg.Connect(ctx, cfg.DB)
	if err != nil {
		log.Error("Failed to connect to database", logger.Component("database"), logger.Error(err))
		os.Exit(1)
	}
	// Run migrations automatically on app start
	if err := pg.Migrate(ctx, db, cfg.DB, log.With("component", "migration")); err != nil {
		log.Error("Failed to migrate database", logger.Component("database.migration"), logger.Error(err))
		os.Exit(1)
	}

	repo := repository.New(db)

	// Setup session manager with JWT transport
	sesMgr := session.NewFromConfig(cfg.Session, &sessionStorage{repo})

	sesJwt, err := sessiontransport.NewJWTFromConfig(cfg.SessionTransport, sesMgr)
	if err != nil {
		log.Error("Failed to create JWT session transport", logger.Component("session.transport.jwt"), logger.Error(err))
		os.Exit(1)
	}

	// Setup new router with custom context and global middlewares
	r := router.New[*Context](
		router.WithContextFactory[*Context](newContext()),
		router.WithMiddleware(
			middleware.RequestID[*Context](),
			middleware.ClientIP[*Context](),
			middleware.Fingerprint[*Context](),
		),
	)

	// Auth helpers that have access to JWT transport
	authHelper := newAuthHelper(sesJwt)
	refreshHelper := newRefreshHelper(sesJwt)

	// Health check endpoints
	r.Get("/live", health.Liveness)
	r.Get("/ready", health.Readiness[*Context](log, pg.Healthcheck(db)))

	// Public auth endpoints (no session middleware - use authHelper pattern)
	r.Post("/auth/signup", signupHandler(repo, authHelper))
	r.Post("/auth/login", loginHandler(repo, authHelper))

	// Refresh doesn't require guest (can be used while authenticated)
	r.Post("/auth/refresh", refreshHandler(refreshHelper))

	// Protected endpoints (require session authentication)
	r.Group(func(protected router.Router[*Context]) {
		protected.Use(middleware.SessionWithConfig[*Context, SessionData](middleware.SessionConfig[*Context, SessionData]{
			Transport:   sesJwt,
			Logger:      log,
			RequireAuth: true,
		}))
		protected.Get("/profile", getProfileHandler(repo))
		protected.Put("/profile/password", updatePasswordHandler(repo))
		protected.Post("/auth/logout", logoutHandler())
	})

	eg, ctx := errgroup.WithContext(ctx)

	s, err := server.NewFromConfig(cfg.Server)
	if err != nil {
		log.Error("Failed to create server", logger.Component("server"), logger.Error(err))
		os.Exit(1)
	}
	eg.Go(s.Run(ctx, r))

	if err := eg.Wait(); err != nil {
		log.Error("Failed to run server", logger.Component("server"), logger.Error(err))
		os.Exit(1)
	}

	log.Info("Application stopped")
}

// authHelper provides authentication functionality with JWT transport access.
type authHelper struct {
	transport *sessiontransport.JWT[SessionData]
}

func newAuthHelper(transport *sessiontransport.JWT[SessionData]) *authHelper {
	return &authHelper{transport: transport}
}

// Authenticate authenticates a user and returns JWT token pair.
// This creates a new session or authenticates an existing one.
func (h *authHelper) Authenticate(ctx *Context, userID uuid.UUID, data SessionData) (sessiontransport.TokenPair, error) {
	sess, tokens, err := h.transport.Authenticate(ctx, userID, data)
	if err != nil {
		return sessiontransport.TokenPair{}, err
	}
	// Update session in context so middleware can handle it
	middleware.SetSession[SessionData](ctx, sess)
	return tokens, nil
}

// refreshHelper provides token refresh functionality with JWT transport access.
type refreshHelper struct {
	transport *sessiontransport.JWT[SessionData]
}

func newRefreshHelper(transport *sessiontransport.JWT[SessionData]) *refreshHelper {
	return &refreshHelper{transport: transport}
}

// Refresh refreshes the session using refresh token and returns new token pair.
func (h *refreshHelper) Refresh(ctx *Context, refreshToken string) (sessiontransport.TokenPair, error) {
	tokens, err := h.transport.Refresh(ctx, refreshToken)
	if err != nil {
		return sessiontransport.TokenPair{}, err
	}
	return tokens, nil
}
