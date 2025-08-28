package i18n_test

import (
	"testing"
	"time"

	"github.com/dmitrymomot/gokit/core/i18n"
	"github.com/stretchr/testify/assert"
)

func TestLocaleFormat_FormatNumber(t *testing.T) {
	t.Run("English format", func(t *testing.T) {
		lf := i18n.NewEnglishFormat()

		assert.Equal(t, "1,234", lf.FormatNumber(1234))
		assert.Equal(t, "1,234.5", lf.FormatNumber(1234.5))
		assert.Equal(t, "1,234,567.89", lf.FormatNumber(1234567.89))
		assert.Equal(t, "-1,234.5", lf.FormatNumber(-1234.5))
		assert.Equal(t, "123", lf.FormatNumber(123))
		assert.Equal(t, "0", lf.FormatNumber(0))
	})

	t.Run("Custom format with options", func(t *testing.T) {
		// European-style formatting
		lf := i18n.NewLocaleFormat(
			i18n.WithDecimalSeparator(","),
			i18n.WithThousandSeparator("."),
		)

		assert.Equal(t, "1.234", lf.FormatNumber(1234))
		assert.Equal(t, "1.234,5", lf.FormatNumber(1234.5))
		assert.Equal(t, "1.234.567,89", lf.FormatNumber(1234567.89))
		assert.Equal(t, "-1.234,5", lf.FormatNumber(-1234.5))
	})

	t.Run("Space as thousand separator", func(t *testing.T) {
		lf := i18n.NewLocaleFormat(
			i18n.WithDecimalSeparator(","),
			i18n.WithThousandSeparator(" "),
		)

		assert.Equal(t, "1 234", lf.FormatNumber(1234))
		assert.Equal(t, "1 234,5", lf.FormatNumber(1234.5))
		assert.Equal(t, "1 234 567,89", lf.FormatNumber(1234567.89))
	})
}

func TestLocaleFormat_FormatCurrency(t *testing.T) {
	t.Run("English/USD format", func(t *testing.T) {
		lf := i18n.NewEnglishFormat()

		assert.Equal(t, "$1,234.50", lf.FormatCurrency(1234.50))
		assert.Equal(t, "$1,234.00", lf.FormatCurrency(1234))
		assert.Equal(t, "-$1,234.50", lf.FormatCurrency(-1234.50))
		assert.Equal(t, "$0.99", lf.FormatCurrency(0.99))
		assert.Equal(t, "$0.00", lf.FormatCurrency(0))
	})

	t.Run("Custom Euro format", func(t *testing.T) {
		lf := i18n.NewLocaleFormat(
			i18n.WithDecimalSeparator(","),
			i18n.WithThousandSeparator("."),
			i18n.WithCurrencySymbol("€"),
			i18n.WithCurrencyPosition("after"),
		)

		assert.Equal(t, "1.234,50 €", lf.FormatCurrency(1234.50))
		assert.Equal(t, "1.234,00 €", lf.FormatCurrency(1234))
		assert.Equal(t, "-1.234,50 €", lf.FormatCurrency(-1234.50))
	})

	t.Run("Custom Pound format", func(t *testing.T) {
		lf := i18n.NewLocaleFormat(
			i18n.WithCurrencySymbol("£"),
		)

		assert.Equal(t, "£1,234.50", lf.FormatCurrency(1234.50))
	})

	t.Run("Custom Brazilian Real format", func(t *testing.T) {
		lf := i18n.NewLocaleFormat(
			i18n.WithDecimalSeparator(","),
			i18n.WithThousandSeparator("."),
			i18n.WithCurrencySymbol("R$"),
		)

		assert.Equal(t, "R$1.234,50", lf.FormatCurrency(1234.50))
	})
}

func TestLocaleFormat_FormatPercent(t *testing.T) {
	t.Run("English format", func(t *testing.T) {
		lf := i18n.NewEnglishFormat()

		assert.Equal(t, "50%", lf.FormatPercent(0.5))
		assert.Equal(t, "100%", lf.FormatPercent(1.0))
		assert.Equal(t, "0%", lf.FormatPercent(0))
		assert.Equal(t, "25.5%", lf.FormatPercent(0.255))
		assert.Equal(t, "-15%", lf.FormatPercent(-0.15))
		assert.Equal(t, "0.5%", lf.FormatPercent(0.005))
	})

	t.Run("Custom format with comma decimal", func(t *testing.T) {
		lf := i18n.NewLocaleFormat(
			i18n.WithDecimalSeparator(","),
		)

		assert.Equal(t, "50%", lf.FormatPercent(0.5))
		assert.Equal(t, "25,5%", lf.FormatPercent(0.255))
		assert.Equal(t, "-15%", lf.FormatPercent(-0.15))
	})
}

func TestLocaleFormat_FormatDate(t *testing.T) {
	testDate := time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC)

	t.Run("English format", func(t *testing.T) {
		lf := i18n.NewEnglishFormat()
		assert.Equal(t, "01/02/2024", lf.FormatDate(testDate))
	})

	t.Run("Custom European format", func(t *testing.T) {
		lf := i18n.NewLocaleFormat(
			i18n.WithDateFormat("02.01.2006"),
		)
		assert.Equal(t, "02.01.2024", lf.FormatDate(testDate))
	})

	t.Run("Custom ISO format", func(t *testing.T) {
		lf := i18n.NewLocaleFormat(
			i18n.WithDateFormat("2006-01-02"),
		)
		assert.Equal(t, "2024-01-02", lf.FormatDate(testDate))
	})
}

func TestLocaleFormat_FormatTime(t *testing.T) {
	testTime := time.Date(2024, 1, 2, 15, 4, 0, 0, time.UTC)

	t.Run("English 12-hour format", func(t *testing.T) {
		lf := i18n.NewEnglishFormat()
		assert.Equal(t, "3:04 PM", lf.FormatTime(testTime))
	})

	t.Run("Custom 24-hour format", func(t *testing.T) {
		lf := i18n.NewLocaleFormat(
			i18n.WithTimeFormat("15:04"),
		)
		assert.Equal(t, "15:04", lf.FormatTime(testTime))
	})

	t.Run("Custom format with seconds", func(t *testing.T) {
		lf := i18n.NewLocaleFormat(
			i18n.WithTimeFormat("15:04:05"),
		)
		assert.Equal(t, "15:04:00", lf.FormatTime(testTime))
	})
}

func TestLocaleFormat_FormatDateTime(t *testing.T) {
	testDateTime := time.Date(2024, 1, 2, 15, 4, 0, 0, time.UTC)

	t.Run("English format", func(t *testing.T) {
		lf := i18n.NewEnglishFormat()
		assert.Equal(t, "01/02/2024 3:04 PM", lf.FormatDateTime(testDateTime))
	})

	t.Run("Custom European format", func(t *testing.T) {
		lf := i18n.NewLocaleFormat(
			i18n.WithDateTimeFormat("02.01.2006 15:04"),
		)
		assert.Equal(t, "02.01.2024 15:04", lf.FormatDateTime(testDateTime))
	})

	t.Run("Custom ISO format", func(t *testing.T) {
		lf := i18n.NewLocaleFormat(
			i18n.WithDateTimeFormat("2006-01-02T15:04:05Z07:00"),
		)
		assert.Equal(t, "2024-01-02T15:04:00Z", lf.FormatDateTime(testDateTime))
	})
}

func TestTranslatorWithFormat(t *testing.T) {
	// Create an i18n instance
	i18nInstance, err := i18n.New(
		i18n.WithDefaultLanguage("en"),
		i18n.WithTranslations("en", "test", map[string]any{
			"test": "test",
		}),
	)
	assert.NoError(t, err)

	t.Run("Default English format", func(t *testing.T) {
		translator := i18n.NewTranslator(i18nInstance, "en", "test")

		assert.Equal(t, "1,234.5", translator.FormatNumber(1234.5))
		assert.Equal(t, "$1,234.50", translator.FormatCurrency(1234.50))
		assert.Equal(t, "25.5%", translator.FormatPercent(0.255))

		testDate := time.Date(2024, 1, 2, 15, 4, 0, 0, time.UTC)
		assert.Equal(t, "01/02/2024", translator.FormatDate(testDate))
		assert.Equal(t, "3:04 PM", translator.FormatTime(testDate))
		assert.Equal(t, "01/02/2024 3:04 PM", translator.FormatDateTime(testDate))
	})

	t.Run("Custom format with NewTranslatorWithFormat", func(t *testing.T) {
		customFormat := i18n.NewLocaleFormat(
			i18n.WithDecimalSeparator(","),
			i18n.WithThousandSeparator("."),
			i18n.WithCurrencySymbol("€"),
			i18n.WithCurrencyPosition("after"),
			i18n.WithDateFormat("02.01.2006"),
			i18n.WithTimeFormat("15:04"),
			i18n.WithDateTimeFormat("02.01.2006 15:04"),
		)

		translator := i18n.NewTranslatorWithFormat(i18nInstance, "en", "test", customFormat)

		assert.Equal(t, "1.234,5", translator.FormatNumber(1234.5))
		assert.Equal(t, "1.234,50 €", translator.FormatCurrency(1234.50))
		assert.Equal(t, "25,5%", translator.FormatPercent(0.255))

		testDate := time.Date(2024, 1, 2, 15, 4, 0, 0, time.UTC)
		assert.Equal(t, "02.01.2024", translator.FormatDate(testDate))
		assert.Equal(t, "15:04", translator.FormatTime(testDate))
		assert.Equal(t, "02.01.2024 15:04", translator.FormatDateTime(testDate))
	})

	t.Run("Access format from translator", func(t *testing.T) {
		translator := i18n.NewTranslator(i18nInstance, "en", "test")
		format := translator.Format()

		assert.NotNil(t, format)
		assert.Equal(t, "1,234.5", format.FormatNumber(1234.5))
	})
}
