package session_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dmitrymomot/foundation/core/session"
)

type testData struct {
	CartItems []string
	Theme     string
}

// Factory method tests

func TestNew_Success(t *testing.T) {
	params := session.NewSessionParams{
		IP:          "192.168.1.1",
		Fingerprint: "test-fingerprint",
		UserAgent:   "Mozilla/5.0",
	}
	ttl := time.Hour

	sess, err := session.New[testData](params, ttl)

	require.NoError(t, err)
	assert.NotEqual(t, uuid.Nil, sess.ID)
	assert.NotEmpty(t, sess.Token)
	assert.Equal(t, uuid.Nil, sess.UserID)
	assert.Equal(t, params.IP, sess.IP)
	assert.Equal(t, params.Fingerprint, sess.Fingerprint)
	assert.Equal(t, params.UserAgent, sess.UserAgent)
	assert.True(t, sess.IsModified())
	assert.False(t, sess.IsAuthenticated())
	assert.False(t, sess.IsDeleted())
	assert.False(t, sess.IsExpired())
	assert.WithinDuration(t, time.Now().Add(ttl), sess.ExpiresAt, time.Second)
}

func TestNew_MissingIP(t *testing.T) {
	params := session.NewSessionParams{
		Fingerprint: "test-fingerprint",
		UserAgent:   "Mozilla/5.0",
	}

	_, err := session.New[testData](params, time.Hour)

	require.Error(t, err)
	assert.ErrorIs(t, err, session.ErrMissingIP)
}

func TestNew_OptionalFields(t *testing.T) {
	params := session.NewSessionParams{
		IP: "192.168.1.1",
		// Fingerprint and UserAgent omitted
	}

	sess, err := session.New[testData](params, time.Hour)

	require.NoError(t, err)
	assert.Empty(t, sess.Fingerprint)
	assert.Empty(t, sess.UserAgent)
}

// Authenticate tests

func TestAuthenticate_Success(t *testing.T) {
	sess, err := session.New[testData](session.NewSessionParams{IP: "127.0.0.1"}, time.Hour)
	require.NoError(t, err)

	originalToken := sess.Token
	userID := uuid.New()

	err = sess.Authenticate(userID)

	require.NoError(t, err)
	assert.Equal(t, userID, sess.UserID)
	assert.True(t, sess.IsAuthenticated())
	assert.True(t, sess.IsModified())
	assert.NotEqual(t, originalToken, sess.Token, "Token should be rotated")
}

func TestAuthenticate_WithData(t *testing.T) {
	sess, err := session.New[testData](session.NewSessionParams{IP: "127.0.0.1"}, time.Hour)
	require.NoError(t, err)

	userID := uuid.New()
	data := testData{
		CartItems: []string{"item1", "item2"},
		Theme:     "dark",
	}

	err = sess.Authenticate(userID, data)

	require.NoError(t, err)
	assert.Equal(t, userID, sess.UserID)
	assert.Equal(t, data, sess.Data)
	assert.True(t, sess.IsModified())
}

// Refresh tests

func TestRefresh_Success(t *testing.T) {
	sess, err := session.New[testData](session.NewSessionParams{IP: "127.0.0.1"}, time.Hour)
	require.NoError(t, err)

	originalToken := sess.Token

	err = sess.Refresh()

	require.NoError(t, err)
	assert.NotEqual(t, originalToken, sess.Token, "Token should be rotated")
	assert.True(t, sess.IsModified())
}

func TestRefresh_PreservesAuthState(t *testing.T) {
	sess, err := session.New[testData](session.NewSessionParams{IP: "127.0.0.1"}, time.Hour)
	require.NoError(t, err)

	userID := uuid.New()
	data := testData{Theme: "light"}
	err = sess.Authenticate(userID, data)
	require.NoError(t, err)

	err = sess.Refresh()

	require.NoError(t, err)
	assert.Equal(t, userID, sess.UserID, "UserID should be preserved")
	assert.Equal(t, data, sess.Data, "Data should be preserved")
	assert.True(t, sess.IsAuthenticated())
}

// Logout tests

func TestLogout_Success(t *testing.T) {
	sess, err := session.New[testData](session.NewSessionParams{IP: "127.0.0.1"}, time.Hour)
	require.NoError(t, err)

	sess.Logout()

	assert.True(t, sess.IsDeleted())
	assert.True(t, sess.IsModified())
	assert.NotZero(t, sess.DeletedAt)
}

func TestLogout_AuthenticatedSession(t *testing.T) {
	sess, err := session.New[testData](session.NewSessionParams{IP: "127.0.0.1"}, time.Hour)
	require.NoError(t, err)

	userID := uuid.New()
	err = sess.Authenticate(userID)
	require.NoError(t, err)

	sess.Logout()

	assert.True(t, sess.IsDeleted())
	assert.Equal(t, userID, sess.UserID, "UserID preserved after logout")
	assert.True(t, sess.IsAuthenticated(), "Still marked as authenticated")
}

// SetData tests

func TestSetData_Success(t *testing.T) {
	sess, err := session.New[testData](session.NewSessionParams{IP: "127.0.0.1"}, time.Hour)
	require.NoError(t, err)

	data := testData{
		CartItems: []string{"product1"},
		Theme:     "dark",
	}

	sess.SetData(data)

	assert.Equal(t, data, sess.Data)
	assert.True(t, sess.IsModified())
}

func TestSetData_OverwriteExisting(t *testing.T) {
	sess, err := session.New[testData](session.NewSessionParams{IP: "127.0.0.1"}, time.Hour)
	require.NoError(t, err)

	firstData := testData{Theme: "light"}
	sess.SetData(firstData)

	secondData := testData{Theme: "dark", CartItems: []string{"item1"}}
	sess.SetData(secondData)

	assert.Equal(t, secondData, sess.Data)
	assert.NotEqual(t, firstData, sess.Data)
}

// Touch tests

func TestTouch_IntervalElapsed(t *testing.T) {
	sess, err := session.New[testData](session.NewSessionParams{IP: "127.0.0.1"}, time.Hour)
	require.NoError(t, err)

	// Simulate old UpdatedAt
	sess.UpdatedAt = time.Now().Add(-10 * time.Minute)
	originalExpiry := sess.ExpiresAt

	sess.Touch(time.Hour, 5*time.Minute)

	assert.True(t, sess.IsModified())
	assert.NotEqual(t, originalExpiry, sess.ExpiresAt)
	assert.WithinDuration(t, time.Now().Add(time.Hour), sess.ExpiresAt, time.Second)
}

func TestTouch_IntervalNotElapsed(t *testing.T) {
	sess, err := session.New[testData](session.NewSessionParams{IP: "127.0.0.1"}, time.Hour)
	require.NoError(t, err)

	originalExpiry := sess.ExpiresAt
	originalUpdated := sess.UpdatedAt

	// Touch immediately (interval not elapsed)
	sess.Touch(time.Hour, 5*time.Minute)

	assert.Equal(t, originalExpiry, sess.ExpiresAt, "Should not extend expiry")
	assert.Equal(t, originalUpdated, sess.UpdatedAt, "Should not update timestamp")
}

func TestTouch_MultipleSequential(t *testing.T) {
	sess, err := session.New[testData](session.NewSessionParams{IP: "127.0.0.1"}, time.Hour)
	require.NoError(t, err)

	// First touch (interval elapsed)
	sess.UpdatedAt = time.Now().Add(-6 * time.Minute)
	sess.Touch(time.Hour, 5*time.Minute)
	firstExpiry := sess.ExpiresAt

	// Second touch (interval not elapsed)
	sess.Touch(time.Hour, 5*time.Minute)
	assert.Equal(t, firstExpiry, sess.ExpiresAt, "Should not touch again")

	// Third touch (interval elapsed)
	sess.UpdatedAt = time.Now().Add(-6 * time.Minute)
	sess.Touch(time.Hour, 5*time.Minute)
	assert.NotEqual(t, firstExpiry, sess.ExpiresAt, "Should touch again")
}

// IsAuthenticated tests

func TestIsAuthenticated_Anonymous(t *testing.T) {
	sess, err := session.New[testData](session.NewSessionParams{IP: "127.0.0.1"}, time.Hour)
	require.NoError(t, err)

	assert.False(t, sess.IsAuthenticated())
}

func TestIsAuthenticated_Authenticated(t *testing.T) {
	sess, err := session.New[testData](session.NewSessionParams{IP: "127.0.0.1"}, time.Hour)
	require.NoError(t, err)

	err = sess.Authenticate(uuid.New())
	require.NoError(t, err)

	assert.True(t, sess.IsAuthenticated())
}

// IsDeleted tests

func TestIsDeleted_NotDeleted(t *testing.T) {
	sess, err := session.New[testData](session.NewSessionParams{IP: "127.0.0.1"}, time.Hour)
	require.NoError(t, err)

	assert.False(t, sess.IsDeleted())
}

func TestIsDeleted_Deleted(t *testing.T) {
	sess, err := session.New[testData](session.NewSessionParams{IP: "127.0.0.1"}, time.Hour)
	require.NoError(t, err)

	sess.Logout()

	assert.True(t, sess.IsDeleted())
}

// IsModified tests

func TestIsModified_NewSession(t *testing.T) {
	sess, err := session.New[testData](session.NewSessionParams{IP: "127.0.0.1"}, time.Hour)
	require.NoError(t, err)

	assert.True(t, sess.IsModified())
}

// IsExpired tests

func TestIsExpired_NotExpired(t *testing.T) {
	sess, err := session.New[testData](session.NewSessionParams{IP: "127.0.0.1"}, time.Hour)
	require.NoError(t, err)

	assert.False(t, sess.IsExpired())
}

func TestIsExpired_Expired(t *testing.T) {
	sess, err := session.New[testData](session.NewSessionParams{IP: "127.0.0.1"}, time.Hour)
	require.NoError(t, err)

	// Simulate expiration
	sess.ExpiresAt = time.Now().Add(-time.Minute)

	assert.True(t, sess.IsExpired())
}

func TestIsExpired_EdgeCase(t *testing.T) {
	sess, err := session.New[testData](session.NewSessionParams{IP: "127.0.0.1"}, time.Millisecond)
	require.NoError(t, err)

	time.Sleep(2 * time.Millisecond)

	assert.True(t, sess.IsExpired())
}

// Complex flow tests

func TestComplexFlow_AuthenticateThenSetData(t *testing.T) {
	sess, err := session.New[testData](session.NewSessionParams{IP: "127.0.0.1"}, time.Hour)
	require.NoError(t, err)

	userID := uuid.New()
	err = sess.Authenticate(userID)
	require.NoError(t, err)

	data := testData{CartItems: []string{"item1"}}
	sess.SetData(data)

	assert.Equal(t, userID, sess.UserID)
	assert.Equal(t, data, sess.Data)
	assert.True(t, sess.IsAuthenticated())
	assert.True(t, sess.IsModified())
}

func TestComplexFlow_AuthenticateRefreshLogout(t *testing.T) {
	sess, err := session.New[testData](session.NewSessionParams{IP: "127.0.0.1"}, time.Hour)
	require.NoError(t, err)

	// Authenticate
	userID := uuid.New()
	err = sess.Authenticate(userID)
	require.NoError(t, err)
	tokenAfterAuth := sess.Token

	// Refresh
	err = sess.Refresh()
	require.NoError(t, err)
	assert.NotEqual(t, tokenAfterAuth, sess.Token)
	assert.Equal(t, userID, sess.UserID)

	// Logout
	sess.Logout()
	assert.True(t, sess.IsDeleted())
	assert.True(t, sess.IsAuthenticated(), "Still authenticated after logout")
}

func TestComplexFlow_MultipleTouches(t *testing.T) {
	sess, err := session.New[testData](session.NewSessionParams{IP: "127.0.0.1"}, time.Hour)
	require.NoError(t, err)

	touchInterval := 100 * time.Millisecond

	// First touch (should not extend - just created)
	sess.Touch(time.Hour, touchInterval)
	firstExpiry := sess.ExpiresAt

	// Wait and touch again
	time.Sleep(150 * time.Millisecond)
	sess.Touch(time.Hour, touchInterval)
	secondExpiry := sess.ExpiresAt

	assert.NotEqual(t, firstExpiry, secondExpiry, "Should extend on second touch")
	assert.True(t, secondExpiry.After(firstExpiry))
}

func TestTokenRotation_UniqueTokens(t *testing.T) {
	sess, err := session.New[testData](session.NewSessionParams{IP: "127.0.0.1"}, time.Hour)
	require.NoError(t, err)

	tokens := map[string]bool{sess.Token: true}

	// Authenticate (rotates)
	err = sess.Authenticate(uuid.New())
	require.NoError(t, err)
	assert.NotContains(t, tokens, sess.Token, "Token should be unique")
	tokens[sess.Token] = true

	// Refresh (rotates)
	err = sess.Refresh()
	require.NoError(t, err)
	assert.NotContains(t, tokens, sess.Token, "Token should be unique")
}
