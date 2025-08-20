// Package sanitizer provides comprehensive input sanitization and data cleaning
// utilities for web applications. It offers protection against XSS attacks,
// SQL injection, and other security vulnerabilities through various sanitization
// functions for strings, HTML, URLs, and structured data.
//
// # Features
//
//   - XSS protection through HTML sanitization
//   - SQL injection prevention for database queries
//   - URL and email validation and cleaning
//   - String normalization and whitespace handling
//   - Collection sanitization for arrays and maps
//   - Numeric value cleaning and validation
//   - Regular expression-based pattern matching
//   - Tag stripping and HTML entity handling
//   - Security-focused input validation
//
// # String Sanitization
//
// Clean and normalize string inputs:
//
//	import "github.com/dmitrymomot/gokit/core/sanitizer"
//
//	// Basic string cleaning
//	clean := sanitizer.String(userInput)
//
//	// Remove excessive whitespace
//	normalized := sanitizer.NormalizeWhitespace("  multiple   spaces  ")
//
//	// Trim and clean
//	result := sanitizer.TrimAndClean(input)
//
//	// Remove control characters
//	safe := sanitizer.RemoveControlChars(input)
//
// # HTML Sanitization
//
// Protect against XSS attacks:
//
//	// Strip all HTML tags
//	plainText := sanitizer.StripHTMLTags("<script>alert('xss')</script>Hello")
//	// Result: "Hello"
//
//	// Allow safe HTML tags
//	safeHTML := sanitizer.SanitizeHTML(`<p>Safe content</p><script>dangerous()</script>`)
//	// Result: "<p>Safe content</p>"
//
//	// Clean user-generated content
//	content := sanitizer.CleanUserContent(userGeneratedHTML)
//
// # URL and Email Sanitization
//
// Validate and clean URLs and email addresses:
//
//	// URL validation and cleaning
//	if sanitizer.IsValidURL(urlString) {
//		cleanURL := sanitizer.SanitizeURL(urlString)
//		// Use cleanURL safely
//	}
//
//	// Email validation
//	if sanitizer.IsValidEmail(emailString) {
//		cleanEmail := sanitizer.CleanEmail(emailString)
//		// Use cleanEmail safely
//	}
//
//	// Domain validation
//	if sanitizer.IsValidDomain(domain) {
//		// Process valid domain
//	}
//
// # SQL Injection Prevention
//
// Clean inputs for database operations:
//
//	// Basic SQL input sanitization
//	safeInput := sanitizer.SQLSafe(userInput)
//
//	// Clean table/column names
//	safeTableName := sanitizer.CleanIdentifier(tableName)
//	safeColumnName := sanitizer.CleanIdentifier(columnName)
//
//	// Note: Always use parameterized queries as primary defense
//	query := "SELECT * FROM " + sanitizer.CleanIdentifier(tableName) + " WHERE id = ?"
//	rows, err := db.Query(query, sanitizer.SQLSafe(userID))
//
// # Numeric Sanitization
//
// Clean and validate numeric inputs:
//
//	// Integer cleaning
//	cleanInt := sanitizer.CleanInteger(intString)
//
//	// Float cleaning
//	cleanFloat := sanitizer.CleanFloat(floatString)
//
//	// Currency cleaning
//	amount := sanitizer.CleanCurrency("$1,234.56")
//	// Result: "1234.56"
//
//	// Phone number cleaning
//	phone := sanitizer.CleanPhoneNumber("+1 (555) 123-4567")
//	// Result: "15551234567"
//
// # Collection Sanitization
//
// Clean arrays, slices, and maps:
//
//	// Sanitize string slice
//	cleanStrings := sanitizer.SanitizeStringSlice([]string{
//		"  normal string  ",
//		"<script>alert('xss')</script>",
//		"another string",
//	})
//
//	// Sanitize map values
//	data := map[string]string{
//		"name":  "  John Doe  ",
//		"email": "john@example.com",
//		"bio":   "<p>Developer</p><script>alert('xss')</script>",
//	}
//	cleanData := sanitizer.SanitizeStringMap(data)
//
//	// Clean struct fields
//	type User struct {
//		Name  string
//		Email string
//		Bio   string
//	}
//	user := User{
//		Name:  "  John  ",
//		Email: "JOHN@EXAMPLE.COM",
//		Bio:   "<b>Developer</b><script>evil()</script>",
//	}
//	cleanUser := sanitizer.SanitizeStruct(user)
//
// # Form Data Sanitization
//
// Clean form inputs comprehensively:
//
//	func sanitizeFormData(formData map[string]string) map[string]string {
//		cleaned := make(map[string]string)
//
//		for key, value := range formData {
//			switch key {
//			case "email":
//				cleaned[key] = sanitizer.CleanEmail(value)
//			case "phone":
//				cleaned[key] = sanitizer.CleanPhoneNumber(value)
//			case "url", "website":
//				cleaned[key] = sanitizer.SanitizeURL(value)
//			case "bio", "description":
//				cleaned[key] = sanitizer.SanitizeHTML(value)
//			default:
//				cleaned[key] = sanitizer.String(value)
//			}
//		}
//
//		return cleaned
//	}
//
// # File Path Sanitization
//
// Clean file paths and names:
//
//	// Clean filename
//	safeFilename := sanitizer.CleanFilename(userFilename)
//
//	// Prevent directory traversal
//	safePath := sanitizer.SanitizePath(userPath)
//
//	// Validate file extension
//	if sanitizer.IsAllowedExtension(filename, []string{".jpg", ".png", ".gif"}) {
//		// Process allowed file
//	}
//
// # Regular Expression Sanitization
//
// Use pattern-based cleaning:
//
//	// Remove patterns
//	cleaned := sanitizer.RemovePattern(input, `[^\w\s]`) // Keep only word chars and spaces
//
//	// Replace patterns
//	normalized := sanitizer.ReplacePattern(input, `\s+`, " ") // Replace multiple spaces
//
//	// Extract valid parts
//	valid := sanitizer.ExtractPattern(input, `[a-zA-Z0-9]+`) // Extract alphanumeric
//
// # Security-Focused Sanitization
//
// Apply security-specific cleaning:
//
//	type UserInput struct {
//		Username string `json:"username"`
//		Password string `json:"password"`
//		Email    string `json:"email"`
//		Profile  struct {
//			Bio     string `json:"bio"`
//			Website string `json:"website"`
//		} `json:"profile"`
//	}
//
//	func secureUserInput(input UserInput) UserInput {
//		return UserInput{
//			Username: sanitizer.CleanUsername(input.Username),
//			Password: input.Password, // Don't sanitize passwords
//			Email:    sanitizer.CleanEmail(input.Email),
//			Profile: struct {
//				Bio     string `json:"bio"`
//				Website string `json:"website"`
//			}{
//				Bio:     sanitizer.SanitizeHTML(input.Profile.Bio),
//				Website: sanitizer.SanitizeURL(input.Profile.Website),
//			},
//		}
//	}
//
// # Middleware Integration
//
// Use sanitization in HTTP middleware:
//
//	func sanitizationMiddleware(next http.HandlerFunc) http.HandlerFunc {
//		return func(w http.ResponseWriter, r *http.Request) {
//			// Sanitize query parameters
//			query := r.URL.Query()
//			for key, values := range query {
//				for i, value := range values {
//					query[key][i] = sanitizer.String(value)
//				}
//			}
//			r.URL.RawQuery = query.Encode()
//
//			// Sanitize form data
//			if r.Method == "POST" || r.Method == "PUT" {
//				r.ParseForm()
//				for key, values := range r.PostForm {
//					for i, value := range values {
//						r.PostForm[key][i] = sanitizer.String(value)
//					}
//				}
//			}
//
//			next(w, r)
//		}
//	}
//
// # Custom Sanitization Rules
//
// Create application-specific sanitizers:
//
//	func sanitizeProductData(product Product) Product {
//		return Product{
//			Name:        sanitizer.String(product.Name),
//			Description: sanitizer.SanitizeHTML(product.Description),
//			Price:       sanitizer.CleanCurrency(product.Price),
//			SKU:         sanitizer.CleanIdentifier(product.SKU),
//			Tags:        sanitizer.SanitizeStringSlice(product.Tags),
//			Images:      sanitizeImageURLs(product.Images),
//		}
//	}
//
//	func sanitizeImageURLs(urls []string) []string {
//		var clean []string
//		for _, url := range urls {
//			if sanitizer.IsValidURL(url) {
//				clean = append(clean, sanitizer.SanitizeURL(url))
//			}
//		}
//		return clean
//	}
//
// # Best Practices
//
//   - Sanitize all user inputs at application boundaries
//   - Use appropriate sanitization based on data context
//   - Combine sanitization with validation for robust security
//   - Apply HTML sanitization for user-generated content
//   - Use parameterized queries in addition to input sanitization
//   - Validate file uploads and sanitize filenames
//   - Log suspicious input patterns for security monitoring
//   - Test sanitization functions with malicious inputs
//   - Keep sanitization rules updated with new attack patterns
//   - Document which sanitization methods are applied to each field
package sanitizer
