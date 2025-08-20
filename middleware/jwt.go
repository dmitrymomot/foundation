package middleware

import (
	"net/http"
	"strings"

	"github.com/dmitrymomot/gokit/core/handler"
	"github.com/dmitrymomot/gokit/core/response"
	"github.com/dmitrymomot/gokit/pkg/jwt"
)

type jwtClaimsContextKey struct{}

type JWTConfig struct {
	Skip           func(ctx handler.Context) bool
	Service        *jwt.Service
	TokenExtractor func(ctx handler.Context) string
	ErrorHandler   func(ctx handler.Context, err error) handler.Response
	ClaimsFactory  func() any
	StoreInContext bool
}

func JWT[C handler.Context](signingKey string) handler.Middleware[C] {
	service, err := jwt.NewFromString(signingKey)
	if err != nil {
		panic("jwt middleware: " + err.Error())
	}

	return JWTWithConfig[C](JWTConfig{
		Service:        service,
		StoreInContext: true,
		ClaimsFactory: func() any {
			return &jwt.StandardClaims{}
		},
	})
}

func JWTWithConfig[C handler.Context](cfg JWTConfig) handler.Middleware[C] {
	if cfg.Service == nil {
		panic("jwt middleware: service is required")
	}

	if cfg.TokenExtractor == nil {
		cfg.TokenExtractor = JWTFromAuthHeader()
	}

	if cfg.ErrorHandler == nil {
		cfg.ErrorHandler = func(ctx handler.Context, err error) handler.Response {
			httpErr := response.ErrUnauthorized
			if err != nil {
				httpErr = httpErr.WithMessage(err.Error())
			}
			return response.Error(httpErr)
		}
	}

	if cfg.ClaimsFactory == nil {
		cfg.ClaimsFactory = func() any {
			return &jwt.StandardClaims{}
		}
	}

	return func(next handler.HandlerFunc[C]) handler.HandlerFunc[C] {
		return func(ctx C) handler.Response {
			if cfg.Skip != nil && cfg.Skip(ctx) {
				return next(ctx)
			}

			token := cfg.TokenExtractor(ctx)
			if token == "" {
				return cfg.ErrorHandler(ctx, jwt.ErrInvalidToken)
			}

			claims := cfg.ClaimsFactory()
			if err := cfg.Service.Parse(token, claims); err != nil {
				return cfg.ErrorHandler(ctx, err)
			}

			if cfg.StoreInContext {
				ctx.SetValue(jwtClaimsContextKey{}, claims)
			}

			return next(ctx)
		}
	}
}

func GetJWTClaims[T any](ctx handler.Context) (T, bool) {
	var zero T
	claims, ok := ctx.Value(jwtClaimsContextKey{}).(T)
	if !ok {
		return zero, false
	}
	return claims, true
}

func GetStandardClaims(ctx handler.Context) (*jwt.StandardClaims, bool) {
	claims, ok := ctx.Value(jwtClaimsContextKey{}).(*jwt.StandardClaims)
	if !ok {
		return nil, false
	}
	return claims, true
}

// Token Extractors

// JWTFromAuthHeader returns an extractor that looks for the token in the Authorization header
// with Bearer scheme. It also accepts tokens without the Bearer prefix.
func JWTFromAuthHeader() func(handler.Context) string {
	return func(ctx handler.Context) string {
		auth := ctx.Request().Header.Get("Authorization")
		if auth == "" {
			return ""
		}
		const bearerPrefix = "Bearer "
		if strings.HasPrefix(auth, bearerPrefix) {
			return auth[len(bearerPrefix):]
		}
		return auth
	}
}

// JWTFromAuthHeaderWithScheme returns an extractor that looks for the token in the Authorization header
// with a custom scheme (e.g., "JWT", "Token").
func JWTFromAuthHeaderWithScheme(scheme string) func(handler.Context) string {
	prefix := scheme + " "
	return func(ctx handler.Context) string {
		auth := ctx.Request().Header.Get("Authorization")
		if auth == "" {
			return ""
		}
		if strings.HasPrefix(auth, prefix) {
			return auth[len(prefix):]
		}
		return ""
	}
}

// JWTFromHeader returns an extractor that looks for the token in a custom header.
func JWTFromHeader(headerName string) func(handler.Context) string {
	return func(ctx handler.Context) string {
		return ctx.Request().Header.Get(headerName)
	}
}

// JWTFromQuery returns an extractor that looks for the token in a URL query parameter.
func JWTFromQuery(paramName string) func(handler.Context) string {
	return func(ctx handler.Context) string {
		return ctx.Request().URL.Query().Get(paramName)
	}
}

// JWTFromCookie returns an extractor that looks for the token in an HTTP cookie.
func JWTFromCookie(cookieName string) func(handler.Context) string {
	return func(ctx handler.Context) string {
		cookie, err := ctx.Request().Cookie(cookieName)
		if err != nil {
			return ""
		}
		return cookie.Value
	}
}

// JWTFromForm returns an extractor that looks for the token in form data.
// This only works for POST/PUT/PATCH requests with appropriate content type.
func JWTFromForm(fieldName string) func(handler.Context) string {
	return func(ctx handler.Context) string {
		// Only parse form for appropriate methods
		method := ctx.Request().Method
		if method != http.MethodPost && method != http.MethodPut && method != http.MethodPatch {
			return ""
		}

		// Parse form if not already parsed
		if err := ctx.Request().ParseForm(); err != nil {
			return ""
		}

		return ctx.Request().FormValue(fieldName)
	}
}

// JWTFromPathParam returns an extractor that looks for the token in a URL path parameter.
// Note: This requires the router to support path parameters and store them in context.
// The paramName should match the parameter name defined in your route.
func JWTFromPathParam(paramName string) func(handler.Context) string {
	return func(ctx handler.Context) string {
		// This is a generic implementation. Different routers may store params differently.
		// For example, if using chi router, you might use chi.URLParam(ctx.Request(), paramName)
		// Since we don't know the specific router, we'll check if the context has a Param method
		type paramGetter interface {
			Param(string) string
		}

		if pg, ok := ctx.(paramGetter); ok {
			return pg.Param(paramName)
		}

		// Fallback: check if params are stored in context values
		// This is router-specific and may need adjustment
		return ""
	}
}

// JWTFromMultiple returns an extractor that tries multiple extractors in order
// and returns the first non-empty token found.
func JWTFromMultiple(extractors ...func(handler.Context) string) func(handler.Context) string {
	return func(ctx handler.Context) string {
		for _, extractor := range extractors {
			if token := extractor(ctx); token != "" {
				return token
			}
		}
		return ""
	}
}
