package cookie

import (
	"net/http"
	"strings"
)

// Config provides environment-based configuration for cookie manager.
type Config struct {
	Secrets  string        `env:"COOKIE_SECRETS" envDefault:""`
	Path     string        `env:"COOKIE_PATH" envDefault:"/"`
	Domain   string        `env:"COOKIE_DOMAIN" envDefault:""`
	MaxAge   int           `env:"COOKIE_MAX_AGE" envDefault:"0"`
	Secure   bool          `env:"COOKIE_SECURE" envDefault:"false"`
	HttpOnly bool          `env:"COOKIE_HTTP_ONLY" envDefault:"true"`
	SameSite http.SameSite `env:"COOKIE_SAME_SITE" envDefault:"2"` // SameSiteLaxMode
	MaxSize  int           `env:"COOKIE_MAX_SIZE" envDefault:"4096"`
	// GDPR consent settings
	ConsentCookieName string `env:"COOKIE_CONSENT_NAME" envDefault:"__cookie_consent"`
	ConsentVersion    string `env:"COOKIE_CONSENT_VERSION" envDefault:"1.0"`
	ConsentMaxAge     int    `env:"COOKIE_CONSENT_MAX_AGE" envDefault:"31536000"` // 1 year
}

// DefaultConfig returns a Config with secure defaults.
func DefaultConfig() Config {
	return Config{
		Secrets:           "",
		Path:              "/",
		Domain:            "",
		MaxAge:            0,
		Secure:            false,
		HttpOnly:          true,
		SameSite:          http.SameSiteLaxMode,
		MaxSize:           MaxCookieSize,
		ConsentCookieName: "__cookie_consent",
		ConsentVersion:    "1.0",
		ConsentMaxAge:     365 * 24 * 60 * 60, // 1 year
	}
}

// parseSecrets splits comma-separated secrets for key rotation support.
// Empty strings are filtered out to prevent cryptographic vulnerabilities.
func (c Config) parseSecrets() []string {
	if c.Secrets == "" {
		return nil
	}

	parts := strings.Split(c.Secrets, ",")
	secrets := make([]string, 0, len(parts))

	for _, s := range parts {
		s = strings.TrimSpace(s)
		if s != "" {
			secrets = append(secrets, s)
		}
	}

	return secrets
}

// NewFromConfig creates a Manager from configuration.
// Only non-zero config values override defaults to preserve secure settings.
func NewFromConfig(cfg Config, opts ...Option) (*Manager, error) {
	secrets := cfg.parseSecrets()

	configOpts := make([]Option, 0)

	if cfg.Path != "" {
		configOpts = append(configOpts, WithPath(cfg.Path))
	}
	if cfg.Domain != "" {
		configOpts = append(configOpts, WithDomain(cfg.Domain))
	}
	if cfg.MaxAge != 0 {
		configOpts = append(configOpts, WithMaxAge(cfg.MaxAge))
	}
	if cfg.Secure {
		configOpts = append(configOpts, WithSecure(cfg.Secure))
	}
	if cfg.HttpOnly {
		configOpts = append(configOpts, WithHTTPOnly(cfg.HttpOnly))
	}
	if cfg.SameSite != 0 {
		configOpts = append(configOpts, WithSameSite(cfg.SameSite))
	}

	// Append user-provided options to override config
	configOpts = append(configOpts, opts...)

	// Create manager with options
	m, err := New(secrets, configOpts...)
	if err != nil {
		return nil, err
	}

	// Configure consent settings directly
	if cfg.ConsentCookieName != "" {
		m.consentCookie = cfg.ConsentCookieName
	}
	if cfg.ConsentVersion != "" {
		m.consentVersion = cfg.ConsentVersion
	}
	if cfg.ConsentMaxAge > 0 {
		m.consentMaxAge = cfg.ConsentMaxAge
	}

	// Set max size
	if cfg.MaxSize > 0 {
		m.maxSize = cfg.MaxSize
	}

	return m, nil
}
