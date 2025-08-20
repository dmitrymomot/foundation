package middleware

import (
	"net/http"

	"github.com/dmitrymomot/gokit/core/handler"
	"github.com/google/uuid"
)

type requestIDContextKey struct{}

type RequestIDConfig struct {
	Skip        func(ctx handler.Context) bool
	Generator   func() string
	HeaderName  string
	UseExisting bool
}

func RequestID[C handler.Context](cfg RequestIDConfig) handler.Middleware[C] {
	if cfg.HeaderName == "" {
		cfg.HeaderName = "X-Request-ID"
	}

	if cfg.Generator == nil {
		cfg.Generator = func() string {
			return uuid.New().String()
		}
	}

	return func(next handler.HandlerFunc[C]) handler.HandlerFunc[C] {
		return func(ctx C) handler.Response {
			if cfg.Skip != nil && cfg.Skip(ctx) {
				return next(ctx)
			}

			var requestID string

			if cfg.UseExisting {
				if existingID := ctx.Request().Header.Get(cfg.HeaderName); existingID != "" {
					requestID = existingID
				}
			}

			if requestID == "" {
				requestID = cfg.Generator()
			}

			ctx.SetValue(requestIDContextKey{}, requestID)

			response := next(ctx)

			return func(w http.ResponseWriter, r *http.Request) error {
				w.Header().Set(cfg.HeaderName, requestID)
				return response(w, r)
			}
		}
	}
}

func GetRequestID(ctx handler.Context) (string, bool) {
	id, ok := ctx.Value(requestIDContextKey{}).(string)
	return id, ok
}
