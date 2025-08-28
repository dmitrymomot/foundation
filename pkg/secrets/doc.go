// Package secrets provides AES-256-GCM encryption with compound key derivation for secure data storage.
//
// This package combines application and workspace keys using HKDF (HMAC-based Key Derivation Function)
// to create encryption keys, supporting both string and byte-level encryption operations with automatic
// memory cleanup and cryptographic best practices.
//
// # Security Model
//
// The package implements a compound key system where:
//   - Application key: Global secret shared across the application
//   - Workspace key: Tenant/workspace-specific secret
//   - Derived key: HKDF-derived encryption key combining both inputs
//
// This design provides tenant isolation while maintaining operational simplicity.
//
// # Usage
//
// Generate secure encryption keys:
//
//	import "github.com/dmitrymomot/foundation/pkg/secrets"
//
//	// Generate 32-byte keys for encryption
//	appKey, err := secrets.GenerateKey()
//	if err != nil {
//		log.Fatal(err)
//	}
//
//	workspaceKey, err := secrets.GenerateKey()
//	if err != nil {
//		log.Fatal(err)
//	}
//
// Encrypt and decrypt strings (returns base64-encoded ciphertext):
//
//	plaintext := "sensitive user data"
//
//	// Encrypt to base64-encoded string
//	ciphertext, err := secrets.EncryptString(appKey, workspaceKey, plaintext)
//	if err != nil {
//		log.Fatal(err)
//	}
//
//	// Decrypt back to original string
//	decrypted, err := secrets.DecryptString(appKey, workspaceKey, ciphertext)
//	if err != nil {
//		log.Fatal(err)
//	}
//
//	fmt.Printf("Original: %s\n", plaintext)
//	fmt.Printf("Decrypted: %s\n", decrypted)
//
// Encrypt and decrypt raw bytes (for binary data):
//
//	data := []byte("Hello, World!")
//
//	encrypted, err := secrets.EncryptBytes(appKey, workspaceKey, data)
//	if err != nil {
//		log.Fatal(err)
//	}
//
//	decrypted, err := secrets.DecryptBytes(appKey, workspaceKey, encrypted)
//	if err != nil {
//		log.Fatal(err)
//	}
//
//	fmt.Printf("Match: %t\n", bytes.Equal(data, decrypted))
//
// # Error Handling
//
// The package defines specific error types for different failure scenarios:
//
//	_, err := secrets.EncryptString(shortKey, workspaceKey, "data")
//	if errors.Is(err, secrets.ErrInvalidAppKey) {
//		// Handle invalid app key (not 32 bytes)
//	}
//
//	_, err = secrets.DecryptString(appKey, workspaceKey, "invalid-base64!")
//	if errors.Is(err, secrets.ErrInvalidCiphertext) {
//		// Handle corrupted or malformed ciphertext
//	}
//
// Available error types:
//   - ErrInvalidAppKey: App key is not 32 bytes
//   - ErrInvalidWorkspaceKey: Workspace key is not 32 bytes
//   - ErrKeyDerivationFailed: HKDF key derivation failed
//   - ErrEncryptionFailed: AES-GCM encryption failed
//   - ErrDecryptionFailed: AES-GCM decryption failed (includes tampering detection)
//   - ErrInvalidCiphertext: Ciphertext format invalid or corrupted
//
// # Security Properties
//
// Encryption algorithm:
//   - AES-256-GCM authenticated encryption
//   - Provides both confidentiality and authenticity
//   - Each encryption uses a unique random nonce
//   - Tampering with ciphertext will be detected during decryption
//
// Key derivation:
//   - HKDF-SHA256 combines app and workspace keys
//   - Derived keys are automatically cleared from memory after use
//   - Constant-time key validation prevents timing attacks
//
// Key requirements:
//   - Both keys must be exactly 32 bytes (256 bits)
//   - Generate using secrets.GenerateKey() or equivalent cryptographically secure source
//   - Store keys securely and never log or expose them
//
// Different workspace keys provide tenant isolation - data encrypted with one workspace
// key cannot be decrypted with another workspace key, even with the same app key.
package secrets
