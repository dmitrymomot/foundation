package cookie

import "net/http"

// Options configures cookie attributes for HTTP cookie operations.
type Options struct {
	Path     string
	Domain   string
	MaxAge   int
	Secure   bool
	HttpOnly bool
	SameSite http.SameSite
	// Essential marks cookie as necessary for basic site functionality.
	// Essential cookies don't require GDPR consent.
	Essential bool
}

// Option is a functional option for configuring cookie options.
type Option func(*Options)

// WithPath sets the cookie path attribute.
func WithPath(path string) Option {
	return func(o *Options) {
		o.Path = path
	}
}

// WithDomain sets the cookie domain attribute.
func WithDomain(domain string) Option {
	return func(o *Options) {
		o.Domain = domain
	}
}

// WithMaxAge sets the cookie max-age in seconds.
// Negative values delete the cookie immediately.
func WithMaxAge(seconds int) Option {
	return func(o *Options) {
		o.MaxAge = seconds
	}
}

// WithSecure sets the secure flag, ensuring cookies are only sent over HTTPS.
func WithSecure(secure bool) Option {
	return func(o *Options) {
		o.Secure = secure
	}
}

// WithHTTPOnly prevents JavaScript access to the cookie.
func WithHTTPOnly(httpOnly bool) Option {
	return func(o *Options) {
		o.HttpOnly = httpOnly
	}
}

// WithSameSite sets the SameSite attribute for CSRF protection.
func WithSameSite(sameSite http.SameSite) Option {
	return func(o *Options) {
		o.SameSite = sameSite
	}
}

// WithEssential marks the cookie as essential for basic functionality.
// Essential cookies bypass GDPR consent requirements.
func WithEssential() Option {
	return func(o *Options) {
		o.Essential = true
	}
}

// applyOptions creates a new Options struct by copying base options and applying modifications.
// This prevents accidental mutation of shared defaults.
func applyOptions(base Options, opts []Option) Options {
	result := Options{
		Path:      base.Path,
		Domain:    base.Domain,
		MaxAge:    base.MaxAge,
		Secure:    base.Secure,
		HttpOnly:  base.HttpOnly,
		SameSite:  base.SameSite,
		Essential: base.Essential,
	}

	for _, opt := range opts {
		opt(&result)
	}

	return result
}
