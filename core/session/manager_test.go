package session_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/dmitrymomot/foundation/core/session"
	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// MockStore implements session.Store for testing
type MockStore[Data any] struct {
	mock.Mock
}

func (m *MockStore[Data]) Get(ctx context.Context, token string) (session.Session[Data], error) {
	args := m.Called(ctx, token)
	if args.Get(0) == nil {
		return session.Session[Data]{}, args.Error(1)
	}
	return args.Get(0).(session.Session[Data]), args.Error(1)
}

func (m *MockStore[Data]) Store(ctx context.Context, sess session.Session[Data]) error {
	args := m.Called(ctx, sess)
	return args.Error(0)
}

func (m *MockStore[Data]) Delete(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

// MockTransport implements session.Transport for testing
type MockTransport struct {
	mock.Mock
}

func (m *MockTransport) Extract(r *http.Request) (string, error) {
	args := m.Called(r)
	return args.String(0), args.Error(1)
}

func (m *MockTransport) Embed(w http.ResponseWriter, r *http.Request, token string, ttl time.Duration) error {
	args := m.Called(w, r, token, ttl)
	return args.Error(0)
}

func (m *MockTransport) Revoke(w http.ResponseWriter, r *http.Request) error {
	args := m.Called(w, r)
	return args.Error(0)
}

func TestManagerNew(t *testing.T) {
	t.Parallel()

	t.Run("creates manager with all dependencies", func(t *testing.T) {
		t.Parallel()

		store := &MockStore[TestData]{}
		transport := &MockTransport{}

		manager, err := session.New(
			session.WithStore[TestData](store),
			session.WithTransport[TestData](transport),
		)

		require.NoError(t, err)
		require.NotNil(t, manager)
	})

	t.Run("fails without store", func(t *testing.T) {
		t.Parallel()

		transport := &MockTransport{}

		manager, err := session.New(
			session.WithTransport[TestData](transport),
		)

		require.Error(t, err)
		require.Equal(t, session.ErrNoStore, err)
		require.Nil(t, manager)
	})

	t.Run("fails without transport", func(t *testing.T) {
		t.Parallel()

		store := &MockStore[TestData]{}

		manager, err := session.New(
			session.WithStore[TestData](store),
		)

		require.Error(t, err)
		require.Equal(t, session.ErrNoTransport, err)
		require.Nil(t, manager)
	})

	t.Run("applies custom configuration", func(t *testing.T) {
		t.Parallel()

		store := &MockStore[TestData]{}
		transport := &MockTransport{}

		customTTL := 2 * time.Hour
		customTouchInterval := 10 * time.Minute

		manager, err := session.New(
			session.WithStore[TestData](store),
			session.WithTransport[TestData](transport),
			session.WithConfig[TestData](
				session.WithTTL(customTTL),
				session.WithTouchInterval(customTouchInterval),
			),
		)

		require.NoError(t, err)
		require.NotNil(t, manager)
	})
}

func TestManagerLoad(t *testing.T) {
	t.Parallel()

	t.Run("creates new session when no token", func(t *testing.T) {
		t.Parallel()

		store := &MockStore[TestData]{}
		transport := &MockTransport{}

		// Transport returns no token
		transport.On("Extract", mock.Anything).Return("", session.ErrNoToken)

		manager, err := session.New(
			session.WithStore[TestData](store),
			session.WithTransport[TestData](transport),
		)
		require.NoError(t, err)

		w := httptest.NewRecorder()
		r := httptest.NewRequest("GET", "/", nil)

		sess, err := manager.Load(w, r)

		require.NoError(t, err)
		require.NotNil(t, sess)
		require.False(t, sess.IsAuthenticated())
		require.NotEqual(t, uuid.Nil, sess.ID)
		require.NotEqual(t, uuid.Nil, sess.DeviceID)
		require.NotEmpty(t, sess.Token)

		transport.AssertExpectations(t)
		store.AssertExpectations(t)
	})

	t.Run("creates new session when token not found in store", func(t *testing.T) {
		t.Parallel()

		store := &MockStore[TestData]{}
		transport := &MockTransport{}

		token := "existing-token"
		transport.On("Extract", mock.Anything).Return(token, nil)
		store.On("Get", mock.Anything, token).Return(session.Session[TestData]{}, session.ErrSessionNotFound)

		manager, err := session.New(
			session.WithStore[TestData](store),
			session.WithTransport[TestData](transport),
		)
		require.NoError(t, err)

		w := httptest.NewRecorder()
		r := httptest.NewRequest("GET", "/", nil)

		sess, err := manager.Load(w, r)

		require.NoError(t, err)
		require.NotNil(t, sess)
		require.False(t, sess.IsAuthenticated())

		transport.AssertExpectations(t)
		store.AssertExpectations(t)
	})

	t.Run("loads existing valid session", func(t *testing.T) {
		t.Parallel()

		store := &MockStore[TestData]{}
		transport := &MockTransport{}

		token := "valid-token"
		existingSession := session.Session[TestData]{
			ID:        uuid.New(),
			Token:     token,
			DeviceID:  uuid.New(),
			UserID:    uuid.New(),
			Data:      TestData{Username: "testuser"},
			ExpiresAt: time.Now().Add(1 * time.Hour),
			CreatedAt: time.Now().Add(-2 * time.Hour),
			UpdatedAt: time.Now().Add(-30 * time.Minute),
		}

		transport.On("Extract", mock.Anything).Return(token, nil)
		store.On("Get", mock.Anything, token).Return(existingSession, nil)

		manager, err := session.New(
			session.WithStore[TestData](store),
			session.WithTransport[TestData](transport),
			session.WithConfig[TestData](
				session.WithTouchInterval(0), // Disable auto-touch for this test
			),
		)
		require.NoError(t, err)

		w := httptest.NewRecorder()
		r := httptest.NewRequest("GET", "/", nil)

		sess, err := manager.Load(w, r)

		require.NoError(t, err)
		require.NotNil(t, sess)
		require.True(t, sess.IsAuthenticated())
		require.Equal(t, existingSession.ID, sess.ID)
		require.Equal(t, existingSession.UserID, sess.UserID)
		require.Equal(t, "testuser", sess.Data.Username)

		transport.AssertExpectations(t)
		store.AssertExpectations(t)
	})

	t.Run("creates new session when existing is expired", func(t *testing.T) {
		t.Parallel()

		store := &MockStore[TestData]{}
		transport := &MockTransport{}

		token := "expired-token"
		expiredSession := session.Session[TestData]{
			ID:        uuid.New(),
			Token:     token,
			DeviceID:  uuid.New(),
			UserID:    uuid.New(),
			Data:      TestData{Username: "testuser"},
			ExpiresAt: time.Now().Add(-1 * time.Hour), // Expired
			CreatedAt: time.Now().Add(-2 * time.Hour),
			UpdatedAt: time.Now().Add(-90 * time.Minute),
		}

		transport.On("Extract", mock.Anything).Return(token, nil)
		store.On("Get", mock.Anything, token).Return(expiredSession, nil)

		manager, err := session.New(
			session.WithStore[TestData](store),
			session.WithTransport[TestData](transport),
		)
		require.NoError(t, err)

		w := httptest.NewRecorder()
		r := httptest.NewRequest("GET", "/", nil)

		sess, err := manager.Load(w, r)

		require.NoError(t, err)
		require.NotNil(t, sess)
		require.False(t, sess.IsAuthenticated())                 // New anonymous session
		require.NotEqual(t, expiredSession.ID, sess.ID)          // Different session
		require.Equal(t, expiredSession.DeviceID, sess.DeviceID) // Same device ID preserved

		transport.AssertExpectations(t)
		store.AssertExpectations(t)
	})

	t.Run("propagates store errors other than not found", func(t *testing.T) {
		t.Parallel()

		store := &MockStore[TestData]{}
		transport := &MockTransport{}

		token := "problematic-token"
		storeError := errors.New("database connection failed")

		transport.On("Extract", mock.Anything).Return(token, nil)
		store.On("Get", mock.Anything, token).Return(session.Session[TestData]{}, storeError)

		manager, err := session.New(
			session.WithStore[TestData](store),
			session.WithTransport[TestData](transport),
		)
		require.NoError(t, err)

		w := httptest.NewRecorder()
		r := httptest.NewRequest("GET", "/", nil)

		sess, err := manager.Load(w, r)

		require.Error(t, err)
		require.Equal(t, storeError, err)
		require.Equal(t, session.Session[TestData]{}, sess)

		transport.AssertExpectations(t)
		store.AssertExpectations(t)
	})

	t.Run("auto-touch extends session when enabled", func(t *testing.T) {
		t.Parallel()

		store := &MockStore[TestData]{}
		transport := &MockTransport{}

		token := "valid-token"
		existingSession := session.Session[TestData]{
			ID:        uuid.New(),
			Token:     token,
			DeviceID:  uuid.New(),
			UserID:    uuid.New(),
			Data:      TestData{Username: "testuser"},
			ExpiresAt: time.Now().Add(1 * time.Hour),
			CreatedAt: time.Now().Add(-2 * time.Hour),
			UpdatedAt: time.Now().Add(-10 * time.Minute), // Old enough to trigger touch
		}

		transport.On("Extract", mock.Anything).Return(token, nil)
		store.On("Get", mock.Anything, token).Return(existingSession, nil)
		// Expect auto-touch to update the session
		store.On("Store", mock.Anything, mock.Anything).Return(nil)
		transport.On("Embed", mock.Anything, mock.Anything, token, mock.Anything).Return(nil)

		manager, err := session.New(
			session.WithStore[TestData](store),
			session.WithTransport[TestData](transport),
			session.WithConfig[TestData](
				session.WithTouchInterval(5*time.Minute), // Enable auto-touch
			),
		)
		require.NoError(t, err)

		w := httptest.NewRecorder()
		r := httptest.NewRequest("GET", "/", nil)

		sess, err := manager.Load(w, r)

		require.NoError(t, err)
		require.NotNil(t, sess)

		transport.AssertExpectations(t)
		store.AssertExpectations(t)
	})
}

func TestManagerSave(t *testing.T) {
	t.Parallel()

	t.Run("saves session to store and updates transport", func(t *testing.T) {
		t.Parallel()

		store := &MockStore[TestData]{}
		transport := &MockTransport{}

		sess := session.Session[TestData]{
			ID:        uuid.New(),
			Token:     "test-token",
			DeviceID:  uuid.New(),
			UserID:    uuid.New(),
			Data:      TestData{Username: "testuser"},
			ExpiresAt: time.Now().Add(1 * time.Hour),
			CreatedAt: time.Now().Add(-1 * time.Hour),
			UpdatedAt: time.Now().Add(-30 * time.Minute),
		}

		store.On("Store", mock.Anything, mock.Anything).Return(nil)

		transport.On("Embed", mock.Anything, mock.Anything, sess.Token, mock.Anything).Return(nil)

		manager, err := session.New(
			session.WithStore[TestData](store),
			session.WithTransport[TestData](transport),
		)
		require.NoError(t, err)

		w := httptest.NewRecorder()
		r := httptest.NewRequest("POST", "/", nil)

		err = manager.Save(w, r, sess)

		require.NoError(t, err)

		transport.AssertExpectations(t)
		store.AssertExpectations(t)
	})

	t.Run("propagates store error", func(t *testing.T) {
		t.Parallel()

		store := &MockStore[TestData]{}
		transport := &MockTransport{}

		sess := session.Session[TestData]{
			ID:        uuid.New(),
			Token:     "test-token",
			DeviceID:  uuid.New(),
			UserID:    uuid.New(),
			Data:      TestData{Username: "testuser"},
			ExpiresAt: time.Now().Add(1 * time.Hour),
			CreatedAt: time.Now().Add(-1 * time.Hour),
			UpdatedAt: time.Now().Add(-30 * time.Minute),
		}

		storeError := errors.New("store write failed")
		store.On("Store", mock.Anything, mock.Anything).Return(storeError)

		manager, err := session.New(
			session.WithStore[TestData](store),
			session.WithTransport[TestData](transport),
		)
		require.NoError(t, err)

		w := httptest.NewRecorder()
		r := httptest.NewRequest("POST", "/", nil)

		err = manager.Save(w, r, sess)

		require.Error(t, err)
		require.Equal(t, storeError, err)

		// Transport should not be called if store fails
		transport.AssertNotCalled(t, "Embed")
		store.AssertExpectations(t)
	})

	t.Run("propagates transport error after successful store", func(t *testing.T) {
		t.Parallel()

		store := &MockStore[TestData]{}
		transport := &MockTransport{}

		sess := session.Session[TestData]{
			ID:        uuid.New(),
			Token:     "test-token",
			DeviceID:  uuid.New(),
			UserID:    uuid.New(),
			Data:      TestData{Username: "testuser"},
			ExpiresAt: time.Now().Add(1 * time.Hour),
			CreatedAt: time.Now().Add(-1 * time.Hour),
			UpdatedAt: time.Now().Add(-30 * time.Minute),
		}

		transportError := errors.New("transport embed failed")
		store.On("Store", mock.Anything, mock.Anything).Return(nil)
		transport.On("Embed", mock.Anything, mock.Anything, sess.Token, mock.Anything).Return(transportError)

		manager, err := session.New(
			session.WithStore[TestData](store),
			session.WithTransport[TestData](transport),
		)
		require.NoError(t, err)

		w := httptest.NewRecorder()
		r := httptest.NewRequest("POST", "/", nil)

		err = manager.Save(w, r, sess)

		require.Error(t, err)
		require.Equal(t, transportError, err)

		transport.AssertExpectations(t)
		store.AssertExpectations(t)
	})
}

func TestManagerAuth(t *testing.T) {
	t.Parallel()

	t.Run("authenticates session with valid user ID", func(t *testing.T) {
		t.Parallel()

		store := &MockStore[TestData]{}
		transport := &MockTransport{}

		// Mock Load behavior - no existing token
		transport.On("Extract", mock.Anything).Return("", session.ErrNoToken)

		// Mock Save behavior - expect new session to be saved
		store.On("Store", mock.Anything, mock.Anything).Return(nil)
		transport.On("Embed", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)

		manager, err := session.New(
			session.WithStore[TestData](store),
			session.WithTransport[TestData](transport),
		)
		require.NoError(t, err)

		w := httptest.NewRecorder()
		r := httptest.NewRequest("POST", "/auth", nil)
		userID := uuid.New()

		err = manager.Auth(w, r, userID)

		require.NoError(t, err)

		transport.AssertExpectations(t)
		store.AssertExpectations(t)
	})

	t.Run("rotates token on authentication", func(t *testing.T) {
		t.Parallel()

		store := &MockStore[TestData]{}
		transport := &MockTransport{}

		oldToken := "old-token"
		existingSession := session.Session[TestData]{
			ID:        uuid.New(),
			Token:     oldToken,
			DeviceID:  uuid.New(),
			UserID:    uuid.Nil, // Anonymous
			Data:      TestData{Username: ""},
			ExpiresAt: time.Now().Add(1 * time.Hour),
			CreatedAt: time.Now().Add(-1 * time.Hour),
			UpdatedAt: time.Now().Add(-30 * time.Minute),
		}

		// Mock Load behavior
		transport.On("Extract", mock.Anything).Return(oldToken, nil)
		store.On("Get", mock.Anything, oldToken).Return(existingSession, nil)

		// Mock Save behavior - expect token to be different
		store.On("Store", mock.Anything, mock.Anything).Return(nil)
		transport.On("Embed", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)

		manager, err := session.New(
			session.WithStore[TestData](store),
			session.WithTransport[TestData](transport),
			session.WithConfig[TestData](
				session.WithTouchInterval(0), // Disable auto-touch for cleaner test
			),
		)
		require.NoError(t, err)

		w := httptest.NewRecorder()
		r := httptest.NewRequest("POST", "/auth", nil)
		userID := uuid.New()

		err = manager.Auth(w, r, userID)

		require.NoError(t, err)

		transport.AssertExpectations(t)
		store.AssertExpectations(t)
	})

	t.Run("fails with invalid user ID", func(t *testing.T) {
		t.Parallel()

		store := &MockStore[TestData]{}
		transport := &MockTransport{}

		manager, err := session.New(
			session.WithStore[TestData](store),
			session.WithTransport[TestData](transport),
		)
		require.NoError(t, err)

		w := httptest.NewRecorder()
		r := httptest.NewRequest("POST", "/auth", nil)

		err = manager.Auth(w, r, uuid.Nil)

		require.Error(t, err)
		require.Equal(t, session.ErrInvalidUserID, err)

		// No calls should be made to store or transport
		transport.AssertNotCalled(t, "Extract")
		store.AssertNotCalled(t, "Store")
	})

	t.Run("propagates Load errors", func(t *testing.T) {
		t.Parallel()

		store := &MockStore[TestData]{}
		transport := &MockTransport{}

		loadError := errors.New("load failed")
		transport.On("Extract", mock.Anything).Return("some-token", nil)
		store.On("Get", mock.Anything, "some-token").Return(session.Session[TestData]{}, loadError)

		manager, err := session.New(
			session.WithStore[TestData](store),
			session.WithTransport[TestData](transport),
		)
		require.NoError(t, err)

		w := httptest.NewRecorder()
		r := httptest.NewRequest("POST", "/auth", nil)
		userID := uuid.New()

		err = manager.Auth(w, r, userID)

		require.Error(t, err)
		require.Equal(t, loadError, err)

		transport.AssertExpectations(t)
		store.AssertExpectations(t)
		// Save should not be called
		store.AssertNotCalled(t, "Store")
	})
}

func TestManagerLogout(t *testing.T) {
	t.Parallel()

	t.Run("creates new anonymous session", func(t *testing.T) {
		t.Parallel()

		store := &MockStore[TestData]{}
		transport := &MockTransport{}

		oldToken := "authenticated-token"
		authenticatedSession := session.Session[TestData]{
			ID:        uuid.New(),
			Token:     oldToken,
			DeviceID:  uuid.New(),
			UserID:    uuid.New(), // Authenticated
			Data:      TestData{Username: "testuser", Counter: 5},
			ExpiresAt: time.Now().Add(1 * time.Hour),
			CreatedAt: time.Now().Add(-1 * time.Hour),
			UpdatedAt: time.Now().Add(-30 * time.Minute),
		}

		// Mock Load behavior
		transport.On("Extract", mock.Anything).Return(oldToken, nil)
		store.On("Get", mock.Anything, oldToken).Return(authenticatedSession, nil)

		// Mock Delete old session
		store.On("Delete", mock.Anything, authenticatedSession.ID).Return(nil)

		// Mock Save new session
		store.On("Store", mock.Anything, mock.Anything).Return(nil)
		transport.On("Embed", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)

		manager, err := session.New(
			session.WithStore[TestData](store),
			session.WithTransport[TestData](transport),
			session.WithConfig[TestData](
				session.WithTouchInterval(0), // Disable auto-touch
			),
		)
		require.NoError(t, err)

		w := httptest.NewRecorder()
		r := httptest.NewRequest("POST", "/logout", nil)

		err = manager.Logout(w, r)

		require.NoError(t, err)

		transport.AssertExpectations(t)
		store.AssertExpectations(t)
	})

	t.Run("preserves custom data when specified", func(t *testing.T) {
		t.Parallel()

		store := &MockStore[TestData]{}
		transport := &MockTransport{}

		oldToken := "authenticated-token"
		authenticatedSession := session.Session[TestData]{
			ID:       uuid.New(),
			Token:    oldToken,
			DeviceID: uuid.New(),
			UserID:   uuid.New(),
			Data: TestData{
				Username: "testuser",
				Counter:  10,
				Settings: map[string]interface{}{
					"theme": "dark",
					"lang":  "en",
				},
			},
			ExpiresAt: time.Now().Add(1 * time.Hour),
			CreatedAt: time.Now().Add(-1 * time.Hour),
			UpdatedAt: time.Now().Add(-30 * time.Minute),
		}

		// Mock Load behavior
		transport.On("Extract", mock.Anything).Return(oldToken, nil)
		store.On("Get", mock.Anything, oldToken).Return(authenticatedSession, nil)

		// Mock Delete old session
		store.On("Delete", mock.Anything, authenticatedSession.ID).Return(nil)

		// Mock Save new session with preserved data
		store.On("Store", mock.Anything, mock.Anything).Return(nil)
		transport.On("Embed", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)

		manager, err := session.New(
			session.WithStore[TestData](store),
			session.WithTransport[TestData](transport),
			session.WithConfig[TestData](
				session.WithTouchInterval(0),
			),
		)
		require.NoError(t, err)

		w := httptest.NewRecorder()
		r := httptest.NewRequest("POST", "/logout", nil)

		// Preserve only settings
		err = manager.Logout(w, r, session.PreserveData(func(old TestData) TestData {
			return TestData{
				Settings: old.Settings,
				// Username and Counter are intentionally omitted (zeroed)
			}
		}))

		require.NoError(t, err)

		transport.AssertExpectations(t)
		store.AssertExpectations(t)
	})

	t.Run("continues on delete error", func(t *testing.T) {
		t.Parallel()

		store := &MockStore[TestData]{}
		transport := &MockTransport{}

		oldToken := "authenticated-token"
		authenticatedSession := session.Session[TestData]{
			ID:        uuid.New(),
			Token:     oldToken,
			DeviceID:  uuid.New(),
			UserID:    uuid.New(),
			Data:      TestData{Username: "testuser"},
			ExpiresAt: time.Now().Add(1 * time.Hour),
			CreatedAt: time.Now().Add(-1 * time.Hour),
			UpdatedAt: time.Now().Add(-30 * time.Minute),
		}

		// Mock Load behavior
		transport.On("Extract", mock.Anything).Return(oldToken, nil)
		store.On("Get", mock.Anything, oldToken).Return(authenticatedSession, nil)

		// Mock Delete failure - should not stop logout process
		deleteError := errors.New("delete failed")
		store.On("Delete", mock.Anything, authenticatedSession.ID).Return(deleteError)

		// Mock Save new session - should still happen
		store.On("Store", mock.Anything, mock.Anything).Return(nil)
		transport.On("Embed", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)

		manager, err := session.New(
			session.WithStore[TestData](store),
			session.WithTransport[TestData](transport),
			session.WithConfig[TestData](
				session.WithTouchInterval(0),
			),
		)
		require.NoError(t, err)

		w := httptest.NewRecorder()
		r := httptest.NewRequest("POST", "/logout", nil)

		err = manager.Logout(w, r)

		require.NoError(t, err) // Should not fail even if delete fails

		transport.AssertExpectations(t)
		store.AssertExpectations(t)
	})
}

func TestManagerDelete(t *testing.T) {
	t.Parallel()

	t.Run("deletes session and revokes transport", func(t *testing.T) {
		t.Parallel()

		store := &MockStore[TestData]{}
		transport := &MockTransport{}

		token := "session-token"
		existingSession := session.Session[TestData]{
			ID:        uuid.New(),
			Token:     token,
			DeviceID:  uuid.New(),
			UserID:    uuid.New(),
			Data:      TestData{Username: "testuser"},
			ExpiresAt: time.Now().Add(1 * time.Hour),
			CreatedAt: time.Now().Add(-1 * time.Hour),
			UpdatedAt: time.Now().Add(-30 * time.Minute),
		}

		transport.On("Extract", mock.Anything).Return(token, nil)
		store.On("Get", mock.Anything, token).Return(existingSession, nil)
		store.On("Delete", mock.Anything, existingSession.ID).Return(nil)
		transport.On("Revoke", mock.Anything, mock.Anything).Return(nil)

		manager, err := session.New(
			session.WithStore[TestData](store),
			session.WithTransport[TestData](transport),
		)
		require.NoError(t, err)

		w := httptest.NewRecorder()
		r := httptest.NewRequest("DELETE", "/session", nil)

		err = manager.Delete(w, r)

		require.NoError(t, err)

		transport.AssertExpectations(t)
		store.AssertExpectations(t)
	})

	t.Run("handles no token gracefully", func(t *testing.T) {
		t.Parallel()

		store := &MockStore[TestData]{}
		transport := &MockTransport{}

		transport.On("Extract", mock.Anything).Return("", session.ErrNoToken)

		manager, err := session.New(
			session.WithStore[TestData](store),
			session.WithTransport[TestData](transport),
		)
		require.NoError(t, err)

		w := httptest.NewRecorder()
		r := httptest.NewRequest("DELETE", "/session", nil)

		err = manager.Delete(w, r)

		require.NoError(t, err)

		transport.AssertExpectations(t)
		// Store should not be called
		store.AssertNotCalled(t, "Get")
		store.AssertNotCalled(t, "Delete")
	})

	t.Run("handles session not found and revokes transport", func(t *testing.T) {
		t.Parallel()

		store := &MockStore[TestData]{}
		transport := &MockTransport{}

		token := "nonexistent-token"

		transport.On("Extract", mock.Anything).Return(token, nil)
		store.On("Get", mock.Anything, token).Return(session.Session[TestData]{}, session.ErrSessionNotFound)
		transport.On("Revoke", mock.Anything, mock.Anything).Return(nil)

		manager, err := session.New(
			session.WithStore[TestData](store),
			session.WithTransport[TestData](transport),
		)
		require.NoError(t, err)

		w := httptest.NewRecorder()
		r := httptest.NewRequest("DELETE", "/session", nil)

		err = manager.Delete(w, r)

		require.NoError(t, err)

		transport.AssertExpectations(t)
		store.AssertExpectations(t)
	})

	t.Run("handles store delete error gracefully", func(t *testing.T) {
		t.Parallel()

		store := &MockStore[TestData]{}
		transport := &MockTransport{}

		token := "problematic-token"
		existingSession := session.Session[TestData]{
			ID:        uuid.New(),
			Token:     token,
			DeviceID:  uuid.New(),
			UserID:    uuid.New(),
			Data:      TestData{Username: "testuser"},
			ExpiresAt: time.Now().Add(1 * time.Hour),
			CreatedAt: time.Now().Add(-1 * time.Hour),
			UpdatedAt: time.Now().Add(-30 * time.Minute),
		}

		deleteError := errors.New("delete failed")
		transport.On("Extract", mock.Anything).Return(token, nil)
		store.On("Get", mock.Anything, token).Return(existingSession, nil)
		store.On("Delete", mock.Anything, existingSession.ID).Return(deleteError)
		// Transport.Revoke should NOT be called when store.Delete fails with non-NotFound error

		manager, err := session.New(
			session.WithStore[TestData](store),
			session.WithTransport[TestData](transport),
		)
		require.NoError(t, err)

		w := httptest.NewRecorder()
		r := httptest.NewRequest("DELETE", "/session", nil)

		err = manager.Delete(w, r)

		require.Error(t, err)
		require.Equal(t, deleteError, err)

		transport.AssertExpectations(t)
		store.AssertExpectations(t)

		// Verify Revoke was NOT called
		transport.AssertNotCalled(t, "Revoke")
	})
}

func TestManagerTouch(t *testing.T) {
	t.Parallel()

	t.Run("extends session expiration", func(t *testing.T) {
		t.Parallel()

		store := &MockStore[TestData]{}
		transport := &MockTransport{}

		token := "valid-token"
		existingSession := session.Session[TestData]{
			ID:        uuid.New(),
			Token:     token,
			DeviceID:  uuid.New(),
			UserID:    uuid.New(),
			Data:      TestData{Username: "testuser"},
			ExpiresAt: time.Now().Add(1 * time.Hour),
			CreatedAt: time.Now().Add(-2 * time.Hour),
			UpdatedAt: time.Now().Add(-10 * time.Minute), // Old enough to allow touch
		}

		transport.On("Extract", mock.Anything).Return(token, nil)
		store.On("Get", mock.Anything, token).Return(existingSession, nil)
		store.On("Store", mock.Anything, mock.Anything).Return(nil)
		transport.On("Embed", mock.Anything, mock.Anything, token, mock.Anything).Return(nil)

		manager, err := session.New(
			session.WithStore[TestData](store),
			session.WithTransport[TestData](transport),
			session.WithConfig[TestData](
				session.WithTouchInterval(5*time.Minute),
			),
		)
		require.NoError(t, err)

		w := httptest.NewRecorder()
		r := httptest.NewRequest("GET", "/api/ping", nil)

		err = manager.Touch(w, r)

		require.NoError(t, err)

		transport.AssertExpectations(t)
		store.AssertExpectations(t)
	})

	t.Run("respects touch interval throttling", func(t *testing.T) {
		t.Parallel()

		store := &MockStore[TestData]{}
		transport := &MockTransport{}

		token := "valid-token"
		recentlyUpdatedSession := session.Session[TestData]{
			ID:        uuid.New(),
			Token:     token,
			DeviceID:  uuid.New(),
			UserID:    uuid.New(),
			Data:      TestData{Username: "testuser"},
			ExpiresAt: time.Now().Add(1 * time.Hour),
			CreatedAt: time.Now().Add(-2 * time.Hour),
			UpdatedAt: time.Now().Add(-1 * time.Minute), // Too recent for touch
		}

		transport.On("Extract", mock.Anything).Return(token, nil)
		store.On("Get", mock.Anything, token).Return(recentlyUpdatedSession, nil)
		// Store and Embed should NOT be called due to throttling

		manager, err := session.New(
			session.WithStore[TestData](store),
			session.WithTransport[TestData](transport),
			session.WithConfig[TestData](
				session.WithTouchInterval(5*time.Minute),
			),
		)
		require.NoError(t, err)

		w := httptest.NewRecorder()
		r := httptest.NewRequest("GET", "/api/ping", nil)

		err = manager.Touch(w, r)

		require.NoError(t, err)

		transport.AssertExpectations(t)
		store.AssertExpectations(t)
		// Verify no store/embed calls were made
		store.AssertNotCalled(t, "Store")
		transport.AssertNotCalled(t, "Embed")
	})

	t.Run("handles no token gracefully", func(t *testing.T) {
		t.Parallel()

		store := &MockStore[TestData]{}
		transport := &MockTransport{}

		transport.On("Extract", mock.Anything).Return("", session.ErrNoToken)

		manager, err := session.New(
			session.WithStore[TestData](store),
			session.WithTransport[TestData](transport),
		)
		require.NoError(t, err)

		w := httptest.NewRecorder()
		r := httptest.NewRequest("GET", "/api/ping", nil)

		err = manager.Touch(w, r)

		require.NoError(t, err) // Should not fail

		transport.AssertExpectations(t)
		// Store should not be called
		store.AssertNotCalled(t, "Get")
	})

	t.Run("handles session not found gracefully", func(t *testing.T) {
		t.Parallel()

		store := &MockStore[TestData]{}
		transport := &MockTransport{}

		token := "nonexistent-token"
		transport.On("Extract", mock.Anything).Return(token, nil)
		store.On("Get", mock.Anything, token).Return(session.Session[TestData]{}, session.ErrSessionNotFound)

		manager, err := session.New(
			session.WithStore[TestData](store),
			session.WithTransport[TestData](transport),
		)
		require.NoError(t, err)

		w := httptest.NewRecorder()
		r := httptest.NewRequest("GET", "/api/ping", nil)

		err = manager.Touch(w, r)

		require.NoError(t, err) // Should not fail

		transport.AssertExpectations(t)
		store.AssertExpectations(t)
	})
}

func TestManagerTokenRotation(t *testing.T) {
	t.Parallel()

	t.Run("Auth rotates token for security", func(t *testing.T) {
		t.Parallel()

		store := &MockStore[TestData]{}
		transport := &MockTransport{}

		oldToken := "original-token"
		existingSession := session.Session[TestData]{
			ID:        uuid.New(),
			Token:     oldToken,
			DeviceID:  uuid.New(),
			UserID:    uuid.Nil, // Anonymous
			Data:      TestData{},
			ExpiresAt: time.Now().Add(1 * time.Hour),
			CreatedAt: time.Now().Add(-1 * time.Hour),
			UpdatedAt: time.Now().Add(-30 * time.Minute),
		}

		// Mock Load behavior
		transport.On("Extract", mock.Anything).Return(oldToken, nil)
		store.On("Get", mock.Anything, oldToken).Return(existingSession, nil)

		// Mock Save behavior - capture the stored session to verify token changed
		var capturedSession session.Session[TestData]
		store.On("Store", mock.Anything, mock.MatchedBy(func(sess session.Session[TestData]) bool {
			capturedSession = sess
			return sess.Token != oldToken // Token must be different
		})).Return(nil)
		transport.On("Embed", mock.Anything, mock.Anything, mock.MatchedBy(func(token string) bool {
			return token != oldToken // New token must be different
		}), mock.Anything).Return(nil)

		manager, err := session.New(
			session.WithStore[TestData](store),
			session.WithTransport[TestData](transport),
			session.WithConfig[TestData](
				session.WithTouchInterval(0), // Disable auto-touch
			),
		)
		require.NoError(t, err)

		w := httptest.NewRecorder()
		r := httptest.NewRequest("POST", "/auth", nil)
		userID := uuid.New()

		err = manager.Auth(w, r, userID)

		require.NoError(t, err)
		require.NotEqual(t, uuid.Nil, capturedSession.ID)                    // Session captured
		require.NotEqual(t, oldToken, capturedSession.Token)                 // Token rotated
		require.Equal(t, userID, capturedSession.UserID)                     // User authenticated
		require.Equal(t, existingSession.ID, capturedSession.ID)             // Same session ID
		require.Equal(t, existingSession.DeviceID, capturedSession.DeviceID) // Same device ID

		transport.AssertExpectations(t)
		store.AssertExpectations(t)
	})
}

func TestManagerDeviceIDPreservation(t *testing.T) {
	t.Parallel()

	t.Run("preserves DeviceID when expired session creates new one", func(t *testing.T) {
		t.Parallel()

		store := &MockStore[TestData]{}
		transport := &MockTransport{}

		originalDeviceID := uuid.New()
		expiredToken := "expired-token"
		expiredSession := session.Session[TestData]{
			ID:        uuid.New(),
			Token:     expiredToken,
			DeviceID:  originalDeviceID,
			UserID:    uuid.New(),
			Data:      TestData{Username: "testuser"},
			ExpiresAt: time.Now().Add(-1 * time.Hour), // Expired
			CreatedAt: time.Now().Add(-2 * time.Hour),
			UpdatedAt: time.Now().Add(-90 * time.Minute),
		}

		transport.On("Extract", mock.Anything).Return(expiredToken, nil)
		store.On("Get", mock.Anything, expiredToken).Return(expiredSession, nil)

		manager, err := session.New(
			session.WithStore[TestData](store),
			session.WithTransport[TestData](transport),
		)
		require.NoError(t, err)

		w := httptest.NewRecorder()
		r := httptest.NewRequest("GET", "/", nil)

		newSession, err := manager.Load(w, r)

		require.NoError(t, err)
		require.NotNil(t, newSession)
		require.NotEqual(t, expiredSession.ID, newSession.ID)   // Different session
		require.Equal(t, originalDeviceID, newSession.DeviceID) // DeviceID preserved
		require.False(t, newSession.IsAuthenticated())          // New anonymous session
		require.NotEqual(t, expiredToken, newSession.Token)     // New token

		transport.AssertExpectations(t)
		store.AssertExpectations(t)
	})

	t.Run("preserves DeviceID during logout", func(t *testing.T) {
		t.Parallel()

		store := &MockStore[TestData]{}
		transport := &MockTransport{}

		originalDeviceID := uuid.New()
		oldToken := "auth-token"
		authenticatedSession := session.Session[TestData]{
			ID:        uuid.New(),
			Token:     oldToken,
			DeviceID:  originalDeviceID,
			UserID:    uuid.New(), // Authenticated
			Data:      TestData{Username: "testuser"},
			ExpiresAt: time.Now().Add(1 * time.Hour),
			CreatedAt: time.Now().Add(-1 * time.Hour),
			UpdatedAt: time.Now().Add(-30 * time.Minute),
		}

		// Mock Load behavior
		transport.On("Extract", mock.Anything).Return(oldToken, nil)
		store.On("Get", mock.Anything, oldToken).Return(authenticatedSession, nil)

		// Mock Delete old session
		store.On("Delete", mock.Anything, authenticatedSession.ID).Return(nil)

		// Mock Save new session - capture to verify DeviceID preserved
		var capturedNewSession session.Session[TestData]
		store.On("Store", mock.Anything, mock.MatchedBy(func(sess session.Session[TestData]) bool {
			capturedNewSession = sess
			return sess.DeviceID == originalDeviceID && sess.UserID == uuid.Nil
		})).Return(nil)
		transport.On("Embed", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)

		manager, err := session.New(
			session.WithStore[TestData](store),
			session.WithTransport[TestData](transport),
			session.WithConfig[TestData](
				session.WithTouchInterval(0),
			),
		)
		require.NoError(t, err)

		w := httptest.NewRecorder()
		r := httptest.NewRequest("POST", "/logout", nil)

		err = manager.Logout(w, r)

		require.NoError(t, err)
		require.NotEqual(t, uuid.Nil, capturedNewSession.ID)                // Session captured
		require.NotEqual(t, authenticatedSession.ID, capturedNewSession.ID) // Different session
		require.Equal(t, originalDeviceID, capturedNewSession.DeviceID)     // DeviceID preserved
		require.False(t, capturedNewSession.IsAuthenticated())              // Anonymous
		require.NotEqual(t, oldToken, capturedNewSession.Token)             // New token

		transport.AssertExpectations(t)
		store.AssertExpectations(t)
	})
}

func TestManagerSessionLifecycle(t *testing.T) {
	t.Parallel()

	t.Run("preserves session ID during authentication", func(t *testing.T) {
		t.Parallel()

		store := &MockStore[TestData]{}
		transport := &MockTransport{}

		originalSessionID := uuid.New()
		originalDeviceID := uuid.New()
		oldToken := "anon-token"
		anonSession := session.Session[TestData]{
			ID:        originalSessionID,
			Token:     oldToken,
			DeviceID:  originalDeviceID,
			UserID:    uuid.Nil, // Anonymous
			Data:      TestData{},
			ExpiresAt: time.Now().Add(1 * time.Hour),
			CreatedAt: time.Now().Add(-1 * time.Hour),
			UpdatedAt: time.Now().Add(-30 * time.Minute),
		}

		// Mock Load behavior
		transport.On("Extract", mock.Anything).Return(oldToken, nil)
		store.On("Get", mock.Anything, oldToken).Return(anonSession, nil)

		// Mock Save behavior - capture to verify IDs preserved
		var capturedSession session.Session[TestData]
		store.On("Store", mock.Anything, mock.MatchedBy(func(sess session.Session[TestData]) bool {
			capturedSession = sess
			return sess.ID == originalSessionID && sess.DeviceID == originalDeviceID
		})).Return(nil)
		transport.On("Embed", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)

		manager, err := session.New(
			session.WithStore[TestData](store),
			session.WithTransport[TestData](transport),
			session.WithConfig[TestData](
				session.WithTouchInterval(0),
			),
		)
		require.NoError(t, err)

		w := httptest.NewRecorder()
		r := httptest.NewRequest("POST", "/auth", nil)
		userID := uuid.New()

		err = manager.Auth(w, r, userID)

		require.NoError(t, err)
		require.NotEqual(t, uuid.Nil, capturedSession.ID)            // Session captured
		require.Equal(t, originalSessionID, capturedSession.ID)      // Same session ID
		require.Equal(t, originalDeviceID, capturedSession.DeviceID) // Same device ID
		require.Equal(t, userID, capturedSession.UserID)             // Now authenticated
		require.NotEqual(t, oldToken, capturedSession.Token)         // Token rotated

		transport.AssertExpectations(t)
		store.AssertExpectations(t)
	})
}

func TestManagerConcurrency(t *testing.T) {
	t.Parallel()

	t.Run("concurrent Load operations", func(t *testing.T) {
		t.Parallel()

		store := &MockStore[TestData]{}
		transport := &MockTransport{}

		// All requests return no token (will create new sessions)
		transport.On("Extract", mock.Anything).Return("", session.ErrNoToken)

		manager, err := session.New(
			session.WithStore[TestData](store),
			session.WithTransport[TestData](transport),
		)
		require.NoError(t, err)

		const numGoroutines = 10
		var wg sync.WaitGroup
		errors := make(chan error, numGoroutines)
		sessions := make(chan session.Session[TestData], numGoroutines)

		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				w := httptest.NewRecorder()
				r := httptest.NewRequest("GET", "/", nil)

				sess, err := manager.Load(w, r)
				if err != nil {
					errors <- err
					return
				}
				sessions <- sess
			}()
		}

		wg.Wait()
		close(errors)
		close(sessions)

		// Check for errors
		for err := range errors {
			t.Errorf("Concurrent Load failed: %v", err)
		}

		// Check that we got the expected number of sessions
		var sessionCount int
		uniqueIDs := make(map[uuid.UUID]bool)
		for sess := range sessions {
			sessionCount++
			require.NotNil(t, sess)
			require.NotEqual(t, uuid.Nil, sess.ID)
			uniqueIDs[sess.ID] = true
		}

		require.Equal(t, numGoroutines, sessionCount)
		require.Equal(t, numGoroutines, len(uniqueIDs)) // All sessions should be unique

		transport.AssertExpectations(t)
		store.AssertExpectations(t)
	})

	t.Run("concurrent Touch operations handle session not found", func(t *testing.T) {
		t.Parallel()

		store := &MockStore[TestData]{}
		transport := &MockTransport{}

		// All extracts return no token, so Touch should handle gracefully
		transport.On("Extract", mock.Anything).Return("", session.ErrNoToken)

		manager, err := session.New(
			session.WithStore[TestData](store),
			session.WithTransport[TestData](transport),
		)
		require.NoError(t, err)

		const numGoroutines = 10
		var wg sync.WaitGroup
		errors := make(chan error, numGoroutines)

		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(index int) {
				defer wg.Done()
				w := httptest.NewRecorder()
				r := httptest.NewRequest("GET", fmt.Sprintf("/api/%d", index), nil)

				// Touch should not fail even when no token exists
				if err := manager.Touch(w, r); err != nil {
					errors <- fmt.Errorf("touch %d failed: %w", index, err)
				}
			}(i)
		}

		wg.Wait()
		close(errors)

		// Check for errors
		for err := range errors {
			t.Errorf("Concurrent Touch failed: %v", err)
		}

		transport.AssertExpectations(t)
		store.AssertExpectations(t)
	})
}

func TestManagerAutoTouch(t *testing.T) {
	t.Parallel()

	t.Run("Load triggers auto-touch when configured and interval passed", func(t *testing.T) {
		t.Parallel()

		store := &MockStore[TestData]{}
		transport := &MockTransport{}

		token := "valid-token"
		existingSession := session.Session[TestData]{
			ID:        uuid.New(),
			Token:     token,
			DeviceID:  uuid.New(),
			UserID:    uuid.New(),
			Data:      TestData{Username: "testuser"},
			ExpiresAt: time.Now().Add(1 * time.Hour),
			CreatedAt: time.Now().Add(-2 * time.Hour),
			UpdatedAt: time.Now().Add(-10 * time.Minute), // Old enough to trigger touch
		}

		transport.On("Extract", mock.Anything).Return(token, nil)
		store.On("Get", mock.Anything, token).Return(existingSession, nil)
		// Expect auto-touch to trigger store and transport updates
		store.On("Store", mock.Anything, mock.Anything).Return(nil)
		transport.On("Embed", mock.Anything, mock.Anything, token, mock.Anything).Return(nil)

		manager, err := session.New(
			session.WithStore[TestData](store),
			session.WithTransport[TestData](transport),
			session.WithConfig[TestData](
				session.WithTouchInterval(5*time.Minute), // Enable auto-touch
			),
		)
		require.NoError(t, err)

		w := httptest.NewRecorder()
		r := httptest.NewRequest("GET", "/", nil)

		sess, err := manager.Load(w, r)

		require.NoError(t, err)
		require.NotNil(t, sess)
		// Note: We can't directly verify the UpdatedAt because Load returns the original session
		// but auto-touch happens in the background. The important thing is that Store was called.

		transport.AssertExpectations(t)
		store.AssertExpectations(t)
	})

	t.Run("Load does not trigger auto-touch when interval not passed", func(t *testing.T) {
		t.Parallel()

		store := &MockStore[TestData]{}
		transport := &MockTransport{}

		token := "valid-token"
		recentSession := session.Session[TestData]{
			ID:        uuid.New(),
			Token:     token,
			DeviceID:  uuid.New(),
			UserID:    uuid.New(),
			Data:      TestData{Username: "testuser"},
			ExpiresAt: time.Now().Add(1 * time.Hour),
			CreatedAt: time.Now().Add(-2 * time.Hour),
			UpdatedAt: time.Now().Add(-1 * time.Minute), // Too recent for touch
		}

		transport.On("Extract", mock.Anything).Return(token, nil)
		store.On("Get", mock.Anything, token).Return(recentSession, nil)
		// Should NOT call Store or Embed due to throttling

		manager, err := session.New(
			session.WithStore[TestData](store),
			session.WithTransport[TestData](transport),
			session.WithConfig[TestData](
				session.WithTouchInterval(5*time.Minute),
			),
		)
		require.NoError(t, err)

		w := httptest.NewRecorder()
		r := httptest.NewRequest("GET", "/", nil)

		sess, err := manager.Load(w, r)

		require.NoError(t, err)
		require.NotNil(t, sess)
		require.Equal(t, recentSession.UpdatedAt, sess.UpdatedAt) // Should NOT be touched

		transport.AssertExpectations(t)
		store.AssertExpectations(t)
		// Verify no touch occurred
		store.AssertNotCalled(t, "Store")
		transport.AssertNotCalled(t, "Embed")
	})

	t.Run("Load handles auto-touch store error gracefully", func(t *testing.T) {
		t.Parallel()

		store := &MockStore[TestData]{}
		transport := &MockTransport{}

		token := "valid-token"
		existingSession := session.Session[TestData]{
			ID:        uuid.New(),
			Token:     token,
			DeviceID:  uuid.New(),
			UserID:    uuid.New(),
			Data:      TestData{Username: "testuser"},
			ExpiresAt: time.Now().Add(1 * time.Hour),
			CreatedAt: time.Now().Add(-2 * time.Hour),
			UpdatedAt: time.Now().Add(-10 * time.Minute), // Old enough to trigger touch
		}

		transport.On("Extract", mock.Anything).Return(token, nil)
		store.On("Get", mock.Anything, token).Return(existingSession, nil)
		// Auto-touch fails but Load should still succeed
		store.On("Store", mock.Anything, mock.Anything).Return(errors.New("touch store failed"))

		manager, err := session.New(
			session.WithStore[TestData](store),
			session.WithTransport[TestData](transport),
			session.WithConfig[TestData](
				session.WithTouchInterval(5*time.Minute),
			),
		)
		require.NoError(t, err)

		w := httptest.NewRecorder()
		r := httptest.NewRequest("GET", "/", nil)

		sess, err := manager.Load(w, r)

		require.NoError(t, err) // Load should succeed despite touch failure
		require.NotNil(t, sess)

		transport.AssertExpectations(t)
		store.AssertExpectations(t)
	})
}

func TestManagerPreserveData(t *testing.T) {
	t.Parallel()

	t.Run("PreserveData option works correctly", func(t *testing.T) {
		t.Parallel()

		originalData := TestData{
			Username: "user123",
			Counter:  42,
			Settings: map[string]interface{}{
				"theme":    "dark",
				"language": "en",
				"notifs":   true,
			},
		}

		// Test that we can create a preservation function that works correctly
		preservationFunc := func(old TestData) TestData {
			return TestData{
				Settings: map[string]interface{}{
					"theme":    old.Settings["theme"],
					"language": old.Settings["language"],
					// notifs intentionally omitted
				},
				// Username and Counter intentionally omitted (will be zero values)
			}
		}

		// Test the preservation function directly
		result := preservationFunc(originalData)

		require.Equal(t, "", result.Username) // Should be zero value
		require.Equal(t, 0, result.Counter)   // Should be zero value
		require.NotNil(t, result.Settings)    // Should be preserved
		require.Equal(t, "dark", result.Settings["theme"])
		require.Equal(t, "en", result.Settings["language"])
		require.Nil(t, result.Settings["notifs"]) // Should be omitted

		// Test that PreserveData creates a valid LogoutOption
		preserveOption := session.PreserveData(preservationFunc)
		require.NotNil(t, preserveOption)

		// The actual functionality is tested in the integration tests and Logout tests
		// where the option is passed to the Logout method
	})
}
