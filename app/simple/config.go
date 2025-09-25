package simple

import (
	"github.com/dmitrymomot/foundation/core/cookie"
	"github.com/dmitrymomot/foundation/core/server"
	"github.com/dmitrymomot/foundation/core/session"
	"github.com/dmitrymomot/foundation/integration/database/pg"
)

type Config struct {
	DB      pg.Config
	Cookie  cookie.Config
	Session session.Config
	Server  server.Config

	AppName  string `env:"APP_NAME" envDefault:"simple-support"`
	Env      string `env:"APP_ENV" envDefault:"development"`
	LogLevel string `env:"LOG_LEVEL" envDefault:"info"`
	HttpHost string `env:"HTTP_HOST" envDefault:"localhost"`
	HttpPort string `env:"HTTP_PORT" envDefault:"8080"`
}
