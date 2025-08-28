package sessiontransport_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/dmitrymomot/foundation/core/cookie"
	"github.com/dmitrymomot/foundation/core/session"
	"github.com/dmitrymomot/foundation/core/sessiontransport"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// createTestCookieManager creates a real cookie manager for testing
func createTestCookieManager() (*cookie.Manager, error) {
	return cookie.New([]string{"test-secret-key-at-least-32-chars-long!!!"})
}

func TestNewCookie(t *testing.T) {
	t.Parallel()

	t.Run("default configuration", func(t *testing.T) {
		t.Parallel()

		manager, err := createTestCookieManager()
		require.NoError(t, err)

		transport := sessiontransport.NewCookie(manager)
		assert.NotNil(t, transport)

		// Test default cookie name by attempting to extract (should fail gracefully)
		r := httptest.NewRequest("GET", "/", nil)
		_, err = transport.Extract(r)
		assert.ErrorIs(t, err, session.ErrNoToken)
	})

	t.Run("custom cookie name", func(t *testing.T) {
		t.Parallel()

		manager, err := createTestCookieManager()
		require.NoError(t, err)

		transport := sessiontransport.NewCookie(
			manager,
			sessiontransport.WithCookieName("custom_session"),
		)
		assert.NotNil(t, transport)

		// Test custom cookie name functionality
		w := httptest.NewRecorder()
		r := httptest.NewRequest("GET", "/", nil)

		err = transport.Embed(w, r, "test-token", time.Hour)
		require.NoError(t, err)

		// Check cookie is set
		cookies := w.Result().Cookies()
		found := false
		for _, c := range cookies {
			if c.Name == "custom_session" {
				found = true
				break
			}
		}
		assert.True(t, found, "Custom cookie name should be used")
	})

	t.Run("empty cookie name uses default", func(t *testing.T) {
		t.Parallel()

		manager, err := createTestCookieManager()
		require.NoError(t, err)

		transport := sessiontransport.NewCookie(
			manager,
			sessiontransport.WithCookieName(""), // Empty name should use default
		)
		assert.NotNil(t, transport)

		// Should use default cookie name
		w := httptest.NewRecorder()
		r := httptest.NewRequest("GET", "/", nil)

		err = transport.Embed(w, r, "test-token", time.Hour)
		require.NoError(t, err)

		// Check default cookie is set
		cookies := w.Result().Cookies()
		found := false
		for _, c := range cookies {
			if c.Name == "__session" {
				found = true
				break
			}
		}
		assert.True(t, found, "Default cookie name should be used")
	})

	t.Run("with custom cookie options", func(t *testing.T) {
		t.Parallel()

		manager, err := createTestCookieManager()
		require.NoError(t, err)

		transport := sessiontransport.NewCookie(
			manager,
			sessiontransport.WithCookieOptions(
				cookie.WithDomain(".example.com"),
				cookie.WithSecure(true),
			),
		)
		assert.NotNil(t, transport)

		w := httptest.NewRecorder()
		r := httptest.NewRequest("GET", "/", nil)

		err = transport.Embed(w, r, "test-token", time.Hour)
		require.NoError(t, err)

		// Check custom options are applied
		cookies := w.Result().Cookies()
		require.Len(t, cookies, 1)
		cookie := cookies[0]
		// Domain might be normalized by cookie manager
		assert.Contains(t, cookie.Domain, "example.com")
		assert.True(t, cookie.Secure)
	})
}

func TestCookieTransport_Extract(t *testing.T) {
	t.Parallel()

	manager, err := createTestCookieManager()
	require.NoError(t, err)

	transport := sessiontransport.NewCookie(manager)

	t.Run("successful extraction", func(t *testing.T) {
		t.Parallel()

		// First embed a token to create a valid cookie
		w := httptest.NewRecorder()
		r1 := httptest.NewRequest("GET", "/", nil)
		expectedToken := "test-session-token"

		err := transport.Embed(w, r1, expectedToken, time.Hour)
		require.NoError(t, err)

		// Get the cookie from the response
		cookies := w.Result().Cookies()
		require.Len(t, cookies, 1)

		// Create new request with the cookie
		r2 := httptest.NewRequest("GET", "/", nil)
		r2.AddCookie(cookies[0])

		token, err := transport.Extract(r2)
		assert.NoError(t, err)
		assert.Equal(t, expectedToken, token)
	})

	t.Run("cookie not found returns ErrNoToken", func(t *testing.T) {
		t.Parallel()

		r := httptest.NewRequest("GET", "/", nil)
		_, err := transport.Extract(r)
		assert.ErrorIs(t, err, session.ErrNoToken)
	})

	t.Run("empty token returns ErrNoToken", func(t *testing.T) {
		t.Parallel()

		// Embed empty token
		w := httptest.NewRecorder()
		r1 := httptest.NewRequest("GET", "/", nil)

		err := transport.Embed(w, r1, "", time.Hour)
		require.NoError(t, err)

		// Extract should return ErrNoToken
		cookies := w.Result().Cookies()
		require.Len(t, cookies, 1)

		r2 := httptest.NewRequest("GET", "/", nil)
		r2.AddCookie(cookies[0])

		_, err = transport.Extract(r2)
		assert.ErrorIs(t, err, session.ErrNoToken)
	})

	t.Run("corrupted cookie returns ErrInvalidToken", func(t *testing.T) {
		t.Parallel()

		// Create request with corrupted cookie
		r := httptest.NewRequest("GET", "/", nil)
		r.AddCookie(&http.Cookie{
			Name:  "__session",
			Value: "corrupted-invalid-encrypted-data",
		})

		_, err := transport.Extract(r)
		assert.ErrorIs(t, err, session.ErrInvalidToken)
	})

	t.Run("custom cookie name", func(t *testing.T) {
		t.Parallel()

		customTransport := sessiontransport.NewCookie(
			manager,
			sessiontransport.WithCookieName("custom_session"),
		)

		// Embed with custom name
		w := httptest.NewRecorder()
		r1 := httptest.NewRequest("GET", "/", nil)
		token := "test-session-token"

		err := customTransport.Embed(w, r1, token, time.Hour)
		require.NoError(t, err)

		// Extract with custom name
		cookies := w.Result().Cookies()
		require.Len(t, cookies, 1)
		assert.Equal(t, "custom_session", cookies[0].Name)

		r2 := httptest.NewRequest("GET", "/", nil)
		r2.AddCookie(cookies[0])

		extractedToken, err := customTransport.Extract(r2)
		assert.NoError(t, err)
		assert.Equal(t, token, extractedToken)
	})
}

func TestCookieTransport_Embed(t *testing.T) {
	t.Parallel()

	manager, err := createTestCookieManager()
	require.NoError(t, err)

	transport := sessiontransport.NewCookie(manager)

	t.Run("successful embed", func(t *testing.T) {
		t.Parallel()

		w := httptest.NewRecorder()
		r := httptest.NewRequest("GET", "/", nil)
		token := "test-session-token"
		ttl := time.Hour

		err := transport.Embed(w, r, token, ttl)
		assert.NoError(t, err)

		// Check cookie is set
		cookies := w.Result().Cookies()
		require.Len(t, cookies, 1)
		cookie := cookies[0]

		assert.Equal(t, "__session", cookie.Name)
		assert.NotEmpty(t, cookie.Value)
		assert.Equal(t, int(ttl.Seconds()), cookie.MaxAge)
	})

	t.Run("zero TTL", func(t *testing.T) {
		t.Parallel()

		w := httptest.NewRecorder()
		r := httptest.NewRequest("GET", "/", nil)
		token := "test-session-token"
		ttl := time.Duration(0)

		err := transport.Embed(w, r, token, ttl)
		assert.NoError(t, err)

		// Check cookie has MaxAge 0
		cookies := w.Result().Cookies()
		require.Len(t, cookies, 1)
		assert.Equal(t, 0, cookies[0].MaxAge)
	})

	t.Run("negative TTL", func(t *testing.T) {
		t.Parallel()

		w := httptest.NewRecorder()
		r := httptest.NewRequest("GET", "/", nil)
		token := "test-session-token"
		ttl := -time.Hour

		err := transport.Embed(w, r, token, ttl)
		assert.NoError(t, err)

		// Check cookie has negative MaxAge (but cookie.Manager might normalize to -1)
		cookies := w.Result().Cookies()
		require.Len(t, cookies, 1)
		// Cookie managers often normalize negative MaxAge to -1
		assert.LessOrEqual(t, cookies[0].MaxAge, 0)
	})

	t.Run("very large TTL", func(t *testing.T) {
		t.Parallel()

		w := httptest.NewRecorder()
		r := httptest.NewRequest("GET", "/", nil)
		token := "test-session-token"
		ttl := time.Hour * 24 * 365 * 10 // 10 years

		err := transport.Embed(w, r, token, ttl)
		assert.NoError(t, err)

		// Check cookie is set with large MaxAge
		cookies := w.Result().Cookies()
		require.Len(t, cookies, 1)
		assert.Equal(t, int(ttl.Seconds()), cookies[0].MaxAge)
	})

	t.Run("empty token embed", func(t *testing.T) {
		t.Parallel()

		w := httptest.NewRecorder()
		r := httptest.NewRequest("GET", "/", nil)
		token := "" // Empty token
		ttl := time.Hour

		err := transport.Embed(w, r, token, ttl)
		assert.NoError(t, err)

		// Check cookie is still set (empty value is valid)
		cookies := w.Result().Cookies()
		require.Len(t, cookies, 1)
		assert.Equal(t, "__session", cookies[0].Name)
	})

	t.Run("cookie too large scenario", func(t *testing.T) {
		t.Parallel()

		// Create manager with very small max size to trigger error
		smallManager, err := cookie.NewWithOptions(
			[]string{"test-secret-key-at-least-32-chars-long!!!"},
			nil,
			cookie.WithMaxSize(10), // Very small size
		)
		require.NoError(t, err)

		smallTransport := sessiontransport.NewCookie(smallManager)

		w := httptest.NewRecorder()
		r := httptest.NewRequest("GET", "/", nil)
		largeToken := "this-is-a-very-large-token-that-should-exceed-the-10-byte-limit-when-encrypted"

		err = smallTransport.Embed(w, r, largeToken, time.Hour)
		assert.ErrorIs(t, err, session.ErrInvalidToken)
	})
}

func TestCookieTransport_Revoke(t *testing.T) {
	t.Parallel()

	manager, err := createTestCookieManager()
	require.NoError(t, err)

	transport := sessiontransport.NewCookie(manager)

	t.Run("successful revoke", func(t *testing.T) {
		t.Parallel()

		// First embed a token
		w1 := httptest.NewRecorder()
		r1 := httptest.NewRequest("GET", "/", nil)

		err := transport.Embed(w1, r1, "test-token", time.Hour)
		require.NoError(t, err)

		// Verify cookie is set
		cookies := w1.Result().Cookies()
		require.Len(t, cookies, 1)
		assert.NotEmpty(t, cookies[0].Value)

		// Revoke the cookie
		w2 := httptest.NewRecorder()
		r2 := httptest.NewRequest("GET", "/", nil)

		err = transport.Revoke(w2, r2)
		assert.NoError(t, err)

		// Check revocation cookie is set (empty value, negative MaxAge)
		revokeCookies := w2.Result().Cookies()
		require.Len(t, revokeCookies, 1)
		revokeCookie := revokeCookies[0]

		assert.Equal(t, "__session", revokeCookie.Name)
		assert.Empty(t, revokeCookie.Value)
		assert.Equal(t, -1, revokeCookie.MaxAge)
	})

	t.Run("custom cookie name", func(t *testing.T) {
		t.Parallel()

		customTransport := sessiontransport.NewCookie(
			manager,
			sessiontransport.WithCookieName("custom_session"),
		)

		w := httptest.NewRecorder()
		r := httptest.NewRequest("GET", "/", nil)

		err := customTransport.Revoke(w, r)
		assert.NoError(t, err)

		// Check custom named cookie is revoked
		cookies := w.Result().Cookies()
		require.Len(t, cookies, 1)
		assert.Equal(t, "custom_session", cookies[0].Name)
		assert.Empty(t, cookies[0].Value)
	})

	t.Run("multiple revokes succeed", func(t *testing.T) {
		t.Parallel()

		w := httptest.NewRecorder()
		r := httptest.NewRequest("GET", "/", nil)

		// Revoke is idempotent - should succeed multiple times
		err1 := transport.Revoke(w, r)
		assert.NoError(t, err1)

		err2 := transport.Revoke(w, r)
		assert.NoError(t, err2)

		// Both should create revocation cookies
		cookies := w.Result().Cookies()
		assert.Len(t, cookies, 2) // Two identical revocation cookies
		for _, cookie := range cookies {
			assert.Equal(t, "__session", cookie.Name)
			assert.Empty(t, cookie.Value)
		}
	})
}

func TestCookieTransport_Integration(t *testing.T) {
	t.Parallel()

	manager, err := createTestCookieManager()
	require.NoError(t, err)

	transport := sessiontransport.NewCookie(manager)

	t.Run("full embed-extract-revoke cycle", func(t *testing.T) {
		t.Parallel()

		token := "test-session-token"

		// 1. Embed token
		w1 := httptest.NewRecorder()
		r1 := httptest.NewRequest("GET", "/", nil)

		err := transport.Embed(w1, r1, token, time.Hour)
		require.NoError(t, err)

		cookies := w1.Result().Cookies()
		require.Len(t, cookies, 1)

		// 2. Extract token
		r2 := httptest.NewRequest("GET", "/", nil)
		r2.AddCookie(cookies[0])

		extractedToken, err := transport.Extract(r2)
		assert.NoError(t, err)
		assert.Equal(t, token, extractedToken)

		// 3. Revoke token
		w3 := httptest.NewRecorder()
		r3 := httptest.NewRequest("GET", "/", nil)

		err = transport.Revoke(w3, r3)
		assert.NoError(t, err)

		// 4. Verify revocation worked
		revokeCookies := w3.Result().Cookies()
		require.Len(t, revokeCookies, 1)
		assert.Empty(t, revokeCookies[0].Value)
	})

	t.Run("extract after revoke fails", func(t *testing.T) {
		t.Parallel()

		// First embed and then revoke to simulate revoked session
		w1 := httptest.NewRecorder()
		r1 := httptest.NewRequest("GET", "/", nil)

		err := transport.Embed(w1, r1, "test-token", time.Hour)
		require.NoError(t, err)

		// Now revoke
		w2 := httptest.NewRecorder()
		r2 := httptest.NewRequest("GET", "/", nil)

		err = transport.Revoke(w2, r2)
		require.NoError(t, err)

		// Try to extract from revoked cookie
		revokeCookies := w2.Result().Cookies()
		require.Len(t, revokeCookies, 1)

		r3 := httptest.NewRequest("GET", "/", nil)
		r3.AddCookie(revokeCookies[0])

		_, err = transport.Extract(r3)
		// Empty cookie should return some error (could be ErrNoToken or ErrInvalidToken)
		assert.Error(t, err) // Any error is acceptable for empty/revoked cookie
	})
}
