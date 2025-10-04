package main

import (
	"time"

	"github.com/dmitrymomot/foundation/core/server"
	"github.com/dmitrymomot/foundation/integration/database/pg"
)

type Config struct {
	AppName       string `env:"APP_NAME" envDefault:"foundation-basic-example"`
	JwtSigningKey string `env:"JWT_SIGNING_KEY,required"`

	// Session configuration
	SessionTTL           time.Duration `env:"SESSION_TTL" envDefault:"168h"`          // 7 days
	SessionTouchInterval time.Duration `env:"SESSION_TOUCH_INTERVAL" envDefault:"5m"` // 5 minutes
	AccessTokenTTL       time.Duration `env:"ACCESS_TOKEN_TTL" envDefault:"15m"`      // 15 minutes

	DB     pg.Config
	Server server.Config
}
