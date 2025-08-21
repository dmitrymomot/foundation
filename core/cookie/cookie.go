package cookie

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"slices"
	"strings"
	"time"
)

const (
	// MaxCookieSize is the maximum size for a cookie (4KB).
	MaxCookieSize = 4096
	// minSecretLength is the minimum secret length for AES-256.
	minSecretLength = 32
	// flashPrefix namespaces flash cookies to prevent conflicts.
	flashPrefix = "__flash_"
	// defaultConsentCookie is the default name for consent storage.
	defaultConsentCookie = "__cookie_consent"
	// defaultConsentVersion is the default consent version.
	defaultConsentVersion = "1.0"
	// defaultConsentMaxAge is the default consent duration (1 year).
	defaultConsentMaxAge = 365 * 24 * 60 * 60
)

// ConsentStatus represents the user's cookie consent state.
type ConsentStatus int

const (
	// ConsentUnknown indicates no consent decision has been made.
	ConsentUnknown ConsentStatus = iota
	// ConsentEssentialOnly allows only essential cookies.
	ConsentEssentialOnly
	// ConsentAll allows all cookies including analytics and marketing.
	ConsentAll
)

// ConsentData stores user's consent preferences with metadata.
type ConsentData struct {
	Status    ConsentStatus `json:"status"`
	Timestamp time.Time     `json:"timestamp"`
	Version   string        `json:"version"`
}

// Manager handles HTTP cookie operations with encryption, signing, and consent support.
type Manager struct {
	secrets  []string
	defaults Options
	maxSize  int
	// Consent management fields
	consentCookie  string
	consentVersion string
	consentMaxAge  int
}

// ManagerOption configures the Manager itself (not individual cookies).
// Use ManagerOption to configure manager-level settings like max cookie size,
// consent cookie name, and consent duration.
// These options affect the behavior of the entire cookie manager.
//
// Example:
//
//	manager, err := NewWithOptions(secrets, cookieOpts,
//	  WithMaxSize(8192),
//	  WithConsentCookie("gdpr_consent"),
//	  WithConsentMaxAge(365*24*60*60))
type ManagerOption func(*Manager)

// WithMaxSize sets the maximum cookie size.
func WithMaxSize(size int) ManagerOption {
	return func(m *Manager) {
		if size > 0 {
			m.maxSize = size
		}
	}
}

// WithConsentCookie sets the name of the consent cookie.
func WithConsentCookie(name string) ManagerOption {
	return func(m *Manager) {
		if name != "" {
			m.consentCookie = name
		}
	}
}

// WithConsentVersion sets the consent version for invalidation.
func WithConsentVersion(version string) ManagerOption {
	return func(m *Manager) {
		if version != "" {
			m.consentVersion = version
		}
	}
}

// WithConsentMaxAge sets how long consent is valid in seconds.
func WithConsentMaxAge(seconds int) ManagerOption {
	return func(m *Manager) {
		if seconds > 0 {
			m.consentMaxAge = seconds
		}
	}
}

// New creates a new cookie manager with the specified secrets and options.
func New(secrets []string, opts ...Option) (*Manager, error) {
	if len(secrets) == 0 {
		return nil, ErrNoSecret
	}

	// Remove empty secrets
	secrets = slices.DeleteFunc(secrets, func(s string) bool { return s == "" })
	if len(secrets) == 0 {
		return nil, ErrNoSecret
	}

	// Validate secret lengths
	for i := range len(secrets) {
		if len(secrets[i]) < minSecretLength {
			return nil, fmt.Errorf("%w: secret %d has %d chars, need at least %d",
				ErrSecretTooShort, i, len(secrets[i]), minSecretLength)
		}
	}

	// Secure defaults
	defaults := Options{
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	}

	defaults = applyOptions(defaults, opts)

	m := &Manager{
		secrets:        secrets,
		defaults:       defaults,
		maxSize:        MaxCookieSize,
		consentCookie:  defaultConsentCookie,
		consentVersion: defaultConsentVersion,
		consentMaxAge:  defaultConsentMaxAge,
	}

	return m, nil
}

// NewWithOptions creates a new cookie manager with additional manager options.
func NewWithOptions(secrets []string, cookieOpts []Option, managerOpts ...ManagerOption) (*Manager, error) {
	m, err := New(secrets, cookieOpts...)
	if err != nil {
		return nil, err
	}

	for _, opt := range managerOpts {
		opt(m)
	}

	return m, nil
}

// Set stores a cookie value with consent checking.
func (m *Manager) Set(w http.ResponseWriter, r *http.Request, name, value string, opts ...Option) error {
	options := applyOptions(m.defaults, opts)

	// Check GDPR consent for non-essential cookies
	if !options.Essential && !m.hasConsent(r) {
		return ErrConsentRequired
	}

	cookie := &http.Cookie{
		Name:     name,
		Value:    value,
		Path:     options.Path,
		Domain:   options.Domain,
		MaxAge:   options.MaxAge,
		Secure:   options.Secure,
		HttpOnly: options.HttpOnly,
		SameSite: options.SameSite,
	}

	// Check size limit
	header := cookie.String()
	if len(header) > m.maxSize {
		return ErrCookieTooLarge{
			Name: name,
			Size: len(header),
			Max:  m.maxSize,
		}
	}

	http.SetCookie(w, cookie)
	return nil
}

// Get retrieves a cookie value.
func (m *Manager) Get(r *http.Request, name string) (string, error) {
	cookie, err := r.Cookie(name)
	if err != nil {
		if errors.Is(err, http.ErrNoCookie) {
			return "", ErrCookieNotFound
		}
		return "", err
	}
	return cookie.Value, nil
}

// Delete removes a cookie.
func (m *Manager) Delete(w http.ResponseWriter, name string) {
	cookie := &http.Cookie{
		Name:     name,
		Value:    "",
		Path:     m.defaults.Path,
		Domain:   m.defaults.Domain,
		MaxAge:   -1,
		Expires:  time.Unix(0, 0),
		HttpOnly: m.defaults.HttpOnly,
		SameSite: m.defaults.SameSite,
		Secure:   m.defaults.Secure,
	}
	http.SetCookie(w, cookie)
}

// SetSigned stores a signed cookie value with consent checking.
func (m *Manager) SetSigned(w http.ResponseWriter, r *http.Request, name, value string, opts ...Option) error {
	signed := m.sign(value)
	return m.Set(w, r, name, signed, opts...)
}

// GetSigned retrieves and verifies a signed cookie value.
func (m *Manager) GetSigned(r *http.Request, name string) (string, error) {
	signed, err := m.Get(r, name)
	if err != nil {
		return "", err
	}
	return m.verify(signed)
}

// SetEncrypted stores an encrypted cookie value with consent checking.
func (m *Manager) SetEncrypted(w http.ResponseWriter, r *http.Request, name, value string, opts ...Option) error {
	encrypted, err := m.encrypt(value)
	if err != nil {
		return err
	}
	return m.Set(w, r, name, encrypted, opts...)
}

// GetEncrypted retrieves and decrypts a cookie value.
func (m *Manager) GetEncrypted(r *http.Request, name string) (string, error) {
	encrypted, err := m.Get(r, name)
	if err != nil {
		return "", err
	}
	return m.decrypt(encrypted)
}

// SetFlash stores a one-time message that's deleted after reading.
func (m *Manager) SetFlash(w http.ResponseWriter, r *http.Request, key string, value any) error {
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("marshal flash: %w", err)
	}
	return m.SetEncrypted(w, r, flashPrefix+key, string(data), WithEssential())
}

// GetFlash retrieves and deletes a flash message.
func (m *Manager) GetFlash(w http.ResponseWriter, r *http.Request, key string, dest any) error {
	cookieName := flashPrefix + key

	data, err := m.GetEncrypted(r, cookieName)
	if err != nil {
		return err
	}

	// Delete immediately after reading
	m.Delete(w, cookieName)

	if err := json.Unmarshal([]byte(data), dest); err != nil {
		return fmt.Errorf("unmarshal flash: %w", err)
	}

	return nil
}

// GetConsent retrieves the current consent status from cookies.
func (m *Manager) GetConsent(r *http.Request) (ConsentData, error) {
	var consent ConsentData

	// Get encrypted consent cookie
	value, err := m.GetEncrypted(r, m.consentCookie)
	if err != nil {
		if err == ErrCookieNotFound {
			// No consent cookie means unknown status
			return ConsentData{Status: ConsentUnknown}, nil
		}
		return consent, err
	}

	// Parse consent data
	if err := json.Unmarshal([]byte(value), &consent); err != nil {
		return ConsentData{Status: ConsentUnknown}, err
	}

	// Check version compatibility
	if consent.Version != m.consentVersion {
		// Outdated consent, treat as unknown
		return ConsentData{Status: ConsentUnknown}, nil
	}

	return consent, nil
}

// StoreConsent saves the user's consent decision.
func (m *Manager) StoreConsent(w http.ResponseWriter, r *http.Request, status ConsentStatus) error {
	consent := ConsentData{
		Status:    status,
		Timestamp: time.Now(),
		Version:   m.consentVersion,
	}

	data, err := json.Marshal(consent)
	if err != nil {
		return err
	}

	// Store as essential encrypted cookie (consent storage is always essential)
	return m.setConsentCookie(w, m.consentCookie, string(data))
}

// ClearConsent removes the consent cookie.
func (m *Manager) ClearConsent(w http.ResponseWriter) {
	m.Delete(w, m.consentCookie)
}

// HasConsent checks if non-essential cookies are allowed.
func (m *Manager) HasConsent(r *http.Request) bool {
	return m.hasConsent(r)
}

// hasConsent is a private helper to check consent status.
func (m *Manager) hasConsent(r *http.Request) bool {
	consent, err := m.GetConsent(r)
	if err != nil {
		return false
	}
	return consent.Status == ConsentAll
}

// setConsentCookie sets the consent cookie without checking consent (internal use).
func (m *Manager) setConsentCookie(w http.ResponseWriter, name, value string) error {
	// Encrypt the consent data
	encrypted, err := m.encrypt(value)
	if err != nil {
		return err
	}

	cookie := &http.Cookie{
		Name:     name,
		Value:    encrypted,
		Path:     m.defaults.Path,
		Domain:   m.defaults.Domain,
		MaxAge:   m.consentMaxAge,
		Secure:   m.defaults.Secure,
		HttpOnly: true,                    // Always HttpOnly for consent
		SameSite: http.SameSiteStrictMode, // Strict for consent
	}

	// Check size limit
	header := cookie.String()
	if len(header) > m.maxSize {
		return ErrCookieTooLarge{
			Name: name,
			Size: len(header),
			Max:  m.maxSize,
		}
	}

	http.SetCookie(w, cookie)
	return nil
}

// sign creates an HMAC signature for the value.
func (m *Manager) sign(value string) string {
	mac := hmac.New(sha256.New, []byte(m.secrets[0]))
	mac.Write([]byte(value))
	signature := base64.URLEncoding.EncodeToString(mac.Sum(nil))
	return base64.URLEncoding.EncodeToString([]byte(value)) + "|" + signature
}

// verify checks the HMAC signature of a signed value.
func (m *Manager) verify(signed string) (string, error) {
	parts := strings.SplitN(signed, "|", 2)
	if len(parts) != 2 {
		return "", ErrInvalidFormat
	}

	encodedValue, signature := parts[0], parts[1]

	value, err := base64.URLEncoding.DecodeString(encodedValue)
	if err != nil {
		return "", ErrInvalidFormat
	}

	// Try all secrets for key rotation support
	validIndex := slices.IndexFunc(m.secrets, func(secret string) bool {
		mac := hmac.New(sha256.New, []byte(secret))
		mac.Write(value)
		expectedSig := base64.URLEncoding.EncodeToString(mac.Sum(nil))
		return subtle.ConstantTimeCompare([]byte(signature), []byte(expectedSig)) == 1
	})

	if validIndex >= 0 {
		return string(value), nil
	}

	return "", ErrInvalidSignature
}

// encrypt encrypts a value using AES-256-GCM.
func (m *Manager) encrypt(value string) (string, error) {
	// Validate key length for AES-256
	if len(m.secrets[0]) < 32 {
		return "", fmt.Errorf("%w: secret must be at least 32 bytes for AES-256", ErrSecretTooShort)
	}

	block, err := aes.NewCipher([]byte(m.secrets[0][:32]))
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}

	ciphertext := gcm.Seal(nonce, nonce, []byte(value), nil)
	return base64.URLEncoding.EncodeToString(ciphertext), nil
}

// decrypt decrypts a value using AES-256-GCM.
func (m *Manager) decrypt(encrypted string) (string, error) {
	ciphertext, err := base64.URLEncoding.DecodeString(encrypted)
	if err != nil {
		return "", ErrInvalidFormat
	}

	// Try all secrets for key rotation
	var lastErr error
	for _, secret := range m.secrets {
		// Validate key length for AES-256
		if len(secret) < 32 {
			lastErr = fmt.Errorf("%w: secret must be at least 32 bytes for AES-256", ErrSecretTooShort)
			continue
		}

		block, err := aes.NewCipher([]byte(secret[:32]))
		if err != nil {
			lastErr = err
			continue
		}

		gcm, err := cipher.NewGCM(block)
		if err != nil {
			lastErr = err
			continue
		}

		if len(ciphertext) < gcm.NonceSize() {
			lastErr = ErrInvalidFormat
			continue
		}

		nonce, ciphertext := ciphertext[:gcm.NonceSize()], ciphertext[gcm.NonceSize():]
		plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
		if err == nil {
			return string(plaintext), nil
		}
		lastErr = err
	}

	if lastErr != nil {
		return "", ErrDecryptionFailed
	}
	return "", ErrDecryptionFailed
}
