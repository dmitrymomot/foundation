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
func I18n[C handler.Context](i18nInstance *i18n.I18n, namespace string) handler.Middleware[C] {
	return I18nWithConfig[C](I18nConfig{
		I18n:      i18nInstance,
		Namespace: namespace,
	})
}

// I18nWithConfig creates an i18n middleware with custom configuration.
// It creates a translator with extracted language and namespace, storing it in context.
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

			// Use the i18n helper to parse Accept-Language header
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
func GetTranslator(ctx context.Context) (*i18n.Translator, bool) {
	translator, ok := ctx.Value(i18nTranslatorContextKey{}).(*i18n.Translator)
	return translator, ok
}
