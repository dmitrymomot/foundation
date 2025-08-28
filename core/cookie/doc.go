// Package cookie provides secure HTTP cookie management with encryption, signing,
// and GDPR consent support for web applications.
//
// Features include AES-256-GCM encryption, HMAC-SHA256 signing, automatic key rotation,
// GDPR consent management, flash messages, and 4KB size limit enforcement with secure defaults.
//
// Basic usage:
//
//	import "github.com/dmitrymomot/foundation/core/cookie"
//
//	// Create manager with secret key (minimum 32 characters)
//	manager, err := cookie.New([]string{"your-32-char-secret-key-here!!!!"})
//	if err != nil {
//		log.Fatal(err)
//	}
//
//	// Set consent for non-essential cookies
//	err = manager.StoreConsent(w, r, cookie.ConsentAll)
//
//	// Set a simple cookie
//	err = manager.Set(w, r, "user_id", "12345", cookie.WithMaxAge(3600))
//
//	// Get cookie value
//	value, err := manager.Get(r, "user_id")
//	if err == cookie.ErrCookieNotFound {
//		// Cookie doesn't exist
//	}
//
//	// Delete cookie
//	manager.Delete(w, "user_id")
//
// # Signed and Encrypted Cookies
//
// Use signed cookies to detect tampering:
//
//	// Set signed cookie
//	err := manager.SetSigned(w, r, "session_id", sessionID,
//		cookie.WithHTTPOnly(true),
//		cookie.WithSecure(true),
//	)
//
//	// Verify signed cookie
//	sessionID, err := manager.GetSigned(r, "session_id")
//	if err == cookie.ErrInvalidSignature {
//		// Cookie was tampered with
//	}
//
// Use encrypted cookies for sensitive data:
//
//	// Set encrypted cookie
//	err := manager.SetEncrypted(w, r, "api_token", secretToken,
//		cookie.WithHTTPOnly(true),
//		cookie.WithSecure(true),
//	)
//
//	// Get encrypted cookie
//	token, err := manager.GetEncrypted(r, "api_token")
//	if err == cookie.ErrDecryptionFailed {
//		// Decryption failed
//	}
//
// # Flash Messages
//
// Flash messages are one-time cookies automatically deleted after reading:
//
//	// Set flash message (JSON-serializable)
//	err := manager.SetFlash(w, r, "success", map[string]string{
//		"message": "Profile updated",
//		"type":    "success",
//	})
//
//	// Get and auto-delete flash message
//	var flash map[string]string
//	err := manager.GetFlash(w, r, "success", &flash)
//	if err == nil {
//		fmt.Printf("Flash: %s\n", flash["message"])
//	}
//
// # GDPR Consent Management
//
// Two-tier consent system for essential vs non-essential cookies:
//
//	// Check consent status
//	consent, err := manager.GetConsent(r)
//	if consent.Status == cookie.ConsentUnknown {
//		// Show consent banner
//	}
//
//	// Store consent decision
//	err := manager.StoreConsent(w, r, cookie.ConsentAll)
//
//	// Essential cookies always allowed
//	err := manager.Set(w, r, "session_id", sessionID, cookie.WithEssential())
//
//	// Non-essential cookies require consent
//	err := manager.Set(w, r, "analytics_id", analyticsID)
//	if err == cookie.ErrConsentRequired {
//		// User hasn't consented to non-essential cookies
//	}
//
// # Key Rotation
//
// Multiple secrets enable graceful key rotation:
//
//	secrets := []string{
//		"new-secret-key-32-characters!!!",  // Used for new cookies
//		"old-secret-key-32-characters!!!",  // Used for reading old cookies
//	}
//	manager, err := cookie.New(secrets)
//
// # Environment Configuration
//
// Configure via environment variables:
//
//	cfg := cookie.Config{
//		Secrets:  "secret1,secret2,secret3",
//		Secure:   true,
//		HttpOnly: true,
//		SameSite: http.SameSiteStrictMode,
//	}
//	manager, err := cookie.NewFromConfig(cfg)
//
// # Error Handling
//
// The package defines specific error types for different failure scenarios:
//
//	err := manager.Set(w, r, "large", strings.Repeat("x", 5000))
//	if e, ok := err.(cookie.ErrCookieTooLarge); ok {
//		log.Printf("Cookie %s exceeds limit: %d > %d bytes", e.Name, e.Size, e.Max)
//	}
package cookie
