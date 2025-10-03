package session_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/dmitrymomot/foundation/core/session"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// MockStore is a mock implementation of session.Store interface
type MockStore[Data any] struct {
	mock.Mock
}

func (m *MockStore[Data]) GetByID(ctx context.Context, id uuid.UUID) (*session.Session[Data], error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	sess := args.Get(0).(session.Session[Data])
	return &sess, args.Error(1)
}

func (m *MockStore[Data]) GetByToken(ctx context.Context, token string) (*session.Session[Data], error) {
	args := m.Called(ctx, token)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	sess := args.Get(0).(session.Session[Data])
	return &sess, args.Error(1)
}

func (m *MockStore[Data]) Save(ctx context.Context, sess *session.Session[Data]) error {
	args := m.Called(ctx, sess)
	return args.Error(0)
}

func (m *MockStore[Data]) Delete(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockStore[Data]) DeleteExpired(ctx context.Context) (int64, error) {
	args := m.Called(ctx)
	return args.Get(0).(int64), args.Error(1)
}

// Test data types
type testData struct {
	Key   string
	Value int
}

func TestNewManager(t *testing.T) {
	t.Parallel()

	t.Run("creates manager with correct configuration", func(t *testing.T) {
		t.Parallel()

		store := &MockStore[string]{}
		ttl := 24 * time.Hour
		touchInterval := 15 * time.Minute

		mgr := session.NewManager(store, ttl, touchInterval)

		require.NotNil(t, mgr)
	})

	t.Run("works with different data types", func(t *testing.T) {
		t.Parallel()

		// String data
		mgrString := session.NewManager(&MockStore[string]{}, time.Hour, time.Minute)
		assert.NotNil(t, mgrString)

		// Struct data
		mgrStruct := session.NewManager(&MockStore[testData]{}, time.Hour, time.Minute)
		assert.NotNil(t, mgrStruct)

		// Map data
		mgrMap := session.NewManager(&MockStore[map[string]interface{}]{}, time.Hour, time.Minute)
		assert.NotNil(t, mgrMap)
	})
}

func TestManager_New(t *testing.T) {
	t.Parallel()

	t.Run("creates anonymous session with valid token and correct expiration", func(t *testing.T) {
		t.Parallel()

		store := &MockStore[string]{}
		ttl := 1 * time.Hour
		touchInterval := 15 * time.Minute
		mgr := session.NewManager(store, ttl, touchInterval)

		ctx := context.Background()

		store.On("Save", ctx, mock.AnythingOfType("*session.Session[string]")).Return(nil)

		beforeCreate := time.Now()
		sess, err := mgr.New(ctx)
		afterCreate := time.Now()

		require.NoError(t, err)

		// Verify token is generated and has correct length (base64 RawURLEncoding of 32 bytes = 43 chars)
		assert.NotEmpty(t, sess.Token)
		assert.Len(t, sess.Token, 43)

		// Verify session fields - should be anonymous (no userID, zero value data)
		assert.NotEqual(t, uuid.Nil, sess.ID)
		assert.Equal(t, uuid.Nil, sess.UserID)
		assert.Equal(t, "", sess.Data) // Zero value for string

		// Verify expiration time is set correctly (TTL from creation time)
		assert.True(t, sess.ExpiresAt.After(beforeCreate.Add(ttl).Add(-time.Second)))
		assert.True(t, sess.ExpiresAt.Before(afterCreate.Add(ttl).Add(time.Second)))

		// Verify timestamps
		assert.True(t, sess.CreatedAt.After(beforeCreate.Add(-time.Second)))
		assert.True(t, sess.CreatedAt.Before(afterCreate.Add(time.Second)))
		assert.Equal(t, sess.CreatedAt, sess.UpdatedAt)

		store.AssertExpectations(t)
	})

	t.Run("generates unique tokens for different sessions", func(t *testing.T) {
		t.Parallel()

		store := &MockStore[string]{}
		mgr := session.NewManager(store, time.Hour, time.Minute)

		ctx := context.Background()

		store.On("Save", ctx, mock.Anything).Return(nil)

		sess1, err1 := mgr.New(ctx)
		require.NoError(t, err1)

		sess2, err2 := mgr.New(ctx)
		require.NoError(t, err2)

		// Tokens should be unique
		assert.NotEqual(t, sess1.Token, sess2.Token)
		assert.NotEqual(t, sess1.ID, sess2.ID)

		store.AssertExpectations(t)
	})

	t.Run("returns error when store save fails", func(t *testing.T) {
		t.Parallel()

		store := &MockStore[string]{}
		mgr := session.NewManager(store, time.Hour, time.Minute)

		ctx := context.Background()
		expectedErr := errors.New("database error")

		store.On("Save", ctx, mock.Anything).Return(expectedErr)

		_, err := mgr.New(ctx)

		assert.Error(t, err)
		assert.ErrorContains(t, err, "failed to save session")
		assert.ErrorIs(t, err, expectedErr)

		store.AssertExpectations(t)
	})

	t.Run("creates anonymous session with struct data type", func(t *testing.T) {
		t.Parallel()

		store := &MockStore[testData]{}
		mgr := session.NewManager(store, time.Hour, time.Minute)

		ctx := context.Background()

		store.On("Save", ctx, mock.Anything).Return(nil)

		sess, err := mgr.New(ctx)

		require.NoError(t, err)
		assert.Equal(t, testData{}, sess.Data) // Zero value

		store.AssertExpectations(t)
	})

	t.Run("creates anonymous session with map data type", func(t *testing.T) {
		t.Parallel()

		store := &MockStore[map[string]interface{}]{}
		mgr := session.NewManager(store, time.Hour, time.Minute)

		ctx := context.Background()

		store.On("Save", ctx, mock.Anything).Return(nil)

		sess, err := mgr.New(ctx)

		require.NoError(t, err)
		assert.Nil(t, sess.Data) // Zero value for map

		store.AssertExpectations(t)
	})
}

func TestManager_GetByID(t *testing.T) {
	t.Parallel()

	t.Run("retrieves valid non-expired session", func(t *testing.T) {
		t.Parallel()

		store := &MockStore[string]{}
		mgr := session.NewManager(store, time.Hour, 15*time.Minute)

		ctx := context.Background()
		sessionID := uuid.New()
		expected := session.Session[string]{
			ID:        sessionID,
			Token:     "test-token",
			UserID:    uuid.New(),
			Data:      "test-data",
			ExpiresAt: time.Now().Add(30 * time.Minute),
			CreatedAt: time.Now().Add(-30 * time.Minute),
			UpdatedAt: time.Now().Add(-30 * time.Minute),
		}

		store.On("GetByID", ctx, sessionID).Return(expected, nil)
		// Touch will be called because UpdatedAt is 30 minutes ago (> 15 minute interval)
		store.On("Save", ctx, mock.Anything).Return(nil)

		sess, err := mgr.GetByID(ctx, sessionID)

		require.NoError(t, err)
		assert.Equal(t, expected.ID, sess.ID)
		assert.Equal(t, expected.Token, sess.Token)

		store.AssertExpectations(t)
	})

	t.Run("returns ErrExpired for expired session", func(t *testing.T) {
		t.Parallel()

		store := &MockStore[string]{}
		mgr := session.NewManager(store, time.Hour, time.Minute)

		ctx := context.Background()
		sessionID := uuid.New()
		expired := session.Session[string]{
			ID:        sessionID,
			Token:     "test-token",
			UserID:    uuid.New(),
			Data:      "test-data",
			ExpiresAt: time.Now().Add(-1 * time.Minute), // Expired 1 minute ago
			CreatedAt: time.Now().Add(-2 * time.Hour),
			UpdatedAt: time.Now().Add(-2 * time.Hour),
		}

		store.On("GetByID", ctx, sessionID).Return(expired, nil)

		_, err := mgr.GetByID(ctx, sessionID)

		assert.Error(t, err)
		assert.ErrorIs(t, err, session.ErrExpired)

		store.AssertExpectations(t)
	})

	t.Run("returns error when store returns error", func(t *testing.T) {
		t.Parallel()

		store := &MockStore[string]{}
		mgr := session.NewManager(store, time.Hour, time.Minute)

		ctx := context.Background()
		sessionID := uuid.New()
		expectedErr := session.ErrNotFound

		store.On("GetByID", ctx, sessionID).Return(nil, expectedErr)

		_, err := mgr.GetByID(ctx, sessionID)

		assert.Error(t, err)
		assert.ErrorIs(t, err, expectedErr)

		store.AssertExpectations(t)
	})

	t.Run("touches session when touch interval has passed", func(t *testing.T) {
		t.Parallel()

		store := &MockStore[string]{}
		touchInterval := 10 * time.Minute
		ttl := 1 * time.Hour
		mgr := session.NewManager(store, ttl, touchInterval)

		ctx := context.Background()
		sessionID := uuid.New()
		oldUpdateTime := time.Now().Add(-15 * time.Minute) // 15 minutes ago
		existing := session.Session[string]{
			ID:        sessionID,
			Token:     "test-token",
			UserID:    uuid.New(),
			Data:      "test-data",
			ExpiresAt: time.Now().Add(30 * time.Minute),
			CreatedAt: time.Now().Add(-1 * time.Hour),
			UpdatedAt: oldUpdateTime,
		}

		store.On("GetByID", ctx, sessionID).Return(existing, nil)
		store.On("Save", ctx, mock.MatchedBy(func(s *session.Session[string]) bool {
			// Verify that ExpiresAt was extended
			expectedExpiry := time.Now().Add(ttl)
			return s.ExpiresAt.After(expectedExpiry.Add(-2*time.Second)) &&
				s.ExpiresAt.Before(expectedExpiry.Add(2*time.Second)) &&
				s.UpdatedAt.After(time.Now().Add(-time.Second))
		})).Return(nil)

		sess, err := mgr.GetByID(ctx, sessionID)

		require.NoError(t, err)

		// Verify session was touched
		assert.True(t, sess.ExpiresAt.After(time.Now().Add(ttl).Add(-2*time.Second)))
		assert.True(t, sess.UpdatedAt.After(oldUpdateTime))

		store.AssertExpectations(t)
	})

	t.Run("does not touch session when touch interval has not passed", func(t *testing.T) {
		t.Parallel()

		store := &MockStore[string]{}
		touchInterval := 10 * time.Minute
		mgr := session.NewManager(store, time.Hour, touchInterval)

		ctx := context.Background()
		sessionID := uuid.New()
		recentUpdateTime := time.Now().Add(-5 * time.Minute) // 5 minutes ago
		existing := session.Session[string]{
			ID:        sessionID,
			Token:     "test-token",
			UserID:    uuid.New(),
			Data:      "test-data",
			ExpiresAt: time.Now().Add(30 * time.Minute),
			CreatedAt: time.Now().Add(-1 * time.Hour),
			UpdatedAt: recentUpdateTime,
		}

		store.On("GetByID", ctx, sessionID).Return(existing, nil)
		// Save should NOT be called because touch interval hasn't passed

		_, err := mgr.GetByID(ctx, sessionID)

		require.NoError(t, err)

		store.AssertExpectations(t)
		store.AssertNotCalled(t, "Save", mock.Anything, mock.Anything)
	})

	t.Run("returns error when touch fails", func(t *testing.T) {
		t.Parallel()

		store := &MockStore[string]{}
		mgr := session.NewManager(store, time.Hour, 10*time.Minute)

		ctx := context.Background()
		sessionID := uuid.New()
		existing := session.Session[string]{
			ID:        sessionID,
			Token:     "test-token",
			UserID:    uuid.New(),
			Data:      "test-data",
			ExpiresAt: time.Now().Add(30 * time.Minute),
			CreatedAt: time.Now().Add(-1 * time.Hour),
			UpdatedAt: time.Now().Add(-15 * time.Minute), // Should trigger touch
		}

		expectedErr := errors.New("database error")
		store.On("GetByID", ctx, sessionID).Return(existing, nil)
		store.On("Save", ctx, mock.Anything).Return(expectedErr)

		_, err := mgr.GetByID(ctx, sessionID)

		assert.Error(t, err)
		assert.ErrorIs(t, err, expectedErr)

		store.AssertExpectations(t)
	})

	t.Run("handles session expiring exactly at current time", func(t *testing.T) {
		t.Parallel()

		store := &MockStore[string]{}
		mgr := session.NewManager(store, time.Hour, time.Minute)

		ctx := context.Background()
		sessionID := uuid.New()
		now := time.Now()
		expired := session.Session[string]{
			ID:        sessionID,
			Token:     "test-token",
			UserID:    uuid.New(),
			Data:      "test-data",
			ExpiresAt: now, // Expires exactly now
			CreatedAt: now.Add(-1 * time.Hour),
			UpdatedAt: now.Add(-1 * time.Hour),
		}

		store.On("GetByID", ctx, sessionID).Return(expired, nil)

		// Small sleep to ensure time.Now() in GetByID is after ExpiresAt
		time.Sleep(10 * time.Millisecond)

		_, err := mgr.GetByID(ctx, sessionID)

		assert.Error(t, err)
		assert.ErrorIs(t, err, session.ErrExpired)

		store.AssertExpectations(t)
	})
}

func TestManager_GetByToken(t *testing.T) {
	t.Parallel()

	t.Run("retrieves valid non-expired session", func(t *testing.T) {
		t.Parallel()

		store := &MockStore[string]{}
		mgr := session.NewManager(store, time.Hour, 15*time.Minute)

		ctx := context.Background()
		token := "valid-token"
		expected := session.Session[string]{
			ID:        uuid.New(),
			Token:     token,
			UserID:    uuid.New(),
			Data:      "test-data",
			ExpiresAt: time.Now().Add(30 * time.Minute),
			CreatedAt: time.Now().Add(-30 * time.Minute),
			UpdatedAt: time.Now().Add(-30 * time.Minute),
		}

		store.On("GetByToken", ctx, token).Return(expected, nil)
		store.On("Save", ctx, mock.Anything).Return(nil)

		sess, err := mgr.GetByToken(ctx, token)

		require.NoError(t, err)
		assert.Equal(t, expected.Token, sess.Token)

		store.AssertExpectations(t)
	})

	t.Run("returns ErrExpired for expired session", func(t *testing.T) {
		t.Parallel()

		store := &MockStore[string]{}
		mgr := session.NewManager(store, time.Hour, time.Minute)

		ctx := context.Background()
		token := "expired-token"
		expired := session.Session[string]{
			ID:        uuid.New(),
			Token:     token,
			UserID:    uuid.New(),
			Data:      "test-data",
			ExpiresAt: time.Now().Add(-5 * time.Minute),
			CreatedAt: time.Now().Add(-2 * time.Hour),
			UpdatedAt: time.Now().Add(-2 * time.Hour),
		}

		store.On("GetByToken", ctx, token).Return(expired, nil)

		_, err := mgr.GetByToken(ctx, token)

		assert.Error(t, err)
		assert.ErrorIs(t, err, session.ErrExpired)

		store.AssertExpectations(t)
	})

	t.Run("returns error when store returns error", func(t *testing.T) {
		t.Parallel()

		store := &MockStore[string]{}
		mgr := session.NewManager(store, time.Hour, time.Minute)

		ctx := context.Background()
		token := "invalid-token"
		expectedErr := session.ErrNotFound

		store.On("GetByToken", ctx, token).Return(nil, expectedErr)

		_, err := mgr.GetByToken(ctx, token)

		assert.Error(t, err)
		assert.ErrorIs(t, err, expectedErr)

		store.AssertExpectations(t)
	})

	t.Run("touches session when touch interval has passed", func(t *testing.T) {
		t.Parallel()

		store := &MockStore[string]{}
		touchInterval := 10 * time.Minute
		ttl := 1 * time.Hour
		mgr := session.NewManager(store, ttl, touchInterval)

		ctx := context.Background()
		token := "test-token"
		oldUpdateTime := time.Now().Add(-20 * time.Minute)
		existing := session.Session[string]{
			ID:        uuid.New(),
			Token:     token,
			UserID:    uuid.New(),
			Data:      "test-data",
			ExpiresAt: time.Now().Add(30 * time.Minute),
			CreatedAt: time.Now().Add(-1 * time.Hour),
			UpdatedAt: oldUpdateTime, // Should trigger touch
		}

		store.On("GetByToken", ctx, token).Return(existing, nil)
		store.On("Save", ctx, mock.Anything).Return(nil)

		sess, err := mgr.GetByToken(ctx, token)

		require.NoError(t, err)
		assert.True(t, sess.UpdatedAt.After(oldUpdateTime))

		store.AssertExpectations(t)
	})

	t.Run("does not touch session when touch interval has not passed", func(t *testing.T) {
		t.Parallel()

		store := &MockStore[string]{}
		mgr := session.NewManager(store, time.Hour, 10*time.Minute)

		ctx := context.Background()
		token := "test-token"
		existing := session.Session[string]{
			ID:        uuid.New(),
			Token:     token,
			UserID:    uuid.New(),
			Data:      "test-data",
			ExpiresAt: time.Now().Add(30 * time.Minute),
			CreatedAt: time.Now().Add(-1 * time.Hour),
			UpdatedAt: time.Now().Add(-5 * time.Minute), // Should NOT trigger touch
		}

		store.On("GetByToken", ctx, token).Return(existing, nil)

		_, err := mgr.GetByToken(ctx, token)

		require.NoError(t, err)

		store.AssertExpectations(t)
		store.AssertNotCalled(t, "Save", mock.Anything, mock.Anything)
	})
}

func TestManager_Save(t *testing.T) {
	t.Parallel()

	t.Run("updates session and sets UpdatedAt", func(t *testing.T) {
		t.Parallel()

		store := &MockStore[string]{}
		mgr := session.NewManager(store, time.Hour, time.Minute)

		ctx := context.Background()
		oldUpdatedAt := time.Now().Add(-10 * time.Minute)
		sess := session.Session[string]{
			ID:        uuid.New(),
			Token:     "test-token",
			UserID:    uuid.New(),
			Data:      "original-data",
			ExpiresAt: time.Now().Add(30 * time.Minute),
			CreatedAt: time.Now().Add(-1 * time.Hour),
			UpdatedAt: oldUpdatedAt,
		}

		store.On("Save", ctx, mock.MatchedBy(func(s *session.Session[string]) bool {
			return s.UpdatedAt.After(oldUpdatedAt)
		})).Return(nil)

		err := mgr.Save(ctx, sess)

		require.NoError(t, err)

		store.AssertExpectations(t)
	})

	t.Run("allows updating session data", func(t *testing.T) {
		t.Parallel()

		store := &MockStore[string]{}
		mgr := session.NewManager(store, time.Hour, time.Minute)

		ctx := context.Background()
		sess := session.Session[string]{
			ID:        uuid.New(),
			Token:     "test-token",
			UserID:    uuid.New(),
			Data:      "original-data",
			ExpiresAt: time.Now().Add(30 * time.Minute),
			CreatedAt: time.Now().Add(-1 * time.Hour),
			UpdatedAt: time.Now().Add(-10 * time.Minute),
		}

		store.On("Save", ctx, mock.Anything).Return(nil)

		sess.Data = "updated-data"
		err := mgr.Save(ctx, sess)

		require.NoError(t, err)

		store.AssertExpectations(t)
	})

	t.Run("returns error when store save fails", func(t *testing.T) {
		t.Parallel()

		store := &MockStore[string]{}
		mgr := session.NewManager(store, time.Hour, time.Minute)

		ctx := context.Background()
		sess := session.Session[string]{
			ID:        uuid.New(),
			Token:     "test-token",
			UserID:    uuid.New(),
			Data:      "test-data",
			ExpiresAt: time.Now().Add(30 * time.Minute),
			CreatedAt: time.Now().Add(-1 * time.Hour),
			UpdatedAt: time.Now().Add(-10 * time.Minute),
		}

		expectedErr := errors.New("database error")
		store.On("Save", ctx, mock.Anything).Return(expectedErr)

		err := mgr.Save(ctx, sess)

		assert.Error(t, err)
		assert.ErrorIs(t, err, expectedErr)

		store.AssertExpectations(t)
	})

	t.Run("updates complex data types", func(t *testing.T) {
		t.Parallel()

		store := &MockStore[testData]{}
		mgr := session.NewManager(store, time.Hour, time.Minute)

		ctx := context.Background()
		sess := session.Session[testData]{
			ID:        uuid.New(),
			Token:     "test-token",
			UserID:    uuid.New(),
			Data:      testData{Key: "original", Value: 1},
			ExpiresAt: time.Now().Add(30 * time.Minute),
			CreatedAt: time.Now().Add(-1 * time.Hour),
			UpdatedAt: time.Now().Add(-10 * time.Minute),
		}

		store.On("Save", ctx, mock.Anything).Return(nil)

		sess.Data = testData{Key: "updated", Value: 42}
		err := mgr.Save(ctx, sess)

		require.NoError(t, err)

		store.AssertExpectations(t)
	})
}

func TestManager_Authenticate(t *testing.T) {
	t.Parallel()

	t.Run("creates new authenticated session with rotated token", func(t *testing.T) {
		t.Parallel()

		store := &MockStore[string]{}
		mgr := session.NewManager(store, time.Hour, 15*time.Minute)

		ctx := context.Background()
		oldSessionID := uuid.New()
		oldToken := "old-token"
		userID := uuid.New()
		data := "test-data"

		oldSession := session.Session[string]{
			ID:        oldSessionID,
			Token:     oldToken,
			UserID:    uuid.Nil, // Anonymous session
			Data:      data,
			ExpiresAt: time.Now().Add(30 * time.Minute),
			CreatedAt: time.Now().Add(-30 * time.Minute),
			UpdatedAt: time.Now().Add(-30 * time.Minute),
		}

		// Expect old session to be deleted
		store.On("Delete", ctx, oldSessionID).Return(nil)
		// Expect new session to be saved
		store.On("Save", ctx, mock.MatchedBy(func(s *session.Session[string]) bool {
			return s.ID != oldSessionID && s.Token != oldToken && s.UserID == userID
		})).Return(nil)

		newSession, err := mgr.Authenticate(ctx, oldSession, userID)

		require.NoError(t, err)

		// Verify new session has different ID and token
		assert.NotEqual(t, oldSessionID, newSession.ID)
		assert.NotEqual(t, oldToken, newSession.Token)
		assert.NotEmpty(t, newSession.Token)

		// Verify userID is set
		assert.Equal(t, userID, newSession.UserID)

		// Verify data is preserved
		assert.Equal(t, data, newSession.Data)

		// Verify expiration is extended
		assert.True(t, newSession.ExpiresAt.After(time.Now()))

		store.AssertExpectations(t)
	})

	t.Run("preserves session data during authentication", func(t *testing.T) {
		t.Parallel()

		store := &MockStore[testData]{}
		mgr := session.NewManager(store, time.Hour, time.Minute)

		ctx := context.Background()
		oldSessionID := uuid.New()
		userID := uuid.New()
		data := testData{Key: "preserved", Value: 123}

		oldSession := session.Session[testData]{
			ID:        oldSessionID,
			Token:     "old-token",
			UserID:    uuid.Nil,
			Data:      data,
			ExpiresAt: time.Now().Add(30 * time.Minute),
			CreatedAt: time.Now().Add(-1 * time.Hour),
			UpdatedAt: time.Now().Add(-1 * time.Hour),
		}

		store.On("Delete", ctx, oldSessionID).Return(nil)
		store.On("Save", ctx, mock.Anything).Return(nil)

		newSession, err := mgr.Authenticate(ctx, oldSession, userID)

		require.NoError(t, err)
		assert.Equal(t, data, newSession.Data)
		assert.Equal(t, userID, newSession.UserID)

		store.AssertExpectations(t)
	})

	t.Run("returns error when delete fails", func(t *testing.T) {
		t.Parallel()

		store := &MockStore[string]{}
		mgr := session.NewManager(store, time.Hour, time.Minute)

		ctx := context.Background()
		oldSession := session.Session[string]{
			ID:        uuid.New(),
			Token:     "old-token",
			UserID:    uuid.Nil,
			Data:      "test-data",
			ExpiresAt: time.Now().Add(30 * time.Minute),
			CreatedAt: time.Now().Add(-1 * time.Hour),
			UpdatedAt: time.Now().Add(-1 * time.Hour),
		}

		expectedErr := errors.New("database error")
		store.On("Delete", ctx, oldSession.ID).Return(expectedErr)

		_, err := mgr.Authenticate(ctx, oldSession, uuid.New())

		assert.Error(t, err)
		assert.ErrorContains(t, err, "failed to delete old session")
		assert.ErrorIs(t, err, expectedErr)

		store.AssertExpectations(t)
	})

	t.Run("returns error when save fails", func(t *testing.T) {
		t.Parallel()

		store := &MockStore[string]{}
		mgr := session.NewManager(store, time.Hour, time.Minute)

		ctx := context.Background()
		oldSession := session.Session[string]{
			ID:        uuid.New(),
			Token:     "old-token",
			UserID:    uuid.Nil,
			Data:      "test-data",
			ExpiresAt: time.Now().Add(30 * time.Minute),
			CreatedAt: time.Now().Add(-1 * time.Hour),
			UpdatedAt: time.Now().Add(-1 * time.Hour),
		}

		expectedErr := errors.New("database error")
		store.On("Delete", ctx, oldSession.ID).Return(nil)
		store.On("Save", ctx, mock.Anything).Return(expectedErr)

		_, err := mgr.Authenticate(ctx, oldSession, uuid.New())

		assert.Error(t, err)
		assert.ErrorContains(t, err, "failed to save authenticated session")
		assert.ErrorIs(t, err, expectedErr)

		store.AssertExpectations(t)
	})
}

func TestManager_Logout(t *testing.T) {
	t.Parallel()

	t.Run("creates new anonymous session with rotated token", func(t *testing.T) {
		t.Parallel()

		store := &MockStore[string]{}
		mgr := session.NewManager(store, time.Hour, time.Minute)

		ctx := context.Background()
		oldSessionID := uuid.New()
		oldToken := "old-token"
		userID := uuid.New()

		oldSession := session.Session[string]{
			ID:        oldSessionID,
			Token:     oldToken,
			UserID:    userID, // Authenticated session
			Data:      "test-data",
			ExpiresAt: time.Now().Add(30 * time.Minute),
			CreatedAt: time.Now().Add(-1 * time.Hour),
			UpdatedAt: time.Now().Add(-10 * time.Minute),
		}

		// Expect old session to be deleted
		store.On("Delete", ctx, oldSessionID).Return(nil)
		// Expect new anonymous session to be saved
		store.On("Save", ctx, mock.MatchedBy(func(s *session.Session[string]) bool {
			return s.ID != oldSessionID && s.Token != oldToken && s.UserID == uuid.Nil
		})).Return(nil)

		newSession, err := mgr.Logout(ctx, oldSession)

		require.NoError(t, err)

		// Verify new session has different ID and token
		assert.NotEqual(t, oldSessionID, newSession.ID)
		assert.NotEqual(t, oldToken, newSession.Token)
		assert.NotEmpty(t, newSession.Token)

		// Verify session is anonymous
		assert.Equal(t, uuid.Nil, newSession.UserID)

		// Verify data is cleared (zero value)
		assert.Equal(t, "", newSession.Data)

		// Verify expiration is extended
		assert.True(t, newSession.ExpiresAt.After(time.Now()))

		store.AssertExpectations(t)
	})

	t.Run("clears complex data types", func(t *testing.T) {
		t.Parallel()

		store := &MockStore[testData]{}
		mgr := session.NewManager(store, time.Hour, time.Minute)

		ctx := context.Background()
		oldSessionID := uuid.New()
		userID := uuid.New()

		oldSession := session.Session[testData]{
			ID:        oldSessionID,
			Token:     "old-token",
			UserID:    userID,
			Data:      testData{Key: "data-to-clear", Value: 999},
			ExpiresAt: time.Now().Add(30 * time.Minute),
			CreatedAt: time.Now().Add(-1 * time.Hour),
			UpdatedAt: time.Now().Add(-10 * time.Minute),
		}

		store.On("Delete", ctx, oldSessionID).Return(nil)
		store.On("Save", ctx, mock.Anything).Return(nil)

		newSession, err := mgr.Logout(ctx, oldSession)

		require.NoError(t, err)
		assert.Equal(t, testData{}, newSession.Data) // Zero value
		assert.Equal(t, uuid.Nil, newSession.UserID)

		store.AssertExpectations(t)
	})

	t.Run("returns error when delete fails", func(t *testing.T) {
		t.Parallel()

		store := &MockStore[string]{}
		mgr := session.NewManager(store, time.Hour, time.Minute)

		ctx := context.Background()
		oldSession := session.Session[string]{
			ID:        uuid.New(),
			Token:     "old-token",
			UserID:    uuid.New(),
			Data:      "test-data",
			ExpiresAt: time.Now().Add(30 * time.Minute),
			CreatedAt: time.Now().Add(-1 * time.Hour),
			UpdatedAt: time.Now().Add(-10 * time.Minute),
		}

		expectedErr := errors.New("database error")
		store.On("Delete", ctx, oldSession.ID).Return(expectedErr)

		_, err := mgr.Logout(ctx, oldSession)

		assert.Error(t, err)
		assert.ErrorContains(t, err, "failed to delete old session")
		assert.ErrorIs(t, err, expectedErr)

		store.AssertExpectations(t)
	})

	t.Run("returns error when save fails", func(t *testing.T) {
		t.Parallel()

		store := &MockStore[string]{}
		mgr := session.NewManager(store, time.Hour, time.Minute)

		ctx := context.Background()
		oldSession := session.Session[string]{
			ID:        uuid.New(),
			Token:     "old-token",
			UserID:    uuid.New(),
			Data:      "test-data",
			ExpiresAt: time.Now().Add(30 * time.Minute),
			CreatedAt: time.Now().Add(-1 * time.Hour),
			UpdatedAt: time.Now().Add(-10 * time.Minute),
		}

		expectedErr := errors.New("database error")
		store.On("Delete", ctx, oldSession.ID).Return(nil)
		store.On("Save", ctx, mock.Anything).Return(expectedErr)

		_, err := mgr.Logout(ctx, oldSession)

		assert.Error(t, err)
		assert.ErrorContains(t, err, "failed to save anonymous session")
		assert.ErrorIs(t, err, expectedErr)

		store.AssertExpectations(t)
	})
}

func TestManager_Delete(t *testing.T) {
	t.Parallel()

	t.Run("deletes session by ID", func(t *testing.T) {
		t.Parallel()

		store := &MockStore[string]{}
		mgr := session.NewManager(store, time.Hour, time.Minute)

		ctx := context.Background()
		sessionID := uuid.New()

		store.On("Delete", ctx, sessionID).Return(nil)

		err := mgr.Delete(ctx, sessionID)

		require.NoError(t, err)

		store.AssertExpectations(t)
	})

	t.Run("returns error when delete fails", func(t *testing.T) {
		t.Parallel()

		store := &MockStore[string]{}
		mgr := session.NewManager(store, time.Hour, time.Minute)

		ctx := context.Background()
		sessionID := uuid.New()
		expectedErr := errors.New("database error")

		store.On("Delete", ctx, sessionID).Return(expectedErr)

		err := mgr.Delete(ctx, sessionID)

		assert.Error(t, err)
		assert.ErrorIs(t, err, expectedErr)

		store.AssertExpectations(t)
	})
}

func TestManager_CleanupExpired(t *testing.T) {
	t.Parallel()

	t.Run("removes expired sessions", func(t *testing.T) {
		t.Parallel()

		store := &MockStore[string]{}
		mgr := session.NewManager(store, time.Hour, time.Minute)

		ctx := context.Background()

		store.On("DeleteExpired", ctx).Return(int64(5), nil)

		err := mgr.CleanupExpired(ctx)

		require.NoError(t, err)

		store.AssertExpectations(t)
	})

	t.Run("returns error when cleanup fails", func(t *testing.T) {
		t.Parallel()

		store := &MockStore[string]{}
		mgr := session.NewManager(store, time.Hour, time.Minute)

		ctx := context.Background()
		expectedErr := errors.New("database error")

		store.On("DeleteExpired", ctx).Return(int64(0), expectedErr)

		err := mgr.CleanupExpired(ctx)

		assert.Error(t, err)
		assert.ErrorIs(t, err, expectedErr)

		store.AssertExpectations(t)
	})

	t.Run("succeeds even when no sessions are deleted", func(t *testing.T) {
		t.Parallel()

		store := &MockStore[string]{}
		mgr := session.NewManager(store, time.Hour, time.Minute)

		ctx := context.Background()

		store.On("DeleteExpired", ctx).Return(int64(0), nil)

		err := mgr.CleanupExpired(ctx)

		require.NoError(t, err)

		store.AssertExpectations(t)
	})
}

func TestSession_WithDifferentDataTypes(t *testing.T) {
	t.Parallel()

	t.Run("session with string data", func(t *testing.T) {
		t.Parallel()

		sess := &session.Session[string]{
			ID:        uuid.New(),
			Token:     "test-token",
			UserID:    uuid.New(),
			Data:      "string-data",
			ExpiresAt: time.Now().Add(1 * time.Hour),
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}

		assert.Equal(t, "string-data", sess.Data)
	})

	t.Run("session with struct data", func(t *testing.T) {
		t.Parallel()

		data := testData{Key: "test", Value: 42}
		sess := &session.Session[testData]{
			ID:        uuid.New(),
			Token:     "test-token",
			UserID:    uuid.New(),
			Data:      data,
			ExpiresAt: time.Now().Add(1 * time.Hour),
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}

		assert.Equal(t, "test", sess.Data.Key)
		assert.Equal(t, 42, sess.Data.Value)
	})

	t.Run("session with map data", func(t *testing.T) {
		t.Parallel()

		data := map[string]interface{}{
			"key1": "value1",
			"key2": 123,
			"key3": true,
		}
		sess := &session.Session[map[string]interface{}]{
			ID:        uuid.New(),
			Token:     "test-token",
			UserID:    uuid.New(),
			Data:      data,
			ExpiresAt: time.Now().Add(1 * time.Hour),
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}

		assert.Equal(t, "value1", sess.Data["key1"])
		assert.Equal(t, 123, sess.Data["key2"])
		assert.Equal(t, true, sess.Data["key3"])
	})

	t.Run("session with empty struct data", func(t *testing.T) {
		t.Parallel()

		type emptyData struct{}
		sess := &session.Session[emptyData]{
			ID:        uuid.New(),
			Token:     "test-token",
			UserID:    uuid.New(),
			Data:      emptyData{},
			ExpiresAt: time.Now().Add(1 * time.Hour),
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}

		assert.Equal(t, emptyData{}, sess.Data)
	})
}

func TestManager_Refresh(t *testing.T) {
	t.Parallel()

	t.Run("successfully refreshes authenticated session", func(t *testing.T) {
		t.Parallel()

		store := &MockStore[string]{}
		ttl := 1 * time.Hour
		mgr := session.NewManager(store, ttl, time.Minute)

		ctx := context.Background()
		oldToken := "old-token"
		sessionID := uuid.New()
		userID := uuid.New()
		data := "test-data"

		oldSession := session.Session[string]{
			ID:        sessionID,
			Token:     oldToken,
			UserID:    userID,
			Data:      data,
			ExpiresAt: time.Now().Add(30 * time.Minute),
			CreatedAt: time.Now().Add(-1 * time.Hour),
			UpdatedAt: time.Now().Add(-30 * time.Minute),
		}

		store.On("Save", ctx, mock.MatchedBy(func(s *session.Session[string]) bool {
			return s.ID == sessionID && s.Token != oldToken && s.UserID == userID && s.Data == data
		})).Return(nil)

		beforeRefresh := time.Now()
		newSession, err := mgr.Refresh(ctx, oldSession)
		afterRefresh := time.Now()

		require.NoError(t, err)

		// Verify session ID stays the same (critical for audit logs)
		assert.Equal(t, sessionID, newSession.ID)

		// Verify token is rotated
		assert.NotEqual(t, oldToken, newSession.Token)
		assert.NotEmpty(t, newSession.Token)
		assert.Len(t, newSession.Token, 43) // base64 RawURLEncoding of 32 bytes

		// Verify user and data preserved
		assert.Equal(t, userID, newSession.UserID)
		assert.Equal(t, data, newSession.Data)

		// Verify expiration is extended with correct TTL
		assert.True(t, newSession.ExpiresAt.After(beforeRefresh.Add(ttl).Add(-time.Second)))
		assert.True(t, newSession.ExpiresAt.Before(afterRefresh.Add(ttl).Add(time.Second)))

		// Verify UpdatedAt is updated
		assert.True(t, newSession.UpdatedAt.After(beforeRefresh.Add(-time.Second)))
		assert.True(t, newSession.UpdatedAt.Before(afterRefresh.Add(time.Second)))

		// Verify CreatedAt is preserved
		assert.Equal(t, oldSession.CreatedAt, newSession.CreatedAt)

		store.AssertExpectations(t)
	})

	t.Run("returns ErrNotAuthenticated for anonymous session", func(t *testing.T) {
		t.Parallel()

		store := &MockStore[string]{}
		mgr := session.NewManager(store, time.Hour, time.Minute)

		ctx := context.Background()
		anonymousSess := session.Session[string]{
			ID:        uuid.New(),
			Token:     "anonymous-token",
			UserID:    uuid.Nil, // Anonymous session
			Data:      "test-data",
			ExpiresAt: time.Now().Add(30 * time.Minute),
			CreatedAt: time.Now().Add(-1 * time.Hour),
			UpdatedAt: time.Now().Add(-30 * time.Minute),
		}

		_, err := mgr.Refresh(ctx, anonymousSess)

		assert.Error(t, err)
		assert.ErrorIs(t, err, session.ErrNotAuthenticated)

		// Store should not be called
		store.AssertNotCalled(t, "Save", mock.Anything, mock.Anything)
	})

	t.Run("returns error when Save fails", func(t *testing.T) {
		t.Parallel()

		store := &MockStore[string]{}
		mgr := session.NewManager(store, time.Hour, time.Minute)

		ctx := context.Background()
		sess := session.Session[string]{
			ID:        uuid.New(),
			Token:     "test-token",
			UserID:    uuid.New(), // Authenticated session
			Data:      "test-data",
			ExpiresAt: time.Now().Add(30 * time.Minute),
			CreatedAt: time.Now().Add(-1 * time.Hour),
			UpdatedAt: time.Now().Add(-30 * time.Minute),
		}

		expectedErr := errors.New("database error")
		store.On("Save", ctx, mock.Anything).Return(expectedErr)

		_, err := mgr.Refresh(ctx, sess)

		assert.Error(t, err)
		assert.ErrorContains(t, err, "failed to save refreshed session")
		assert.ErrorIs(t, err, expectedErr)

		store.AssertExpectations(t)
	})

	t.Run("generates unique token on each refresh", func(t *testing.T) {
		t.Parallel()

		store := &MockStore[string]{}
		mgr := session.NewManager(store, time.Hour, time.Minute)

		ctx := context.Background()
		sessionID := uuid.New()
		userID := uuid.New()

		sess := session.Session[string]{
			ID:        sessionID,
			Token:     "original-token",
			UserID:    userID,
			Data:      "test-data",
			ExpiresAt: time.Now().Add(30 * time.Minute),
			CreatedAt: time.Now().Add(-1 * time.Hour),
			UpdatedAt: time.Now().Add(-30 * time.Minute),
		}

		store.On("Save", ctx, mock.Anything).Return(nil)

		// First refresh
		refreshed1, err := mgr.Refresh(ctx, sess)
		require.NoError(t, err)

		// Second refresh (using the already refreshed session)
		refreshed2, err := mgr.Refresh(ctx, refreshed1)
		require.NoError(t, err)

		// All tokens should be different
		assert.NotEqual(t, sess.Token, refreshed1.Token)
		assert.NotEqual(t, sess.Token, refreshed2.Token)
		assert.NotEqual(t, refreshed1.Token, refreshed2.Token)

		// Session ID should stay the same throughout
		assert.Equal(t, sessionID, refreshed1.ID)
		assert.Equal(t, sessionID, refreshed2.ID)

		store.AssertExpectations(t)
	})

	t.Run("refreshes session with complex data types", func(t *testing.T) {
		t.Parallel()

		store := &MockStore[testData]{}
		mgr := session.NewManager(store, time.Hour, time.Minute)

		ctx := context.Background()
		sessionID := uuid.New()
		userID := uuid.New()
		data := testData{Key: "important", Value: 999}

		sess := session.Session[testData]{
			ID:        sessionID,
			Token:     "old-token",
			UserID:    userID,
			Data:      data,
			ExpiresAt: time.Now().Add(30 * time.Minute),
			CreatedAt: time.Now().Add(-1 * time.Hour),
			UpdatedAt: time.Now().Add(-30 * time.Minute),
		}

		store.On("Save", ctx, mock.Anything).Return(nil)

		refreshed, err := mgr.Refresh(ctx, sess)

		require.NoError(t, err)
		assert.Equal(t, sessionID, refreshed.ID)
		assert.Equal(t, userID, refreshed.UserID)
		assert.Equal(t, data, refreshed.Data) // Data preserved
		assert.NotEqual(t, sess.Token, refreshed.Token)

		store.AssertExpectations(t)
	})
}
