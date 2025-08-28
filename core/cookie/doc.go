// Package cookie provides secure HTTP cookie management with encryption, signing,
// and GDPR consent support. It offers a comprehensive solution for handling cookies
// in web applications with strong security defaults and flexible configuration.
//
// # Features
//
//   - AES-256-GCM encryption for sensitive data
//   - HMAC-SHA256 signing for tamper detection
//   - Automatic key rotation support
//   - GDPR consent management with two-tier approach
//   - Flash messages (one-time read cookies)
//   - 4KB size limit enforcement
//   - Secure defaults (HttpOnly, SameSite protection)
//   - Environment-based configuration
//   - Thread-safe operations
//
// # Basic Usage
//
// Create a manager with secret keys and use it to manage cookies:
//
//	import "github.com/dmitrymomot/foundation/core/cookie"
//
//	// Create manager with secret key(s)
//	manager, err := cookie.New([]string{"your-32-char-secret-key-here!!!!"})
//	if err != nil {
//		log.Fatal(err)
//	}
//
//	// Set a simple cookie (requires request for consent checking)
//	err = manager.Set(w, r, "user_id", "12345", cookie.WithMaxAge(3600))
//
//	// Get a cookie value
//	value, err := manager.Get(r, "user_id")
//	if err == cookie.ErrCookieNotFound {
//		// Cookie doesn't exist
//	}
//
//	// Delete a cookie
//	manager.Delete(w, "user_id")
//
// # Signed Cookies
//
// Use signed cookies to detect tampering:
//
//	// Set a signed cookie
//	err := manager.SetSigned(w, r, "session_id", sessionID,
//		cookie.WithHTTPOnly(true),
//		cookie.WithSecure(true),
//	)
//
//	// Get and verify signed cookie
//	sessionID, err := manager.GetSigned(r, "session_id")
//	if err == cookie.ErrInvalidSignature {
//		// Cookie was tampered with
//	}
//
// # Encrypted Cookies
//
// Store sensitive data with AES-256-GCM encryption:
//
//	// Set encrypted cookie
//	err := manager.SetEncrypted(w, r, "api_token", secretToken,
//		cookie.WithHTTPOnly(true),
//		cookie.WithSecure(true),
//		cookie.WithSameSite(http.SameSiteStrictMode),
//	)
//
//	// Get and decrypt cookie
//	token, err := manager.GetEncrypted(r, "api_token")
//	if err == cookie.ErrDecryptionFailed {
//		// Decryption failed (wrong key or corrupted)
//	}
//
// # Flash Messages
//
// Flash messages are automatically deleted after being read:
//
//	// Set flash message (any JSON-serializable value)
//	err := manager.SetFlash(w, r, "success", map[string]string{
//		"message": "Profile updated successfully",
//		"type": "success",
//	})
//
//	// In the next request, retrieve and auto-delete
//	var flash map[string]string
//	err := manager.GetFlash(w, r, "success", &flash)
//	if err == nil {
//		fmt.Printf("Flash: %s\n", flash["message"])
//	}
//
// # GDPR Consent Management
//
// Implement two-tier consent (essential vs non-essential cookies):
//
//	// Check current consent status
//	consent, _ := manager.GetConsent(r)
//	if consent.Status == cookie.ConsentUnknown {
//		// Show consent banner
//	}
//
//	// Store user's consent decision
//	err := manager.StoreConsent(w, r, cookie.ConsentAll) // or ConsentEssentialOnly
//
//	// Set essential cookie (always allowed)
//	err := manager.Set(w, r, "session_id", sessionID,
//		cookie.WithEssential(), // Bypasses consent check
//	)
//
//	// Set non-essential cookie (automatically checks consent)
//	err := manager.Set(w, r, "analytics_id", analyticsID)
//	// Returns ErrConsentRequired if no consent
//
// # Key Rotation
//
// Support graceful key rotation by providing multiple secrets:
//
//	// Newest key first, older keys for decryption only
//	secrets := []string{
//		"new-secret-key-32-characters!!!",  // Used for encryption
//		"old-secret-key-32-characters!!!",  // Used for decryption only
//		"older-secret-key-32-chars!!!!!!",  // Used for decryption only
//	}
//
//	manager, _ := cookie.New(secrets)
//
//	// New cookies use the first secret
//	// Existing cookies can be decrypted with any secret
//
// # Configuration
//
// Use environment variables for production configuration:
//
//	cfg := cookie.Config{
//		Secrets:  os.Getenv("COOKIE_SECRETS"), // Comma-separated
//		Secure:   true,
//		HttpOnly: true,
//		SameSite: http.SameSiteStrictMode,
//		MaxSize:  4096,
//	}
//
//	manager, err := cookie.NewFromConfig(cfg)
//
// # Size Limits
//
// The manager enforces the 4KB cookie size limit:
//
//	largeData := strings.Repeat("x", 5000)
//	err := manager.Set(w, r, "large", largeData)
//	if e, ok := err.(cookie.ErrCookieTooLarge); ok {
//		fmt.Printf("Cookie %s exceeds limit: %d > %d\n",
//			e.Name, e.Size, e.Max)
//	}
//
// # Integration with Application Context
//
// Integrate with your application's context pattern:
//
//	type AppContext struct {
//		handler.Context
//		cookies *cookie.Manager
//	}
//
//	func (c *AppContext) SetCookie(name string, value any) error {
//		data, _ := json.Marshal(value)
//		return c.cookies.SetEncrypted(c.ResponseWriter(), c.Request(), name, string(data))
//	}
//
//	func (c *AppContext) GetCookie(name string, dest any) error {
//		data, err := c.cookies.GetEncrypted(c.Request(), name)
//		if err != nil {
//			return err
//		}
//		return json.Unmarshal([]byte(data), dest)
//	}
//
// # Security Considerations
//
//   - Always use HTTPS in production (WithSecure option)
//   - Set HttpOnly for cookies not needed by JavaScript
//   - Use SameSite attribute for CSRF protection
//   - Rotate secrets periodically
//   - Encrypt sensitive data (tokens, personal info)
//   - Sign cookies that must not be modified
//   - Keep secrets in secure storage (environment variables, secret managers)
//
// # Best Practices
//
//   - Use meaningful cookie names
//   - Set appropriate MaxAge or session cookies
//   - Clean up cookies when no longer needed
//   - Monitor cookie size to stay under 4KB limit
//   - Use flash messages for one-time notifications
//   - Implement proper GDPR consent flow
//   - Test with multiple secrets for key rotation
//   - Use WithEssential() for critical functionality cookies
package cookie
