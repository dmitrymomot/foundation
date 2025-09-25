package simple

import (
	"errors"
	"log/slog"

	"github.com/dmitrymomot/foundation/core/config"
	"github.com/dmitrymomot/foundation/core/cookie"
	"github.com/dmitrymomot/foundation/core/logger"
	"github.com/dmitrymomot/foundation/core/router"
	"github.com/dmitrymomot/foundation/core/server"
	"github.com/dmitrymomot/foundation/core/session"
	"github.com/dmitrymomot/foundation/core/sessiontransport"
)

type App struct {
	config  Config
	router  router.Router[*Context]
	server  *server.Server
	cookie  *cookie.Manager
	session *session.Manager[SessionData]
	logger  *slog.Logger
}

type AppOption func(*App) error

func NewApp(opts ...AppOption) (*App, error) {
	var cfg Config
	if err := config.Load(&cfg); err != nil {
		return nil, err
	}

	app := &App{
		config: cfg,
		logger: logger.New(),
	}

	for _, opt := range opts {
		if err := opt(app); err != nil {
			return nil, err
		}
	}

	if app.router == nil {
		app.router = router.New(router.WithContextFactory(newContext))
	}

	if app.cookie == nil {
		cm, err := cookie.NewFromConfig(app.config.Cookie)
		if err != nil {
			return nil, err
		}
		app.cookie = cm
	}

	if app.session == nil {
		sm, err := session.NewFromConfig[SessionData](
			app.config.Session,
			session.WithLogger[SessionData](app.logger),
			session.WithTransport[SessionData](sessiontransport.NewCookie(app.cookie)),
		)
		if err != nil {
			return nil, err
		}
		app.session = sm
	}

	if app.server == nil {
		s, err := server.NewFromConfig(app.config.Server)
		if err != nil {
			return nil, err
		}
		app.server = s
	}

	return app, nil
}

func WithLogger(logger *slog.Logger) AppOption {
	return func(app *App) error {
		if logger == nil {
			return errors.New("logger cannot be nil")
		}
		app.logger = logger
		return nil
	}
}

func WithRouter(router router.Router[*Context]) AppOption {
	return func(app *App) error {
		if router == nil {
			return errors.New("router cannot be nil")
		}
		app.router = router
		return nil
	}
}

func WithServer(server *server.Server) AppOption {
	return func(app *App) error {
		if server == nil {
			return errors.New("server cannot be nil")
		}
		app.server = server
		return nil
	}
}

func WithCookieManager(cookie *cookie.Manager) AppOption {
	return func(app *App) error {
		if cookie == nil {
			return errors.New("cookie manager cannot be nil")
		}
		app.cookie = cookie
		return nil
	}
}

func WithSessionManager(session *session.Manager[SessionData]) AppOption {
	return func(app *App) error {
		if session == nil {
			return errors.New("session manager cannot be nil")
		}
		app.session = session
		return nil
	}
}
