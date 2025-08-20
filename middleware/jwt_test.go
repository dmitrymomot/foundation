package middleware_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dmitrymomot/gokit/core/handler"
	"github.com/dmitrymomot/gokit/core/response"
	"github.com/dmitrymomot/gokit/core/router"
	"github.com/dmitrymomot/gokit/middleware"
	"github.com/dmitrymomot/gokit/pkg/jwt"
)

const testSigningKey = "test-secret-key-at-least-32-bytes-long"

// httpErrorHandler is a test helper that properly handles response.HTTPError types
func httpErrorHandler(ctx *router.Context, err error) {
	w := ctx.ResponseWriter()

	// Check if error is HTTPError type
	var httpErr response.HTTPError
	if errors.As(err, &httpErr) {
		w.WriteHeader(httpErr.Status)
		// Write the error message as JSON
		w.Header().Set("Content-Type", "application/json")
		respBody, _ := json.Marshal(httpErr)
		w.Write(respBody)
	} else {
		// Fall back to default error handling
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func TestJWTSimpleFactory(t *testing.T) {
	t.Parallel()

	jwtService, err := jwt.NewFromString(testSigningKey)
	require.NoError(t, err)

	// No need for custom error handler anymore - default handler now supports StatusCode interface
	r := router.New[*router.Context]()
	r.Use(middleware.JWT[*router.Context](testSigningKey))

	r.Get("/protected", func(ctx *router.Context) handler.Response {
		claims, ok := middleware.GetStandardClaims(ctx)
		assert.True(t, ok, "Claims should be available")
		assert.NotNil(t, claims)
		return response.JSON(map[string]string{"status": "ok", "user": claims.Subject})
	})

	t.Run("valid token", func(t *testing.T) {
		claims := &jwt.StandardClaims{
			Subject:   "user123",
			ExpiresAt: time.Now().Add(time.Hour).Unix(),
		}
		token, err := jwtService.Generate(claims)
		require.NoError(t, err)

		req := httptest.NewRequest(http.MethodGet, "/protected", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), "user123")
	})

	t.Run("missing token", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/protected", nil)
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("invalid token", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/protected", nil)
		req.Header.Set("Authorization", "Bearer invalid.token.here")
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})
}

func TestJWTWithConfigSkipFunction(t *testing.T) {
	t.Parallel()

	jwtService, err := jwt.NewFromString(testSigningKey)
	require.NoError(t, err)

	r := router.New[*router.Context](router.WithErrorHandler(httpErrorHandler))
	r.Use(middleware.JWTWithConfig[*router.Context](middleware.JWTConfig{
		Service: jwtService,
		Skip: func(ctx handler.Context) bool {
			return ctx.Request().URL.Path == "/public"
		},
		StoreInContext: true,
	}))

	r.Get("/public", func(ctx *router.Context) handler.Response {
		claims, ok := middleware.GetStandardClaims(ctx)
		assert.False(t, ok, "Claims should not be available for skipped routes")
		assert.Nil(t, claims)
		return response.JSON(map[string]string{"status": "public"})
	})

	r.Get("/protected", func(ctx *router.Context) handler.Response {
		claims, ok := middleware.GetStandardClaims(ctx)
		assert.True(t, ok, "Claims should be available")
		assert.NotNil(t, claims)
		return response.JSON(map[string]string{"status": "protected"})
	})

	t.Run("skip public endpoint", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/public", nil)
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), "public")
	})

	t.Run("require auth for protected endpoint", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/protected", nil)
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})
}

func TestJWTCustomTokenExtractor(t *testing.T) {
	t.Parallel()

	jwtService, err := jwt.NewFromString(testSigningKey)
	require.NoError(t, err)

	r := router.New[*router.Context](router.WithErrorHandler(httpErrorHandler))
	r.Use(middleware.JWTWithConfig[*router.Context](middleware.JWTConfig{
		Service: jwtService,
		TokenExtractor: func(ctx handler.Context) string {
			return ctx.Request().Header.Get("X-API-Token")
		},
		StoreInContext: true,
	}))

	r.Get("/api", func(ctx *router.Context) handler.Response {
		claims, ok := middleware.GetStandardClaims(ctx)
		assert.True(t, ok)
		return response.JSON(map[string]string{"user": claims.Subject})
	})

	claims := &jwt.StandardClaims{
		Subject:   "api-user",
		ExpiresAt: time.Now().Add(time.Hour).Unix(),
	}
	token, err := jwtService.Generate(claims)
	require.NoError(t, err)

	t.Run("custom header", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api", nil)
		req.Header.Set("X-API-Token", token)
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), "api-user")
	})

	t.Run("authorization header ignored", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})
}

func TestJWTCustomErrorHandler(t *testing.T) {
	t.Parallel()

	jwtService, err := jwt.NewFromString(testSigningKey)
	require.NoError(t, err)

	r := router.New[*router.Context]()
	r.Use(middleware.JWTWithConfig[*router.Context](middleware.JWTConfig{
		Service: jwtService,
		ErrorHandler: func(ctx handler.Context, err error) handler.Response {
			if errors.Is(err, jwt.ErrExpiredToken) {
				return response.JSONWithStatus(
					map[string]string{"error": "token expired", "action": "refresh"},
					http.StatusUnauthorized,
				)
			}
			return response.JSONWithStatus(
				map[string]string{"error": "auth failed"},
				http.StatusForbidden,
			)
		},
		StoreInContext: true,
	}))

	r.Get("/test", func(ctx *router.Context) handler.Response {
		return response.JSON(map[string]string{"status": "ok"})
	})

	t.Run("expired token", func(t *testing.T) {
		claims := &jwt.StandardClaims{
			Subject:   "user123",
			ExpiresAt: time.Now().Add(-time.Hour).Unix(),
		}
		token, err := jwtService.Generate(claims)
		require.NoError(t, err)

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
		assert.Contains(t, w.Body.String(), "token expired")
		assert.Contains(t, w.Body.String(), "refresh")
	})

	t.Run("invalid token", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.Header.Set("Authorization", "Bearer invalid")
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusForbidden, w.Code)
		assert.Contains(t, w.Body.String(), "auth failed")
	})
}

type CustomClaims struct {
	jwt.StandardClaims
	Role        string   `json:"role"`
	Permissions []string `json:"permissions"`
}

func TestJWTCustomClaims(t *testing.T) {
	t.Parallel()

	jwtService, err := jwt.NewFromString(testSigningKey)
	require.NoError(t, err)

	r := router.New[*router.Context](router.WithErrorHandler(httpErrorHandler))
	r.Use(middleware.JWTWithConfig[*router.Context](middleware.JWTConfig{
		Service: jwtService,
		ClaimsFactory: func() any {
			return &CustomClaims{}
		},
		StoreInContext: true,
	}))

	r.Get("/admin", func(ctx *router.Context) handler.Response {
		claims, ok := middleware.GetJWTClaims[*CustomClaims](ctx)
		assert.True(t, ok, "Custom claims should be available")
		assert.NotNil(t, claims)

		if claims.Role != "admin" {
			return response.JSONWithStatus(
				response.ErrForbidden.WithMessage("Admin access required"),
				response.ErrForbidden.Status,
			)
		}

		return response.JSON(map[string]any{
			"user":        claims.Subject,
			"role":        claims.Role,
			"permissions": claims.Permissions,
		})
	})

	t.Run("admin access granted", func(t *testing.T) {
		claims := &CustomClaims{
			StandardClaims: jwt.StandardClaims{
				Subject:   "admin-user",
				ExpiresAt: time.Now().Add(time.Hour).Unix(),
			},
			Role:        "admin",
			Permissions: []string{"read", "write", "delete"},
		}
		token, err := jwtService.Generate(claims)
		require.NoError(t, err)

		req := httptest.NewRequest(http.MethodGet, "/admin", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), "admin-user")
		assert.Contains(t, w.Body.String(), "admin")
		assert.Contains(t, w.Body.String(), "delete")
	})

	t.Run("non-admin access denied", func(t *testing.T) {
		claims := &CustomClaims{
			StandardClaims: jwt.StandardClaims{
				Subject:   "regular-user",
				ExpiresAt: time.Now().Add(time.Hour).Unix(),
			},
			Role:        "user",
			Permissions: []string{"read"},
		}
		token, err := jwtService.Generate(claims)
		require.NoError(t, err)

		req := httptest.NewRequest(http.MethodGet, "/admin", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusForbidden, w.Code)
		assert.Contains(t, w.Body.String(), "Admin access required")
	})
}

func TestJWTStoreInContextFalse(t *testing.T) {
	t.Parallel()

	jwtService, err := jwt.NewFromString(testSigningKey)
	require.NoError(t, err)

	r := router.New[*router.Context](router.WithErrorHandler(httpErrorHandler))
	r.Use(middleware.JWTWithConfig[*router.Context](middleware.JWTConfig{
		Service:        jwtService,
		StoreInContext: false,
	}))

	r.Get("/test", func(ctx *router.Context) handler.Response {
		claims, ok := middleware.GetStandardClaims(ctx)
		assert.False(t, ok, "Claims should not be in context when StoreInContext is false")
		assert.Nil(t, claims)
		return response.JSON(map[string]string{"status": "ok"})
	})

	claims := &jwt.StandardClaims{
		Subject:   "user123",
		ExpiresAt: time.Now().Add(time.Hour).Unix(),
	}
	token, err := jwtService.Generate(claims)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestJWTNotBeforeClaim(t *testing.T) {
	t.Parallel()

	jwtService, err := jwt.NewFromString(testSigningKey)
	require.NoError(t, err)

	r := router.New[*router.Context](router.WithErrorHandler(httpErrorHandler))
	r.Use(middleware.JWT[*router.Context](testSigningKey))

	r.Get("/test", func(ctx *router.Context) handler.Response {
		return response.JSON(map[string]string{"status": "ok"})
	})

	t.Run("token not yet valid", func(t *testing.T) {
		claims := &jwt.StandardClaims{
			Subject:   "user123",
			NotBefore: time.Now().Add(time.Hour).Unix(),
			ExpiresAt: time.Now().Add(2 * time.Hour).Unix(),
		}
		token, err := jwtService.Generate(claims)
		require.NoError(t, err)

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("token valid after NotBefore", func(t *testing.T) {
		claims := &jwt.StandardClaims{
			Subject:   "user123",
			NotBefore: time.Now().Add(-time.Hour).Unix(),
			ExpiresAt: time.Now().Add(time.Hour).Unix(),
		}
		token, err := jwtService.Generate(claims)
		require.NoError(t, err)

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})
}

func TestJWTWithoutBearerPrefix(t *testing.T) {
	t.Parallel()

	jwtService, err := jwt.NewFromString(testSigningKey)
	require.NoError(t, err)

	r := router.New[*router.Context](router.WithErrorHandler(httpErrorHandler))
	r.Use(middleware.JWT[*router.Context](testSigningKey))

	r.Get("/test", func(ctx *router.Context) handler.Response {
		return response.JSON(map[string]string{"status": "ok"})
	})

	claims := &jwt.StandardClaims{
		Subject:   "user123",
		ExpiresAt: time.Now().Add(time.Hour).Unix(),
	}
	token, err := jwtService.Generate(claims)
	require.NoError(t, err)

	t.Run("token without Bearer prefix", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.Header.Set("Authorization", token)
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})
}

func TestJWTMultipleMiddleware(t *testing.T) {
	t.Parallel()

	jwtService, err := jwt.NewFromString(testSigningKey)
	require.NoError(t, err)

	r := router.New[*router.Context](router.WithErrorHandler(httpErrorHandler))

	var claimsInMiddleware2 *jwt.StandardClaims

	r.Use(middleware.JWT[*router.Context](testSigningKey))

	middleware2 := func(next handler.HandlerFunc[*router.Context]) handler.HandlerFunc[*router.Context] {
		return func(ctx *router.Context) handler.Response {
			claims, ok := middleware.GetStandardClaims(ctx)
			assert.True(t, ok, "Claims should be available in subsequent middleware")
			claimsInMiddleware2 = claims
			return next(ctx)
		}
	}

	r.Use(middleware2)

	r.Get("/test", func(ctx *router.Context) handler.Response {
		claims, ok := middleware.GetStandardClaims(ctx)
		assert.True(t, ok)
		assert.Equal(t, claimsInMiddleware2, claims)
		return response.JSON(map[string]string{"status": "ok"})
	})

	claims := &jwt.StandardClaims{
		Subject:   "user123",
		ExpiresAt: time.Now().Add(time.Hour).Unix(),
	}
	token, err := jwtService.Generate(claims)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.NotNil(t, claimsInMiddleware2)
	assert.Equal(t, "user123", claimsInMiddleware2.Subject)
}

func TestJWTFromHeader(t *testing.T) {
	t.Parallel()

	jwtService, err := jwt.NewFromString(testSigningKey)
	require.NoError(t, err)

	r := router.New[*router.Context](router.WithErrorHandler(httpErrorHandler))
	r.Use(middleware.JWTWithConfig[*router.Context](middleware.JWTConfig{
		Service:        jwtService,
		TokenExtractor: middleware.JWTFromHeader("X-Token"),
		StoreInContext: true,
	}))

	r.Get("/test", func(ctx *router.Context) handler.Response {
		claims, ok := middleware.GetStandardClaims(ctx)
		assert.True(t, ok)
		return response.JSON(map[string]string{"user": claims.Subject})
	})

	claims := &jwt.StandardClaims{
		Subject:   "user123",
		ExpiresAt: time.Now().Add(time.Hour).Unix(),
	}
	token, err := jwtService.Generate(claims)
	require.NoError(t, err)

	t.Run("token in custom header", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.Header.Set("X-Token", token)
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), "user123")
	})

	t.Run("missing token", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})
}

func TestJWTFromQuery(t *testing.T) {
	t.Parallel()

	jwtService, err := jwt.NewFromString(testSigningKey)
	require.NoError(t, err)

	r := router.New[*router.Context](router.WithErrorHandler(httpErrorHandler))
	r.Use(middleware.JWTWithConfig[*router.Context](middleware.JWTConfig{
		Service:        jwtService,
		TokenExtractor: middleware.JWTFromQuery("token"),
		StoreInContext: true,
	}))

	r.Get("/test", func(ctx *router.Context) handler.Response {
		claims, ok := middleware.GetStandardClaims(ctx)
		assert.True(t, ok)
		return response.JSON(map[string]string{"user": claims.Subject})
	})

	claims := &jwt.StandardClaims{
		Subject:   "user123",
		ExpiresAt: time.Now().Add(time.Hour).Unix(),
	}
	token, err := jwtService.Generate(claims)
	require.NoError(t, err)

	t.Run("token in query parameter", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/test?token="+token, nil)
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), "user123")
	})

	t.Run("missing query parameter", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})
}

func TestJWTFromCookie(t *testing.T) {
	t.Parallel()

	jwtService, err := jwt.NewFromString(testSigningKey)
	require.NoError(t, err)

	r := router.New[*router.Context](router.WithErrorHandler(httpErrorHandler))
	r.Use(middleware.JWTWithConfig[*router.Context](middleware.JWTConfig{
		Service:        jwtService,
		TokenExtractor: middleware.JWTFromCookie("jwt-token"),
		StoreInContext: true,
	}))

	r.Get("/test", func(ctx *router.Context) handler.Response {
		claims, ok := middleware.GetStandardClaims(ctx)
		assert.True(t, ok)
		return response.JSON(map[string]string{"user": claims.Subject})
	})

	claims := &jwt.StandardClaims{
		Subject:   "user123",
		ExpiresAt: time.Now().Add(time.Hour).Unix(),
	}
	token, err := jwtService.Generate(claims)
	require.NoError(t, err)

	t.Run("token in cookie", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.AddCookie(&http.Cookie{
			Name:  "jwt-token",
			Value: token,
		})
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), "user123")
	})

	t.Run("missing cookie", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})
}

func TestJWTFromForm(t *testing.T) {
	t.Parallel()

	jwtService, err := jwt.NewFromString(testSigningKey)
	require.NoError(t, err)

	r := router.New[*router.Context](router.WithErrorHandler(httpErrorHandler))
	r.Use(middleware.JWTWithConfig[*router.Context](middleware.JWTConfig{
		Service:        jwtService,
		TokenExtractor: middleware.JWTFromForm("access_token"),
		StoreInContext: true,
	}))

	r.Post("/test", func(ctx *router.Context) handler.Response {
		claims, ok := middleware.GetStandardClaims(ctx)
		assert.True(t, ok)
		return response.JSON(map[string]string{"user": claims.Subject})
	})

	claims := &jwt.StandardClaims{
		Subject:   "user123",
		ExpiresAt: time.Now().Add(time.Hour).Unix(),
	}
	token, err := jwtService.Generate(claims)
	require.NoError(t, err)

	t.Run("token in form data", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/test", strings.NewReader("access_token="+token))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), "user123")
	})

	t.Run("missing form field", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/test", strings.NewReader("other_field=value"))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("GET request ignored", func(t *testing.T) {
		r.Get("/test-get", func(ctx *router.Context) handler.Response {
			return response.JSON(map[string]string{"status": "ok"})
		})

		req := httptest.NewRequest(http.MethodGet, "/test-get?access_token="+token, nil)
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})
}

func TestJWTFromAuthHeaderWithScheme(t *testing.T) {
	t.Parallel()

	jwtService, err := jwt.NewFromString(testSigningKey)
	require.NoError(t, err)

	r := router.New[*router.Context](router.WithErrorHandler(httpErrorHandler))
	r.Use(middleware.JWTWithConfig[*router.Context](middleware.JWTConfig{
		Service:        jwtService,
		TokenExtractor: middleware.JWTFromAuthHeaderWithScheme("JWT"),
		StoreInContext: true,
	}))

	r.Get("/test", func(ctx *router.Context) handler.Response {
		claims, ok := middleware.GetStandardClaims(ctx)
		assert.True(t, ok)
		return response.JSON(map[string]string{"user": claims.Subject})
	})

	claims := &jwt.StandardClaims{
		Subject:   "user123",
		ExpiresAt: time.Now().Add(time.Hour).Unix(),
	}
	token, err := jwtService.Generate(claims)
	require.NoError(t, err)

	t.Run("token with JWT scheme", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.Header.Set("Authorization", "JWT "+token)
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), "user123")
	})

	t.Run("wrong scheme ignored", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("no scheme", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.Header.Set("Authorization", token)
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})
}

func TestJWTFromMultiple(t *testing.T) {
	t.Parallel()

	jwtService, err := jwt.NewFromString(testSigningKey)
	require.NoError(t, err)

	r := router.New[*router.Context](router.WithErrorHandler(httpErrorHandler))
	r.Use(middleware.JWTWithConfig[*router.Context](middleware.JWTConfig{
		Service: jwtService,
		TokenExtractor: middleware.JWTFromMultiple(
			middleware.JWTFromAuthHeader(),
			middleware.JWTFromHeader("X-Token"),
			middleware.JWTFromQuery("token"),
			middleware.JWTFromCookie("auth"),
		),
		StoreInContext: true,
	}))

	r.Get("/test", func(ctx *router.Context) handler.Response {
		claims, ok := middleware.GetStandardClaims(ctx)
		assert.True(t, ok)
		return response.JSON(map[string]string{"user": claims.Subject})
	})

	claims := &jwt.StandardClaims{
		Subject:   "user123",
		ExpiresAt: time.Now().Add(time.Hour).Unix(),
	}
	token, err := jwtService.Generate(claims)
	require.NoError(t, err)

	t.Run("token in Authorization header", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), "user123")
	})

	t.Run("token in custom header", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.Header.Set("X-Token", token)
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), "user123")
	})

	t.Run("token in query", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/test?token="+token, nil)
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), "user123")
	})

	t.Run("token in cookie", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.AddCookie(&http.Cookie{
			Name:  "auth",
			Value: token,
		})
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), "user123")
	})

	t.Run("first extractor takes precedence", func(t *testing.T) {
		// Create tokens with different subjects
		claims1 := &jwt.StandardClaims{
			Subject:   "auth-user",
			ExpiresAt: time.Now().Add(time.Hour).Unix(),
		}
		token1, err := jwtService.Generate(claims1)
		require.NoError(t, err)

		claims2 := &jwt.StandardClaims{
			Subject:   "header-user",
			ExpiresAt: time.Now().Add(time.Hour).Unix(),
		}
		token2, err := jwtService.Generate(claims2)
		require.NoError(t, err)

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.Header.Set("Authorization", "Bearer "+token1)
		req.Header.Set("X-Token", token2)
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), "auth-user") // First extractor wins
	})

	t.Run("no token found", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})
}
