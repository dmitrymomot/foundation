package cookie

import (
	"errors"
	"fmt"
)

// Error variables define specific failure scenarios in cookie management,
// providing clear, actionable error information for robust error handling.
var (
	// ErrNoSecret indicates no secret was provided for cookie encryption/signing.
	ErrNoSecret = errors.New("no secret provided for cookie manager")

	// ErrSecretTooShort indicates the secret doesn't meet minimum length requirements.
	// Secrets must be at least 32 characters for AES-256 encryption.
	ErrSecretTooShort = errors.New("secret must be at least 32 characters long")

	// ErrInvalidSignature indicates cookie signature verification failed,
	// suggesting tampering or corruption.
	ErrInvalidSignature = errors.New("cookie signature verification failed")

	// ErrDecryptionFailed indicates the cookie value couldn't be decrypted,
	// possibly due to corruption or use of wrong key.
	ErrDecryptionFailed = errors.New("failed to decrypt cookie value")

	// ErrCookieNotFound indicates the requested cookie doesn't exist in the request.
	ErrCookieNotFound = errors.New("cookie not found in request")

	// ErrInvalidFormat indicates the cookie value has unexpected format,
	// typically during decoding operations.
	ErrInvalidFormat = errors.New("invalid cookie format")

	// ErrConsentRequired indicates the operation requires user consent for non-essential cookies.
	ErrConsentRequired = errors.New("user consent required for non-essential cookies")
)

// ErrCookieTooLarge indicates the cookie exceeds the maximum allowed size.
type ErrCookieTooLarge struct {
	Name string
	Size int
	Max  int
}

// Error implements the error interface.
func (e ErrCookieTooLarge) Error() string {
	return fmt.Sprintf("cookie %q size %d exceeds maximum %d bytes", e.Name, e.Size, e.Max)
}
