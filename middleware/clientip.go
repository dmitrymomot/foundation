package middleware

import (
	"net/http"

	"github.com/dmitrymomot/gokit/core/handler"
	"github.com/dmitrymomot/gokit/core/response"
	"github.com/dmitrymomot/gokit/pkg/clientip"
)

type clientIPContextKey struct{}

type ClientIPConfig struct {
	Skip           func(ctx handler.Context) bool
	StoreInContext bool
	HeaderName     string
	StoreInHeader  bool
	ValidateFunc   func(ctx handler.Context, ip string) error
}

func ClientIP[C handler.Context]() handler.Middleware[C] {
	return ClientIPWithConfig[C](ClientIPConfig{
		StoreInContext: true,
	})
}

func ClientIPWithConfig[C handler.Context](cfg ClientIPConfig) handler.Middleware[C] {
	if cfg.HeaderName == "" {
		cfg.HeaderName = "X-Client-IP"
	}

	if !cfg.StoreInContext && !cfg.StoreInHeader && cfg.ValidateFunc == nil {
		cfg.StoreInContext = true
	}

	return func(next handler.HandlerFunc[C]) handler.HandlerFunc[C] {
		return func(ctx C) handler.Response {
			if cfg.Skip != nil && cfg.Skip(ctx) {
				return next(ctx)
			}

			ip := clientip.GetIP(ctx.Request())

			if cfg.StoreInContext {
				ctx.SetValue(clientIPContextKey{}, ip)
			}

			if cfg.ValidateFunc != nil {
				if err := cfg.ValidateFunc(ctx, ip); err != nil {
					return response.JSONWithStatus(response.ErrForbidden.WithError(err), response.ErrForbidden.Status)
				}
			}

			resp := next(ctx)

			if cfg.StoreInHeader {
				return func(w http.ResponseWriter, r *http.Request) error {
					w.Header().Set(cfg.HeaderName, ip)
					return resp(w, r)
				}
			}

			return resp
		}
	}
}

func GetClientIP(ctx handler.Context) (string, bool) {
	ip, ok := ctx.Value(clientIPContextKey{}).(string)
	return ip, ok
}
