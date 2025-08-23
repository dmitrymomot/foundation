package i18n

import "strings"

// PluralRule determines which plural form to use for a given count.
// It follows Unicode CLDR (Common Locale Data Repository) guidelines.
type PluralRule func(n int) string

// Plural category constants as defined by Unicode CLDR.
// Not all languages use all categories.
const (
	PluralZero  = "zero"  // Used for 0 in some languages
	PluralOne   = "one"   // Singular form
	PluralTwo   = "two"   // Dual form (used in Arabic, Hebrew, etc.)
	PluralFew   = "few"   // Paucal form (used in Slavic languages, etc.)
	PluralMany  = "many"  // Used for larger quantities in some languages
	PluralOther = "other" // Default/catch-all form
)

// DefaultPluralRule provides a generic plural rule that works reasonably
// well for languages without specific rules. It distinguishes between
// zero, one, few, many, and other.
var DefaultPluralRule PluralRule = func(n int) string {
	if n == 0 {
		return PluralZero
	}

	// Handle negative numbers by using absolute value
	absN := n
	if n < 0 {
		absN = -n
	}

	if absN == 1 {
		return PluralOne
	}
	if absN >= 2 && absN <= 4 {
		return PluralFew
	}
	if absN > 4 && absN < 20 {
		return PluralMany
	}
	return PluralOther
}

// EnglishPluralRule implements plural rules for English and similar languages.
// Categories: zero (0), one (1), other (everything else)
var EnglishPluralRule PluralRule = func(n int) string {
	if n == 0 {
		return PluralZero
	}
	if n == 1 || n == -1 {
		return PluralOne
	}
	return PluralOther
}

// SlavicPluralRule implements plural rules for Slavic languages
// (Polish, Czech, Ukrainian, Croatian, Serbian, etc.)
// Categories: zero, one, few, many
var SlavicPluralRule PluralRule = func(n int) string {
	if n == 0 {
		return PluralZero
	}
	if n == 1 || n == -1 {
		return PluralOne
	}

	// Handle negative numbers by using absolute value
	absN := n
	if n < 0 {
		absN = -n
	}

	mod10 := absN % 10
	mod100 := absN % 100

	// Numbers ending in 2, 3, 4 (except 12, 13, 14) use "few"
	if mod10 >= 2 && mod10 <= 4 && (mod100 < 12 || mod100 > 14) {
		return PluralFew
	}

	return PluralMany
}

// RomancePluralRule implements plural rules for Romance languages
// (French, Italian, Portuguese, but NOT Spanish which is simpler)
// Categories: one (0, 1), many (1,000,000+), other
var RomancePluralRule PluralRule = func(n int) string {
	if n == 0 || n == 1 || n == -1 {
		return PluralOne
	}
	// Handle negative numbers
	absN := n
	if n < 0 {
		absN = -n
	}
	if absN >= 1000000 {
		return PluralMany
	}
	return PluralOther
}

// GermanicPluralRule implements plural rules for Germanic languages
// (German, Dutch, Swedish, Norwegian, Danish)
// Categories: one (1), other (everything else including 0)
var GermanicPluralRule PluralRule = func(n int) string {
	if n == 1 || n == -1 {
		return PluralOne
	}
	return PluralOther
}

// AsianPluralRule implements plural rules for Asian languages
// that don't distinguish plural forms
// (Japanese, Chinese, Korean, Thai, Vietnamese)
// Categories: other (all numbers)
var AsianPluralRule PluralRule = func(n int) string {
	return PluralOther
}

// ArabicPluralRule implements complex plural rules for Arabic.
// Categories: zero, one, two, few, many, other
var ArabicPluralRule PluralRule = func(n int) string {
	if n == 0 {
		return PluralZero
	}
	if n == 1 || n == -1 {
		return PluralOne
	}
	if n == 2 || n == -2 {
		return PluralTwo
	}

	// Handle negative numbers by using absolute value
	absN := n
	if n < 0 {
		absN = -n
	}

	mod100 := absN % 100

	// 3-10 use "few"
	if mod100 >= 3 && mod100 <= 10 {
		return PluralFew
	}

	// 11-99 use "many"
	if mod100 >= 11 && mod100 <= 99 {
		return PluralMany
	}

	return PluralOther
}

// SpanishPluralRule implements plural rules for Spanish.
// Simpler than other Romance languages.
// Categories: one (1), many (1,000,000+), other
var SpanishPluralRule PluralRule = func(n int) string {
	if n == 1 || n == -1 {
		return PluralOne
	}
	// Handle negative numbers
	absN := n
	if n < 0 {
		absN = -n
	}
	if absN >= 1000000 {
		return PluralMany
	}
	return PluralOther
}

// GetPluralRuleForLanguage returns the appropriate plural rule for a given language code.
// It uses the two-letter ISO 639-1 language code (e.g., "en", "fr", "pl").
// Falls back to DefaultPluralRule for unknown languages.
func GetPluralRuleForLanguage(lang string) PluralRule {
	// Normalize language code: take first two letters and lowercase
	if len(lang) >= 2 {
		lang = strings.ToLower(lang[:2])
	}

	switch lang {
	// English and similar
	case "en":
		return EnglishPluralRule

	// Slavic languages
	case "pl", "ru", "cs", "uk", "hr", "sr", "sk", "sl", "bg":
		return SlavicPluralRule

	// Romance languages (excluding Spanish)
	case "fr", "it", "pt":
		return RomancePluralRule

	// Spanish (simpler than other Romance languages)
	case "es":
		return SpanishPluralRule

	// Germanic languages
	case "de", "nl", "sv", "no", "da", "is":
		return GermanicPluralRule

	// Asian languages (no plurals)
	case "ja", "zh", "ko", "th", "vi", "id", "ms":
		return AsianPluralRule

	// Arabic
	case "ar":
		return ArabicPluralRule

	default:
		return DefaultPluralRule
	}
}

// SupportedPluralForms returns which plural forms a rule actually uses.
// This is useful for validation when loading translations.
func SupportedPluralForms(rule PluralRule) []string {
	// Test the rule with various numbers to determine which forms it uses
	forms := make(map[string]bool)

	// Test numbers that typically trigger different plural forms
	testNumbers := []int{0, 1, 2, 3, 4, 5, 10, 11, 12, 13, 14, 20, 21, 22, 100, 1000, 1000000}

	for _, n := range testNumbers {
		form := rule(n)
		forms[form] = true
	}

	// Convert map to sorted slice
	var result []string
	// Order matters for consistency
	order := []string{PluralZero, PluralOne, PluralTwo, PluralFew, PluralMany, PluralOther}
	for _, form := range order {
		if forms[form] {
			result = append(result, form)
		}
	}

	return result
}
