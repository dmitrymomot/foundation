package main

import (
	"context"
	"foundation-basic-example/db/repository"
	"os"
	"os/signal"
	"syscall"

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

	// Setup session manager
	sesStorage := &sessionStorage{repo}
	sesJwt, err := sessiontransport.NewJWT(cfg.JwtSigningKey, sesStorage)
	ses, err := session.NewFromConfig[SessionData](cfg.Session,
		session.WithStore(sesStorage),
		session.WithTransport[SessionData](sesJwt),
	)
	if err != nil {
		log.Error("Failed to create session manager", logger.Component("session"), logger.Error(err))
		os.Exit(1)
	}

	// Setup new router with global middlewares and default context
	r := router.New(
		router.WithMiddleware(
			middleware.RequestID[*router.Context](),
			middleware.ClientIP[*router.Context](),
		),
	)

	r.Get("/live", health.Liveness)
	r.Get("/ready", health.Readiness[*router.Context](log, pg.Healthcheck(db))) // ping db connection

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
