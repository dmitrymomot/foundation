// Package token provides compact, URL-safe token generation and verification using HMAC signatures.
//
// This package combines JSON payloads with truncated HMAC-SHA256 signatures to create secure,
// lightweight tokens suitable for authentication, session management, and data integrity
// verification. Tokens are designed to be compact enough for URLs while maintaining security.
//
// # Features
//
// - Compact token format (payload.signature)
// - JSON payload support with Go generics
// - HMAC-SHA256 signatures truncated to 8 bytes for size optimization
// - Base64 URL-safe encoding (no padding)
// - Constant-time signature verification
// - No external dependencies beyond standard library
//
// # Token Format
//
// Tokens follow the format: `<base64url-payload>.<base64url-signature>`
//
// Where:
//   - Payload: JSON-encoded data, base64url-encoded
//   - Signature: First 8 bytes of HMAC-SHA256, base64url-encoded
//   - Total overhead: ~15-20 characters plus payload size
//
// # Usage
//
// Basic token generation and parsing:
//
//	type UserClaims struct {
//		UserID   int    `json:"user_id"`
//		Username string `json:"username"`
//		Expires  int64  `json:"exp"`
//	}
//
//	secret := "your-secret-key"
//
//	// Generate token
//	claims := UserClaims{
//		UserID:   123,
//		Username: "john.doe",
//		Expires:  time.Now().Add(time.Hour).Unix(),
//	}
//
//	token, err := token.GenerateToken(claims, secret)
//	if err != nil {
//		log.Fatal(err)
//	}
//	fmt.Printf("Token: %s\n", token)
//
//	// Parse token
//	var parsedClaims UserClaims
//	parsedClaims, err = token.ParseToken[UserClaims](token, secret)
//	if err != nil {
//		log.Printf("Token parsing failed: %v", err)
//		return
//	}
//
//	fmt.Printf("User ID: %d, Username: %s\n", parsedClaims.UserID, parsedClaims.Username)
//
// # HTTP Integration
//
// Authentication middleware:
//
//	func AuthMiddleware(secret string) func(http.Handler) http.Handler {
//		return func(next http.Handler) http.Handler {
//			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
//				authHeader := r.Header.Get("Authorization")
//				if authHeader == "" {
//					http.Error(w, "Missing authorization", http.StatusUnauthorized)
//					return
//				}
//
//				// Extract Bearer token
//				tokenString := strings.TrimPrefix(authHeader, "Bearer ")
//				if tokenString == authHeader {
//					http.Error(w, "Invalid authorization format", http.StatusUnauthorized)
//					return
//				}
//
//				// Parse and validate token
//				claims, err := token.ParseToken[UserClaims](tokenString, secret)
//				if err != nil {
//					switch err {
//					case token.ErrInvalidToken:
//						http.Error(w, "Malformed token", http.StatusBadRequest)
//					case token.ErrSignatureInvalid:
//						http.Error(w, "Invalid token signature", http.StatusUnauthorized)
//					default:
//						http.Error(w, "Token processing error", http.StatusInternalServerError)
//					}
//					return
//				}
//
//				// Check expiration
//				if time.Now().Unix() > claims.Expires {
//					http.Error(w, "Token expired", http.StatusUnauthorized)
//					return
//				}
//
//				// Add claims to request context
//				ctx := context.WithValue(r.Context(), "user_claims", claims)
//				next.ServeHTTP(w, r.WithContext(ctx))
//			})
//		}
//	}
//
// Login endpoint example:
//
//	func loginHandler(w http.ResponseWriter, r *http.Request) {
//		var loginReq struct {
//			Username string `json:"username"`
//			Password string `json:"password"`
//		}
//
//		if err := json.NewDecoder(r.Body).Decode(&loginReq); err != nil {
//			http.Error(w, "Invalid JSON", http.StatusBadRequest)
//			return
//		}
//
//		// Authenticate user (your logic here)
//		user, err := authenticateUser(loginReq.Username, loginReq.Password)
//		if err != nil {
//			http.Error(w, "Authentication failed", http.StatusUnauthorized)
//			return
//		}
//
//		// Generate token
//		claims := UserClaims{
//			UserID:   user.ID,
//			Username: user.Username,
//			Expires:  time.Now().Add(24 * time.Hour).Unix(),
//		}
//
//		tokenString, err := token.GenerateToken(claims, secret)
//		if err != nil {
//			http.Error(w, "Token generation failed", http.StatusInternalServerError)
//			return
//		}
//
//		// Return token to client
//		response := map[string]any{
//			"token":    tokenString,
//			"user_id":  user.ID,
//			"username": user.Username,
//			"expires":  claims.Expires,
//		}
//		json.NewEncoder(w).Encode(response)
//	}
//
// # URL Parameter Tokens
//
// For password reset, email verification, etc.:
//
//	type ResetClaims struct {
//		UserID  int   `json:"user_id"`
//		Purpose string `json:"purpose"`
//		Expires int64 `json:"exp"`
//	}
//
//	func generateResetToken(userID int) (string, error) {
//		claims := ResetClaims{
//			UserID:  userID,
//			Purpose: "password_reset",
//			Expires: time.Now().Add(1 * time.Hour).Unix(), // Short expiry
//		}
//
//		return token.GenerateToken(claims, resetTokenSecret)
//	}
//
//	func handlePasswordReset(w http.ResponseWriter, r *http.Request) {
//		tokenParam := r.URL.Query().Get("token")
//		if tokenParam == "" {
//			http.Error(w, "Missing token", http.StatusBadRequest)
//			return
//		}
//
//		claims, err := token.ParseToken[ResetClaims](tokenParam, resetTokenSecret)
//		if err != nil {
//			http.Error(w, "Invalid or expired token", http.StatusBadRequest)
//			return
//		}
//
//		if claims.Purpose != "password_reset" {
//			http.Error(w, "Invalid token purpose", http.StatusBadRequest)
//			return
//		}
//
//		if time.Now().Unix() > claims.Expires {
//			http.Error(w, "Token expired", http.StatusGone)
//			return
//		}
//
//		// Process password reset for user claims.UserID
//		// ...
//	}
//
// # Security Considerations
//
// Signature truncation:
//   - 8-byte (64-bit) signatures provide ~34 bits of security against forgery
//   - Suitable for short-lived tokens (minutes to hours)
//   - Not recommended for long-term authentication tokens
//   - Consider full HMAC for high-security applications
//
// Secret management:
//   - Use cryptographically random secrets (at least 32 bytes)
//   - Rotate secrets periodically
//   - Store securely (environment variables, key management systems)
//   - Use different secrets for different token types
//
// Token expiration:
//   - Always include expiration times in claims
//   - Use short expiration periods for access tokens
//   - Implement refresh token mechanism for long sessions
//   - Clear expired tokens from client storage
//
// # Performance Characteristics
//
// Token operations:
//   - Generation: ~10-50 µs (dominated by JSON marshaling)
//   - Parsing: ~20-60 µs (includes signature verification)
//   - Memory usage: ~1-2 KB per operation
//   - Token size: ~15-50 characters overhead plus JSON payload
//
// Comparison with alternatives:
//   - JWT: Larger tokens (~100+ char overhead), standardized
//   - Simple signatures: Similar performance, custom format
//   - Session cookies: Server storage required, larger
//
// # Error Handling
//
// The package defines specific error types:
//   - ErrInvalidToken: Malformed token format
//   - ErrSignatureInvalid: Signature verification failed (tampering detected)
//
// JSON unmarshaling errors are returned as-is for application handling.
//
// # Use Cases
//
// Session tokens (short-lived):
//
//	type SessionToken struct {
//		UserID    int    `json:"user_id"`
//		SessionID string `json:"session_id"`
//		Expires   int64  `json:"exp"`
//	}
//
//	// Generate with 15-minute expiry
//	session := SessionToken{
//		UserID:    user.ID,
//		SessionID: generateSessionID(),
//		Expires:   time.Now().Add(15 * time.Minute).Unix(),
//	}
//
// API keys with metadata:
//
//	type APIKeyToken struct {
//		ClientID    string   `json:"client_id"`
//		Permissions []string `json:"permissions"`
//		RateLimit   int      `json:"rate_limit"`
//		Expires     int64    `json:"exp"`
//	}
//
// Email verification:
//
//	type EmailToken struct {
//		Email   string `json:"email"`
//		Action  string `json:"action"`
//		Expires int64  `json:"exp"`
//	}
//
// # Best Practices
//
// Token design:
//   - Keep payloads small for URL compatibility
//   - Include expiration times
//   - Add purpose/type fields for validation
//   - Use specific secrets for different token types
//
// Security:
//   - Validate all claims after parsing
//   - Check expiration times
//   - Use HTTPS for token transmission
//   - Store tokens securely on client side
//
// Error handling:
//   - Distinguish between malformed and expired tokens
//   - Log security-relevant events (signature failures)
//   - Provide meaningful error messages to clients
//   - Implement proper token refresh flows
//
// # Integration Examples
//
// Microservices authentication:
//
//	type ServiceClaims struct {
//		ServiceName string   `json:"service"`
//		Permissions []string `json:"perms"`
//		Expires     int64    `json:"exp"`
//	}
//
//	func authenticateService(tokenString string) (*ServiceClaims, error) {
//		claims, err := token.ParseToken[ServiceClaims](tokenString, serviceSecret)
//		if err != nil {
//			return nil, fmt.Errorf("authentication failed: %w", err)
//		}
//
//		if time.Now().Unix() > claims.Expires {
//			return nil, errors.New("token expired")
//		}
//
//		return &claims, nil
//	}
//
// Webhook signatures:
//
//	type WebhookToken struct {
//		WebhookID string `json:"webhook_id"`
//		Timestamp int64  `json:"timestamp"`
//	}
//
//	func signWebhook(webhookID string) (string, error) {
//		claims := WebhookToken{
//			WebhookID: webhookID,
//			Timestamp: time.Now().Unix(),
//		}
//		return token.GenerateToken(claims, webhookSecret)
//	}
package token
