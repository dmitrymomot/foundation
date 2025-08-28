package i18n

import "time"

// Translator provides a simplified translation interface with a fixed language and namespace context.
// It wraps an I18n instance and eliminates the need to specify language and namespace for each translation.
type Translator struct {
	i18n      *I18n
	language  string
	namespace string
	format    *LocaleFormat
}

// NewTranslator creates a new Translator with the specified language and namespace context.
// It uses the default English formatting for numbers, dates, and currency.
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
		format:    NewEnglishFormat(),
	}
}

// NewTranslatorWithFormat creates a new Translator with a custom LocaleFormat.
// This allows customization of number, date, and currency formatting.
func NewTranslatorWithFormat(i18n *I18n, language, namespace string, format *LocaleFormat) *Translator {
	if i18n == nil {
		panic("localization service is not provided")
	}
	if language == "" {
		language = i18n.DefaultLanguage()
	}
	if format == nil {
		format = NewEnglishFormat()
	}
	return &Translator{
		i18n:      i18n,
		language:  language,
		namespace: namespace,
		format:    format,
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
// For example, with English format: 1234.5 -> "1,234.5"
func (t *Translator) FormatNumber(n float64) string {
	return t.format.FormatNumber(n)
}

// FormatCurrency formats a currency amount with locale-specific formatting.
// For example, with English format: 1234.50 -> "$1,234.50"
func (t *Translator) FormatCurrency(amount float64) string {
	return t.format.FormatCurrency(amount)
}

// FormatPercent formats a percentage with locale-specific formatting.
// The input should be a decimal (0.5 for 50%).
// For example, with English format: 0.5 -> "50%"
func (t *Translator) FormatPercent(n float64) string {
	return t.format.FormatPercent(n)
}

// FormatDate formats a date with locale-specific formatting.
// For example, with US English format: "01/02/2006"
func (t *Translator) FormatDate(date time.Time) string {
	return t.format.FormatDate(date)
}

// FormatTime formats a time with locale-specific formatting.
// For example, with US English format: "3:04 PM"
func (t *Translator) FormatTime(time time.Time) string {
	return t.format.FormatTime(time)
}

// FormatDateTime formats a datetime with locale-specific formatting.
// Combines date and time formatting for the locale.
func (t *Translator) FormatDateTime(datetime time.Time) string {
	return t.format.FormatDateTime(datetime)
}

// Format returns the LocaleFormat used by this translator.
// This can be useful for accessing the formatting configuration.
func (t *Translator) Format() *LocaleFormat {
	return t.format
}
