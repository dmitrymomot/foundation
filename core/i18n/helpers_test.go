package i18n_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/dmitrymomot/foundation/core/i18n"
)

func TestParseAcceptLanguage(t *testing.T) {
	tests := []struct {
		name      string
		header    string
		available []string
		expected  string
	}{
		{
			name:      "empty header returns first available",
			header:    "",
			available: []string{"en", "pl", "de"},
			expected:  "en",
		},
		{
			name:      "empty available returns empty",
			header:    "en-US,en;q=0.9",
			available: []string{},
			expected:  "",
		},
		{
			name:      "exact match",
			header:    "pl",
			available: []string{"en", "pl", "de"},
			expected:  "pl",
		},
		{
			name:      "match with quality values",
			header:    "de;q=0.5,pl;q=0.9,en;q=0.8",
			available: []string{"en", "pl", "de"},
			expected:  "pl",
		},
		{
			name:      "language with region matches base",
			header:    "en-US",
			available: []string{"en", "pl", "de"},
			expected:  "en",
		},
		{
			name:      "base language matches regional variant",
			header:    "en",
			available: []string{"en-US", "pl", "de"},
			expected:  "en-US",
		},
		{
			name:      "multiple languages with decreasing quality",
			header:    "fr,en-US;q=0.9,en;q=0.8,pl;q=0.7",
			available: []string{"pl", "en"},
			expected:  "en",
		},
		{
			name:      "no match returns first available",
			header:    "fr,es,it",
			available: []string{"en", "pl", "de"},
			expected:  "en",
		},
		{
			name:      "complex header with multiple regions",
			header:    "en-GB,en-US;q=0.9,en;q=0.8,pl-PL;q=0.7,pl;q=0.6",
			available: []string{"pl", "en"},
			expected:  "en",
		},
		{
			name:      "case insensitive matching",
			header:    "EN-us,PL;q=0.9",
			available: []string{"pl", "en"},
			expected:  "en",
		},
		{
			name:      "whitespace handling",
			header:    " en , pl ; q=0.9 , de ; q=0.8 ",
			available: []string{"de", "pl"},
			expected:  "pl",
		},
		{
			name:      "invalid quality value defaults to 1.0",
			header:    "en;q=invalid,pl;q=0.5",
			available: []string{"en", "pl"},
			expected:  "en",
		},
		{
			name:      "wildcard is ignored",
			header:    "*,en;q=0.5",
			available: []string{"en", "pl"},
			expected:  "en",
		},
		{
			name:      "first match wins for same quality",
			header:    "en,pl",
			available: []string{"pl", "en"},
			expected:  "pl",
		},
		{
			name:      "regional variant exact match preferred",
			header:    "en-US,en;q=0.9",
			available: []string{"en", "en-US"},
			expected:  "en-US",
		},
		{
			name:      "oversized header is truncated safely",
			header:    strings.Repeat("en,", 2000) + "pl",
			available: []string{"en", "pl", "de"},
			expected:  "en",
		},
		{
			name:      "quality values outside 0-1 range default to 1.0",
			header:    "en;q=2.5,pl;q=-0.5,de;q=0.5",
			available: []string{"en", "pl", "de"},
			expected:  "en", // en gets q=1.0 (invalid 2.5), pl gets q=1.0 (invalid -0.5), de gets q=0.5
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := i18n.ParseAcceptLanguage(tt.header, tt.available)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestReplacePlaceholders(t *testing.T) {
	tests := []struct {
		name         string
		template     string
		placeholders i18n.M
		expected     string
	}{
		{
			name:         "no placeholders",
			template:     "Hello, World!",
			placeholders: nil,
			expected:     "Hello, World!",
		},
		{
			name:         "single placeholder",
			template:     "Hello, %{name}!",
			placeholders: i18n.M{"name": "John"},
			expected:     "Hello, John!",
		},
		{
			name:         "multiple placeholders",
			template:     "Welcome, %{name}! You have %{count} messages.",
			placeholders: i18n.M{"name": "Alice", "count": 5},
			expected:     "Welcome, Alice! You have 5 messages.",
		},
		{
			name:         "missing placeholder remains unchanged",
			template:     "Hello, %{name}! Your ID is %{id}.",
			placeholders: i18n.M{"name": "Bob"},
			expected:     "Hello, Bob! Your ID is %{id}.",
		},
		{
			name:         "integer values",
			template:     "You have %{count} items in your cart.",
			placeholders: i18n.M{"count": 42},
			expected:     "You have 42 items in your cart.",
		},
		{
			name:         "float values",
			template:     "Your balance is $%{amount}.",
			placeholders: i18n.M{"amount": 123.45},
			expected:     "Your balance is $123.45.",
		},
		{
			name:         "boolean values",
			template:     "Feature enabled: %{enabled}",
			placeholders: i18n.M{"enabled": true},
			expected:     "Feature enabled: true",
		},
		{
			name:         "repeated placeholders",
			template:     "%{name} is here. Hello, %{name}!",
			placeholders: i18n.M{"name": "Charlie"},
			expected:     "Charlie is here. Hello, Charlie!",
		},
		{
			name:         "empty placeholder map",
			template:     "Hello, %{name}!",
			placeholders: i18n.M{},
			expected:     "Hello, %{name}!",
		},
		{
			name:         "placeholder with special characters",
			template:     "Path: %{path}",
			placeholders: i18n.M{"path": "/usr/local/bin"},
			expected:     "Path: /usr/local/bin",
		},
		{
			name:         "nil value",
			template:     "Value: %{val}",
			placeholders: i18n.M{"val": nil},
			expected:     "Value: <nil>",
		},
		{
			name:         "placeholder names with underscores",
			template:     "User %{user_name} has %{item_count} items",
			placeholders: i18n.M{"user_name": "Dave", "item_count": 10},
			expected:     "User Dave has 10 items",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := i18n.ReplacePlaceholders(tt.template, tt.placeholders)
			assert.Equal(t, tt.expected, result)
		})
	}
}
