package middleware

import (
	"net/http"

	"github.com/dmitrymomot/gokit/core/handler"
	"github.com/dmitrymomot/gokit/core/response"
	"github.com/dmitrymomot/gokit/pkg/fingerprint"
)

type fingerprintContextKey struct{}

type FingerprintConfig struct {
	Skip           func(ctx handler.Context) bool
	HeaderName     string
	StoreInContext bool
	StoreInHeader  bool
	ValidateFunc   func(ctx handler.Context, fingerprint string) error
}

func Fingerprint[C handler.Context]() handler.Middleware[C] {
	return FingerprintWithConfig[C](FingerprintConfig{
		StoreInContext: true,
	})
}

func FingerprintWithConfig[C handler.Context](cfg FingerprintConfig) handler.Middleware[C] {
	if cfg.HeaderName == "" {
		cfg.HeaderName = "X-Device-Fingerprint"
	}

	if !cfg.StoreInContext && !cfg.StoreInHeader && cfg.ValidateFunc == nil {
		cfg.StoreInContext = true
	}

	return func(next handler.HandlerFunc[C]) handler.HandlerFunc[C] {
		return func(ctx C) handler.Response {
			if cfg.Skip != nil && cfg.Skip(ctx) {
				return next(ctx)
			}

			fp := fingerprint.Generate(ctx.Request())

			if cfg.StoreInContext {
				ctx.SetValue(fingerprintContextKey{}, fp)
			}

			if cfg.ValidateFunc != nil {
				if err := cfg.ValidateFunc(ctx, fp); err != nil {
					return response.JSONWithStatus(response.ErrBadRequest.WithError(err), response.ErrBadRequest.Status)
				}
			}

			response := next(ctx)

			if cfg.StoreInHeader {
				return func(w http.ResponseWriter, r *http.Request) error {
					w.Header().Set(cfg.HeaderName, fp)
					return response(w, r)
				}
			}

			return response
		}
	}
}

func GetFingerprint(ctx handler.Context) (string, bool) {
	fp, ok := ctx.Value(fingerprintContextKey{}).(string)
	return fp, ok
}
