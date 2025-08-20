// Package totp implements RFC 6238 Time-based One-Time Password (TOTP) authentication.
//
// This package provides secure secret key generation, TOTP URI creation for authenticator apps,
// and code generation/validation with clock drift tolerance for two-factor authentication systems.
// It fully complies with RFC 6238 and RFC 4226 standards and is compatible with popular
// authenticator applications like Google Authenticator, Authy, and 1Password.
//
// # Features
//
// - RFC 6238 compliant TOTP implementation
// - RFC 4226 compliant HOTP algorithm (used internally)
// - Cryptographically secure secret generation
// - Clock drift tolerance (±30 seconds)
// - QR code-compatible URI generation
// - Base32 secret key encoding/validation
// - Configurable parameters (digits, period, algorithm)
//
// # Usage
//
// Basic TOTP setup and validation:
//
//	// Generate a new secret for a user
//	secret, err := totp.GenerateSecretKey()
//	if err != nil {
//		log.Fatal(err)
//	}
//	fmt.Printf("Secret: %s\n", secret) // Store this securely
//
//	// Create TOTP URI for QR code
//	params := totp.TOTPParams{
//		Secret:      secret,
//		AccountName: "user@example.com",
//		Issuer:      "MyApp",
//	}
//
//	uri, err := totp.GetTOTPURI(params)
//	if err != nil {
//		log.Fatal(err)
//	}
//	fmt.Printf("QR Code URI: %s\n", uri)
//
//	// Generate current TOTP code
//	currentCode, err := totp.GenerateTOTP(secret)
//	if err != nil {
//		log.Fatal(err)
//	}
//	fmt.Printf("Current code: %s\n", currentCode)
//
//	// Validate user-provided code
//	userCode := "123456" // from user input
//	isValid, err := totp.ValidateTOTP(secret, userCode)
//	if err != nil {
//		log.Fatal(err)
//	}
//
//	if isValid {
//		fmt.Println("Code is valid!")
//	} else {
//		fmt.Println("Invalid code")
//	}
//
// # QR Code Integration
//
// Generate QR codes for mobile authenticator setup:
//
//	// Generate TOTP URI
//	params := totp.TOTPParams{
//		Secret:      secret,
//		AccountName: "john.doe@company.com",
//		Issuer:      "Company Name",
//	}
//
//	uri, err := totp.GetTOTPURI(params)
//	if err != nil {
//		return err
//	}
//
//	// Generate QR code image (using qrcode package)
//	qrImage, err := qrcode.GenerateBase64Image(uri, 256)
//	if err != nil {
//		return err
//	}
//
//	// Display to user for scanning with authenticator app
//	fmt.Printf(`
//	<h3>Scan with your authenticator app:</h3>
//	<img src="%s" alt="TOTP QR Code">
//	<p>Manual entry: %s</p>
//	`, qrImage, secret)
//
// # HTTP API Integration
//
// Two-factor authentication setup endpoint:
//
//	func handleTOTPSetup(w http.ResponseWriter, r *http.Request) {
//		userID := getUserIDFromSession(r)
//
//		// Generate new secret
//		secret, err := totp.GenerateSecretKey()
//		if err != nil {
//			http.Error(w, "Failed to generate secret", http.StatusInternalServerError)
//			return
//		}
//
//		// Store secret temporarily (confirm setup before making permanent)
//		setTemporaryTOTPSecret(userID, secret)
//
//		// Generate QR code URI
//		params := totp.TOTPParams{
//			Secret:      secret,
//			AccountName: getUserEmail(userID),
//			Issuer:      "MyApplication",
//		}
//
//		uri, err := totp.GetTOTPURI(params)
//		if err != nil {
//			http.Error(w, "Failed to generate URI", http.StatusInternalServerError)
//			return
//		}
//
//		response := map[string]string{
//			"secret":     secret,
//			"qr_uri":     uri,
//			"backup_codes": generateBackupCodes(), // Optional backup codes
//		}
//
//		json.NewEncoder(w).Encode(response)
//	}
//
// TOTP verification endpoint:
//
//	func handleTOTPVerify(w http.ResponseWriter, r *http.Request) {
//		var req struct {
//			Code string `json:"code"`
//		}
//
//		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
//			http.Error(w, "Invalid JSON", http.StatusBadRequest)
//			return
//		}
//
//		userID := getUserIDFromSession(r)
//		secret := getUserTOTPSecret(userID)
//
//		if secret == "" {
//			http.Error(w, "TOTP not configured", http.StatusBadRequest)
//			return
//		}
//
//		isValid, err := totp.ValidateTOTP(secret, req.Code)
//		if err != nil {
//			http.Error(w, "Validation error", http.StatusInternalServerError)
//			return
//		}
//
//		if !isValid {
//			http.Error(w, "Invalid code", http.StatusUnauthorized)
//			return
//		}
//
//		// TOTP validation successful
//		setUserTOTPVerified(userID, true)
//		json.NewEncoder(w).Encode(map[string]bool{"verified": true})
//	}
//
// # Authentication Middleware
//
// TOTP requirement for sensitive operations:
//
//	func RequireTOTP(next http.Handler) http.Handler {
//		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
//			userID := getUserIDFromSession(r)
//
//			// Check if user has TOTP enabled
//			if !isUserTOTPEnabled(userID) {
//				http.Error(w, "2FA required", http.StatusForbidden)
//				return
//			}
//
//			// Check for TOTP code in header
//			totpCode := r.Header.Get("X-TOTP-Code")
//			if totpCode == "" {
//				http.Error(w, "TOTP code required", http.StatusUnauthorized)
//				return
//			}
//
//			// Validate TOTP code
//			secret := getUserTOTPSecret(userID)
//			isValid, err := totp.ValidateTOTP(secret, totpCode)
//			if err != nil || !isValid {
//				http.Error(w, "Invalid TOTP code", http.StatusUnauthorized)
//				return
//			}
//
//			// Prevent replay attacks (optional)
//			if hasRecentlyUsedCode(userID, totpCode) {
//				http.Error(w, "Code already used", http.StatusUnauthorized)
//				return
//			}
//			markCodeAsUsed(userID, totpCode, time.Now())
//
//			next.ServeHTTP(w, r)
//		})
//	}
//
// # Advanced Configuration
//
// Custom TOTP parameters:
//
//	params := totp.TOTPParams{
//		Secret:      secret,
//		AccountName: "user@example.com",
//		Issuer:      "MyApp",
//		Algorithm:   "SHA1",   // SHA1 (default), SHA256, SHA512
//		Digits:      6,        // 6 digits (default), can be 8
//		Period:      30,       // 30 seconds (default), can be 60
//	}
//
//	uri, err := totp.GetTOTPURI(params)
//
// Generate code for specific time (useful for testing):
//
//	// Generate code for a specific timestamp
//	testTime := time.Date(2023, 1, 1, 12, 0, 0, 0, time.UTC)
//	code, err := totp.GenerateTOTPWithTime(secret, testTime)
//	if err != nil {
//		log.Fatal(err)
//	}
//	fmt.Printf("Code for %v: %s\n", testTime, code)
//
// # Security Considerations
//
// Secret key management:
//   - Store secrets encrypted in the database
//   - Use secure random generation (crypto/rand)
//   - 160-bit secrets provide adequate security
//   - Never log or expose secrets in plain text
//
// Clock drift tolerance:
//   - Default ±30 second tolerance handles most scenarios
//   - Network latency and user input delays are accommodated
//   - Server time synchronization (NTP) is recommended
//
// Replay attack prevention:
//   - Track recently used codes per user
//   - Codes are valid for 90 seconds (current + previous + next window)
//   - Store used codes with expiration times
//
// # Algorithm Details
//
// TOTP Algorithm (RFC 6238):
//  1. Current time divided by period (30 seconds) = counter
//  2. HOTP(secret, counter) = 6-digit code
//  3. Code changes every 30 seconds
//  4. Clock drift handled by checking adjacent windows
//
// HOTP Algorithm (RFC 4226):
//  1. HMAC-SHA1(secret, counter) = 20-byte hash
//  2. Dynamic truncation extracts 4-byte value
//  3. Modulo 10^digits produces final code
//
// # Error Handling
//
// The package defines specific error types:
//   - ErrMissingSecret: Secret parameter is empty
//   - ErrInvalidSecret: Secret is not valid Base32
//   - ErrMissingAccountName: Account name parameter is empty
//   - ErrMissingIssuer: Issuer parameter is empty
//   - ErrInvalidOTP: OTP code format is invalid
//   - ErrFailedToGenerateSecretKey: Random generation failed
//   - ErrFailedToGenerateTOTP: Code generation failed
//   - ErrFailedToValidateTOTP: Code validation failed
//
// # Performance Characteristics
//
// - Secret generation: ~1-10 µs (cryptographic random)
// - Code generation: ~10-50 µs (HMAC-SHA1 + formatting)
// - Code validation: ~30-150 µs (validates 3 time windows)
// - URI generation: ~5-20 µs (string formatting + URL encoding)
//
// # Testing Support
//
// The package provides utilities for deterministic testing:
//
//	// Generate code for specific time
//	testTime := time.Unix(1609459200, 0) // 2021-01-01 00:00:00 UTC
//	code, err := totp.GenerateTOTPWithTime(secret, testTime)
//
//	// Test validation with known codes
//	knownSecret := "JBSWY3DPEHPK3PXP" // Base32 "Hello world!"
//	knownCode := "282760" // Code at Unix timestamp 59
//	testTime = time.Unix(59, 0)
//	code, _ = totp.GenerateTOTPWithTime(knownSecret, testTime)
//	assert.Equal(t, knownCode, code)
//
// # Compatibility
//
// Fully compatible with:
//   - Google Authenticator
//   - Authy
//   - 1Password
//   - Bitwarden
//   - Microsoft Authenticator
//   - Any RFC 6238 compliant TOTP implementation
//
// QR code format follows the Key URI Format specification:
// https://github.com/google/google-authenticator/wiki/Key-Uri-Format
package totp
