// Package token provides compact, URL-safe token generation and verification using HMAC signatures.
//
// This package creates signed tokens by combining JSON payloads with truncated HMAC-SHA256
// signatures. The tokens are compact enough for URLs while providing data integrity verification.
//
// # Token Format
//
// Tokens follow the format: `<base64url-payload>.<base64url-signature>`
//
// Where:
//   - Payload: JSON-encoded data, base64url-encoded (no padding)
//   - Signature: First 8 bytes of HMAC-SHA256, base64url-encoded
//   - Total overhead: ~15-20 characters plus payload size
//
// # Basic Usage
//
//	import "github.com/dmitrymomot/foundation/pkg/token"
//
//	type Claims struct {
//		UserID  int   `json:"user_id"`
//		Expires int64 `json:"exp"`
//	}
//
//	secret := "your-secret-key"
//
//	// Generate token
//	claims := Claims{UserID: 123, Expires: 1640995200}
//	tokenStr, err := token.GenerateToken(claims, secret)
//	if err != nil {
//		// Handle error
//	}
//
//	// Parse token
//	parsed, err := token.ParseToken[Claims](tokenStr, secret)
//	if err != nil {
//		// Handle parsing errors (ErrInvalidToken, ErrSignatureInvalid)
//	}
//
// # Error Handling
//
// The package defines two specific error types:
//   - ErrInvalidToken: Token format is malformed or contains invalid base64
//   - ErrSignatureInvalid: Signature verification failed (token was tampered with)
//
// JSON unmarshaling errors are returned directly from json.Unmarshal.
//
//	parsed, err := token.ParseToken[Claims](tokenStr, secret)
//	switch err {
//	case token.ErrInvalidToken:
//		// Token is malformed
//	case token.ErrSignatureInvalid:
//		// Token was tampered with or wrong secret
//	case nil:
//		// Success
//	default:
//		// JSON unmarshaling error or other issue
//	}
//
// # Security Notes
//
// Signature truncation provides ~34 bits of security against forgery, suitable for
// short-lived tokens (minutes to hours). For high-security applications requiring
// longer token lifetimes, consider using full HMAC signatures.
//
// Always use cryptographically secure secrets and include expiration times in your claims.
package token
