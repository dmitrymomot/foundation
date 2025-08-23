package i18n_test

import (
	"testing"

	"github.com/dmitrymomot/gokit/core/i18n"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	t.Run("creates instance with defaults", func(t *testing.T) {
		i18nInstance, err := i18n.New()
		require.NoError(t, err)
		assert.NotNil(t, i18nInstance)
	})

	t.Run("sets custom default language", func(t *testing.T) {
		i18nInstance, err := i18n.New(
			i18n.WithDefaultLanguage("pl"),
		)
		require.NoError(t, err)
		assert.NotNil(t, i18nInstance)
	})

	t.Run("returns error for empty default language", func(t *testing.T) {
		_, err := i18n.New(
			i18n.WithDefaultLanguage(""),
		)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "language cannot be empty")
	})

	t.Run("loads translations", func(t *testing.T) {
		i18nInstance, err := i18n.New(
			i18n.WithTranslations("en", "general", map[string]any{
				"hello": "Hello",
			}),
		)
		require.NoError(t, err)
		assert.NotNil(t, i18nInstance)
	})

	t.Run("returns error for empty language in translations", func(t *testing.T) {
		_, err := i18n.New(
			i18n.WithTranslations("", "general", map[string]any{
				"hello": "Hello",
			}),
		)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "language cannot be empty")
	})

	t.Run("returns error for empty namespace in translations", func(t *testing.T) {
		_, err := i18n.New(
			i18n.WithTranslations("en", "", map[string]any{
				"hello": "Hello",
			}),
		)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "namespace cannot be empty")
	})

	t.Run("allows empty translations map", func(t *testing.T) {
		i18nInstance, err := i18n.New(
			i18n.WithTranslations("en", "general", map[string]any{}),
		)
		require.NoError(t, err)
		assert.NotNil(t, i18nInstance)
	})

	t.Run("sets custom plural rule", func(t *testing.T) {
		customRule := func(n int) string {
			if n == 1 {
				return "one"
			}
			return "other"
		}

		i18nInstance, err := i18n.New(
			i18n.WithPluralRule("en", customRule),
		)
		require.NoError(t, err)
		assert.NotNil(t, i18nInstance)
	})

	t.Run("returns error for nil plural rule", func(t *testing.T) {
		_, err := i18n.New(
			i18n.WithPluralRule("en", nil),
		)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "plural rule cannot be nil")
	})
}

func TestT(t *testing.T) {
	setup := func() *i18n.I18n {
		i18nInstance, _ := i18n.New(
			i18n.WithDefaultLanguage("en"),
			i18n.WithTranslations("en", "general", map[string]any{
				"hello":   "Hello",
				"welcome": "Welcome, %{name}!",
				"goodbye": "Goodbye, %{name}! See you %{when}.",
				"errors": map[string]any{
					"not_found": "Resource not found",
					"validation": map[string]any{
						"required": "Field %{field} is required",
						"email":    "Invalid email format",
					},
				},
			}),
			i18n.WithTranslations("pl", "general", map[string]any{
				"hello":   "Cześć",
				"welcome": "Witaj, %{name}!",
				"errors": map[string]any{
					"not_found": "Zasób nie znaleziony",
				},
			}),
		)
		return i18nInstance
	}

	t.Run("returns simple translation", func(t *testing.T) {
		i18nInstance := setup()
		result := i18nInstance.T("en", "general", "hello")
		assert.Equal(t, "Hello", result)
	})

	t.Run("returns translation with placeholder", func(t *testing.T) {
		i18nInstance := setup()
		result := i18nInstance.T("en", "general", "welcome", i18n.M{"name": "John"})
		assert.Equal(t, "Welcome, John!", result)
	})

	t.Run("returns translation with multiple placeholders", func(t *testing.T) {
		i18nInstance := setup()
		result := i18nInstance.T("en", "general", "goodbye", i18n.M{
			"name": "Alice",
			"when": "tomorrow",
		})
		assert.Equal(t, "Goodbye, Alice! See you tomorrow.", result)
	})

	t.Run("merges multiple placeholder maps", func(t *testing.T) {
		i18nInstance := setup()
		result := i18nInstance.T("en", "general", "goodbye",
			i18n.M{"name": "Bob"},
			i18n.M{"when": "later"},
		)
		assert.Equal(t, "Goodbye, Bob! See you later.", result)
	})

	t.Run("later placeholder maps override earlier ones", func(t *testing.T) {
		i18nInstance := setup()
		result := i18nInstance.T("en", "general", "welcome",
			i18n.M{"name": "Initial"},
			i18n.M{"name": "Override"},
		)
		assert.Equal(t, "Welcome, Override!", result)
	})

	t.Run("returns nested translation using dot notation", func(t *testing.T) {
		i18nInstance := setup()
		result := i18nInstance.T("en", "general", "errors.not_found")
		assert.Equal(t, "Resource not found", result)
	})

	t.Run("returns deeply nested translation", func(t *testing.T) {
		i18nInstance := setup()
		result := i18nInstance.T("en", "general", "errors.validation.email")
		assert.Equal(t, "Invalid email format", result)
	})

	t.Run("returns nested translation with placeholder", func(t *testing.T) {
		i18nInstance := setup()
		result := i18nInstance.T("en", "general", "errors.validation.required",
			i18n.M{"field": "username"},
		)
		assert.Equal(t, "Field username is required", result)
	})

	t.Run("falls back to default language", func(t *testing.T) {
		i18nInstance := setup()
		// "goodbye" doesn't exist in Polish
		result := i18nInstance.T("pl", "general", "goodbye", i18n.M{
			"name": "Anna",
			"when": "jutro",
		})
		assert.Equal(t, "Goodbye, Anna! See you jutro.", result)
	})

	t.Run("returns key when translation not found", func(t *testing.T) {
		i18nInstance := setup()
		result := i18nInstance.T("en", "general", "non.existent.key")
		assert.Equal(t, "non.existent.key", result)
	})

	t.Run("returns key when namespace not found", func(t *testing.T) {
		i18nInstance := setup()
		result := i18nInstance.T("en", "nonexistent", "hello")
		assert.Equal(t, "hello", result)
	})

	t.Run("leaves unmatched placeholders unchanged", func(t *testing.T) {
		i18nInstance := setup()
		result := i18nInstance.T("en", "general", "welcome", i18n.M{"other": "value"})
		assert.Equal(t, "Welcome, %{name}!", result)
	})

	t.Run("handles empty placeholder maps", func(t *testing.T) {
		i18nInstance := setup()
		result := i18nInstance.T("en", "general", "welcome")
		assert.Equal(t, "Welcome, %{name}!", result)
	})

	t.Run("handles nil placeholder maps", func(t *testing.T) {
		i18nInstance := setup()
		result := i18nInstance.T("en", "general", "welcome", nil)
		assert.Equal(t, "Welcome, %{name}!", result)
	})
}

func TestTn(t *testing.T) {
	setup := func() *i18n.I18n {
		i18nInstance, _ := i18n.New(
			i18n.WithDefaultLanguage("en"),
			i18n.WithPluralRule("en", i18n.EnglishPluralRule),
			i18n.WithPluralRule("pl", i18n.SlavicPluralRule),
			i18n.WithTranslations("en", "general", map[string]any{
				"items": map[string]any{
					"zero":  "No items",
					"one":   "%{count} item",
					"other": "%{count} items",
				},
				"messages": map[string]any{
					"one":   "You have %{count} new message",
					"other": "You have %{count} new messages",
				},
			}),
			i18n.WithTranslations("pl", "general", map[string]any{
				"items": map[string]any{
					"zero": "Brak elementów",
					"one":  "%{count} element",
					"few":  "%{count} elementy",
					"many": "%{count} elementów",
				},
			}),
		)
		return i18nInstance
	}

	t.Run("selects correct plural form for English", func(t *testing.T) {
		i18nInstance := setup()

		assert.Equal(t, "No items", i18nInstance.Tn("en", "general", "items", 0))
		assert.Equal(t, "1 item", i18nInstance.Tn("en", "general", "items", 1))
		assert.Equal(t, "2 items", i18nInstance.Tn("en", "general", "items", 2))
		assert.Equal(t, "5 items", i18nInstance.Tn("en", "general", "items", 5))
		assert.Equal(t, "100 items", i18nInstance.Tn("en", "general", "items", 100))
	})

	t.Run("selects correct plural form for Polish", func(t *testing.T) {
		i18nInstance := setup()

		assert.Equal(t, "Brak elementów", i18nInstance.Tn("pl", "general", "items", 0))
		assert.Equal(t, "1 element", i18nInstance.Tn("pl", "general", "items", 1))
		assert.Equal(t, "2 elementy", i18nInstance.Tn("pl", "general", "items", 2))
		assert.Equal(t, "3 elementy", i18nInstance.Tn("pl", "general", "items", 3))
		assert.Equal(t, "4 elementy", i18nInstance.Tn("pl", "general", "items", 4))
		assert.Equal(t, "5 elementów", i18nInstance.Tn("pl", "general", "items", 5))
		assert.Equal(t, "12 elementów", i18nInstance.Tn("pl", "general", "items", 12))
		assert.Equal(t, "22 elementy", i18nInstance.Tn("pl", "general", "items", 22))
		assert.Equal(t, "100 elementów", i18nInstance.Tn("pl", "general", "items", 100))
	})

	t.Run("falls back to other form when specific form not found", func(t *testing.T) {
		i18nInstance := setup()
		// "messages" doesn't have "zero" form
		assert.Equal(t, "You have 0 new messages", i18nInstance.Tn("en", "general", "messages", 0))
	})

	t.Run("injects count placeholder automatically", func(t *testing.T) {
		i18nInstance := setup()
		result := i18nInstance.Tn("en", "general", "items", 5)
		assert.Equal(t, "5 items", result)
	})

	t.Run("merges additional placeholders with count", func(t *testing.T) {
		i18nInstance, _ := i18n.New(
			i18n.WithTranslations("en", "general", map[string]any{
				"files": map[string]any{
					"one":   "%{count} file in %{folder}",
					"other": "%{count} files in %{folder}",
				},
			}),
		)

		result := i18nInstance.Tn("en", "general", "files", 3, i18n.M{"folder": "Documents"})
		assert.Equal(t, "3 files in Documents", result)
	})

	t.Run("additional placeholders can override count", func(t *testing.T) {
		i18nInstance := setup()
		result := i18nInstance.Tn("en", "general", "items", 5, i18n.M{"count": "many"})
		assert.Equal(t, "many items", result)
	})

	t.Run("falls back to default language", func(t *testing.T) {
		i18nInstance := setup()
		// "messages" doesn't exist in Polish
		result := i18nInstance.Tn("pl", "general", "messages", 3)
		assert.Equal(t, "You have 3 new messages", result)
	})

	t.Run("returns key when translation not found", func(t *testing.T) {
		i18nInstance := setup()
		result := i18nInstance.Tn("en", "general", "nonexistent", 5)
		assert.Equal(t, "nonexistent", result)
	})

	t.Run("uses auto-assigned plural rule based on language code", func(t *testing.T) {
		i18nInstance, _ := i18n.New(
			i18n.WithTranslations("fr", "general", map[string]any{
				"items": map[string]any{
					"one":   "%{count} élément",
					"many":  "%{count} éléments (beaucoup)",
					"other": "%{count} éléments",
				},
			}),
		)

		// French uses RomancePluralRule: one (0,1), many (1M+), other
		assert.Equal(t, "0 élément", i18nInstance.Tn("fr", "general", "items", 0))
		assert.Equal(t, "1 élément", i18nInstance.Tn("fr", "general", "items", 1))
		assert.Equal(t, "3 éléments", i18nInstance.Tn("fr", "general", "items", 3))
		assert.Equal(t, "10 éléments", i18nInstance.Tn("fr", "general", "items", 10))
		assert.Equal(t, "100 éléments", i18nInstance.Tn("fr", "general", "items", 100))
		assert.Equal(t, "1000000 éléments (beaucoup)", i18nInstance.Tn("fr", "general", "items", 1000000))
	})

	t.Run("handles negative numbers", func(t *testing.T) {
		i18nInstance := setup()
		assert.Equal(t, "-1 item", i18nInstance.Tn("en", "general", "items", -1))
		assert.Equal(t, "-5 items", i18nInstance.Tn("en", "general", "items", -5))
	})
}

func TestFlattenTranslations(t *testing.T) {
	t.Run("flattens nested structures correctly", func(t *testing.T) {
		i18nInstance, err := i18n.New(
			i18n.WithTranslations("en", "test", map[string]any{
				"simple": "Simple value",
				"nested": map[string]any{
					"level1": "Level 1",
					"deeper": map[string]any{
						"level2": "Level 2",
						"evenDeeper": map[string]any{
							"level3": "Level 3",
						},
					},
				},
				"plural": map[string]string{
					"one":   "One item",
					"other": "Many items",
				},
				"number":  42,
				"boolean": true,
			}),
		)
		require.NoError(t, err)

		// Test flattened keys work
		assert.Equal(t, "Simple value", i18nInstance.T("en", "test", "simple"))
		assert.Equal(t, "Level 1", i18nInstance.T("en", "test", "nested.level1"))
		assert.Equal(t, "Level 2", i18nInstance.T("en", "test", "nested.deeper.level2"))
		assert.Equal(t, "Level 3", i18nInstance.T("en", "test", "nested.deeper.evenDeeper.level3"))
		assert.Equal(t, "One item", i18nInstance.T("en", "test", "plural.one"))
		assert.Equal(t, "Many items", i18nInstance.T("en", "test", "plural.other"))
		assert.Equal(t, "42", i18nInstance.T("en", "test", "number"))
		assert.Equal(t, "true", i18nInstance.T("en", "test", "boolean"))
	})
}

func TestConcurrency(t *testing.T) {
	t.Run("concurrent reads are safe", func(t *testing.T) {
		i18nInstance, err := i18n.New(
			i18n.WithDefaultLanguage("en"),
			i18n.WithTranslations("en", "general", map[string]any{
				"hello": "Hello",
				"world": "World",
				"items": map[string]any{
					"one":   "%{count} item",
					"other": "%{count} items",
				},
			}),
		)
		require.NoError(t, err)

		// Run multiple goroutines accessing the same instance
		done := make(chan bool, 100)
		for i := 0; i < 100; i++ {
			go func(n int) {
				defer func() { done <- true }()

				// Mix different types of operations
				switch n % 3 {
				case 0:
					result := i18nInstance.T("en", "general", "hello")
					assert.Equal(t, "Hello", result)
				case 1:
					result := i18nInstance.T("en", "general", "world")
					assert.Equal(t, "World", result)
				case 2:
					result := i18nInstance.Tn("en", "general", "items", n)
					if n == 1 {
						assert.Equal(t, "1 item", result)
					} else {
						assert.Contains(t, result, "items")
					}
				}
			}(i)
		}

		// Wait for all goroutines to complete
		for i := 0; i < 100; i++ {
			<-done
		}
	})
}

func TestAutoPluraRuleAssignment(t *testing.T) {
	t.Run("automatically assigns appropriate plural rule based on language", func(t *testing.T) {
		i18nInstance, err := i18n.New(
			// Don't explicitly set plural rules
			i18n.WithTranslations("en", "general", map[string]any{
				"items": map[string]any{
					"zero":  "No items",
					"one":   "One item",
					"other": "%{count} items",
				},
			}),
			i18n.WithTranslations("pl", "general", map[string]any{
				"items": map[string]any{
					"zero": "Brak",
					"one":  "Jeden",
					"few":  "Kilka %{count}",
					"many": "Wiele %{count}",
				},
			}),
			i18n.WithTranslations("ar", "general", map[string]any{
				"items": map[string]any{
					"zero":  "صفر",
					"one":   "واحد",
					"two":   "اثنان",
					"few":   "قليل %{count}",
					"many":  "كثير %{count}",
					"other": "آخر %{count}",
				},
			}),
		)
		require.NoError(t, err)

		// Test English (should use EnglishPluralRule)
		assert.Equal(t, "No items", i18nInstance.Tn("en", "general", "items", 0))
		assert.Equal(t, "One item", i18nInstance.Tn("en", "general", "items", 1))
		assert.Equal(t, "5 items", i18nInstance.Tn("en", "general", "items", 5))

		// Test Polish (should use SlavicPluralRule)
		assert.Equal(t, "Kilka 3", i18nInstance.Tn("pl", "general", "items", 3))
		assert.Equal(t, "Wiele 5", i18nInstance.Tn("pl", "general", "items", 5))

		// Test Arabic (should use ArabicPluralRule)
		assert.Equal(t, "واحد", i18nInstance.Tn("ar", "general", "items", 1))
		assert.Equal(t, "اثنان", i18nInstance.Tn("ar", "general", "items", 2))
		assert.Equal(t, "قليل 3", i18nInstance.Tn("ar", "general", "items", 3))
		assert.Equal(t, "كثير 11", i18nInstance.Tn("ar", "general", "items", 11))
	})
}
