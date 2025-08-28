package i18n

import (
	"fmt"
	"maps"
	"sort"
)

// DefaultLang is the default language code used when no default language is specified.
const DefaultLang = "en"

// I18n provides internationalization support with translations and pluralization.
// It is immutable after creation, making it safe for concurrent use.
type I18n struct {
	// Flattened translations map for O(1) lookups
	// Key format: "lang:namespace:key.path"
	translations map[string]string

	// Plural rules per language
	pluralRules map[string]PluralRule

	// Default/fallback language
	defaultLang string

	// Pre-computed list of available languages (for O(1) access)
	languages []string

	// Optional handler called when a translation key is not found
	missingKeyHandler func(lang, namespace, key string)
}

// Option configures the I18n instance during construction.
type Option func(*I18n) error

// New creates a new I18n instance with the given options.
// All configuration happens during construction, making the instance
// immutable and thread-safe from creation.
func New(opts ...Option) (*I18n, error) {
	i := &I18n{
		translations: make(map[string]string),
		pluralRules:  make(map[string]PluralRule),
		defaultLang:  DefaultLang,
	}

	// Apply all options
	for _, opt := range opts {
		if err := opt(i); err != nil {
			return nil, fmt.Errorf("failed to apply option: %w", err)
		}
	}

	// Validate configuration
	if i.defaultLang == "" {
		return nil, fmt.Errorf("default language cannot be empty")
	}

	// Build pre-computed language list for O(1) access during requests
	i.languages = i.buildLanguagesList()

	return i, nil
}

// WithDefaultLanguage sets the default/fallback language.
func WithDefaultLanguage(lang string) Option {
	return func(i *I18n) error {
		if lang == "" {
			return fmt.Errorf("language cannot be empty")
		}
		i.defaultLang = lang
		return nil
	}
}

// WithPluralRule registers a custom plural rule for a language.
func WithPluralRule(lang string, rule PluralRule) Option {
	return func(i *I18n) error {
		if lang == "" {
			return fmt.Errorf("language cannot be empty")
		}
		if rule == nil {
			return fmt.Errorf("plural rule cannot be nil")
		}
		i.pluralRules[lang] = rule
		return nil
	}
}

// WithLanguages sets the supported languages for the I18n instance.
// The default language will always be included and placed first in the list.
// Other languages will be sorted alphabetically.
func WithLanguages(langs ...string) Option {
	return func(i *I18n) error {
		if len(langs) == 0 {
			return nil
		}

		// Use a map to deduplicate
		langSet := make(map[string]bool)
		for _, lang := range langs {
			if lang != "" {
				langSet[lang] = true
			}
		}

		// Build the final list with default first
		i.languages = make([]string, 0, len(langSet)+1)
		i.languages = append(i.languages, i.defaultLang)

		// Remove default from set if present
		delete(langSet, i.defaultLang)

		// Add remaining languages sorted
		if len(langSet) > 0 {
			otherLangs := make([]string, 0, len(langSet))
			for lang := range langSet {
				otherLangs = append(otherLangs, lang)
			}
			sort.Strings(otherLangs)
			i.languages = append(i.languages, otherLangs...)
		}

		return nil
	}
}

// WithMissingKeyHandler sets a handler function that will be called when a translation
// key is not found in any language (including the default fallback).
// This is useful for logging missing translations during development.
// The handler receives the requested language, namespace, and key.
func WithMissingKeyHandler(handler func(lang, namespace, key string)) Option {
	return func(i *I18n) error {
		i.missingKeyHandler = handler
		return nil
	}
}

// WithTranslations loads translations for a specific language and namespace.
// The translations map can be nested; it will be flattened internally for
// efficient lookups.
func WithTranslations(lang, namespace string, translations map[string]any) Option {
	return func(i *I18n) error {
		if lang == "" {
			return fmt.Errorf("language cannot be empty")
		}
		if namespace == "" {
			return fmt.Errorf("namespace cannot be empty")
		}
		if len(translations) == 0 {
			return nil // Empty translations are allowed
		}

		// Flatten the translations map
		flattened := flattenTranslations(translations, "")

		// Store with composite keys
		for key, value := range flattened {
			compositeKey := buildKey(lang, namespace, key)
			i.translations[compositeKey] = value
		}

		// Auto-apply plural rule based on language if not set
		if _, exists := i.pluralRules[lang]; !exists {
			i.pluralRules[lang] = GetPluralRuleForLanguage(lang)
		}

		return nil
	}
}

// T retrieves a translation for the given language, namespace, and key.
// Placeholders in the translation are replaced with values from the provided maps.
// Falls back to the default language if translation is not found.
// Returns the key itself if no translation exists.
func (i *I18n) T(lang, namespace, key string, placeholders ...M) string {
	// Try to get translation for requested language
	compositeKey := buildKey(lang, namespace, key)
	if translation, exists := i.translations[compositeKey]; exists {
		return replacePlaceholdersWithMerge(translation, placeholders...)
	}

	// Fall back to default language if different
	if lang != i.defaultLang {
		defaultKey := buildKey(i.defaultLang, namespace, key)
		if translation, exists := i.translations[defaultKey]; exists {
			return replacePlaceholdersWithMerge(translation, placeholders...)
		}
	}

	// Call missing key handler if set
	if i.missingKeyHandler != nil {
		i.missingKeyHandler(lang, namespace, key)
	}

	// Return the key as last resort
	return key
}

// Languages returns all configured languages in the I18n instance.
// The default language is always returned first, followed by other languages sorted alphabetically.
// This is an O(1) operation as the list is pre-computed during construction.
func (i *I18n) Languages() []string {
	return i.languages
}

// DefaultLanguage returns the default language code configured for the I18n instance.
// If no default language was explicitly set, returns DefaultLang ("en").
func (i *I18n) DefaultLanguage() string {
	return i.defaultLang
}

// buildLanguagesList builds the pre-computed list of languages.
// Called once during construction after all options are applied.
// If no languages were explicitly configured, returns only the default language.
func (i *I18n) buildLanguagesList() []string {
	// If languages were already set by WithLanguages, return as-is
	if len(i.languages) > 0 {
		return i.languages
	}

	// Otherwise, return just the default language
	return []string{i.defaultLang}
}

// Tn retrieves a pluralized translation for the given count.
// It automatically selects the appropriate plural form based on the language's plural rule
// and injects the count as a placeholder.
func (i *I18n) Tn(lang, namespace, key string, n int, placeholders ...M) string {
	// Get the plural rule for the language
	rule, exists := i.pluralRules[lang]
	if !exists {
		// Fall back to default language rule or default rule
		if rule, exists = i.pluralRules[i.defaultLang]; !exists {
			rule = DefaultPluralRule
		}
	}

	// Determine the plural form
	form := rule(n)

	// Try to get the specific plural form
	pluralKey := key + "." + form

	// Build composite key for direct lookup
	compositeKey := buildKey(lang, namespace, pluralKey)

	// Try exact form first
	var translation string
	var found bool

	if trans, exists := i.translations[compositeKey]; exists {
		translation = trans
		found = true
	} else {
		// Try fallback forms
		fallbackForms := getPluralFallbackForms(form)
		for _, fallbackForm := range fallbackForms {
			fallbackKey := buildKey(lang, namespace, key+"."+fallbackForm)
			if trans, exists := i.translations[fallbackKey]; exists {
				translation = trans
				found = true
				break
			}
		}
	}

	// If not found in requested language, try default language
	if !found && lang != i.defaultLang {
		compositeKey = buildKey(i.defaultLang, namespace, pluralKey)
		if trans, exists := i.translations[compositeKey]; exists {
			translation = trans
			found = true
		} else {
			// Try fallback forms in default language
			for _, fallbackForm := range getPluralFallbackForms(form) {
				fallbackKey := buildKey(i.defaultLang, namespace, key+"."+fallbackForm)
				if trans, exists := i.translations[fallbackKey]; exists {
					translation = trans
					found = true
					break
				}
			}
		}
	}

	// If still not found, return the key
	if !found {
		// Call missing key handler if set
		if i.missingKeyHandler != nil {
			i.missingKeyHandler(lang, namespace, key)
		}
		return key
	}

	// Merge count placeholder with other placeholders
	mergedPlaceholders := M{"count": n}
	for _, p := range placeholders {
		maps.Copy(mergedPlaceholders, p)
	}

	return ReplacePlaceholders(translation, mergedPlaceholders)
}

// buildKey creates a composite key for the translations map.
func buildKey(lang, namespace, key string) string {
	return lang + ":" + namespace + ":" + key
}

// flattenTranslations recursively flattens a nested map into dot-notation keys.
func flattenTranslations(data map[string]any, prefix string) map[string]string {
	result := make(map[string]string)

	for key, value := range data {
		fullKey := key
		if prefix != "" {
			fullKey = prefix + "." + key
		}

		switch v := value.(type) {
		case string:
			result[fullKey] = v
		case map[string]any:
			// Recursively flatten nested maps
			nested := flattenTranslations(v, fullKey)
			maps.Copy(result, nested)
		case map[string]string:
			// Handle map[string]string (common for plural forms)
			for subKey, subVal := range v {
				result[fullKey+"."+subKey] = subVal
			}
		default:
			// Convert other types to string
			result[fullKey] = fmt.Sprintf("%v", v)
		}
	}

	return result
}

// replacePlaceholdersWithMerge replaces placeholders in a template with values from multiple maps.
func replacePlaceholdersWithMerge(template string, placeholders ...M) string {
	if len(placeholders) == 0 {
		return template
	}

	// Merge all placeholder maps
	merged := make(M)
	for _, p := range placeholders {
		maps.Copy(merged, p)
	}

	return ReplacePlaceholders(template, merged)
}

// getPluralFallbackForms returns the fallback hierarchy for a given plural form.
// This ensures graceful degradation when specific plural forms are missing,
// following Unicode CLDR recommendations for fallback order.
func getPluralFallbackForms(form string) []string {
	switch form {
	case PluralZero:
		return []string{PluralOther}
	case PluralOne:
		return []string{PluralOther}
	case PluralTwo:
		return []string{PluralFew, PluralMany, PluralOther}
	case PluralFew:
		return []string{PluralMany, PluralOther}
	case PluralMany:
		return []string{PluralOther}
	case PluralOther:
		return []string{} // No fallback for "other"
	default:
		return []string{PluralOther}
	}
}
