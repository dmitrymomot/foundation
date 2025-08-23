# Architecture Decision Record: I18n Package

## Status

Active

## Implementation Progress

- [x] Type definitions (`M` type for placeholder maps)
- [x] Helper functions (Accept-Language parsing, placeholder replacement)
- [x] Plural rule implementations (CLDR-compliant)
- [x] Comprehensive tests for helpers and plural rules
- [ ] Core I18n struct and constructor
- [ ] Translation methods (T, Tn)
- [ ] Integration tests

## Context

We need a simple, efficient internationalization (i18n) package for Go applications that supports:

- Multiple languages and namespaces
- Translation with named placeholders
- Pluralization with language-specific rules
- Immutable runtime behavior for thread safety
- Minimal dependencies (standard library only)

## Decision

### Core Design Principles

1. **Immutable by Construction**: All translations loaded via constructor options, making the instance immutable and thread-safe from creation.
2. **Namespace-Based Organization**: Translations are organized by language and namespace to allow logical grouping and prevent key collisions.
3. **Simple API**: Minimal method surface with clear semantics - `T()` for translations, `Tn()` for plurals.
4. **No External Dependencies**: Use only Go standard library for maximum portability.

### Architecture

#### Data Structure

```go
type I18n struct {
    translations map[string]map[string]map[string]any  // lang -> namespace -> key -> value
    pluralRules  map[string]PluralRule                 // lang -> rule function
    defaultLang  string                                // fallback language
}
```

#### Type Definitions

```go
// M is a convenience type for placeholder maps
type M map[string]any

// PluralRule determines which plural form to use for a given count
type PluralRule func(n int) string

// Option function type
type Option func(*I18n) error
```

#### Core API

```go
// Constructor with options - returns error if configuration is invalid
func New(opts ...Option) (*I18n, error)

// Options for configuration
func WithDefaultLanguage(lang string) Option
func WithPluralRule(lang string, rule PluralRule) Option
func WithTranslations(lang, namespace string, translations map[string]any) Option

// Runtime methods (available immediately after construction)
func (i *I18n) T(lang, namespace, key string, placeholders ...M) string
func (i *I18n) Tn(lang, namespace, key string, n int, placeholders ...M) string
```

### Translation Format

#### Basic Translations

```go
map[string]any{
    "hello": "Hello",
    "welcome": "Welcome, %{name}!",
}
```

#### Nested Translations

```go
map[string]any{
    "errors": map[string]any{
        "not_found": "Resource not found",
        "validation": map[string]any{
            "required": "Field %{field} is required",
        },
    },
}
```

Accessed via dot notation: `"errors.validation.required"`

#### Plural Translations

```go
map[string]any{
    "items": map[string]any{
        "zero":  "No items",
        "one":   "%{count} item",
        "few":   "%{count} items",
        "many":  "%{count} items",
        "other": "%{count} items",
    },
}
```

### Pluralization Rules

#### Plural Forms

The package supports five plural forms:

- `zero`: Used for 0 items
- `one`: Used for single item
- `few`: Used for small quantities (2-4 in default rule)
- `many`: Used for larger quantities
- `other`: Default/fallback form

#### Default Plural Rule

```go
func defaultPluralRule(n int) string {
    if n == 0 { return "zero" }
    if n == 1 { return "one" }
    if n >= 2 && n <= 4 { return "few" }
    if n > 4 && n < 20 { return "many" }
    return "other"
}
```

#### Language-Specific Rules

```go
// English - simpler rules
var EnglishPlural = func(n int) string {
    if n == 0 { return "zero" }
    if n == 1 { return "one" }
    return "other"
}

// Slavic languages (Polish, Russian, Czech)
var SlavicPlural = func(n int) string {
    if n == 0 { return "zero" }
    if n == 1 { return "one" }
    if n%10 >= 2 && n%10 <= 4 && (n%100 < 12 || n%100 > 14) {
        return "few"
    }
    return "many"
}
```

### Placeholder System

- Format: `%{placeholder_name}`
- Supports any value that can be formatted with `fmt.Sprintf`
- Automatic `count` placeholder injection for `Tn()` method

Example:

```go
// Template: "Welcome, %{name}! You have %{count} messages."
i18n.T("en", "general", "welcome", i18n.M{
    "name": "John",
    "count": 5,
})
// Result: "Welcome, John! You have 5 messages."
```

### Fallback Strategy

1. **Translation Fallback**:
    - Try requested language and key
    - Try default language and key
    - Return key as last resort

2. **Plural Form Fallback**:
    - Try exact plural form
    - `few` → `many` → `other`
    - `zero`, `one`, `many` → `other`
    - Return first available form

3. **Missing Placeholder**:
    - Leave placeholder unchanged in output

### Thread Safety

- **Fully thread-safe from creation**: All data is immutable after constructor returns
- **No locks needed**: Read-only operations throughout lifetime
- **No setup phase**: Constructor handles all initialization atomically

### Usage Example

```go
// All configuration happens in constructor
i18n, err := i18n.New(
    i18n.WithDefaultLanguage("en"),
    i18n.WithPluralRule("en", i18n.EnglishPlural),
    i18n.WithPluralRule("pl", i18n.SlavicPlural),
    i18n.WithTranslations("en", "general", map[string]any{
        "hello":   "Hello",
        "welcome": "Welcome, %{name}!",
        "items": map[string]any{
            "zero":  "No items",
            "one":   "%{count} item",
            "other": "%{count} items",
        },
    }),
    i18n.WithTranslations("pl", "general", map[string]any{
        "hello":   "Cześć",
        "welcome": "Witaj, %{name}!",
        "items": map[string]any{
            "zero":  "Brak elementów",
            "one":   "%{count} element",
            "few":   "%{count} elementy",
            "many":  "%{count} elementów",
        },
    }),
)
if err != nil {
    log.Fatal(err)
}

// Ready to use immediately
greeting := i18n.T("en", "general", "hello")
// "Hello"

welcome := i18n.T("pl", "general", "welcome", i18n.M{"name": "Anna"})
// "Witaj, Anna!"

items := i18n.Tn("pl", "general", "items", 3)
// "3 elementy"
```

## Consequences

### Positive

- **Thread-safe by design**: Immutable from creation, no synchronization ever needed
- **Simpler API**: No separate loading phase, just constructor and runtime methods
- **Fail-fast**: Configuration errors caught at startup
- **Predictable behavior**: All translations known at startup
- **Fast runtime**: No locks, no state checks, no dynamic loading
- **Type-safe**: Compile-time type checking for placeholders
- **Flexible organization**: Namespace support for large applications
- **Language-specific pluralization**: Proper support for different language rules

### Negative

- **No dynamic loading**: Cannot add translations at runtime
- **Memory usage**: All translations loaded into memory
- **No lazy loading**: Must load all translations upfront
- **Manual plural rules**: Must implement plural rules for each language

### Trade-offs

- **Simplicity over features**: No advanced features like gender, context, or number formatting
- **Memory over complexity**: Keep all in memory rather than implement caching
- **Explicit over magic**: Require explicit namespace and key parameters

## References

- [Unicode CLDR Plural Rules](https://cldr.unicode.org/index/cldr-spec/plural-rules)
- [GNU gettext plural forms](https://www.gnu.org/software/gettext/manual/html_node/Plural-forms.html)
- [Go i18n packages survey](https://github.com/avelino/awesome-go#natural-language-processing)
