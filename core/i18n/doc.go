// Package i18n provides internationalization support with immutable, thread-safe design
// and comprehensive locale handling for Go applications.
//
// This package offers zero-dependency internationalization with O(1) translation lookups,
// CLDR-compliant plural rules for 20+ languages, placeholder replacement, and robust
// language fallback mechanisms. All configuration is done at construction time, making
// instances immutable and safe for concurrent use.
//
// # Basic Usage
//
// Create an I18n instance with translations and retrieve localized text:
//
//	import "github.com/dmitrymomot/gokit/core/i18n"
//
//	// Create I18n instance with English and Spanish translations
//	i18nInstance, err := i18n.New(
//		i18n.WithDefaultLanguage("en"),
//		i18n.WithTranslations("en", "app", map[string]any{
//			"welcome": "Welcome to our application",
//			"goodbye": "Goodbye, %{name}!",
//		}),
//		i18n.WithTranslations("es", "app", map[string]any{
//			"welcome": "Bienvenido a nuestra aplicación",
//			"goodbye": "¡Adiós, %{name}!",
//		}),
//	)
//	if err != nil {
//		log.Fatal(err)
//	}
//
//	// Get basic translation
//	msg := i18nInstance.T("es", "app", "welcome")
//	// Output: "Bienvenido a nuestra aplicación"
//
//	// Translation with placeholders
//	farewell := i18nInstance.T("es", "app", "goodbye", i18n.M{"name": "Juan"})
//	// Output: "¡Adiós, Juan!"
//
// # Nested Translations
//
// The package supports nested translation structures that are automatically flattened
// for efficient lookups using dot notation:
//
//	i18nInstance, _ := i18n.New(
//		i18n.WithTranslations("en", "ui", map[string]any{
//			"buttons": map[string]any{
//				"save":   "Save",
//				"cancel": "Cancel",
//				"delete": "Delete",
//			},
//			"messages": map[string]any{
//				"success": "Operation completed successfully",
//				"error":   "An error occurred: %{details}",
//			},
//		}),
//	)
//
//	// Access nested translations with dot notation
//	saveBtn := i18nInstance.T("en", "ui", "buttons.save")
//	// Output: "Save"
//
//	errorMsg := i18nInstance.T("en", "ui", "messages.error",
//		i18n.M{"details": "Database connection failed"})
//	// Output: "An error occurred: Database connection failed"
//
// # Pluralization
//
// The package includes CLDR-compliant plural rules for major language families.
// Use Tn() for pluralized translations:
//
//	i18nInstance, _ := i18n.New(
//		i18n.WithTranslations("en", "items", map[string]any{
//			"count": map[string]string{
//				"zero":  "No items",
//				"one":   "1 item",
//				"other": "%{count} items",
//			},
//		}),
//		i18n.WithTranslations("pl", "items", map[string]any{
//			"count": map[string]string{
//				"zero": "Brak elementów",
//				"one":  "1 element",
//				"few":  "%{count} elementy",
//				"many": "%{count} elementów",
//			},
//		}),
//	)
//
//	// English pluralization (uses EnglishPluralRule)
//	fmt.Println(i18nInstance.Tn("en", "items", "count", 0))  // "No items"
//	fmt.Println(i18nInstance.Tn("en", "items", "count", 1))  // "1 item"
//	fmt.Println(i18nInstance.Tn("en", "items", "count", 5))  // "5 items"
//
//	// Polish pluralization (uses SlavicPluralRule)
//	fmt.Println(i18nInstance.Tn("pl", "items", "count", 1))  // "1 element"
//	fmt.Println(i18nInstance.Tn("pl", "items", "count", 2))  // "2 elementy"
//	fmt.Println(i18nInstance.Tn("pl", "items", "count", 5))  // "5 elementów"
//
// # Language Fallback
//
// When a translation is not found in the requested language, the package
// automatically falls back to the default language, then to the key itself:
//
//	i18nInstance, _ := i18n.New(
//		i18n.WithDefaultLanguage("en"),
//		i18n.WithTranslations("en", "app", map[string]any{
//			"hello": "Hello",
//			"world": "World",
//		}),
//		// Spanish only has partial translations
//		i18n.WithTranslations("es", "app", map[string]any{
//			"hello": "Hola",
//			// "world" is missing
//		}),
//	)
//
//	// Available in Spanish
//	fmt.Println(i18nInstance.T("es", "app", "hello"))  // "Hola"
//
//	// Falls back to English default
//	fmt.Println(i18nInstance.T("es", "app", "world"))  // "World"
//
//	// Falls back to key when not found anywhere
//	fmt.Println(i18nInstance.T("es", "app", "missing"))  // "missing"
//
// # Custom Plural Rules
//
// Define custom plural rules for languages not covered by built-in rules:
//
//	// Custom rule for a hypothetical language
//	customRule := func(n int) string {
//		if n == 0 {
//			return i18n.PluralZero
//		}
//		if n == 1 {
//			return i18n.PluralOne
//		}
//		if n <= 10 {
//			return i18n.PluralFew
//		}
//		return i18n.PluralOther
//	}
//
//	i18nInstance, _ := i18n.New(
//		i18n.WithPluralRule("xx", customRule),
//		i18n.WithTranslations("xx", "test", map[string]any{
//			"items": map[string]string{
//				"zero":  "nothing",
//				"one":   "single item",
//				"few":   "several items",
//				"other": "many items",
//			},
//		}),
//	)
//
// # Accept-Language Header Processing
//
// Parse HTTP Accept-Language headers to determine the best language match:
//
//	availableLanguages := []string{"en", "es", "fr"}
//
//	// Parse client's language preferences
//	acceptLang := "es-ES,es;q=0.9,en;q=0.8,fr;q=0.7"
//	bestMatch := i18n.ParseAcceptLanguage(acceptLang, availableLanguages)
//	// Returns: "es" (highest quality match)
//
//	// Use the matched language for translations
//	greeting := i18nInstance.T(bestMatch, "app", "welcome")
//
// # Placeholder Replacement
//
// Replace placeholders in translation strings using the %{name} format:
//
//	template := "Welcome back, %{name}! You have %{count} new messages."
//	placeholders := i18n.M{
//		"name":  "Alice",
//		"count": 3,
//	}
//
//	result := i18n.ReplacePlaceholders(template, placeholders)
//	// Output: "Welcome back, Alice! You have 3 new messages."
//
// # Built-in Plural Rules
//
// The package includes CLDR-compliant plural rules for major language families:
//
//   - EnglishPluralRule: English and similar languages (zero, one, other)
//   - SlavicPluralRule: Polish, Ukrainian, Czech, etc. (zero, one, few, many)
//   - RomancePluralRule: French, Italian, Portuguese (one for 0,1; many for 1M+; other)
//   - SpanishPluralRule: Spanish (one, many for 1M+, other)
//   - GermanicPluralRule: German, Dutch, Swedish, etc. (one, other)
//   - AsianPluralRule: Japanese, Chinese, Korean, etc. (other only)
//   - ArabicPluralRule: Arabic (zero, one, two, few, many, other)
//   - DefaultPluralRule: Generic rule for unknown languages
//
// # Thread Safety and Performance
//
// The I18n struct is immutable after creation, making it safe for concurrent use
// without additional synchronization. Translation lookups are O(1) through internal
// key flattening that occurs during construction.
//
//	// Safe to use concurrently across goroutines
//	var i18nInstance *i18n.I18n // initialized once
//
//	go func() {
//		msg := i18nInstance.T("en", "app", "hello") // Safe
//	}()
//
//	go func() {
//		msg := i18nInstance.Tn("es", "items", "count", 5) // Safe
//	}()
//
// # Namespace Organization
//
// Use namespaces to organize translations and prevent key collisions:
//
//	i18nInstance, _ := i18n.New(
//		// User interface translations
//		i18n.WithTranslations("en", "ui", map[string]any{
//			"save": "Save",
//			"edit": "Edit",
//		}),
//		// Email template translations
//		i18n.WithTranslations("en", "email", map[string]any{
//			"save": "Your changes have been saved",
//			"edit": "Someone edited your document",
//		}),
//		// API error messages
//		i18n.WithTranslations("en", "api", map[string]any{
//			"save": "Failed to save data",
//			"edit": "Edit operation not permitted",
//		}),
//	)
//
//	// Same key, different contexts
//	uiMsg := i18nInstance.T("en", "ui", "save")     // "Save"
//	emailMsg := i18nInstance.T("en", "email", "save") // "Your changes have been saved"
//	apiMsg := i18nInstance.T("en", "api", "save")   // "Failed to save data"
//
// # Simplified Translation with Translator
//
// The Translator type provides a simplified interface for translations by fixing
// the language and namespace context. This is especially useful in web applications
// where you want to translate content for a specific user's language and context.
//
//	// Create a translator for a specific user language and namespace
//	translator := i18n.NewTranslator(i18nInstance, "es", "ui")
//
//	// Use simplified methods without specifying language/namespace
//	saveButton := translator.T("buttons.save")
//	// Output: "Guardar"
//
//	cancelButton := translator.T("buttons.cancel")
//	// Output: "Cancelar"
//
//	// Pluralization with simplified interface
//	itemCount := translator.Tn("items.count", 5)
//	// Output: "5 elementos"
//
//	// Access the translator's context
//	currentLang := translator.Language()  // Returns: "es"
//	namespace := translator.Namespace()   // Returns: "ui"
//
// The Translator is particularly useful in web handlers where the language is
// determined once per request but used multiple times:
//
//	func handleUserProfile(translator *i18n.Translator) {
//		title := translator.T("profile.title")
//		description := translator.T("profile.description", i18n.M{
//			"username": user.Name,
//		})
//		saveBtn := translator.T("buttons.save")
//		// No need to repeat language and namespace for each translation
//	}
package i18n
