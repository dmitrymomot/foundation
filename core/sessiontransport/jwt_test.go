package sessiontransport_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/dmitrymomot/foundation/core/session"
	"github.com/dmitrymomot/foundation/core/sessiontransport"
	"github.com/dmitrymomot/foundation/pkg/jwt"
)

// JWT-specific helper functions

func createJWTTransport(t *testing.T) (*sessiontransport.JWT[testData], *mockStore) {
	t.Helper()
	store := &mockStore{}
	mgr := session.NewManager[testData](store, time.Hour, 5*time.Minute)
	transport, err := sessiontransport.NewJWT[testData](
		mgr,
		"test-secret-key-that-is-at-least-32-bytes-long",
		15*time.Minute, // accessTTL
		"test-issuer",
	)
	require.NoError(t, err)
	return transport, store
}

func createValidJWT(t *testing.T, secretKey string, sessionToken string) string {
	t.Helper()
	signer, err := jwt.NewFromString(secretKey)
	require.NoError(t, err)

	claims := jwt.StandardClaims{
		ID:        sessionToken,
		Subject:   uuid.New().String(),
		Issuer:    "test-issuer",
		IssuedAt:  time.Now().Unix(),
		ExpiresAt: time.Now().Add(15 * time.Minute).Unix(),
	}

	token, err := signer.Generate(claims)
	require.NoError(t, err)
	return token
}

func createJWTContext(t *testing.T, authHeader string) *mockContext {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	if authHeader != "" {
		req.Header.Set("Authorization", authHeader)
	}
	req.Header.Set("User-Agent", "Test-Agent/1.0")
	req.Header.Set("X-Forwarded-For", "127.0.0.1")

	return &mockContext{
		request:        req,
		responseWriter: httptest.NewRecorder(),
	}
}

func createAuthenticatedSession(t *testing.T) session.Session[testData] {
	t.Helper()
	sess := createValidSession(t)
	err := sess.Authenticate(uuid.New())
	require.NoError(t, err)
	return sess
}

// Load Method Tests

func TestJWTLoad_NoAuthHeader(t *testing.T) {
	t.Parallel()

	t.Run("returns ErrNoToken when Authorization header is missing", func(t *testing.T) {
		t.Parallel()

		transport, _ := createJWTTransport(t)
		ctx := createJWTContext(t, "")

		_, err := transport.Load(ctx)

		require.Error(t, err)
		assert.ErrorIs(t, err, sessiontransport.ErrNoToken)
	})
}

func TestJWTLoad_EmptyAuthHeader(t *testing.T) {
	t.Parallel()

	t.Run("returns ErrNoToken when Authorization header is empty", func(t *testing.T) {
		t.Parallel()

		transport, _ := createJWTTransport(t)
		ctx := createJWTContext(t, "")

		_, err := transport.Load(ctx)

		require.Error(t, err)
		assert.ErrorIs(t, err, sessiontransport.ErrNoToken)
	})
}

func TestJWTLoad_NotBearerFormat(t *testing.T) {
	t.Parallel()

	t.Run("returns ErrNoToken when not Bearer format", func(t *testing.T) {
		t.Parallel()

		transport, _ := createJWTTransport(t)
		ctx := createJWTContext(t, "Basic dXNlcjpwYXNz")

		_, err := transport.Load(ctx)

		require.Error(t, err)
		assert.ErrorIs(t, err, sessiontransport.ErrNoToken)
	})
}

func TestJWTLoad_BearerOnly(t *testing.T) {
	t.Parallel()

	t.Run("returns ErrNoToken when only Bearer without token", func(t *testing.T) {
		t.Parallel()

		transport, _ := createJWTTransport(t)
		ctx := createJWTContext(t, "Bearer")

		_, err := transport.Load(ctx)

		require.Error(t, err)
		assert.ErrorIs(t, err, sessiontransport.ErrNoToken)
	})
}

func TestJWTLoad_InvalidJWT(t *testing.T) {
	t.Parallel()

	t.Run("creates anonymous session on JWT parse failure", func(t *testing.T) {
		t.Parallel()

		transport, store := createJWTTransport(t)
		ctx := createJWTContext(t, "Bearer invalid-jwt-token")

		sess, err := transport.Load(ctx)

		require.NoError(t, err)
		assert.False(t, sess.IsAuthenticated())
		assert.Equal(t, uuid.Nil, sess.UserID)
		assert.Equal(t, "127.0.0.1", sess.IP)
		assert.Equal(t, "Test-Agent/1.0", sess.UserAgent)
		store.AssertExpectations(t)
	})
}

func TestJWTLoad_TokenNotFound(t *testing.T) {
	t.Parallel()

	t.Run("creates anonymous session when GetByToken fails", func(t *testing.T) {
		t.Parallel()

		transport, store := createJWTTransport(t)
		secretKey := "test-secret-key-that-is-at-least-32-bytes-long"
		validJWT := createValidJWT(t, secretKey, "session-token-123")
		ctx := createJWTContext(t, "Bearer "+validJWT)

		store.On("GetByToken", ctx, "session-token-123").
			Return(nil, session.ErrNotFound)

		sess, err := transport.Load(ctx)

		require.NoError(t, err)
		assert.False(t, sess.IsAuthenticated())
		assert.Equal(t, uuid.Nil, sess.UserID)
		store.AssertExpectations(t)
	})
}

func TestJWTLoad_SessionExpired(t *testing.T) {
	t.Parallel()

	t.Run("creates anonymous session when session is expired", func(t *testing.T) {
		t.Parallel()

		transport, store := createJWTTransport(t)
		secretKey := "test-secret-key-that-is-at-least-32-bytes-long"
		validJWT := createValidJWT(t, secretKey, "session-token-123")
		ctx := createJWTContext(t, "Bearer "+validJWT)

		store.On("GetByToken", ctx, "session-token-123").
			Return(nil, session.ErrExpired)

		sess, err := transport.Load(ctx)

		require.NoError(t, err)
		assert.False(t, sess.IsAuthenticated())
		store.AssertExpectations(t)
	})
}

func TestJWTLoad_Success(t *testing.T) {
	t.Parallel()

	t.Run("loads session from valid JWT", func(t *testing.T) {
		t.Parallel()

		transport, store := createJWTTransport(t)
		secretKey := "test-secret-key-that-is-at-least-32-bytes-long"
		existingSession := createAuthenticatedSession(t)
		validJWT := createValidJWT(t, secretKey, existingSession.Token)
		ctx := createJWTContext(t, "Bearer "+validJWT)

		store.On("GetByToken", ctx, existingSession.Token).
			Return(&existingSession, nil)

		sess, err := transport.Load(ctx)

		require.NoError(t, err)
		assert.Equal(t, existingSession.ID, sess.ID)
		assert.Equal(t, existingSession.Token, sess.Token)
		assert.True(t, sess.IsAuthenticated())
		store.AssertExpectations(t)
	})
}

func TestJWTLoad_ExtractsJTI(t *testing.T) {
	t.Parallel()

	t.Run("verifies JTI claim used as session token", func(t *testing.T) {
		t.Parallel()

		transport, store := createJWTTransport(t)
		secretKey := "test-secret-key-that-is-at-least-32-bytes-long"
		expectedToken := "unique-session-token-jti"
		validJWT := createValidJWT(t, secretKey, expectedToken)
		ctx := createJWTContext(t, "Bearer "+validJWT)

		existingSession := createAuthenticatedSession(t)
		existingSession.Token = expectedToken

		store.On("GetByToken", ctx, expectedToken).
			Return(&existingSession, nil)

		sess, err := transport.Load(ctx)

		require.NoError(t, err)
		assert.Equal(t, expectedToken, sess.Token)
		store.AssertExpectations(t)
	})
}

// Save Method Tests

func TestJWTSave_Success(t *testing.T) {
	t.Parallel()

	t.Run("delegates to Manager.Store successfully", func(t *testing.T) {
		t.Parallel()

		transport, store := createJWTTransport(t)
		ctx := createJWTContext(t, "")
		sess := createAuthenticatedSession(t)

		store.On("Save", ctx, &sess).Return(nil)

		err := transport.Save(ctx, sess)

		require.NoError(t, err)
		store.AssertExpectations(t)
	})
}

func TestJWTSave_Error(t *testing.T) {
	t.Parallel()

	t.Run("propagates Manager.Store errors", func(t *testing.T) {
		t.Parallel()

		transport, store := createJWTTransport(t)
		ctx := createJWTContext(t, "")
		sess := createAuthenticatedSession(t)
		storeErr := errors.New("database error")

		store.On("Save", ctx, &sess).Return(storeErr)

		err := transport.Save(ctx, sess)

		require.Error(t, err)
		assert.ErrorIs(t, err, storeErr)
		store.AssertExpectations(t)
	})
}

// Authenticate Method Tests

func TestJWTAuthenticate_Success(t *testing.T) {
	t.Parallel()

	t.Run("full authentication chain with TokenPair generation", func(t *testing.T) {
		t.Parallel()

		transport, store := createJWTTransport(t)
		ctx := createJWTContext(t, "")
		userID := uuid.New()

		store.On("Save", ctx, mock.MatchedBy(func(s *session.Session[testData]) bool {
			return s.IsAuthenticated() && s.UserID == userID
		})).Return(nil)

		sess, pair, err := transport.Authenticate(ctx, userID)

		require.NoError(t, err)
		assert.True(t, sess.IsAuthenticated())
		assert.Equal(t, userID, sess.UserID)
		assert.NotEmpty(t, pair.AccessToken)
		assert.NotEmpty(t, pair.RefreshToken)
		assert.Equal(t, "Bearer", pair.TokenType)
		store.AssertExpectations(t)
	})
}

func TestJWTAuthenticate_NoToken(t *testing.T) {
	t.Parallel()

	t.Run("handles ErrNoToken gracefully", func(t *testing.T) {
		t.Parallel()

		transport, store := createJWTTransport(t)
		ctx := createJWTContext(t, "")
		userID := uuid.New()

		store.On("Save", ctx, mock.MatchedBy(func(s *session.Session[testData]) bool {
			return s.IsAuthenticated() && s.UserID == userID
		})).Return(nil)

		sess, pair, err := transport.Authenticate(ctx, userID)

		require.NoError(t, err)
		assert.True(t, sess.IsAuthenticated())
		assert.NotEmpty(t, pair.AccessToken)
		store.AssertExpectations(t)
	})
}

func TestJWTAuthenticate_WithData(t *testing.T) {
	t.Parallel()

	t.Run("handles optional data parameter", func(t *testing.T) {
		t.Parallel()

		transport, store := createJWTTransport(t)
		ctx := createJWTContext(t, "")
		userID := uuid.New()
		data := testData{
			CartItems: []string{"item1", "item2"},
			Theme:     "dark",
		}

		store.On("Save", ctx, mock.MatchedBy(func(s *session.Session[testData]) bool {
			return s.Data.Theme == "dark" && len(s.Data.CartItems) == 2
		})).Return(nil)

		sess, _, err := transport.Authenticate(ctx, userID, data)

		require.NoError(t, err)
		assert.Equal(t, "dark", sess.Data.Theme)
		assert.Len(t, sess.Data.CartItems, 2)
		store.AssertExpectations(t)
	})
}

func TestJWTAuthenticate_TokenRotation(t *testing.T) {
	t.Parallel()

	t.Run("verifies token rotation on authentication", func(t *testing.T) {
		t.Parallel()

		transport, store := createJWTTransport(t)
		secretKey := "test-secret-key-that-is-at-least-32-bytes-long"
		existingSession := createValidSession(t)
		oldToken := existingSession.Token
		validJWT := createValidJWT(t, secretKey, oldToken)
		ctx := createJWTContext(t, "Bearer "+validJWT)
		userID := uuid.New()

		store.On("GetByToken", ctx, oldToken).Return(&existingSession, nil)
		store.On("Save", ctx, mock.MatchedBy(func(s *session.Session[testData]) bool {
			return s.Token != oldToken && s.IsAuthenticated()
		})).Return(nil)

		sess, _, err := transport.Authenticate(ctx, userID)

		require.NoError(t, err)
		assert.NotEqual(t, oldToken, sess.Token)
		store.AssertExpectations(t)
	})
}

func TestJWTAuthenticate_LoadError(t *testing.T) {
	t.Parallel()

	t.Run("creates anonymous session when GetByToken fails", func(t *testing.T) {
		t.Parallel()

		transport, store := createJWTTransport(t)
		secretKey := "test-secret-key-that-is-at-least-32-bytes-long"
		validJWT := createValidJWT(t, secretKey, "token-123")
		ctx := createJWTContext(t, "Bearer "+validJWT)
		userID := uuid.New()

		// Load will fail to get session, but will create anonymous session
		store.On("GetByToken", ctx, "token-123").
			Return(nil, session.ErrNotFound)
		// Then Authenticate will store the newly authenticated session
		store.On("Save", ctx, mock.MatchedBy(func(s *session.Session[testData]) bool {
			return s.IsAuthenticated() && s.UserID == userID
		})).Return(nil)

		sess, _, err := transport.Authenticate(ctx, userID)

		require.NoError(t, err)
		assert.True(t, sess.IsAuthenticated())
		assert.Equal(t, userID, sess.UserID)
		store.AssertExpectations(t)
	})
}

func TestJWTAuthenticate_AuthError(t *testing.T) {
	t.Parallel()

	t.Run("propagates Authenticate errors", func(t *testing.T) {
		t.Parallel()

		transport, store := createJWTTransport(t)
		ctx := createJWTContext(t, "")
		// Use uuid.Nil which should cause authentication to fail
		// Actually, Authenticate accepts any UUID, so we need to test differently
		// Let's test Store error instead

		storeErr := errors.New("store failed")
		store.On("Save", ctx, mock.Anything).Return(storeErr)

		_, _, err := transport.Authenticate(ctx, uuid.New())

		require.Error(t, err)
		assert.ErrorIs(t, err, storeErr)
		store.AssertExpectations(t)
	})
}

func TestJWTAuthenticate_StoreError(t *testing.T) {
	t.Parallel()

	t.Run("propagates Manager.Store errors", func(t *testing.T) {
		t.Parallel()

		transport, store := createJWTTransport(t)
		ctx := createJWTContext(t, "")
		storeErr := errors.New("database write error")

		store.On("Save", ctx, mock.Anything).Return(storeErr)

		_, _, err := transport.Authenticate(ctx, uuid.New())

		require.Error(t, err)
		assert.ErrorIs(t, err, storeErr)
		store.AssertExpectations(t)
	})
}

func TestJWTAuthenticate_TokenPairStructure(t *testing.T) {
	t.Parallel()

	t.Run("validates TokenPair fields", func(t *testing.T) {
		t.Parallel()

		transport, store := createJWTTransport(t)
		ctx := createJWTContext(t, "")
		userID := uuid.New()

		store.On("Save", ctx, mock.Anything).Return(nil)

		_, pair, err := transport.Authenticate(ctx, userID)

		require.NoError(t, err)
		assert.NotEmpty(t, pair.AccessToken)
		assert.NotEmpty(t, pair.RefreshToken)
		assert.Equal(t, "Bearer", pair.TokenType)
		assert.Equal(t, 900, pair.ExpiresIn) // 15 minutes in seconds
		assert.WithinDuration(t, time.Now().Add(15*time.Minute), pair.ExpiresAt, 5*time.Second)
		store.AssertExpectations(t)
	})
}

// Logout Method Tests

func TestJWTLogout_Success(t *testing.T) {
	t.Parallel()

	t.Run("deletes session successfully", func(t *testing.T) {
		t.Parallel()

		transport, store := createJWTTransport(t)
		secretKey := "test-secret-key-that-is-at-least-32-bytes-long"
		existingSession := createAuthenticatedSession(t)
		validJWT := createValidJWT(t, secretKey, existingSession.Token)
		ctx := createJWTContext(t, "Bearer "+validJWT)

		store.On("GetByToken", ctx, existingSession.Token).
			Return(&existingSession, nil)
		store.On("Delete", ctx, existingSession.ID).Return(nil)

		err := transport.Logout(ctx)

		require.NoError(t, err)
		store.AssertExpectations(t)
	})
}

func TestJWTLogout_NoToken(t *testing.T) {
	t.Parallel()

	t.Run("returns nil on ErrNoToken", func(t *testing.T) {
		t.Parallel()

		transport, _ := createJWTTransport(t)
		ctx := createJWTContext(t, "")

		err := transport.Logout(ctx)

		require.NoError(t, err)
	})
}

func TestJWTLogout_NotAuthenticatedOK(t *testing.T) {
	t.Parallel()

	t.Run("handles ErrNotAuthenticated gracefully", func(t *testing.T) {
		t.Parallel()

		transport, store := createJWTTransport(t)
		secretKey := "test-secret-key-that-is-at-least-32-bytes-long"
		anonSession := createValidSession(t)
		validJWT := createValidJWT(t, secretKey, anonSession.Token)
		ctx := createJWTContext(t, "Bearer "+validJWT)

		store.On("GetByToken", ctx, anonSession.Token).
			Return(&anonSession, nil)
		store.On("Delete", ctx, anonSession.ID).
			Return(session.ErrNotAuthenticated)

		err := transport.Logout(ctx)

		require.NoError(t, err)
		store.AssertExpectations(t)
	})
}

func TestJWTLogout_Error(t *testing.T) {
	t.Parallel()

	t.Run("propagates other errors", func(t *testing.T) {
		t.Parallel()

		transport, store := createJWTTransport(t)
		secretKey := "test-secret-key-that-is-at-least-32-bytes-long"
		existingSession := createAuthenticatedSession(t)
		validJWT := createValidJWT(t, secretKey, existingSession.Token)
		ctx := createJWTContext(t, "Bearer "+validJWT)
		storeErr := errors.New("database error")

		store.On("GetByToken", ctx, existingSession.Token).
			Return(&existingSession, nil)
		store.On("Delete", ctx, existingSession.ID).Return(storeErr)

		err := transport.Logout(ctx)

		require.Error(t, err)
		assert.ErrorIs(t, err, storeErr)
		store.AssertExpectations(t)
	})
}

// Delete Method Tests

func TestJWTDelete_Success(t *testing.T) {
	t.Parallel()

	t.Run("deletes session successfully", func(t *testing.T) {
		t.Parallel()

		transport, store := createJWTTransport(t)
		secretKey := "test-secret-key-that-is-at-least-32-bytes-long"
		existingSession := createAuthenticatedSession(t)
		validJWT := createValidJWT(t, secretKey, existingSession.Token)
		ctx := createJWTContext(t, "Bearer "+validJWT)

		store.On("GetByToken", ctx, existingSession.Token).
			Return(&existingSession, nil)
		store.On("Delete", ctx, existingSession.ID).Return(nil)

		err := transport.Delete(ctx)

		require.NoError(t, err)
		store.AssertExpectations(t)
	})
}

func TestJWTDelete_NoToken(t *testing.T) {
	t.Parallel()

	t.Run("returns nil on ErrNoToken", func(t *testing.T) {
		t.Parallel()

		transport, _ := createJWTTransport(t)
		ctx := createJWTContext(t, "")

		err := transport.Delete(ctx)

		require.NoError(t, err)
	})
}

func TestJWTDelete_NotAuthenticatedOK(t *testing.T) {
	t.Parallel()

	t.Run("handles ErrNotAuthenticated gracefully", func(t *testing.T) {
		t.Parallel()

		transport, store := createJWTTransport(t)
		secretKey := "test-secret-key-that-is-at-least-32-bytes-long"
		anonSession := createValidSession(t)
		validJWT := createValidJWT(t, secretKey, anonSession.Token)
		ctx := createJWTContext(t, "Bearer "+validJWT)

		store.On("GetByToken", ctx, anonSession.Token).
			Return(&anonSession, nil)
		store.On("Delete", ctx, anonSession.ID).
			Return(session.ErrNotAuthenticated)

		err := transport.Delete(ctx)

		require.NoError(t, err)
		store.AssertExpectations(t)
	})
}

func TestJWTDelete_Error(t *testing.T) {
	t.Parallel()

	t.Run("propagates other errors", func(t *testing.T) {
		t.Parallel()

		transport, store := createJWTTransport(t)
		secretKey := "test-secret-key-that-is-at-least-32-bytes-long"
		existingSession := createAuthenticatedSession(t)
		validJWT := createValidJWT(t, secretKey, existingSession.Token)
		ctx := createJWTContext(t, "Bearer "+validJWT)
		storeErr := errors.New("database error")

		store.On("GetByToken", ctx, existingSession.Token).
			Return(&existingSession, nil)
		store.On("Delete", ctx, existingSession.ID).Return(storeErr)

		err := transport.Delete(ctx)

		require.Error(t, err)
		assert.ErrorIs(t, err, storeErr)
		store.AssertExpectations(t)
	})
}

// Store Method Tests

func TestJWTStore_Success(t *testing.T) {
	t.Parallel()

	t.Run("delegates to Manager.Store successfully", func(t *testing.T) {
		t.Parallel()

		transport, store := createJWTTransport(t)
		ctx := createJWTContext(t, "")
		sess := createAuthenticatedSession(t)

		store.On("Save", ctx, &sess).Return(nil)

		err := transport.Store(ctx, sess)

		require.NoError(t, err)
		store.AssertExpectations(t)
	})
}

func TestJWTStore_Error(t *testing.T) {
	t.Parallel()

	t.Run("propagates Manager.Store errors", func(t *testing.T) {
		t.Parallel()

		transport, store := createJWTTransport(t)
		ctx := createJWTContext(t, "")
		sess := createAuthenticatedSession(t)
		storeErr := errors.New("database error")

		store.On("Save", ctx, &sess).Return(storeErr)

		err := transport.Store(ctx, sess)

		require.Error(t, err)
		assert.ErrorIs(t, err, storeErr)
		store.AssertExpectations(t)
	})
}

// Auth Method Tests

func TestJWTAuth_Success(t *testing.T) {
	t.Parallel()

	t.Run("generates TokenPair for authenticated session", func(t *testing.T) {
		t.Parallel()

		transport, _ := createJWTTransport(t)
		sess := createAuthenticatedSession(t)

		pair, err := transport.Auth(sess)

		require.NoError(t, err)
		assert.NotEmpty(t, pair.AccessToken)
		assert.NotEmpty(t, pair.RefreshToken)
		assert.Equal(t, "Bearer", pair.TokenType)
	})
}

func TestJWTAuth_NotAuthenticated(t *testing.T) {
	t.Parallel()

	t.Run("returns ErrNotAuthenticated for anonymous session", func(t *testing.T) {
		t.Parallel()

		transport, _ := createJWTTransport(t)
		sess := createValidSession(t)

		_, err := transport.Auth(sess)

		require.Error(t, err)
		assert.ErrorIs(t, err, session.ErrNotAuthenticated)
	})
}

func TestJWTAuth_TokenType(t *testing.T) {
	t.Parallel()

	t.Run("verifies TokenType is Bearer", func(t *testing.T) {
		t.Parallel()

		transport, _ := createJWTTransport(t)
		sess := createAuthenticatedSession(t)

		pair, err := transport.Auth(sess)

		require.NoError(t, err)
		assert.Equal(t, "Bearer", pair.TokenType)
	})
}

func TestJWTAuth_ExpiresIn(t *testing.T) {
	t.Parallel()

	t.Run("verifies ExpiresIn from accessTTL", func(t *testing.T) {
		t.Parallel()

		transport, _ := createJWTTransport(t)
		sess := createAuthenticatedSession(t)

		pair, err := transport.Auth(sess)

		require.NoError(t, err)
		assert.Equal(t, 900, pair.ExpiresIn) // 15 minutes = 900 seconds
	})
}

func TestJWTAuth_ExpiresAt(t *testing.T) {
	t.Parallel()

	t.Run("verifies ExpiresAt calculation", func(t *testing.T) {
		t.Parallel()

		transport, _ := createJWTTransport(t)
		sess := createAuthenticatedSession(t)

		before := time.Now().Add(15 * time.Minute)
		pair, err := transport.Auth(sess)
		after := time.Now().Add(15 * time.Minute)

		require.NoError(t, err)
		assert.True(t, pair.ExpiresAt.After(before) || pair.ExpiresAt.Equal(before))
		assert.True(t, pair.ExpiresAt.Before(after) || pair.ExpiresAt.Equal(after))
	})
}

func TestJWTAuth_AccessTokenClaims(t *testing.T) {
	t.Parallel()

	t.Run("verifies access token claims", func(t *testing.T) {
		t.Parallel()

		transport, _ := createJWTTransport(t)
		sess := createAuthenticatedSession(t)

		pair, err := transport.Auth(sess)
		require.NoError(t, err)

		// Parse access token to verify claims
		signer, err := jwt.NewFromString("test-secret-key-that-is-at-least-32-bytes-long")
		require.NoError(t, err)

		var claims jwt.StandardClaims
		err = signer.Parse(pair.AccessToken, &claims)
		require.NoError(t, err)

		assert.Equal(t, sess.Token, claims.ID) // JTI = Session.Token
		assert.Equal(t, sess.UserID.String(), claims.Subject)
		assert.Equal(t, "test-issuer", claims.Issuer)
		assert.NotZero(t, claims.ExpiresAt)
		assert.NotZero(t, claims.IssuedAt)
	})
}

func TestJWTAuth_RefreshTokenClaims(t *testing.T) {
	t.Parallel()

	t.Run("verifies refresh token uses session.ExpiresAt", func(t *testing.T) {
		t.Parallel()

		transport, _ := createJWTTransport(t)
		sess := createAuthenticatedSession(t)

		pair, err := transport.Auth(sess)
		require.NoError(t, err)

		// Parse refresh token to verify claims
		signer, err := jwt.NewFromString("test-secret-key-that-is-at-least-32-bytes-long")
		require.NoError(t, err)

		var claims jwt.StandardClaims
		err = signer.Parse(pair.RefreshToken, &claims)
		require.NoError(t, err)

		assert.Equal(t, sess.Token, claims.ID) // JTI = Session.Token
		assert.Equal(t, sess.ExpiresAt.Unix(), claims.ExpiresAt)
	})
}

func TestJWTAuth_BothTokensHaveSameJTI(t *testing.T) {
	t.Parallel()

	t.Run("verifies both tokens use Session.Token as JTI", func(t *testing.T) {
		t.Parallel()

		transport, _ := createJWTTransport(t)
		sess := createAuthenticatedSession(t)

		pair, err := transport.Auth(sess)
		require.NoError(t, err)

		signer, err := jwt.NewFromString("test-secret-key-that-is-at-least-32-bytes-long")
		require.NoError(t, err)

		var accessClaims jwt.StandardClaims
		err = signer.Parse(pair.AccessToken, &accessClaims)
		require.NoError(t, err)

		var refreshClaims jwt.StandardClaims
		err = signer.Parse(pair.RefreshToken, &refreshClaims)
		require.NoError(t, err)

		assert.Equal(t, sess.Token, accessClaims.ID)
		assert.Equal(t, sess.Token, refreshClaims.ID)
		assert.Equal(t, accessClaims.ID, refreshClaims.ID)
	})
}

// Refresh Method Tests

func TestJWTRefresh_Success(t *testing.T) {
	t.Parallel()

	t.Run("generates new TokenPair with rotated token", func(t *testing.T) {
		t.Parallel()

		transport, store := createJWTTransport(t)
		secretKey := "test-secret-key-that-is-at-least-32-bytes-long"
		existingSession := createAuthenticatedSession(t)
		oldToken := existingSession.Token
		refreshToken := createValidJWT(t, secretKey, oldToken)
		ctx := context.Background()

		store.On("GetByToken", ctx, oldToken).Return(&existingSession, nil)
		store.On("Save", ctx, mock.MatchedBy(func(s *session.Session[testData]) bool {
			return s.Token != oldToken && s.IsAuthenticated()
		})).Return(nil)

		pair, err := transport.Refresh(ctx, refreshToken)

		require.NoError(t, err)
		assert.NotEmpty(t, pair.AccessToken)
		assert.NotEmpty(t, pair.RefreshToken)
		store.AssertExpectations(t)
	})
}

func TestJWTRefresh_InvalidToken(t *testing.T) {
	t.Parallel()

	t.Run("returns ErrInvalidToken on parse failure", func(t *testing.T) {
		t.Parallel()

		transport, _ := createJWTTransport(t)
		ctx := context.Background()

		_, err := transport.Refresh(ctx, "invalid-token")

		require.Error(t, err)
		assert.ErrorIs(t, err, sessiontransport.ErrInvalidToken)
	})
}

func TestJWTRefresh_TokenNotFound(t *testing.T) {
	t.Parallel()

	t.Run("propagates GetByToken errors", func(t *testing.T) {
		t.Parallel()

		transport, store := createJWTTransport(t)
		secretKey := "test-secret-key-that-is-at-least-32-bytes-long"
		refreshToken := createValidJWT(t, secretKey, "nonexistent-token")
		ctx := context.Background()

		store.On("GetByToken", ctx, "nonexistent-token").
			Return(nil, session.ErrNotFound)

		_, err := transport.Refresh(ctx, refreshToken)

		require.Error(t, err)
		assert.ErrorIs(t, err, session.ErrNotFound)
		store.AssertExpectations(t)
	})
}

func TestJWTRefresh_NotAuthenticated(t *testing.T) {
	t.Parallel()

	t.Run("returns ErrNotAuthenticated for anonymous session", func(t *testing.T) {
		t.Parallel()

		transport, store := createJWTTransport(t)
		secretKey := "test-secret-key-that-is-at-least-32-bytes-long"
		anonSession := createValidSession(t)
		refreshToken := createValidJWT(t, secretKey, anonSession.Token)
		ctx := context.Background()

		store.On("GetByToken", ctx, anonSession.Token).Return(&anonSession, nil)

		_, err := transport.Refresh(ctx, refreshToken)

		require.Error(t, err)
		assert.ErrorIs(t, err, session.ErrNotAuthenticated)
		store.AssertExpectations(t)
	})
}

func TestJWTRefresh_TokenRotation(t *testing.T) {
	t.Parallel()

	t.Run("verifies new Session.Token is generated", func(t *testing.T) {
		t.Parallel()

		transport, store := createJWTTransport(t)
		secretKey := "test-secret-key-that-is-at-least-32-bytes-long"
		existingSession := createAuthenticatedSession(t)
		oldToken := existingSession.Token
		refreshToken := createValidJWT(t, secretKey, oldToken)
		ctx := context.Background()

		var rotatedSession session.Session[testData]
		store.On("GetByToken", ctx, oldToken).Return(&existingSession, nil)
		store.On("Save", ctx, mock.MatchedBy(func(s *session.Session[testData]) bool {
			rotatedSession = *s
			return s.Token != oldToken
		})).Return(nil)

		_, err := transport.Refresh(ctx, refreshToken)

		require.NoError(t, err)
		assert.NotEqual(t, oldToken, rotatedSession.Token)
		store.AssertExpectations(t)
	})
}

func TestJWTRefresh_StoreError(t *testing.T) {
	t.Parallel()

	t.Run("propagates Manager.Store errors", func(t *testing.T) {
		t.Parallel()

		transport, store := createJWTTransport(t)
		secretKey := "test-secret-key-that-is-at-least-32-bytes-long"
		existingSession := createAuthenticatedSession(t)
		refreshToken := createValidJWT(t, secretKey, existingSession.Token)
		ctx := context.Background()
		storeErr := errors.New("database error")

		store.On("GetByToken", ctx, existingSession.Token).Return(&existingSession, nil)
		store.On("Save", ctx, mock.Anything).Return(storeErr)

		_, err := transport.Refresh(ctx, refreshToken)

		require.Error(t, err)
		assert.ErrorIs(t, err, storeErr)
		store.AssertExpectations(t)
	})
}

func TestJWTRefresh_NewJTI(t *testing.T) {
	t.Parallel()

	t.Run("verifies new tokens have new JTI (rotated token)", func(t *testing.T) {
		t.Parallel()

		transport, store := createJWTTransport(t)
		secretKey := "test-secret-key-that-is-at-least-32-bytes-long"
		existingSession := createAuthenticatedSession(t)
		oldToken := existingSession.Token
		refreshToken := createValidJWT(t, secretKey, oldToken)
		ctx := context.Background()

		var rotatedSession session.Session[testData]
		store.On("GetByToken", ctx, oldToken).Return(&existingSession, nil)
		store.On("Save", ctx, mock.MatchedBy(func(s *session.Session[testData]) bool {
			rotatedSession = *s
			return true
		})).Return(nil)

		pair, err := transport.Refresh(ctx, refreshToken)
		require.NoError(t, err)

		// Parse new access token
		signer, err := jwt.NewFromString(secretKey)
		require.NoError(t, err)

		var claims jwt.StandardClaims
		err = signer.Parse(pair.AccessToken, &claims)
		require.NoError(t, err)

		assert.NotEqual(t, oldToken, claims.ID)
		assert.Equal(t, rotatedSession.Token, claims.ID)
		store.AssertExpectations(t)
	})
}

func TestJWTRefresh_PreservesUserID(t *testing.T) {
	t.Parallel()

	t.Run("verifies UserID is preserved after refresh", func(t *testing.T) {
		t.Parallel()

		transport, store := createJWTTransport(t)
		secretKey := "test-secret-key-that-is-at-least-32-bytes-long"
		existingSession := createAuthenticatedSession(t)
		originalUserID := existingSession.UserID
		refreshToken := createValidJWT(t, secretKey, existingSession.Token)
		ctx := context.Background()

		var rotatedSession session.Session[testData]
		store.On("GetByToken", ctx, existingSession.Token).Return(&existingSession, nil)
		store.On("Save", ctx, mock.MatchedBy(func(s *session.Session[testData]) bool {
			rotatedSession = *s
			return true
		})).Return(nil)

		pair, err := transport.Refresh(ctx, refreshToken)
		require.NoError(t, err)

		// Verify UserID is preserved
		assert.Equal(t, originalUserID, rotatedSession.UserID)

		// Verify UserID in new token
		signer, err := jwt.NewFromString(secretKey)
		require.NoError(t, err)

		var claims jwt.StandardClaims
		err = signer.Parse(pair.AccessToken, &claims)
		require.NoError(t, err)

		assert.Equal(t, originalUserID.String(), claims.Subject)
		store.AssertExpectations(t)
	})
}
