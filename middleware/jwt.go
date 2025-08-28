package middleware

import (
	"net/http"
	"strings"

	"github.com/dmitrymomot/foundation/core/handler"
	"github.com/dmitrymomot/foundation/core/response"
	"github.com/dmitrymomot/foundation/pkg/jwt"
)

// jwtClaimsContextKey is used as a key for storing JWT claims in request context.
type jwtClaimsContextKey struct{}

// JWTConfig configures the JWT authentication middleware.
type JWTConfig struct {
	// Skip defines a function to skip middleware execution for specific requests
	Skip func(ctx handler.Context) bool
	// Service is the JWT service instance used for token parsing and validation
	Service *jwt.Service
	// TokenExtractor defines how to extract the token from the request (default: from Authorization header)
	TokenExtractor func(ctx handler.Context) string
	// ErrorHandler defines how to handle authentication errors (default: returns 401 Unauthorized)
	ErrorHandler func(ctx handler.Context, err error) handler.Response
	// ClaimsFactory creates a new claims instance for token parsing (default: StandardClaims)
	ClaimsFactory func() any
	// StoreInContext determines whether to store parsed claims in request context
	StoreInContext bool
}

// JWT creates a JWT authentication middleware with a signing key.
// It uses standard claims and stores them in the request context by default.
// Panics if the signing key is invalid.
//
// This is the most common way to add JWT authentication to your application.
// Use this when you have a simple authentication setup with standard JWT claims.
//
// Usage:
//
//	// Apply JWT middleware to protected routes
//	r.Use(middleware.JWT[*MyContext]("your-secret-signing-key"))
//
//	// Or apply to specific route groups
//	protected := r.Group("/protected")
//	protected.Use(middleware.JWT[*MyContext]("your-secret-signing-key"))
//	protected.GET("/profile", handleProfile)
//
//	// Use claims in handlers
//	func handleProfile(ctx *MyContext) handler.Response {
//		claims, ok := middleware.GetStandardClaims(ctx)
//		if !ok {
//			return response.Error(response.ErrUnauthorized)
//		}
//
//		userID := claims.Subject
//		expiresAt := claims.ExpiresAt
//
//		return response.JSON(map[string]any{
//			"user_id": userID,
//			"expires": expiresAt,
//		})
//	}
//
// The middleware automatically:
// - Extracts JWT token from Authorization header (Bearer scheme)
// - Validates token signature and expiration
// - Parses standard claims (subject, issued at, expires at, etc.)
// - Stores claims in request context for handler access
// - Returns 401 Unauthorized for invalid or missing tokens
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

// JWTWithConfig creates a JWT authentication middleware with custom configuration.
// It validates JWT tokens and optionally stores parsed claims in the request context.
// Panics if the JWT service is not provided.
//
// Use this for advanced JWT configurations: custom claims, multiple token sources,
// custom error handling, or when you need to skip authentication for certain requests.
//
// Advanced Usage Examples:
//
//	// Custom claims structure
//	type CustomClaims struct {
//		jwt.StandardClaims
//		Role     string `json:"role"`
//		TenantID string `json:"tenant_id"`
//	}
//
//	cfg := middleware.JWTConfig{
//		Service: jwtService,
//		ClaimsFactory: func() any {
//			return &CustomClaims{}
//		},
//		StoreInContext: true,
//	}
//	r.Use(middleware.JWTWithConfig[*MyContext](cfg))
//
//	// Multiple token sources (header, cookie, query param)
//	cfg := middleware.JWTConfig{
//		Service: jwtService,
//		TokenExtractor: middleware.JWTFromMultiple(
//			middleware.JWTFromAuthHeader(),
//			middleware.JWTFromCookie("auth_token"),
//			middleware.JWTFromQuery("token"),
//		),
//	}
//
//	// Skip authentication for public endpoints
//	cfg := middleware.JWTConfig{
//		Service: jwtService,
//		Skip: func(ctx handler.Context) bool {
//			path := ctx.Request().URL.Path
//			return path == "/health" || strings.HasPrefix(path, "/public/")
//		},
//	}
//
//	// Custom error handling
//	cfg := middleware.JWTConfig{
//		Service: jwtService,
//		ErrorHandler: func(ctx handler.Context, err error) handler.Response {
//			// Log authentication failures
//			log.Printf("JWT auth failed: %v", err)
//
//			// Return custom error response
//			return response.Error(response.ErrUnauthorized.WithMessage("Please log in"))
//		},
//	}
//
// Common patterns:
// - API keys in custom headers: TokenExtractor: JWTFromHeader("X-API-Key")
// - Mobile apps with refresh tokens: Store refresh logic in ErrorHandler
// - Multi-tenant apps: Use custom claims with tenant information
// - Development mode: Skip authentication with environment-based Skip function
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

// GetJWTClaims retrieves JWT claims of the specified type from the request context.
// Returns the claims and a boolean indicating whether they were found and of the correct type.
//
// Use this function with custom claims types:
//
//	type CustomClaims struct {
//		jwt.StandardClaims
//		Role     string `json:"role"`
//		TenantID string `json:"tenant_id"`
//	}
//
//	func handleAdmin(ctx *MyContext) handler.Response {
//		claims, ok := middleware.GetJWTClaims[*CustomClaims](ctx)
//		if !ok {
//			return response.Error(response.ErrUnauthorized)
//		}
//
//		if claims.Role != "admin" {
//			return response.Error(response.ErrForbidden)
//		}
//
//		return response.JSON(map[string]string{
//			"tenant_id": claims.TenantID,
//			"role":      claims.Role,
//		})
//	}
func GetJWTClaims[T any](ctx handler.Context) (T, bool) {
	var zero T
	claims, ok := ctx.Value(jwtClaimsContextKey{}).(T)
	if !ok {
		return zero, false
	}
	return claims, true
}

// GetStandardClaims retrieves standard JWT claims from the request context.
// Returns the claims and a boolean indicating whether they were found.
// This is a convenience function for the most common use case.
//
// Standard claims include:
// - Subject: User ID or identifier
// - ExpiresAt: Token expiration timestamp
// - IssuedAt: Token creation timestamp
// - NotBefore: Token valid-from timestamp
// - Issuer: Token issuer
// - Audience: Intended token audience
//
// Usage:
//
//	func handleProtected(ctx *MyContext) handler.Response {
//		claims, ok := middleware.GetStandardClaims(ctx)
//		if !ok {
//			return response.Error(response.ErrUnauthorized)
//		}
//
//		// Access standard claim fields
//		userID := claims.Subject
//		expires := time.Unix(claims.ExpiresAt, 0)
//		issuer := claims.Issuer
//
//		return response.JSON(map[string]any{
//			"user_id": userID,
//			"expires": expires.Format(time.RFC3339),
//			"issuer":  issuer,
//		})
//	}
func GetStandardClaims(ctx handler.Context) (*jwt.StandardClaims, bool) {
	claims, ok := ctx.Value(jwtClaimsContextKey{}).(*jwt.StandardClaims)
	if !ok {
		return nil, false
	}
	return claims, true
}

// Token Extractors
//
// The following functions provide various strategies for extracting JWT tokens
// from HTTP requests. They can be used individually or combined using JWTFromMultiple.

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
		// Only parse form for appropriate methods to avoid unnecessary work
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
		// This is a generic implementation since different routers store params differently.
		// For example, chi router uses chi.URLParam(ctx.Request(), paramName)
		// We try to detect if the context implements a Param method
		type paramGetter interface {
			Param(string) string
		}

		if pg, ok := ctx.(paramGetter); ok {
			return pg.Param(paramName)
		}

		// Fallback: params might be stored in context values (router-specific)
		// This implementation may need adjustment based on your router
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
