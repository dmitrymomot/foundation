package cookie_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dmitrymomot/foundation/core/cookie"
)

const testSecret = "test-secret-key-32-characters!!!"
const testSecret2 = "another-secret-key-32-chars!!!!!"

func TestManager_BasicOperations(t *testing.T) {
	t.Run("set and get cookie with consent", func(t *testing.T) {
		m, err := cookie.New([]string{testSecret})
		require.NoError(t, err)

		w := httptest.NewRecorder()
		r := httptest.NewRequest("GET", "/", nil)

		// Store consent first
		err = m.StoreConsent(w, r, cookie.ConsentAll)
		assert.NoError(t, err)

		// Update request with consent cookie
		r = &http.Request{Header: http.Header{}}
		r.Header.Set("Cookie", w.Header().Get("Set-Cookie"))

		// Now set a regular cookie
		w = httptest.NewRecorder()
		err = m.Set(w, r, "test", "value123")
		assert.NoError(t, err)

		// Get the cookie
		req := &http.Request{Header: http.Header{}}
		req.Header.Set("Cookie", w.Header().Get("Set-Cookie"))

		value, err := m.Get(req, "test")
		assert.NoError(t, err)
		assert.Equal(t, "value123", value)
	})

	t.Run("set essential cookie without consent", func(t *testing.T) {
		m, err := cookie.New([]string{testSecret})
		require.NoError(t, err)

		w := httptest.NewRecorder()
		r := httptest.NewRequest("GET", "/", nil)

		// Essential cookies should work without consent
		err = m.Set(w, r, "session", "abc123", cookie.WithEssential())
		assert.NoError(t, err)

		req := &http.Request{Header: http.Header{}}
		req.Header.Set("Cookie", w.Header().Get("Set-Cookie"))

		value, err := m.Get(req, "session")
		assert.NoError(t, err)
		assert.Equal(t, "abc123", value)
	})

	t.Run("set non-essential cookie without consent fails", func(t *testing.T) {
		m, err := cookie.New([]string{testSecret})
		require.NoError(t, err)

		w := httptest.NewRecorder()
		r := httptest.NewRequest("GET", "/", nil)

		// Non-essential cookies should fail without consent
		err = m.Set(w, r, "analytics", "ga123")
		assert.ErrorIs(t, err, cookie.ErrConsentRequired)
	})

	t.Run("cookie not found", func(t *testing.T) {
		m, err := cookie.New([]string{testSecret})
		require.NoError(t, err)

		req := httptest.NewRequest("GET", "/", nil)
		_, err = m.Get(req, "nonexistent")
		assert.ErrorIs(t, err, cookie.ErrCookieNotFound)
	})

	t.Run("delete cookie", func(t *testing.T) {
		m, err := cookie.New([]string{testSecret})
		require.NoError(t, err)

		w := httptest.NewRecorder()
		m.Delete(w, "test")

		cookies := w.Result().Cookies()
		require.Len(t, cookies, 1)
		assert.Equal(t, "test", cookies[0].Name)
		assert.Equal(t, "", cookies[0].Value)
		assert.Equal(t, -1, cookies[0].MaxAge)
	})
}

func TestManager_SignedCookies(t *testing.T) {
	t.Run("set and get signed cookie", func(t *testing.T) {
		m, err := cookie.New([]string{testSecret})
		require.NoError(t, err)

		w := httptest.NewRecorder()
		r := httptest.NewRequest("GET", "/", nil)

		// Store consent
		err = m.StoreConsent(w, r, cookie.ConsentAll)
		assert.NoError(t, err)
		r.Header.Set("Cookie", w.Header().Get("Set-Cookie"))

		// Set signed cookie
		w = httptest.NewRecorder()
		err = m.SetSigned(w, r, "signed", "secret-value")
		assert.NoError(t, err)

		req := &http.Request{Header: http.Header{}}
		req.Header.Set("Cookie", w.Header().Get("Set-Cookie"))

		value, err := m.GetSigned(req, "signed")
		assert.NoError(t, err)
		assert.Equal(t, "secret-value", value)
	})

	t.Run("detect tampering", func(t *testing.T) {
		m, err := cookie.New([]string{testSecret})
		require.NoError(t, err)

		w := httptest.NewRecorder()
		r := httptest.NewRequest("GET", "/", nil)

		// Set signed cookie as essential (no consent needed)
		err = m.SetSigned(w, r, "signed", "secret-value", cookie.WithEssential())
		assert.NoError(t, err)

		// Get the actual signed value
		req := &http.Request{Header: http.Header{}}
		req.Header.Set("Cookie", w.Header().Get("Set-Cookie"))
		signedValue, err := m.Get(req, "signed")
		require.NoError(t, err)

		// Tamper with the signed value
		parts := strings.Split(signedValue, "|")
		require.Len(t, parts, 2)
		tampered := parts[0] + "|" + "tampered-signature"

		// Create new request with tampered cookie
		w2 := httptest.NewRecorder()
		http.SetCookie(w2, &http.Cookie{Name: "signed", Value: tampered})

		req2 := &http.Request{Header: http.Header{}}
		req2.Header.Set("Cookie", w2.Header().Get("Set-Cookie"))

		_, err = m.GetSigned(req2, "signed")
		assert.ErrorIs(t, err, cookie.ErrInvalidSignature)
	})
}

func TestManager_EncryptedCookies(t *testing.T) {
	t.Run("set and get encrypted cookie", func(t *testing.T) {
		m, err := cookie.New([]string{testSecret})
		require.NoError(t, err)

		w := httptest.NewRecorder()
		r := httptest.NewRequest("GET", "/", nil)

		// Set encrypted cookie as essential
		err = m.SetEncrypted(w, r, "encrypted", "sensitive-data", cookie.WithEssential())
		assert.NoError(t, err)

		req := &http.Request{Header: http.Header{}}
		req.Header.Set("Cookie", w.Header().Get("Set-Cookie"))

		value, err := m.GetEncrypted(req, "encrypted")
		assert.NoError(t, err)
		assert.Equal(t, "sensitive-data", value)
	})

	t.Run("cannot decrypt with wrong key", func(t *testing.T) {
		m1, err := cookie.New([]string{testSecret})
		require.NoError(t, err)

		w := httptest.NewRecorder()
		r := httptest.NewRequest("GET", "/", nil)

		err = m1.SetEncrypted(w, r, "encrypted", "sensitive-data", cookie.WithEssential())
		assert.NoError(t, err)

		// Try to decrypt with different key
		m2, err := cookie.New([]string{testSecret2})
		require.NoError(t, err)

		req := &http.Request{Header: http.Header{}}
		req.Header.Set("Cookie", w.Header().Get("Set-Cookie"))

		_, err = m2.GetEncrypted(req, "encrypted")
		assert.ErrorIs(t, err, cookie.ErrDecryptionFailed)
	})
}

func TestManager_KeyRotation(t *testing.T) {
	t.Run("decrypt with old key", func(t *testing.T) {
		// Set cookie with old key
		m1, err := cookie.New([]string{testSecret})
		require.NoError(t, err)

		w := httptest.NewRecorder()
		r := httptest.NewRequest("GET", "/", nil)

		err = m1.SetEncrypted(w, r, "data", "value", cookie.WithEssential())
		assert.NoError(t, err)

		// Read with new key but old key in rotation
		m2, err := cookie.New([]string{testSecret2, testSecret})
		require.NoError(t, err)

		req := &http.Request{Header: http.Header{}}
		req.Header.Set("Cookie", w.Header().Get("Set-Cookie"))

		value, err := m2.GetEncrypted(req, "data")
		assert.NoError(t, err)
		assert.Equal(t, "value", value)
	})

	t.Run("verify signature with old key", func(t *testing.T) {
		// Sign with old key
		m1, err := cookie.New([]string{testSecret})
		require.NoError(t, err)

		w := httptest.NewRecorder()
		r := httptest.NewRequest("GET", "/", nil)

		err = m1.SetSigned(w, r, "signed", "data", cookie.WithEssential())
		assert.NoError(t, err)

		// Verify with new key but old key in rotation
		m2, err := cookie.New([]string{testSecret2, testSecret})
		require.NoError(t, err)

		req := &http.Request{Header: http.Header{}}
		req.Header.Set("Cookie", w.Header().Get("Set-Cookie"))

		value, err := m2.GetSigned(req, "signed")
		assert.NoError(t, err)
		assert.Equal(t, "data", value)
	})
}

func TestManager_FlashMessages(t *testing.T) {
	t.Run("set and get flash message", func(t *testing.T) {
		m, err := cookie.New([]string{testSecret})
		require.NoError(t, err)

		w := httptest.NewRecorder()
		r := httptest.NewRequest("GET", "/", nil)

		// Set flash message (always essential)
		flashData := map[string]string{"message": "Success!", "type": "success"}
		err = m.SetFlash(w, r, "notification", flashData)
		assert.NoError(t, err)

		// Get flash message
		req := &http.Request{Header: http.Header{}}
		req.Header.Set("Cookie", w.Header().Get("Set-Cookie"))

		w2 := httptest.NewRecorder()
		var result map[string]string
		err = m.GetFlash(w2, req, "notification", &result)
		assert.NoError(t, err)
		assert.Equal(t, flashData, result)

		// Verify deletion cookie was set
		deleteCookie := w2.Header().Get("Set-Cookie")
		assert.Contains(t, deleteCookie, "__flash_notification=")
		assert.Contains(t, deleteCookie, "Max-Age=0")
	})

	t.Run("flash message auto-deleted", func(t *testing.T) {
		m, err := cookie.New([]string{testSecret})
		require.NoError(t, err)

		w := httptest.NewRecorder()
		r := httptest.NewRequest("GET", "/", nil)

		err = m.SetFlash(w, r, "once", "read-once")
		assert.NoError(t, err)

		req := &http.Request{Header: http.Header{}}
		req.Header.Set("Cookie", w.Header().Get("Set-Cookie"))

		// First read succeeds
		w2 := httptest.NewRecorder()
		var result string
		err = m.GetFlash(w2, req, "once", &result)
		assert.NoError(t, err)
		assert.Equal(t, "read-once", result)

		// Second read fails (cookie deleted)
		req2 := &http.Request{Header: http.Header{}}
		req2.Header.Set("Cookie", w2.Header().Get("Set-Cookie"))

		w3 := httptest.NewRecorder()
		err = m.GetFlash(w3, req2, "once", &result)
		// The error could be either decryption failed (empty value) or not found
		assert.Error(t, err)
	})
}

func TestManager_SizeLimit(t *testing.T) {
	t.Run("enforce size limit", func(t *testing.T) {
		m, err := cookie.NewWithOptions([]string{testSecret}, nil,
			cookie.WithMaxSize(100))
		require.NoError(t, err)

		w := httptest.NewRecorder()
		r := httptest.NewRequest("GET", "/", nil)

		largeValue := strings.Repeat("x", 200)
		err = m.Set(w, r, "large", largeValue, cookie.WithEssential())

		var sizeErr cookie.ErrCookieTooLarge
		require.ErrorAs(t, err, &sizeErr)
		assert.Equal(t, "large", sizeErr.Name)
		assert.Equal(t, 100, sizeErr.Max)
		assert.Greater(t, sizeErr.Size, 100)
	})

	t.Run("allow within limit", func(t *testing.T) {
		m, err := cookie.NewWithOptions([]string{testSecret}, nil,
			cookie.WithMaxSize(1000))
		require.NoError(t, err)

		w := httptest.NewRecorder()
		r := httptest.NewRequest("GET", "/", nil)

		err = m.Set(w, r, "normal", "small-value", cookie.WithEssential())
		assert.NoError(t, err)
	})
}

func TestManager_Options(t *testing.T) {
	t.Run("apply options", func(t *testing.T) {
		m, err := cookie.New([]string{testSecret},
			cookie.WithPath("/admin"),
			cookie.WithDomain("example.com"),
			cookie.WithMaxAge(3600),
			cookie.WithSecure(true),
			cookie.WithHTTPOnly(false),
			cookie.WithSameSite(http.SameSiteStrictMode),
		)
		require.NoError(t, err)

		w := httptest.NewRecorder()
		r := httptest.NewRequest("GET", "/", nil)

		err = m.Set(w, r, "test", "value", cookie.WithEssential())
		assert.NoError(t, err)

		cookies := w.Result().Cookies()
		require.Len(t, cookies, 1)

		c := cookies[0]
		assert.Equal(t, "/admin", c.Path)
		assert.Equal(t, "example.com", c.Domain)
		assert.Equal(t, 3600, c.MaxAge)
		assert.True(t, c.Secure)
		assert.False(t, c.HttpOnly)
		assert.Equal(t, http.SameSiteStrictMode, c.SameSite)
	})

	t.Run("override defaults with per-cookie options", func(t *testing.T) {
		m, err := cookie.New([]string{testSecret},
			cookie.WithHTTPOnly(true),
		)
		require.NoError(t, err)

		w := httptest.NewRecorder()
		r := httptest.NewRequest("GET", "/", nil)

		err = m.Set(w, r, "test", "value",
			cookie.WithEssential(),
			cookie.WithHTTPOnly(false),
			cookie.WithMaxAge(7200),
		)
		assert.NoError(t, err)

		cookies := w.Result().Cookies()
		require.Len(t, cookies, 1)
		assert.False(t, cookies[0].HttpOnly)
		assert.Equal(t, 7200, cookies[0].MaxAge)
	})
}

func TestManager_Consent(t *testing.T) {
	t.Run("store and retrieve consent", func(t *testing.T) {
		m, err := cookie.New([]string{testSecret})
		require.NoError(t, err)

		w := httptest.NewRecorder()
		r := httptest.NewRequest("GET", "/", nil)

		err = m.StoreConsent(w, r, cookie.ConsentAll)
		assert.NoError(t, err)

		req := &http.Request{Header: http.Header{}}
		req.Header.Set("Cookie", w.Header().Get("Set-Cookie"))

		consent, err := m.GetConsent(req)
		assert.NoError(t, err)
		assert.Equal(t, cookie.ConsentAll, consent.Status)
		assert.WithinDuration(t, time.Now(), consent.Timestamp, 1*time.Second)
		assert.Equal(t, "1.0", consent.Version)
	})

	t.Run("check consent status", func(t *testing.T) {
		m, err := cookie.New([]string{testSecret})
		require.NoError(t, err)

		// No consent initially
		req := httptest.NewRequest("GET", "/", nil)
		assert.False(t, m.HasConsent(req))

		// Store essential only consent
		w := httptest.NewRecorder()
		r := httptest.NewRequest("GET", "/", nil)
		err = m.StoreConsent(w, r, cookie.ConsentEssentialOnly)
		assert.NoError(t, err)

		req = &http.Request{Header: http.Header{}}
		req.Header.Set("Cookie", w.Header().Get("Set-Cookie"))
		assert.False(t, m.HasConsent(req))

		// Store full consent
		w = httptest.NewRecorder()
		err = m.StoreConsent(w, req, cookie.ConsentAll)
		assert.NoError(t, err)

		req = &http.Request{Header: http.Header{}}
		req.Header.Set("Cookie", w.Header().Get("Set-Cookie"))
		assert.True(t, m.HasConsent(req))
	})

	t.Run("essential cookies bypass consent", func(t *testing.T) {
		m, err := cookie.New([]string{testSecret})
		require.NoError(t, err)

		w := httptest.NewRecorder()
		r := httptest.NewRequest("GET", "/", nil)

		// Essential cookie should always work
		err = m.Set(w, r, "session", "abc123", cookie.WithEssential())
		assert.NoError(t, err)

		// Non-essential cookie should fail without consent
		err = m.Set(w, r, "analytics", "ga123")
		assert.ErrorIs(t, err, cookie.ErrConsentRequired)
	})

	t.Run("consent enforcement", func(t *testing.T) {
		m, err := cookie.New([]string{testSecret})
		require.NoError(t, err)

		w := httptest.NewRecorder()
		r := httptest.NewRequest("GET", "/", nil)

		// Try to set non-essential cookie without consent
		err = m.Set(w, r, "tracking", "123")
		assert.ErrorIs(t, err, cookie.ErrConsentRequired)

		// Grant consent
		err = m.StoreConsent(w, r, cookie.ConsentAll)
		assert.NoError(t, err)

		// Update request with consent cookie
		r = &http.Request{Header: http.Header{}}
		r.Header.Set("Cookie", w.Header().Get("Set-Cookie"))

		// Now non-essential cookie should work
		w = httptest.NewRecorder()
		err = m.Set(w, r, "tracking", "123")
		assert.NoError(t, err)
	})
}

func TestManager_Validation(t *testing.T) {
	t.Run("require secret", func(t *testing.T) {
		_, err := cookie.New([]string{})
		assert.ErrorIs(t, err, cookie.ErrNoSecret)

		_, err = cookie.New([]string{""})
		assert.ErrorIs(t, err, cookie.ErrNoSecret)
	})

	t.Run("validate secret length", func(t *testing.T) {
		_, err := cookie.New([]string{"short"})
		assert.ErrorIs(t, err, cookie.ErrSecretTooShort)

		_, err = cookie.New([]string{strings.Repeat("x", 31)})
		assert.ErrorIs(t, err, cookie.ErrSecretTooShort)

		_, err = cookie.New([]string{strings.Repeat("x", 32)})
		assert.NoError(t, err)
	})
}

func TestConfig(t *testing.T) {
	t.Run("create from config", func(t *testing.T) {
		cfg := cookie.Config{
			Secrets:  testSecret + "," + testSecret2,
			Path:     "/app",
			Domain:   "example.com",
			MaxAge:   7200,
			Secure:   true,
			HttpOnly: true,
			SameSite: http.SameSiteStrictMode,
			MaxSize:  2048,
		}

		m, err := cookie.NewFromConfig(cfg)
		require.NoError(t, err)

		w := httptest.NewRecorder()
		r := httptest.NewRequest("GET", "/", nil)

		err = m.Set(w, r, "test", "value", cookie.WithEssential())
		assert.NoError(t, err)

		cookies := w.Result().Cookies()
		require.Len(t, cookies, 1)

		c := cookies[0]
		assert.Equal(t, "/app", c.Path)
		assert.Equal(t, "example.com", c.Domain)
		assert.Equal(t, 7200, c.MaxAge)
		assert.True(t, c.Secure)
		assert.True(t, c.HttpOnly)
		assert.Equal(t, http.SameSiteStrictMode, c.SameSite)
	})

	t.Run("parse comma-separated secrets", func(t *testing.T) {
		cfg := cookie.Config{
			Secrets: testSecret + ", " + testSecret2 + ", ",
		}

		m, err := cookie.NewFromConfig(cfg)
		require.NoError(t, err)

		// Test key rotation works
		w := httptest.NewRecorder()
		r := httptest.NewRequest("GET", "/", nil)

		err = m.SetEncrypted(w, r, "test", "data", cookie.WithEssential())
		assert.NoError(t, err)
	})

	t.Run("default config", func(t *testing.T) {
		cfg := cookie.DefaultConfig()
		assert.Equal(t, "/", cfg.Path)
		assert.True(t, cfg.HttpOnly)
		assert.Equal(t, http.SameSiteLaxMode, cfg.SameSite)
		assert.Equal(t, 4096, cfg.MaxSize)
		assert.Equal(t, "__cookie_consent", cfg.ConsentCookieName)
	})

	t.Run("consent configuration", func(t *testing.T) {
		cfg := cookie.Config{
			Secrets:           testSecret,
			ConsentCookieName: "__gdpr_consent",
			ConsentVersion:    "2.0",
			ConsentMaxAge:     30 * 24 * 60 * 60, // 30 days
		}

		m, err := cookie.NewFromConfig(cfg)
		require.NoError(t, err)

		w := httptest.NewRecorder()
		r := httptest.NewRequest("GET", "/", nil)

		// Store consent
		err = m.StoreConsent(w, r, cookie.ConsentAll)
		assert.NoError(t, err)

		// Check cookie was set with custom name
		cookies := w.Result().Cookies()
		found := false
		for _, c := range cookies {
			if c.Name == "__gdpr_consent" {
				found = true
				assert.Equal(t, 30*24*60*60, c.MaxAge)
				break
			}
		}
		assert.True(t, found, "Consent cookie with custom name should be set")
	})
}
