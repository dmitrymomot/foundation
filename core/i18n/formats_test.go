package i18n_test

import (
	"testing"
	"time"

	"github.com/dmitrymomot/gokit/core/i18n"
	"github.com/stretchr/testify/assert"
)

func TestFormatNumber(t *testing.T) {
	tests := []struct {
		name     string
		number   float64
		lang     string
		expected string
	}{
		// English formatting
		{"English integer", 1234, "en", "1,234"},
		{"English decimal", 1234.5, "en", "1,234.5"},
		{"English large number", 1234567.89, "en", "1,234,567.89"},
		{"English negative", -1234.5, "en", "-1,234.5"},
		{"English small number", 123, "en", "123"},
		{"English zero", 0, "en", "0"},

		// German formatting
		{"German integer", 1234, "de", "1.234"},
		{"German decimal", 1234.5, "de", "1.234,5"},
		{"German large number", 1234567.89, "de", "1.234.567,89"},
		{"German negative", -1234.5, "de", "-1.234,5"},

		// French formatting
		{"French integer", 1234, "fr", "1 234"},
		{"French decimal", 1234.5, "fr", "1 234,5"},
		{"French large number", 1234567.89, "fr", "1 234 567,89"},

		// Spanish formatting
		{"Spanish integer", 1234, "es", "1.234"},
		{"Spanish decimal", 1234.5, "es", "1.234,5"},

		// Fallback to English for unknown language
		{"Unknown language", 1234.5, "xx", "1,234.5"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := i18n.FormatNumber(tt.number, tt.lang)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFormatCurrency(t *testing.T) {
	tests := []struct {
		name     string
		amount   float64
		lang     string
		expected string
	}{
		// English/USD formatting
		{"English positive", 1234.50, "en", "$1,234.50"},
		{"English integer", 1234, "en", "$1,234.00"},
		{"English negative", -1234.50, "en", "-$1,234.50"},
		{"English cents only", 0.99, "en", "$0.99"},
		{"English zero", 0, "en", "$0.00"},

		// German/Euro formatting
		{"German positive", 1234.50, "de", "1.234,50 €"},
		{"German integer", 1234, "de", "1.234,00 €"},
		{"German negative", -1234.50, "de", "-1.234,50 €"},

		// Spanish/Euro formatting
		{"Spanish positive", 1234.50, "es", "1.234,50 €"},

		// French/Euro formatting
		{"French positive", 1234.50, "fr", "1 234,50 €"},

		// Japanese/Yen formatting
		{"Japanese positive", 1234.50, "ja", "¥1,234.50"},

		// Portuguese/Real formatting
		{"Portuguese positive", 1234.50, "pt", "R$1.234,50"},

		// Russian/Ruble formatting
		{"Russian positive", 1234.50, "ru", "1 234,50 ₽"},

		// Unknown language fallback
		{"Unknown language", 1234.50, "xx", "$1,234.50"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := i18n.FormatCurrency(tt.amount, tt.lang)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFormatPercent(t *testing.T) {
	tests := []struct {
		name     string
		value    float64
		lang     string
		expected string
	}{
		// English formatting
		{"English 50%", 0.5, "en", "50%"},
		{"English 100%", 1.0, "en", "100%"},
		{"English 0%", 0, "en", "0%"},
		{"English 25.5%", 0.255, "en", "25.5%"},
		{"English negative", -0.15, "en", "-15%"},
		{"English small", 0.005, "en", "0.5%"},

		// German formatting
		{"German 50%", 0.5, "de", "50%"},
		{"German 25.5%", 0.255, "de", "25,5%"},

		// French formatting
		{"French 50%", 0.5, "fr", "50%"},
		{"French 25.5%", 0.255, "fr", "25,5%"},

		// Unknown language fallback
		{"Unknown language", 0.5, "xx", "50%"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := i18n.FormatPercent(tt.value, tt.lang)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFormatDate(t *testing.T) {
	// Use a fixed date for testing: January 2, 2024
	testDate := time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC)

	tests := []struct {
		name     string
		date     time.Time
		lang     string
		expected string
	}{
		{"English date", testDate, "en", "01/02/2024"},
		{"German date", testDate, "de", "02.01.2024"},
		{"French date", testDate, "fr", "02/01/2024"},
		{"Spanish date", testDate, "es", "02/01/2024"},
		{"Japanese date", testDate, "ja", "2024/01/02"},
		{"Chinese date", testDate, "zh", "2024/01/02"},
		{"Dutch date", testDate, "nl", "02-01-2024"},
		{"Russian date", testDate, "ru", "02.01.2024"},
		{"Unknown language", testDate, "xx", "01/02/2024"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := i18n.FormatDate(tt.date, tt.lang)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFormatTime(t *testing.T) {
	// Use a fixed time for testing: 3:04 PM
	testTime := time.Date(2024, 1, 2, 15, 4, 0, 0, time.UTC)

	tests := []struct {
		name     string
		time     time.Time
		lang     string
		expected string
	}{
		{"English time", testTime, "en", "3:04 PM"},
		{"German time", testTime, "de", "15:04"},
		{"French time", testTime, "fr", "15:04"},
		{"Spanish time", testTime, "es", "15:04"},
		{"Japanese time", testTime, "ja", "15:04"},
		{"Unknown language", testTime, "xx", "3:04 PM"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := i18n.FormatTime(tt.time, tt.lang)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFormatDateTime(t *testing.T) {
	// Use a fixed datetime for testing: January 2, 2024 3:04 PM
	testDateTime := time.Date(2024, 1, 2, 15, 4, 0, 0, time.UTC)

	tests := []struct {
		name     string
		datetime time.Time
		lang     string
		expected string
	}{
		{"English datetime", testDateTime, "en", "01/02/2024 3:04 PM"},
		{"German datetime", testDateTime, "de", "02.01.2024 15:04"},
		{"French datetime", testDateTime, "fr", "02/01/2024 15:04"},
		{"Spanish datetime", testDateTime, "es", "02/01/2024 15:04"},
		{"Japanese datetime", testDateTime, "ja", "2024/01/02 15:04"},
		{"Chinese datetime", testDateTime, "zh", "2024/01/02 15:04"},
		{"Dutch datetime", testDateTime, "nl", "02-01-2024 15:04"},
		{"Russian datetime", testDateTime, "ru", "02.01.2024 15:04"},
		{"Unknown language", testDateTime, "xx", "01/02/2024 3:04 PM"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := i18n.FormatDateTime(tt.datetime, tt.lang)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestTranslatorFormatMethods(t *testing.T) {
	// Create an i18n instance
	i18nInstance, err := i18n.New(
		i18n.WithDefaultLanguage("en"),
		i18n.WithTranslations("en", "test", map[string]any{
			"test": "test",
		}),
	)
	assert.NoError(t, err)

	t.Run("FormatNumber", func(t *testing.T) {
		translator := i18n.NewTranslator(i18nInstance, "de", "test")
		result := translator.FormatNumber(1234.5)
		assert.Equal(t, "1.234,5", result)
	})

	t.Run("FormatCurrency", func(t *testing.T) {
		translator := i18n.NewTranslator(i18nInstance, "de", "test")
		result := translator.FormatCurrency(1234.50)
		assert.Equal(t, "1.234,50 €", result)
	})

	t.Run("FormatPercent", func(t *testing.T) {
		translator := i18n.NewTranslator(i18nInstance, "fr", "test")
		result := translator.FormatPercent(0.255)
		assert.Equal(t, "25,5%", result)
	})

	t.Run("FormatDate", func(t *testing.T) {
		translator := i18n.NewTranslator(i18nInstance, "ja", "test")
		testDate := time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC)
		result := translator.FormatDate(testDate)
		assert.Equal(t, "2024/01/02", result)
	})

	t.Run("FormatTime", func(t *testing.T) {
		translator := i18n.NewTranslator(i18nInstance, "en", "test")
		testTime := time.Date(2024, 1, 2, 15, 4, 0, 0, time.UTC)
		result := translator.FormatTime(testTime)
		assert.Equal(t, "3:04 PM", result)
	})

	t.Run("FormatDateTime", func(t *testing.T) {
		translator := i18n.NewTranslator(i18nInstance, "es", "test")
		testDateTime := time.Date(2024, 1, 2, 15, 4, 0, 0, time.UTC)
		result := translator.FormatDateTime(testDateTime)
		assert.Equal(t, "02/01/2024 15:04", result)
	})
}
