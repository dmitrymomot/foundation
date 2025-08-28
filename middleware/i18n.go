package middleware

import (
	"context"

	"github.com/dmitrymomot/gokit/core/handler"
	"github.com/dmitrymomot/gokit/core/i18n"
)

// i18nTranslatorContextKey is used as a key for storing i18n translator in request context.
type i18nTranslatorContextKey struct{}

// I18nConfig configures the i18n middleware.
type I18nConfig struct {
	// Skip defines a function to skip middleware execution for specific requests
	Skip func(ctx handler.Context) bool
	// I18n is the i18n instance to use for translations (required)
	I18n *i18n.I18n
	// LanguageExtractor defines how to extract the language from the request
	// Default: extracts from Accept-Language header
	LanguageExtractor func(ctx handler.Context) string
	// Namespace is the translation namespace to use
	Namespace string
	// FallbackLanguage is the language to use if extraction fails
	// Default: uses I18n's default language
	FallbackLanguage string
}

// I18n creates an i18n middleware with default configuration.
// It extracts language from Accept-Language header and stores a translator in context.
//
// Use this middleware when you need automatic language detection and translation
// capabilities in your handlers. The middleware extracts the preferred language
// from the Accept-Language header and creates a translator that can be retrieved
// in handlers using GetTranslator.
//
// Usage:
//
//	// Initialize i18n with supported languages
//	i18nInstance, err := i18n.New([]string{"en", "es", "fr"}, "en")
//	if err != nil {
//		log.Fatal(err)
//	}
//
//	// Load translations for different namespaces
//	i18nInstance.LoadMessages("web", "en", map[string]string{
//		"hello":   "Hello",
//		"goodbye": "Goodbye",
//	})
//	i18nInstance.LoadMessages("web", "es", map[string]string{
//		"hello":   "Hola",
//		"goodbye": "Adiós",
//	})
//
//	// Apply middleware to your router
//	r.Use(middleware.I18n[*MyContext](i18nInstance, "web"))
//
//	// Use in handlers
//	r.GET("/greeting", func(ctx *MyContext) handler.Response {
//		translator, ok := middleware.GetTranslator(ctx)
//		if !ok {
//			return response.Error(response.ErrInternalServerError)
//		}
//
//		greeting := translator.T("hello")
//		return response.JSON(map[string]string{"message": greeting})
//	})
//
// The middleware automatically:
// - Extracts language from Accept-Language header
// - Falls back to the i18n instance's default language if extraction fails
// - Creates a namespace-specific translator
// - Stores the translator in request context for handler access
func I18n[C handler.Context](i18nInstance *i18n.I18n, namespace string) handler.Middleware[C] {
	return I18nWithConfig[C](I18nConfig{
		I18n:      i18nInstance,
		Namespace: namespace,
	})
}

// I18nWithConfig creates an i18n middleware with custom configuration.
// It creates a translator with extracted language and namespace, storing it in context.
//
// Use this when you need more control over language extraction, validation, or want to
// skip i18n processing for certain requests (e.g., API endpoints, health checks).
//
// Advanced Usage Examples:
//
//	// Custom language extraction from URL path
//	cfg := middleware.I18nConfig{
//		I18n:      i18nInstance,
//		Namespace: "web",
//		LanguageExtractor: func(ctx handler.Context) string {
//			// Extract language from URL path: /es/products, /fr/about
//			path := strings.TrimPrefix(ctx.Request().URL.Path, "/")
//			parts := strings.Split(path, "/")
//			if len(parts) > 0 && len(parts[0]) == 2 {
//				return parts[0] // Return language code
//			}
//			return "" // Will use fallback
//		},
//		FallbackLanguage: "en",
//	}
//	r.Use(middleware.I18nWithConfig[*MyContext](cfg))
//
//	// Skip i18n for API routes
//	cfg := middleware.I18nConfig{
//		I18n:      i18nInstance,
//		Namespace: "web",
//		Skip: func(ctx handler.Context) bool {
//			return strings.HasPrefix(ctx.Request().URL.Path, "/api/")
//		},
//	}
//	r.Use(middleware.I18nWithConfig[*MyContext](cfg))
//
//	// Custom language extraction from cookie
//	cfg := middleware.I18nConfig{
//		I18n:      i18nInstance,
//		Namespace: "web",
//		LanguageExtractor: func(ctx handler.Context) string {
//			cookie, err := ctx.Request().Cookie("language")
//			if err != nil {
//				return ""
//			}
//			return cookie.Value
//		},
//		FallbackLanguage: "en",
//	}
//
// Common Configuration Patterns:
// - Skip health checks: Skip: func(ctx) bool { return ctx.Request().URL.Path == "/health" }
// - Extract from subdomain: Extract language from subdomain like es.example.com
// - Database-based fallback: Look up user's language preference from database
// - Multi-namespace support: Use different namespaces for admin vs public content
func I18nWithConfig[C handler.Context](cfg I18nConfig) handler.Middleware[C] {
	if cfg.I18n == nil {
		panic("i18n middleware: i18n instance is required")
	}

	if cfg.Namespace == "" {
		panic("i18n middleware: namespace is required")
	}

	if cfg.FallbackLanguage == "" {
		cfg.FallbackLanguage = cfg.I18n.DefaultLanguage()
	}

	if cfg.LanguageExtractor == nil {
		cfg.LanguageExtractor = func(ctx handler.Context) string {
			acceptLang := ctx.Request().Header.Get("Accept-Language")
			if acceptLang == "" {
				return cfg.FallbackLanguage
			}

			lang := i18n.ParseAcceptLanguage(acceptLang, cfg.I18n.Languages())
			if lang == "" {
				return cfg.FallbackLanguage
			}
			return lang
		}
	}

	return func(next handler.HandlerFunc[C]) handler.HandlerFunc[C] {
		return func(ctx C) handler.Response {
			if cfg.Skip != nil && cfg.Skip(ctx) {
				return next(ctx)
			}

			// Extract language from request
			language := cfg.LanguageExtractor(ctx)
			if language == "" {
				language = cfg.FallbackLanguage
			}

			// Create translator with extracted language and configured namespace
			translator := i18n.NewTranslator(cfg.I18n, language, cfg.Namespace)

			// Store translator in context
			ctx.SetValue(i18nTranslatorContextKey{}, translator)

			return next(ctx)
		}
	}
}

// GetTranslator retrieves the i18n translator from the context.
// Returns the translator and a boolean indicating whether it was found.
// Works with any context.Context, not just handler.Context.
//
// Usage in handlers:
//
//	func MyHandler(ctx *MyContext) handler.Response {
//		translator, ok := middleware.GetTranslator(ctx)
//		if !ok {
//			// Handle missing translator (middleware not applied or skipped)
//			return response.Error(response.ErrInternalServerError.WithMessage("Translation unavailable"))
//		}
//
//		// Basic translation
//		title := translator.T("page.title")
//
//		// Translation with parameters
//		welcome := translator.T("welcome", map[string]any{
//			"Name": "John",
//		})
//
//		// Plural translation
//		count := 5
//		items := translator.Plural("items", count, map[string]any{
//			"Count": count,
//		})
//
//		return response.JSON(map[string]string{
//			"title":   title,
//			"welcome": welcome,
//			"items":   items,
//		})
//	}
//
// The translator provides these methods:
// - T(key, params): Basic translation with optional parameters
// - Plural(key, count, params): Plural-aware translation
// - Language(): Get current language code
// - Namespace(): Get current namespace
func GetTranslator(ctx context.Context) (*i18n.Translator, bool) {
	translator, ok := ctx.Value(i18nTranslatorContextKey{}).(*i18n.Translator)
	return translator, ok
}
