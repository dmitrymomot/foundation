package main

import (
	"github.com/dmitrymomot/foundation/core/server"
	"github.com/dmitrymomot/foundation/core/session"
	"github.com/dmitrymomot/foundation/core/sessiontransport"
	"github.com/dmitrymomot/foundation/integration/database/pg"
)

type Config struct {
	AppName string `env:"APP_NAME" envDefault:"foundation-basic-example"`

	Session          session.Config
	SessionTransport sessiontransport.JWTConfig

	DB     pg.Config
	Server server.Config
}
