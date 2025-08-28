package i18n

import "time"

// Translator provides a simplified translation interface with a fixed language and namespace context.
// It wraps an I18n instance and eliminates the need to specify language and namespace for each translation.
type Translator struct {
	i18n      *I18n
	language  string
	namespace string
}

// NewTranslator creates a new Translator with the specified language and namespace context.
func NewTranslator(i18n *I18n, language, namespace string) *Translator {
	if i18n == nil {
		panic("localization service is not provided")
	}
	if language == "" {
		language = i18n.DefaultLanguage()
	}
	return &Translator{
		i18n:      i18n,
		language:  language,
		namespace: namespace,
	}
}

// T translates a key using the translator's language and namespace context.
// Placeholders in the translation are replaced with values from the provided maps.
func (t *Translator) T(key string, placeholders ...M) string {
	return t.i18n.T(t.language, t.namespace, key, placeholders...)
}

// Tn translates a key with pluralization using the translator's language and namespace context.
// It automatically selects the appropriate plural form based on the count and language rules.
func (t *Translator) Tn(key string, n int, placeholders ...M) string {
	return t.i18n.Tn(t.language, t.namespace, key, n, placeholders...)
}

// Language returns the current language context of the translator.
func (t *Translator) Language() string {
	return t.language
}

// Namespace returns the current namespace context of the translator.
func (t *Translator) Namespace() string {
	return t.namespace
}

// FormatNumber formats a number with locale-specific separators.
// For example, in English: 1234.5 -> "1,234.5", in German: "1.234,5"
func (t *Translator) FormatNumber(n float64) string {
	return FormatNumber(n, t.language)
}

// FormatCurrency formats a currency amount with locale-specific formatting.
// For example, in English: 1234.50 -> "$1,234.50", in German: "1.234,50 €"
func (t *Translator) FormatCurrency(amount float64) string {
	return FormatCurrency(amount, t.language)
}

// FormatPercent formats a percentage with locale-specific formatting.
// The input should be a decimal (0.5 for 50%).
// For example, in English: 0.5 -> "50%", in French: "50 %"
func (t *Translator) FormatPercent(n float64) string {
	return FormatPercent(n, t.language)
}

// FormatDate formats a date with locale-specific formatting.
// For example, in US English: "01/02/2006", in German: "02.01.2006"
func (t *Translator) FormatDate(date time.Time) string {
	return FormatDate(date, t.language)
}

// FormatTime formats a time with locale-specific formatting.
// For example, in US English: "3:04 PM", in German: "15:04"
func (t *Translator) FormatTime(time time.Time) string {
	return FormatTime(time, t.language)
}

// FormatDateTime formats a datetime with locale-specific formatting.
// Combines date and time formatting for the locale.
func (t *Translator) FormatDateTime(datetime time.Time) string {
	return FormatDateTime(datetime, t.language)
}
