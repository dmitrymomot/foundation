package sessiontransport_test

import (
	"context"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/dmitrymomot/gokit/core/session"
	"github.com/dmitrymomot/gokit/core/sessiontransport"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testSigningKey = "test-secret-key-at-least-32-bytes-long"

func TestJWTTransport_Extract(t *testing.T) {
	transport, err := sessiontransport.NewJWT(testSigningKey, nil)
	require.NoError(t, err)

	t.Run("extract valid token with Bearer prefix", func(t *testing.T) {
		// First embed a token
		w := httptest.NewRecorder()
		r := httptest.NewRequest("GET", "/", nil)
		sessionToken := "test-session-token"
		ttl := time.Hour

		err := transport.Embed(w, r, sessionToken, ttl)
		require.NoError(t, err)

		// Get the JWT from the response header
		jwtToken := w.Header().Get("Authorization")
		require.NotEmpty(t, jwtToken)

		// Now try to extract it
		r2 := httptest.NewRequest("GET", "/", nil)
		r2.Header.Set("Authorization", jwtToken)

		extractedToken, err := transport.Extract(r2)
		assert.NoError(t, err)
		assert.Equal(t, sessionToken, extractedToken)
	})

	t.Run("no token in header", func(t *testing.T) {
		r := httptest.NewRequest("GET", "/", nil)
		_, err := transport.Extract(r)
		assert.ErrorIs(t, err, session.ErrNoToken)
	})

	t.Run("invalid Bearer format", func(t *testing.T) {
		r := httptest.NewRequest("GET", "/", nil)
		r.Header.Set("Authorization", "InvalidFormat token")
		_, err := transport.Extract(r)
		assert.ErrorIs(t, err, session.ErrInvalidToken)
	})

	t.Run("empty Bearer token", func(t *testing.T) {
		r := httptest.NewRequest("GET", "/", nil)
		r.Header.Set("Authorization", "Bearer ")
		_, err := transport.Extract(r)
		assert.ErrorIs(t, err, session.ErrNoToken)
	})

	t.Run("invalid JWT signature", func(t *testing.T) {
		r := httptest.NewRequest("GET", "/", nil)
		r.Header.Set("Authorization", "Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzZXNzaW9uX3Rva2VuIjoidGVzdCJ9.invalid")
		_, err := transport.Extract(r)
		assert.ErrorIs(t, err, session.ErrInvalidToken)
	})
}

func TestJWTTransport_ExtractWithoutBearerPrefix(t *testing.T) {
	transport, err := sessiontransport.NewJWT(
		testSigningKey,
		nil,
		sessiontransport.WithJWTBearerPrefix(false),
	)
	require.NoError(t, err)

	// First embed a token
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/", nil)
	sessionToken := "test-session-token"
	ttl := time.Hour

	err = transport.Embed(w, r, sessionToken, ttl)
	require.NoError(t, err)

	// Get the JWT from the response header (should not have Bearer prefix)
	jwtToken := w.Header().Get("Authorization")
	require.NotEmpty(t, jwtToken)
	assert.NotContains(t, jwtToken, "Bearer ")

	// Now try to extract it
	r2 := httptest.NewRequest("GET", "/", nil)
	r2.Header.Set("Authorization", jwtToken)

	extractedToken, err := transport.Extract(r2)
	assert.NoError(t, err)
	assert.Equal(t, sessionToken, extractedToken)
}

func TestJWTTransport_CustomHeader(t *testing.T) {
	transport, err := sessiontransport.NewJWT(
		testSigningKey,
		nil,
		sessiontransport.WithJWTHeaderName("X-Session-Token"),
	)
	require.NoError(t, err)

	// Embed a token
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/", nil)
	sessionToken := "test-session-token"
	ttl := time.Hour

	err = transport.Embed(w, r, sessionToken, ttl)
	require.NoError(t, err)

	// Check it's in the custom header
	jwtToken := w.Header().Get("X-Session-Token")
	require.NotEmpty(t, jwtToken)
	assert.Contains(t, jwtToken, "Bearer ")

	// Authorization header should be empty
	assert.Empty(t, w.Header().Get("Authorization"))

	// Extract from custom header
	r2 := httptest.NewRequest("GET", "/", nil)
	r2.Header.Set("X-Session-Token", jwtToken)

	extractedToken, err := transport.Extract(r2)
	assert.NoError(t, err)
	assert.Equal(t, sessionToken, extractedToken)
}

func TestJWTTransport_Embed(t *testing.T) {
	transport, err := sessiontransport.NewJWT(
		testSigningKey,
		nil,
		sessiontransport.WithJWTIssuer("test-issuer"),
		sessiontransport.WithJWTAudience("test-audience"),
	)
	require.NoError(t, err)

	t.Run("embed token successfully", func(t *testing.T) {
		w := httptest.NewRecorder()
		r := httptest.NewRequest("GET", "/", nil)
		sessionToken := "test-session-token"
		ttl := time.Hour

		err := transport.Embed(w, r, sessionToken, ttl)
		assert.NoError(t, err)

		// Check header is set
		authHeader := w.Header().Get("Authorization")
		assert.NotEmpty(t, authHeader)
		assert.Contains(t, authHeader, "Bearer ")
	})
}

func TestJWTTransport_Revoke(t *testing.T) {
	transport, err := sessiontransport.NewJWT(testSigningKey, nil)
	require.NoError(t, err)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/", nil)

	// First embed a token
	err = transport.Embed(w, r, "test-token", time.Hour)
	require.NoError(t, err)
	require.NotEmpty(t, w.Header().Get("Authorization"))

	// Revoke should remove the header
	err = transport.Revoke(w, r)
	assert.NoError(t, err)
	assert.Empty(t, w.Header().Get("Authorization"))
}

// mockRevoker implements the Revoker interface for testing
type mockRevoker struct {
	revokedJTIs map[string]bool
	revokeError error
	checkError  error
}

func newMockRevoker() *mockRevoker {
	return &mockRevoker{
		revokedJTIs: make(map[string]bool),
	}
}

func (m *mockRevoker) IsRevoked(ctx context.Context, jti string) (bool, error) {
	if m.checkError != nil {
		return false, m.checkError
	}
	return m.revokedJTIs[jti], nil
}

func (m *mockRevoker) Revoke(ctx context.Context, jti string) error {
	if m.revokeError != nil {
		return m.revokeError
	}
	m.revokedJTIs[jti] = true
	return nil
}

func TestJWTTransport_WithRevoker(t *testing.T) {
	revoker := newMockRevoker()
	transport, err := sessiontransport.NewJWT(testSigningKey, revoker)
	require.NoError(t, err)

	// Embed a token
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/", nil)
	sessionToken := "test-session-token"
	err = transport.Embed(w, r, sessionToken, time.Hour)
	require.NoError(t, err)

	jwtToken := w.Header().Get("Authorization")
	require.NotEmpty(t, jwtToken)

	t.Run("extract non-revoked token", func(t *testing.T) {
		r2 := httptest.NewRequest("GET", "/", nil)
		r2.Header.Set("Authorization", jwtToken)

		extractedToken, err := transport.Extract(r2)
		assert.NoError(t, err)
		assert.Equal(t, sessionToken, extractedToken)
	})

	t.Run("revoke token", func(t *testing.T) {
		// Revoke the token
		w2 := httptest.NewRecorder()
		r2 := httptest.NewRequest("GET", "/", nil)
		r2.Header.Set("Authorization", jwtToken)

		err := transport.Revoke(w2, r2)
		assert.NoError(t, err)

		// The JWT ID should be revoked (we need to parse the token to get the JWT ID)
		// For now, we'll check that at least one JWT ID is revoked
		assert.Len(t, revoker.revokedJTIs, 1)
	})

	t.Run("extract revoked token fails", func(t *testing.T) {
		r2 := httptest.NewRequest("GET", "/", nil)
		r2.Header.Set("Authorization", jwtToken)

		_, err := transport.Extract(r2)
		assert.ErrorIs(t, err, session.ErrInvalidToken)
	})
}

func TestJWTTransport_SessionTokenAsJWTID(t *testing.T) {
	transport, err := sessiontransport.NewJWT(testSigningKey, nil)
	require.NoError(t, err)

	// Embed a token
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/", nil)
	sessionToken := "test-session-token-as-jti"
	err = transport.Embed(w, r, sessionToken, time.Hour)
	require.NoError(t, err)

	// Extract and verify session token is returned
	jwtToken := w.Header().Get("Authorization")
	require.NotEmpty(t, jwtToken)

	r2 := httptest.NewRequest("GET", "/", nil)
	r2.Header.Set("Authorization", jwtToken)

	extractedToken, err := transport.Extract(r2)
	assert.NoError(t, err)
	assert.Equal(t, sessionToken, extractedToken)
	// The session token is now used as JWT ID internally
}

func TestJWTTransport_ExpiredToken(t *testing.T) {
	transport, err := sessiontransport.NewJWT(testSigningKey, nil)
	require.NoError(t, err)

	// Embed a token with negative TTL to ensure it's already expired
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/", nil)
	sessionToken := "test-session-token"
	ttl := -1 * time.Second // Negative TTL - token is already expired

	err = transport.Embed(w, r, sessionToken, ttl)
	require.NoError(t, err)

	jwtToken := w.Header().Get("Authorization")
	require.NotEmpty(t, jwtToken)

	// Try to extract expired token
	r2 := httptest.NewRequest("GET", "/", nil)
	r2.Header.Set("Authorization", jwtToken)

	_, err = transport.Extract(r2)
	assert.ErrorIs(t, err, session.ErrInvalidToken)
}

func TestNoOpRevoker(t *testing.T) {
	revoker := sessiontransport.NoOpRevoker{}
	ctx := context.Background()

	t.Run("IsRevoked always returns false", func(t *testing.T) {
		revoked, err := revoker.IsRevoked(ctx, "any-token")
		assert.NoError(t, err)
		assert.False(t, revoked)
	})

	t.Run("Revoke does nothing", func(t *testing.T) {
		err := revoker.Revoke(ctx, "any-token")
		assert.NoError(t, err)

		// Token should still not be revoked
		revoked, err := revoker.IsRevoked(ctx, "any-token")
		assert.NoError(t, err)
		assert.False(t, revoked)
	})
}
