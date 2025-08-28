package i18n

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
