package main

import (
	"time"

	"github.com/dmitrymomot/foundation/core/cookie"
	"github.com/dmitrymomot/foundation/core/server"
	"github.com/dmitrymomot/foundation/integration/database/pg"
)

type Config struct {
	AppName string `env:"APP_NAME" envDefault:"foundation-web-example"`

	// Cookie configuration
	CookieName string `env:"COOKIE_NAME" envDefault:"session"`

	// Session configuration
	SessionTTL           time.Duration `env:"SESSION_TTL" envDefault:"168h"` // 7 days
	SessionTouchInterval time.Duration `env:"SESSION_TOUCH_INTERVAL" envDefault:"5m"`

	Cookie cookie.Config
	DB     pg.Config
	Server server.Config
}
