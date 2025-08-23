package i18n_test

import (
	"testing"

	"github.com/dmitrymomot/gokit/core/i18n"
	"github.com/stretchr/testify/assert"
)

func TestDefaultPluralRule(t *testing.T) {
	tests := []struct {
		n        int
		expected string
	}{
		{0, i18n.PluralZero},
		{1, i18n.PluralOne},
		{2, i18n.PluralFew},
		{3, i18n.PluralFew},
		{4, i18n.PluralFew},
		{5, i18n.PluralMany},
		{10, i18n.PluralMany},
		{19, i18n.PluralMany},
		{20, i18n.PluralOther},
		{100, i18n.PluralOther},
		{-1, i18n.PluralOne},
		{-5, i18n.PluralMany},
	}

	for _, tt := range tests {
		t.Run(string(rune(tt.n)), func(t *testing.T) {
			result := i18n.DefaultPluralRule(tt.n)
			assert.Equal(t, tt.expected, result, "For n=%d", tt.n)
		})
	}
}

func TestEnglishPluralRule(t *testing.T) {
	tests := []struct {
		n        int
		expected string
	}{
		{0, i18n.PluralZero},
		{1, i18n.PluralOne},
		{2, i18n.PluralOther},
		{5, i18n.PluralOther},
		{10, i18n.PluralOther},
		{100, i18n.PluralOther},
		{1000, i18n.PluralOther},
		{-1, i18n.PluralOne},
		{-2, i18n.PluralOther},
	}

	for _, tt := range tests {
		t.Run(string(rune(tt.n)), func(t *testing.T) {
			result := i18n.EnglishPluralRule(tt.n)
			assert.Equal(t, tt.expected, result, "For n=%d", tt.n)
		})
	}
}

func TestSlavicPluralRule(t *testing.T) {
	tests := []struct {
		n        int
		expected string
	}{
		// Zero
		{0, i18n.PluralZero},
		// One
		{1, i18n.PluralOne},
		// Few (2-4, but not 12-14)
		{2, i18n.PluralFew},
		{3, i18n.PluralFew},
		{4, i18n.PluralFew},
		{22, i18n.PluralFew},
		{23, i18n.PluralFew},
		{24, i18n.PluralFew},
		{102, i18n.PluralFew},
		// Many (includes 12-14)
		{5, i18n.PluralMany},
		{10, i18n.PluralMany},
		{11, i18n.PluralMany},
		{12, i18n.PluralMany},
		{13, i18n.PluralMany},
		{14, i18n.PluralMany},
		{15, i18n.PluralMany},
		{20, i18n.PluralMany},
		{21, i18n.PluralMany},
		{25, i18n.PluralMany},
		{100, i18n.PluralMany},
		{112, i18n.PluralMany},
		// Negative numbers
		{-1, i18n.PluralOne},
		{-2, i18n.PluralFew},
		{-5, i18n.PluralMany},
		{-12, i18n.PluralMany},
	}

	for _, tt := range tests {
		t.Run(string(rune(tt.n)), func(t *testing.T) {
			result := i18n.SlavicPluralRule(tt.n)
			assert.Equal(t, tt.expected, result, "For n=%d", tt.n)
		})
	}
}

func TestRomancePluralRule(t *testing.T) {
	tests := []struct {
		n        int
		expected string
	}{
		{0, i18n.PluralOne},
		{1, i18n.PluralOne},
		{2, i18n.PluralOther},
		{10, i18n.PluralOther},
		{100, i18n.PluralOther},
		{999999, i18n.PluralOther},
		{1000000, i18n.PluralMany},
		{2000000, i18n.PluralMany},
		{-1, i18n.PluralOne},
		{-2, i18n.PluralOther},
	}

	for _, tt := range tests {
		t.Run(string(rune(tt.n)), func(t *testing.T) {
			result := i18n.RomancePluralRule(tt.n)
			assert.Equal(t, tt.expected, result, "For n=%d", tt.n)
		})
	}
}

func TestGermanicPluralRule(t *testing.T) {
	tests := []struct {
		n        int
		expected string
	}{
		{0, i18n.PluralOther},
		{1, i18n.PluralOne},
		{2, i18n.PluralOther},
		{10, i18n.PluralOther},
		{100, i18n.PluralOther},
		{-1, i18n.PluralOne},
		{-2, i18n.PluralOther},
	}

	for _, tt := range tests {
		t.Run(string(rune(tt.n)), func(t *testing.T) {
			result := i18n.GermanicPluralRule(tt.n)
			assert.Equal(t, tt.expected, result, "For n=%d", tt.n)
		})
	}
}

func TestAsianPluralRule(t *testing.T) {
	tests := []struct {
		n        int
		expected string
	}{
		{0, i18n.PluralOther},
		{1, i18n.PluralOther},
		{2, i18n.PluralOther},
		{10, i18n.PluralOther},
		{100, i18n.PluralOther},
		{1000000, i18n.PluralOther},
		{-1, i18n.PluralOther},
		{-100, i18n.PluralOther},
	}

	for _, tt := range tests {
		t.Run(string(rune(tt.n)), func(t *testing.T) {
			result := i18n.AsianPluralRule(tt.n)
			assert.Equal(t, tt.expected, result, "For n=%d", tt.n)
		})
	}
}

func TestArabicPluralRule(t *testing.T) {
	tests := []struct {
		n        int
		expected string
	}{
		// Zero
		{0, i18n.PluralZero},
		// One
		{1, i18n.PluralOne},
		// Two
		{2, i18n.PluralTwo},
		// Few (3-10, 103-110, 203-210, etc.)
		{3, i18n.PluralFew},
		{4, i18n.PluralFew},
		{10, i18n.PluralFew},
		{103, i18n.PluralFew},
		{110, i18n.PluralFew},
		{203, i18n.PluralFew},
		{210, i18n.PluralFew},
		// Many (11-99, 111-199, 211-299, etc.)
		{11, i18n.PluralMany},
		{15, i18n.PluralMany},
		{20, i18n.PluralMany},
		{50, i18n.PluralMany},
		{99, i18n.PluralMany},
		{111, i18n.PluralMany},
		{150, i18n.PluralMany},
		{199, i18n.PluralMany},
		// Other (100, 101, 102, 200, 201, 202, etc.)
		{100, i18n.PluralOther},
		{101, i18n.PluralOther},
		{102, i18n.PluralOther},
		{200, i18n.PluralOther},
		{300, i18n.PluralOther},
		{1000, i18n.PluralOther},
		// Negative numbers
		{-1, i18n.PluralOne},
		{-2, i18n.PluralTwo},
		{-3, i18n.PluralFew},
		{-11, i18n.PluralMany},
		{-100, i18n.PluralOther},
	}

	for _, tt := range tests {
		t.Run(string(rune(tt.n)), func(t *testing.T) {
			result := i18n.ArabicPluralRule(tt.n)
			assert.Equal(t, tt.expected, result, "For n=%d", tt.n)
		})
	}
}

func TestSpanishPluralRule(t *testing.T) {
	tests := []struct {
		n        int
		expected string
	}{
		{0, i18n.PluralOther},
		{1, i18n.PluralOne},
		{2, i18n.PluralOther},
		{10, i18n.PluralOther},
		{100, i18n.PluralOther},
		{999999, i18n.PluralOther},
		{1000000, i18n.PluralMany},
		{2000000, i18n.PluralMany},
		{-1, i18n.PluralOne},
		{-2, i18n.PluralOther},
	}

	for _, tt := range tests {
		t.Run(string(rune(tt.n)), func(t *testing.T) {
			result := i18n.SpanishPluralRule(tt.n)
			assert.Equal(t, tt.expected, result, "For n=%d", tt.n)
		})
	}
}

func TestGetPluralRuleForLanguage(t *testing.T) {
	tests := []struct {
		lang     string
		n        int
		expected string
	}{
		// English
		{"en", 1, i18n.PluralOne},
		{"en-US", 2, i18n.PluralOther},
		{"EN", 0, i18n.PluralZero},

		// Slavic
		{"pl", 2, i18n.PluralFew},
		{"ru", 5, i18n.PluralMany},
		{"cs", 1, i18n.PluralOne},
		{"uk", 12, i18n.PluralMany},

		// Romance
		{"fr", 0, i18n.PluralOne},
		{"it", 1000000, i18n.PluralMany},
		{"pt-BR", 2, i18n.PluralOther},

		// Spanish (different from other Romance)
		{"es", 1, i18n.PluralOne},
		{"es-MX", 1000000, i18n.PluralMany},

		// Germanic
		{"de", 1, i18n.PluralOne},
		{"nl", 0, i18n.PluralOther},
		{"sv", 2, i18n.PluralOther},

		// Asian
		{"ja", 0, i18n.PluralOther},
		{"zh", 1, i18n.PluralOther},
		{"ko", 100, i18n.PluralOther},

		// Arabic
		{"ar", 0, i18n.PluralZero},
		{"ar", 2, i18n.PluralTwo},
		{"ar", 3, i18n.PluralFew},
		{"ar", 11, i18n.PluralMany},

		// Unknown (uses default)
		{"xyz", 0, i18n.PluralZero},
		{"", 1, i18n.PluralOne},
		{"unknown", 2, i18n.PluralFew},

		// Edge cases
		{"e", 1, i18n.PluralOne},       // Too short, uses default
		{"english", 1, i18n.PluralOne}, // "en" prefix matches English
	}

	for _, tt := range tests {
		t.Run(tt.lang+"_"+string(rune(tt.n)), func(t *testing.T) {
			rule := i18n.GetPluralRuleForLanguage(tt.lang)
			result := rule(tt.n)
			assert.Equal(t, tt.expected, result, "For lang=%s, n=%d", tt.lang, tt.n)
		})
	}
}

func TestSupportedPluralForms(t *testing.T) {
	tests := []struct {
		name     string
		rule     i18n.PluralRule
		expected []string
	}{
		{
			name:     "Default",
			rule:     i18n.DefaultPluralRule,
			expected: []string{i18n.PluralZero, i18n.PluralOne, i18n.PluralFew, i18n.PluralMany, i18n.PluralOther},
		},
		{
			name:     "English",
			rule:     i18n.EnglishPluralRule,
			expected: []string{i18n.PluralZero, i18n.PluralOne, i18n.PluralOther},
		},
		{
			name:     "Slavic",
			rule:     i18n.SlavicPluralRule,
			expected: []string{i18n.PluralZero, i18n.PluralOne, i18n.PluralFew, i18n.PluralMany},
		},
		{
			name:     "Romance",
			rule:     i18n.RomancePluralRule,
			expected: []string{i18n.PluralOne, i18n.PluralMany, i18n.PluralOther},
		},
		{
			name:     "Germanic",
			rule:     i18n.GermanicPluralRule,
			expected: []string{i18n.PluralOne, i18n.PluralOther},
		},
		{
			name:     "Asian",
			rule:     i18n.AsianPluralRule,
			expected: []string{i18n.PluralOther},
		},
		{
			name:     "Arabic",
			rule:     i18n.ArabicPluralRule,
			expected: []string{i18n.PluralZero, i18n.PluralOne, i18n.PluralTwo, i18n.PluralFew, i18n.PluralMany, i18n.PluralOther},
		},
		{
			name:     "Spanish",
			rule:     i18n.SpanishPluralRule,
			expected: []string{i18n.PluralOne, i18n.PluralMany, i18n.PluralOther},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := i18n.SupportedPluralForms(tt.rule)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func BenchmarkPluralRules(b *testing.B) {
	benchmarks := []struct {
		name string
		rule i18n.PluralRule
	}{
		{"Default", i18n.DefaultPluralRule},
		{"English", i18n.EnglishPluralRule},
		{"Slavic", i18n.SlavicPluralRule},
		{"Romance", i18n.RomancePluralRule},
		{"Germanic", i18n.GermanicPluralRule},
		{"Asian", i18n.AsianPluralRule},
		{"Arabic", i18n.ArabicPluralRule},
		{"Spanish", i18n.SpanishPluralRule},
	}

	testNumbers := []int{0, 1, 2, 3, 5, 11, 21, 100, 1000}

	for _, bench := range benchmarks {
		b.Run(bench.name, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				for _, n := range testNumbers {
					_ = bench.rule(n)
				}
			}
		})
	}
}

func BenchmarkGetPluralRuleForLanguage(b *testing.B) {
	languages := []string{"en", "pl", "fr", "de", "ja", "ar", "es", "xyz"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, lang := range languages {
			_ = i18n.GetPluralRuleForLanguage(lang)
		}
	}
}
