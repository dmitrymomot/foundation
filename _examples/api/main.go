package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/dmitrymomot/foundation/_examples/01_basic/db/repository"
	"github.com/dmitrymomot/foundation/core/config"
	"github.com/dmitrymomot/foundation/core/health"
	"github.com/dmitrymomot/foundation/core/logger"
	"github.com/dmitrymomot/foundation/core/router"
	"github.com/dmitrymomot/foundation/core/server"
	"github.com/dmitrymomot/foundation/core/session"
	"github.com/dmitrymomot/foundation/core/sessiontransport"
	"github.com/dmitrymomot/foundation/integration/database/pg"
	"github.com/dmitrymomot/foundation/middleware"
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
	sesMgr := session.NewManager[SessionData](
		&sessionStorage{repo},
		cfg.SessionTTL,
		cfg.SessionTouchInterval,
	)

	sesJwt, err := sessiontransport.NewJWT(
		sesMgr,
		cfg.JwtSigningKey,
		cfg.AccessTokenTTL,
		cfg.AppName,
	)
	if err != nil {
		log.Error("Failed to create JWT session transport", logger.Component("session.transport.jwt"), logger.Error(err))
		os.Exit(1)
	}

	// Setup new router with custom context and global middlewares
	r := router.New[*Context](
		router.WithContextFactory[*Context](newContext(sesJwt)),
		router.WithMiddleware(
			middleware.RequestID[*Context](),
			middleware.ClientIP[*Context](),
		),
	)

	// Health check endpoints
	r.Get("/live", health.Liveness)
	r.Get("/ready", health.Readiness[*Context](log, pg.Healthcheck(db)))

	// Public auth endpoints
	r.Post("/auth/signup", signupHandler(repo))
	r.Post("/auth/login", loginHandler(repo))
	r.Post("/auth/refresh", refreshHandler())

	// Protected endpoints (require session authentication)
	r.Group(func(protected router.Router[*Context]) {
		protected.Use(middleware.SessionWithConfig[*Context, SessionData](middleware.SessionConfig[*Context, SessionData]{
			Transport:   sesJwt,
			Logger:      log,
			RequireAuth: true,
		}))
		protected.Get("/api/profile", getProfileHandler(repo))
		protected.Put("/api/profile/password", updatePasswordHandler(repo))
		protected.Post("/api/auth/logout", logoutHandler())
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
