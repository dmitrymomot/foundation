package main

import (
	"context"
	"html/template"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/dmitrymomot/foundation/_examples/web/db/repository"
	"github.com/dmitrymomot/foundation/core/config"
	"github.com/dmitrymomot/foundation/core/cookie"
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
	"golang.org/x/sync/errgroup"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	var cfg Config
	config.MustLoad(&cfg) // panic on error

	log := logger.New(logger.WithDevelopment(cfg.AppName))

	// Init postgres connection
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
	sesMgr := session.NewFromConfig(cfg.Session, &sessionStorage{repo})

	// Setup cookie manager
	cookieMgr, err := cookie.NewFromConfig(cfg.Cookie)
	if err != nil {
		log.Error("Failed to create cookie manager", logger.Component("cookie"), logger.Error(err))
		os.Exit(1)
	}

	// Setup cookie-based session transport
	sesCookie := sessiontransport.NewCookieFromConfig(cfg.SessionTransport, sesMgr, cookieMgr)

	// Load templates
	templates, err := loadTemplates()
	if err != nil {
		log.Error("Failed to load templates", logger.Component("templates"), logger.Error(err))
		os.Exit(1)
	}

	// Setup new router with custom context, error handler, and global middlewares
	r := router.New[*Context](
		router.WithContextFactory[*Context](newContext()),
		router.WithErrorHandler[*Context](errorHandler(templates.error)),
		router.WithMiddleware(
			middleware.RequestID[*Context](),
			middleware.ClientIP[*Context](),
			middleware.Fingerprint[*Context](),
			middleware.LoggingWithLogger[*Context](log.With(logger.Component("http.request"))),
		),
	)

	// Health check endpoints
	r.Get("/live", health.Liveness)
	r.Get("/ready", health.Readiness[*Context](log, pg.Healthcheck(db)))

	// Public routes (require guest - redirect authenticated users to profile)
	r.Group(func(public router.Router[*Context]) {
		public.Use(middleware.SessionWithConfig[*Context, SessionData](middleware.SessionConfig[*Context, SessionData]{
			Transport:    sesCookie,
			Logger:       log,
			RequireGuest: true,
			ErrorHandler: func(ctx *Context, err error) handler.Response {
				// Redirect authenticated users to profile page
				return response.Redirect("/")
			},
		}))
		public.Get("/signup", signupPageHandler(templates.signup))
		public.Post("/signup", signupHandler(repo, templates.signup))
		public.Get("/login", loginPageHandler(templates.login))
		public.Post("/login", loginHandler(repo, templates.login))
	})

	// Protected routes (require authentication)
	r.Group(func(protected router.Router[*Context]) {
		protected.Use(middleware.SessionWithConfig[*Context, SessionData](middleware.SessionConfig[*Context, SessionData]{
			Transport:   sesCookie,
			Logger:      log,
			RequireAuth: true,
			ErrorHandler: func(ctx *Context, err error) handler.Response {
				// Redirect to login for HTML requests instead of returning 401
				return response.Redirect("/login")
			},
		}))
		protected.Get("/", profileHandler(repo, templates.profile))
		protected.Post("/logout", logoutHandler())
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

// templates holds all parsed templates
type templates struct {
	signup  *template.Template
	login   *template.Template
	profile *template.Template
	error   *template.Template
}

// loadTemplates loads and parses all HTML templates
func loadTemplates() (*templates, error) {
	templateDir := "templates"

	// Parse each template with the layout
	signup, err := template.ParseFiles(
		filepath.Join(templateDir, "layout.html"),
		filepath.Join(templateDir, "signup.html"),
	)
	if err != nil {
		return nil, err
	}

	login, err := template.ParseFiles(
		filepath.Join(templateDir, "layout.html"),
		filepath.Join(templateDir, "login.html"),
	)
	if err != nil {
		return nil, err
	}

	profile, err := template.ParseFiles(
		filepath.Join(templateDir, "layout.html"),
		filepath.Join(templateDir, "profile.html"),
	)
	if err != nil {
		return nil, err
	}

	errorTmpl, err := template.ParseFiles(
		filepath.Join(templateDir, "layout.html"),
		filepath.Join(templateDir, "error.html"),
	)
	if err != nil {
		return nil, err
	}

	return &templates{
		signup:  signup,
		login:   login,
		profile: profile,
		error:   errorTmpl,
	}, nil
}
