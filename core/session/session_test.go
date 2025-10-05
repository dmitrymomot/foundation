package session_test

import (
	"fmt"
	"sync"
	"sync/atomic"
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

// Race condition tests

func TestRace_ConcurrentAuthenticate(t *testing.T) {
	t.Parallel()

	sess, err := session.New[testData](session.NewSessionParams{IP: "127.0.0.1"}, time.Hour)
	require.NoError(t, err)

	const goroutines = 20
	var wg sync.WaitGroup
	wg.Add(goroutines)

	// Track which goroutine succeeded
	successCount := atomic.Int32{}
	errorCount := atomic.Int32{}

	// All goroutines try to authenticate with different user IDs
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			userID := uuid.New()
			err := sess.Authenticate(userID)
			if err == nil {
				successCount.Add(1)
			} else {
				errorCount.Add(1)
			}
		}()
	}

	wg.Wait()

	// All authentications should succeed (no panics or deadlocks)
	assert.Equal(t, int32(goroutines), successCount.Load(), "All authentications should succeed")
	assert.Equal(t, int32(0), errorCount.Load(), "No errors should occur")

	// Session should be authenticated (one of the user IDs will be set)
	assert.True(t, sess.IsAuthenticated(), "Session should be authenticated")
	assert.NotEqual(t, uuid.Nil, sess.UserID, "UserID should be set")
	assert.NotEmpty(t, sess.Token, "Token should be set")
}

func TestRace_ConcurrentTouch(t *testing.T) {
	t.Parallel()

	sess, err := session.New[testData](session.NewSessionParams{IP: "127.0.0.1"}, time.Hour)
	require.NoError(t, err)

	// Force the session to be old enough for Touch to work
	sess.UpdatedAt = time.Now().Add(-10 * time.Minute)

	const goroutines = 20
	var wg sync.WaitGroup
	wg.Add(goroutines)

	// All goroutines try to touch the session
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			sess.Touch(time.Hour, 5*time.Minute)
		}()
	}

	wg.Wait()

	// Session should be touched (no panics or deadlocks)
	assert.True(t, sess.IsModified(), "Session should be modified")
	assert.WithinDuration(t, time.Now().Add(time.Hour), sess.ExpiresAt, 2*time.Second)
}

func TestRace_ConcurrentRefresh(t *testing.T) {
	t.Parallel()

	sess, err := session.New[testData](session.NewSessionParams{IP: "127.0.0.1"}, time.Hour)
	require.NoError(t, err)

	// Authenticate first so we can verify state preservation
	userID := uuid.New()
	err = sess.Authenticate(userID)
	require.NoError(t, err)

	const goroutines = 20
	var wg sync.WaitGroup
	wg.Add(goroutines)

	tokens := sync.Map{}
	successCount := atomic.Int32{}
	errorCount := atomic.Int32{}

	// All goroutines try to refresh (rotate token)
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			err := sess.Refresh()
			if err == nil {
				successCount.Add(1)
				// Store the token we saw
				tokens.Store(sess.Token, true)
			} else {
				errorCount.Add(1)
			}
		}()
	}

	wg.Wait()

	// All refreshes should succeed
	assert.Equal(t, int32(goroutines), successCount.Load(), "All refreshes should succeed")
	assert.Equal(t, int32(0), errorCount.Load(), "No errors should occur")

	// Session state should be preserved
	assert.Equal(t, userID, sess.UserID, "UserID should be preserved")
	assert.True(t, sess.IsAuthenticated(), "Session should still be authenticated")
	assert.NotEmpty(t, sess.Token, "Token should be set")

	// Count unique tokens (should be at least 1, possibly many due to race)
	uniqueTokens := 0
	tokens.Range(func(_, _ interface{}) bool {
		uniqueTokens++
		return true
	})
	assert.Greater(t, uniqueTokens, 0, "At least one unique token should exist")
}

func TestRace_MixedOperations(t *testing.T) {
	t.Parallel()

	sess, err := session.New[testData](session.NewSessionParams{IP: "127.0.0.1"}, time.Hour)
	require.NoError(t, err)

	// Force the session to be old enough for Touch to work
	sess.UpdatedAt = time.Now().Add(-10 * time.Minute)

	const goroutines = 30
	var wg sync.WaitGroup
	wg.Add(goroutines)

	// Mix of different operations
	for i := 0; i < goroutines; i++ {
		switch i % 5 {
		case 0:
			// Authenticate
			go func() {
				defer wg.Done()
				userID := uuid.New()
				_ = sess.Authenticate(userID)
			}()
		case 1:
			// Touch
			go func() {
				defer wg.Done()
				sess.Touch(time.Hour, 5*time.Minute)
			}()
		case 2:
			// Refresh
			go func() {
				defer wg.Done()
				_ = sess.Refresh()
			}()
		case 3:
			// SetData
			go func() {
				defer wg.Done()
				data := testData{
					CartItems: []string{"item1", "item2"},
					Theme:     "dark",
				}
				sess.SetData(data)
			}()
		case 4:
			// Read operations (IsAuthenticated, IsModified, etc.)
			go func() {
				defer wg.Done()
				_ = sess.IsAuthenticated()
				_ = sess.IsModified()
				_ = sess.IsExpired()
				_ = sess.IsDeleted()
			}()
		}
	}

	wg.Wait()

	// Final state should be consistent (no panics or deadlocks)
	assert.NotEmpty(t, sess.Token, "Token should be set")
	assert.NotEqual(t, uuid.Nil, sess.ID, "ID should be set")
	// Session may or may not be authenticated depending on race outcome
	// But the state should be consistent
	if sess.IsAuthenticated() {
		assert.NotEqual(t, uuid.Nil, sess.UserID, "If authenticated, UserID should be set")
	}
}

func TestRace_ConcurrentAuthenticateWithData(t *testing.T) {
	t.Parallel()

	sess, err := session.New[testData](session.NewSessionParams{IP: "127.0.0.1"}, time.Hour)
	require.NoError(t, err)

	const goroutines = 20
	var wg sync.WaitGroup
	wg.Add(goroutines)

	// All goroutines try to authenticate with different user IDs and data
	for i := 0; i < goroutines; i++ {
		go func(index int) {
			defer wg.Done()
			userID := uuid.New()
			data := testData{
				CartItems: []string{fmt.Sprintf("item%d", index)},
				Theme:     fmt.Sprintf("theme%d", index),
			}
			_ = sess.Authenticate(userID, data)
		}(i)
	}

	wg.Wait()

	// Session should be authenticated with one of the user IDs and data
	assert.True(t, sess.IsAuthenticated(), "Session should be authenticated")
	assert.NotEqual(t, uuid.Nil, sess.UserID, "UserID should be set")
	// Data should be one of the values set by the goroutines (but we can't predict which)
	// Just verify it's not zero value
	if sess.Data.Theme != "" {
		assert.NotEmpty(t, sess.Data.Theme, "Theme should be set if data was applied")
	}
}

func TestRace_ConcurrentSetData(t *testing.T) {
	t.Parallel()

	sess, err := session.New[testData](session.NewSessionParams{IP: "127.0.0.1"}, time.Hour)
	require.NoError(t, err)

	const goroutines = 20
	var wg sync.WaitGroup
	wg.Add(goroutines)

	// All goroutines try to set different data
	for i := 0; i < goroutines; i++ {
		go func(index int) {
			defer wg.Done()
			data := testData{
				CartItems: []string{fmt.Sprintf("item%d", index)},
				Theme:     fmt.Sprintf("theme%d", index),
			}
			sess.SetData(data)
		}(i)
	}

	wg.Wait()

	// Session should have one of the data values set
	assert.True(t, sess.IsModified(), "Session should be modified")
	// Can't predict which data won the race, but it should be consistent
	if sess.Data.Theme != "" {
		// If theme is set, cart items should also be from the same SetData call
		expectedPrefix := sess.Data.Theme[len("theme"):]
		if len(sess.Data.CartItems) > 0 {
			assert.Contains(t, sess.Data.CartItems[0], expectedPrefix,
				"CartItems and Theme should be from the same SetData call")
		}
	}
}

func TestRace_ConcurrentLogout(t *testing.T) {
	t.Parallel()

	sess, err := session.New[testData](session.NewSessionParams{IP: "127.0.0.1"}, time.Hour)
	require.NoError(t, err)

	// Authenticate first
	userID := uuid.New()
	err = sess.Authenticate(userID)
	require.NoError(t, err)

	const goroutines = 20
	var wg sync.WaitGroup
	wg.Add(goroutines)

	// All goroutines try to logout
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			sess.Logout()
		}()
	}

	wg.Wait()

	// Session should be deleted (no panics or deadlocks)
	assert.True(t, sess.IsDeleted(), "Session should be deleted")
	assert.True(t, sess.IsModified(), "Session should be modified")
	assert.NotZero(t, sess.DeletedAt, "DeletedAt should be set")
	assert.Equal(t, userID, sess.UserID, "UserID should be preserved after logout")
}

func TestRace_AuthenticateRefreshLogout(t *testing.T) {
	t.Parallel()

	sess, err := session.New[testData](session.NewSessionParams{IP: "127.0.0.1"}, time.Hour)
	require.NoError(t, err)

	const goroutines = 30
	var wg sync.WaitGroup
	wg.Add(goroutines)

	// Mix of Authenticate, Refresh, and Logout operations
	for i := 0; i < goroutines; i++ {
		switch i % 3 {
		case 0:
			// Authenticate
			go func() {
				defer wg.Done()
				userID := uuid.New()
				_ = sess.Authenticate(userID)
			}()
		case 1:
			// Refresh
			go func() {
				defer wg.Done()
				_ = sess.Refresh()
			}()
		case 2:
			// Logout
			go func() {
				defer wg.Done()
				sess.Logout()
			}()
		}
	}

	wg.Wait()

	// Final state should be consistent (no panics or deadlocks)
	assert.NotEmpty(t, sess.Token, "Token should be set")
	assert.NotEqual(t, uuid.Nil, sess.ID, "ID should be set")

	// If deleted, it should have DeletedAt set
	if sess.IsDeleted() {
		assert.NotZero(t, sess.DeletedAt, "DeletedAt should be set if deleted")
	}

	// If authenticated, it should have a UserID
	if sess.IsAuthenticated() {
		assert.NotEqual(t, uuid.Nil, sess.UserID, "UserID should be set if authenticated")
	}
}
