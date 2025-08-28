package i18n

import (
	"fmt"
	"math"
	"strings"
	"time"
)

// LocaleFormat contains formatting rules for a specific locale
type LocaleFormat struct {
	DecimalSeparator  string
	ThousandSeparator string
	CurrencySymbol    string
	CurrencyPosition  string // "before" or "after"
	PercentSymbol     string
	DateFormat        string // Default date format
	TimeFormat        string // Default time format
	DateTimeFormat    string // Default datetime format
}

// Default locale formats for common languages
var localeFormats = map[string]*LocaleFormat{
	"en": {
		DecimalSeparator:  ".",
		ThousandSeparator: ",",
		CurrencySymbol:    "$",
		CurrencyPosition:  "before",
		PercentSymbol:     "%",
		DateFormat:        "01/02/2006",
		TimeFormat:        "3:04 PM",
		DateTimeFormat:    "01/02/2006 3:04 PM",
	},
	"es": {
		DecimalSeparator:  ",",
		ThousandSeparator: ".",
		CurrencySymbol:    "€",
		CurrencyPosition:  "after",
		PercentSymbol:     "%",
		DateFormat:        "02/01/2006",
		TimeFormat:        "15:04",
		DateTimeFormat:    "02/01/2006 15:04",
	},
	"fr": {
		DecimalSeparator:  ",",
		ThousandSeparator: " ",
		CurrencySymbol:    "€",
		CurrencyPosition:  "after",
		PercentSymbol:     "%",
		DateFormat:        "02/01/2006",
		TimeFormat:        "15:04",
		DateTimeFormat:    "02/01/2006 15:04",
	},
	"de": {
		DecimalSeparator:  ",",
		ThousandSeparator: ".",
		CurrencySymbol:    "€",
		CurrencyPosition:  "after",
		PercentSymbol:     "%",
		DateFormat:        "02.01.2006",
		TimeFormat:        "15:04",
		DateTimeFormat:    "02.01.2006 15:04",
	},
	"ja": {
		DecimalSeparator:  ".",
		ThousandSeparator: ",",
		CurrencySymbol:    "¥",
		CurrencyPosition:  "before",
		PercentSymbol:     "%",
		DateFormat:        "2006/01/02",
		TimeFormat:        "15:04",
		DateTimeFormat:    "2006/01/02 15:04",
	},
	"zh": {
		DecimalSeparator:  ".",
		ThousandSeparator: ",",
		CurrencySymbol:    "¥",
		CurrencyPosition:  "before",
		PercentSymbol:     "%",
		DateFormat:        "2006/01/02",
		TimeFormat:        "15:04",
		DateTimeFormat:    "2006/01/02 15:04",
	},
	"pt": {
		DecimalSeparator:  ",",
		ThousandSeparator: ".",
		CurrencySymbol:    "R$",
		CurrencyPosition:  "before",
		PercentSymbol:     "%",
		DateFormat:        "02/01/2006",
		TimeFormat:        "15:04",
		DateTimeFormat:    "02/01/2006 15:04",
	},
	"ru": {
		DecimalSeparator:  ",",
		ThousandSeparator: " ",
		CurrencySymbol:    "₽",
		CurrencyPosition:  "after",
		PercentSymbol:     "%",
		DateFormat:        "02.01.2006",
		TimeFormat:        "15:04",
		DateTimeFormat:    "02.01.2006 15:04",
	},
	"it": {
		DecimalSeparator:  ",",
		ThousandSeparator: ".",
		CurrencySymbol:    "€",
		CurrencyPosition:  "after",
		PercentSymbol:     "%",
		DateFormat:        "02/01/2006",
		TimeFormat:        "15:04",
		DateTimeFormat:    "02/01/2006 15:04",
	},
	"nl": {
		DecimalSeparator:  ",",
		ThousandSeparator: ".",
		CurrencySymbol:    "€",
		CurrencyPosition:  "before",
		PercentSymbol:     "%",
		DateFormat:        "02-01-2006",
		TimeFormat:        "15:04",
		DateTimeFormat:    "02-01-2006 15:04",
	},
}

// getLocaleFormat returns the format for a language, falling back to English
func getLocaleFormat(lang string) *LocaleFormat {
	if format, ok := localeFormats[lang]; ok {
		return format
	}
	// Try language without region (e.g., "en-US" -> "en")
	if idx := strings.Index(lang, "-"); idx > 0 {
		if format, ok := localeFormats[lang[:idx]]; ok {
			return format
		}
	}
	// Fall back to English
	return localeFormats["en"]
}

// FormatNumber formats a number with locale-specific separators
func FormatNumber(n float64, lang string) string {
	format := getLocaleFormat(lang)

	// Handle negative numbers
	negative := n < 0
	if negative {
		n = -n
	}

	// Split into integer and decimal parts
	intPart := int64(n)
	decPart := n - float64(intPart)

	// Format integer part with thousand separators
	intStr := formatIntegerWithSeparator(intPart, format.ThousandSeparator)

	// Handle decimal part
	result := intStr
	if decPart > 0 {
		// Round to 2 decimal places
		decPart = math.Round(decPart*100) / 100
		decStr := fmt.Sprintf("%.2f", decPart)[2:] // Remove "0."
		// Remove trailing zeros
		decStr = strings.TrimRight(decStr, "0")
		if decStr != "" {
			result = intStr + format.DecimalSeparator + decStr
		}
	}

	if negative {
		result = "-" + result
	}

	return result
}

// FormatCurrency formats a currency amount with locale-specific formatting
func FormatCurrency(amount float64, lang string) string {
	format := getLocaleFormat(lang)

	// Check if negative and remove sign for formatting
	negative := amount < 0
	if negative {
		amount = -amount
	}

	// Format the number part (without negative sign)
	numStr := formatCurrencyNumber(amount, format)

	// Add currency symbol
	var result string
	if format.CurrencyPosition == "before" {
		if format.CurrencySymbol == "$" || format.CurrencySymbol == "R$" || format.CurrencySymbol == "¥" {
			result = format.CurrencySymbol + numStr
		} else {
			result = format.CurrencySymbol + " " + numStr
		}
	} else {
		result = numStr + " " + format.CurrencySymbol
	}

	// Add negative sign at the beginning if needed
	if negative {
		result = "-" + result
	}

	return result
}

// FormatPercent formats a percentage with locale-specific formatting
func FormatPercent(n float64, lang string) string {
	format := getLocaleFormat(lang)

	// Convert to percentage (multiply by 100)
	percentage := n * 100

	// Format the number
	numStr := formatPercentNumber(percentage, format)

	return numStr + format.PercentSymbol
}

// FormatDate formats a date with locale-specific formatting
func FormatDate(t time.Time, lang string) string {
	format := getLocaleFormat(lang)
	return t.Format(format.DateFormat)
}

// FormatTime formats a time with locale-specific formatting
func FormatTime(t time.Time, lang string) string {
	format := getLocaleFormat(lang)
	return t.Format(format.TimeFormat)
}

// FormatDateTime formats a datetime with locale-specific formatting
func FormatDateTime(t time.Time, lang string) string {
	format := getLocaleFormat(lang)
	return t.Format(format.DateTimeFormat)
}

// formatIntegerWithSeparator adds thousand separators to an integer
func formatIntegerWithSeparator(n int64, sep string) string {
	if n < 1000 {
		return fmt.Sprintf("%d", n)
	}

	str := fmt.Sprintf("%d", n)
	var result []string

	for i := len(str); i > 0; i -= 3 {
		start := i - 3
		if start < 0 {
			start = 0
		}
		result = append([]string{str[start:i]}, result...)
	}

	return strings.Join(result, sep)
}

// formatCurrencyNumber formats a number for currency display (always 2 decimal places)
func formatCurrencyNumber(n float64, format *LocaleFormat) string {
	// Note: negative handling is done in FormatCurrency
	// Round to 2 decimal places
	n = math.Round(n*100) / 100

	// Split into integer and decimal parts
	intPart := int64(n)
	decPart := n - float64(intPart)

	// Format integer part with thousand separators
	intStr := formatIntegerWithSeparator(intPart, format.ThousandSeparator)

	// Always show 2 decimal places for currency
	decStr := fmt.Sprintf("%.2f", decPart)[2:] // Remove "0."
	result := intStr + format.DecimalSeparator + decStr

	return result
}

// formatPercentNumber formats a number for percentage display
func formatPercentNumber(n float64, format *LocaleFormat) string {
	// Handle negative numbers
	negative := n < 0
	if negative {
		n = -n
	}

	// Round to 1 decimal place for percentages
	n = math.Round(n*10) / 10

	// Split into integer and decimal parts
	intPart := int64(n)
	decPart := n - float64(intPart)

	// Format integer part (no thousand separators for percentages typically)
	intStr := fmt.Sprintf("%d", intPart)

	// Handle decimal part
	result := intStr
	if decPart > 0 {
		decStr := fmt.Sprintf("%.1f", decPart)[2:] // Remove "0."
		// Remove trailing zeros
		decStr = strings.TrimRight(decStr, "0")
		if decStr != "" {
			result = intStr + format.DecimalSeparator + decStr
		}
	}

	if negative {
		result = "-" + result
	}

	return result
}
