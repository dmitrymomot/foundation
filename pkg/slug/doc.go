// Package slug generates URL-safe slugs from arbitrary strings with Unicode normalization.
//
// This package converts text to web-friendly identifiers by normalizing diacritics, replacing
// special characters with separators, and offering configurable length limits and collision-resistant
// suffixes. It's designed for creating clean URLs, file names, and identifiers from user input.
//
// # Features
//
// - Unicode diacritic normalization (é → e, ñ → n, etc.)
// - Configurable separator characters (default: "-")
// - Optional length limits with intelligent truncation
// - Random suffix generation for collision avoidance
// - Custom character replacements and stripping
// - Case control (uppercase/lowercase)
// - Rune-aware length counting (proper Unicode support)
//
// # Usage
//
// Basic slug generation:
//
//	slug := slug.Make("Hello, World!")
//	// Output: "hello-world"
//
//	slug2 := slug.Make("Café & Restaurant")
//	// Output: "cafe-restaurant"
//
//	slug3 := slug.Make("Straße in München")
//	// Output: "strasse-in-munchen"
//
// With options:
//
//	slug := slug.Make("Long article title here",
//		slug.MaxLength(20),
//		slug.WithSuffix(6),
//	)
//	// Output: "long-article-a7b2x9"
//
// Custom separator and case:
//
//	slug := slug.Make("Product Name",
//		slug.Separator("_"),
//		slug.Lowercase(false),
//	)
//	// Output: "Product_Name"
//
// # Configuration Options
//
// MaxLength: Limit slug length (rune-based counting):
//
//	slug := slug.Make("Very long title that exceeds limits",
//		slug.MaxLength(15),
//	)
//	// Output: "very-long-title"
//
// Separator: Custom separators for different contexts:
//
//	// For file names
//	filename := slug.Make("Document Title", slug.Separator("_"))
//	// Output: "document_title"
//
//	// For CSS classes
//	cssClass := slug.Make("Component Name", slug.Separator("-"))
//	// Output: "component-name"
//
// WithSuffix: Add random suffix to avoid collisions:
//
//	// For database slugs that must be unique
//	slug := slug.Make("Article Title", slug.WithSuffix(8))
//	// Output: "article-title-a3f7k2m9"
//
//	// With length limit
//	slug2 := slug.Make("Long Article Title",
//		slug.MaxLength(20),
//		slug.WithSuffix(6),
//	)
//	// Output: "long-ar-k7x2f9"
//
// Custom replacements: Handle special cases:
//
//	replacements := map[string]string{
//		"&":  "and",
//		"@":  "at",
//		"#":  "hash",
//		"C++": "cpp",
//	}
//
//	slug := slug.Make("C++ & Go @ GitHub",
//		slug.CustomReplace(replacements),
//	)
//	// Output: "cpp-and-go-at-github"
//
// Strip unwanted characters:
//
//	slug := slug.Make("Price: $100.00",
//		slug.StripChars("$:"),
//	)
//	// Output: "price-100-00"
//
// # Unicode Support
//
// The package handles various Unicode characters and normalizes common diacritics:
//
//	examples := []string{
//		"Café français",           // "cafe-francais"
//		"Москва",                 // "moskva" (Cyrillic not supported, becomes separator)
//		"北京",                    // "" (Chinese not supported)
//		"München straße",         // "munchen-strasse"
//		"naïve résumé",          // "naive-resume"
//		"Zürich über Bäckerei",  // "zurich-uber-backerei"
//	}
//
// Supported diacritic families:
//   - À, Á, Â, Ã, Ä, Å → A (and lowercase variants)
//   - È, É, Ê, Ë → E
//   - Ì, Í, Î, Ï → I
//   - Ò, Ó, Ô, Õ, Ö, Ø → O
//   - Ù, Ú, Û, Ü → U
//   - Ñ, Ń, Ň → N
//   - Ç, Ć, Č → C
//   - And many more European language variants
//
// # Performance Characteristics
//
// - Slug generation: ~5-50 µs depending on input length and options
// - Memory allocation: ~2-3x input string length for builder
// - Diacritic normalization: O(1) map lookup per character
// - Length limits use rune counting (proper Unicode)
//
// # URL and File System Usage
//
// Blog post URLs:
//
//	func createBlogPostURL(title string) string {
//		slug := slug.Make(title,
//			slug.MaxLength(60),        // SEO-friendly length
//			slug.WithSuffix(4),        // Avoid collisions
//		)
//		return fmt.Sprintf("/blog/%s", slug)
//	}
//
// File naming:
//
//	func createFileName(title string, ext string) string {
//		name := slug.Make(title,
//			slug.Separator("_"),       // Underscores for file names
//			slug.MaxLength(50),        // File system limits
//			slug.StripChars("."),      // Remove dots before extension
//		)
//		return name + ext
//	}
//
// Database slugs with uniqueness:
//
//	type Article struct {
//		ID    int    `json:"id"`
//		Title string `json:"title"`
//		Slug  string `json:"slug"`
//	}
//
//	func generateUniqueSlug(title string, db *sql.DB) (string, error) {
//		baseSlug := slug.Make(title, slug.MaxLength(50))
//
//		// Check if slug already exists
//		if !slugExists(db, baseSlug) {
//			return baseSlug, nil
//		}
//
//		// Generate with suffix if collision
//		for i := 0; i < 5; i++ {
//			candidate := slug.Make(title,
//				slug.MaxLength(50),
//				slug.WithSuffix(6),
//			)
//			if !slugExists(db, candidate) {
//				return candidate, nil
//			}
//		}
//
//		return "", errors.New("failed to generate unique slug")
//	}
//
// # API Integration
//
// REST API endpoint slugs:
//
//	func handleCreateResource(w http.ResponseWriter, r *http.Request) {
//		var req struct {
//			Name string `json:"name"`
//		}
//
//		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
//			http.Error(w, "Invalid JSON", http.StatusBadRequest)
//			return
//		}
//
//		resourceSlug := slug.Make(req.Name,
//			slug.MaxLength(30),
//			slug.WithSuffix(8),
//		)
//
//		// Create resource with slug as identifier
//		resource := createResource(req.Name, resourceSlug)
//		json.NewEncoder(w).Encode(resource)
//	}
//
// # Best Practices
//
// Length management:
//   - Web URLs: 60-80 characters maximum
//   - File names: 50-100 characters (system dependent)
//   - Database identifiers: 30-50 characters
//   - CSS classes: 20-30 characters
//
// Collision avoidance:
//   - Use suffixes for user-generated content
//   - Check database uniqueness before finalizing
//   - Consider timestamp-based suffixes for time-sensitive content
//
// Character handling:
//   - Use custom replacements for domain-specific terms
//   - Strip currency symbols and mathematical operators
//   - Consider locale-specific normalization needs
//
// # Integration Examples
//
// Content Management System:
//
//	type CMSPage struct {
//		Title string `json:"title"`
//		Slug  string `json:"slug"`
//	}
//
//	func (p *CMSPage) GenerateSlug() {
//		p.Slug = slug.Make(p.Title,
//			slug.MaxLength(60),
//			slug.WithSuffix(6),
//			slug.CustomReplace(map[string]string{
//				"&": "and",
//				"%": "percent",
//			}),
//		)
//	}
//
// E-commerce product URLs:
//
//	func generateProductURL(name, sku string) string {
//		slugName := slug.Make(name, slug.MaxLength(40))
//		slugSKU := slug.Make(sku, slug.Separator("-"))
//		return fmt.Sprintf("/products/%s-%s", slugName, slugSKU)
//	}
//
// # Error Handling
//
// The Make function never returns errors and always produces valid output:
//   - Empty input returns empty string
//   - All-special-character input returns empty string or suffix-only
//   - Invalid options use sensible defaults
//   - Crypto/rand failures fall back to deterministic suffix
//
// For applications requiring validation:
//
//	func validateSlug(input string) (string, error) {
//		result := slug.Make(input, slug.MaxLength(50))
//		if result == "" {
//			return "", errors.New("input produces empty slug")
//		}
//		if len(result) < 3 {
//			return "", errors.New("slug too short")
//		}
//		return result, nil
//	}
package slug
