package session_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/dmitrymomot/foundation/core/session"
)

// TestData represents custom session data for testing
type TestData struct {
	Username string         `json:"username"`
	Settings map[string]any `json:"settings"`
	Counter  int            `json:"counter"`
}

func TestSessionIsAuthenticated(t *testing.T) {
	t.Parallel()

	t.Run("returns true for authenticated session", func(t *testing.T) {
		t.Parallel()

		userID := uuid.New()
		s := session.Session[TestData]{
			ID:       uuid.New(),
			Token:    "test-token",
			DeviceID: uuid.New(),
			UserID:   userID,
			Data:     TestData{Username: "testuser"},
		}

		require.True(t, s.IsAuthenticated())
	})

	t.Run("returns false for anonymous session", func(t *testing.T) {
		t.Parallel()

		s := session.Session[TestData]{
			ID:       uuid.New(),
			Token:    "test-token",
			DeviceID: uuid.New(),
			UserID:   uuid.Nil, // Anonymous
			Data:     TestData{},
		}

		require.False(t, s.IsAuthenticated())
	})

	t.Run("returns false for zero UUID", func(t *testing.T) {
		t.Parallel()

		var zeroUUID uuid.UUID
		s := session.Session[TestData]{
			ID:       uuid.New(),
			Token:    "test-token",
			DeviceID: uuid.New(),
			UserID:   zeroUUID,
			Data:     TestData{},
		}

		require.False(t, s.IsAuthenticated())
	})
}

func TestSessionIsExpired(t *testing.T) {
	t.Parallel()

	t.Run("returns false for future expiration", func(t *testing.T) {
		t.Parallel()

		s := session.Session[TestData]{
			ID:        uuid.New(),
			Token:     "test-token",
			DeviceID:  uuid.New(),
			UserID:    uuid.Nil,
			Data:      TestData{},
			ExpiresAt: time.Now().Add(1 * time.Hour),
		}

		require.False(t, s.IsExpired())
	})

	t.Run("returns true for past expiration", func(t *testing.T) {
		t.Parallel()

		s := session.Session[TestData]{
			ID:        uuid.New(),
			Token:     "test-token",
			DeviceID:  uuid.New(),
			UserID:    uuid.Nil,
			Data:      TestData{},
			ExpiresAt: time.Now().Add(-1 * time.Hour),
		}

		require.True(t, s.IsExpired())
	})

	t.Run("returns true for exactly now expiration", func(t *testing.T) {
		t.Parallel()

		// Set expiration to very slightly in the past to account for execution time
		s := session.Session[TestData]{
			ID:        uuid.New(),
			Token:     "test-token",
			DeviceID:  uuid.New(),
			UserID:    uuid.Nil,
			Data:      TestData{},
			ExpiresAt: time.Now().Add(-1 * time.Nanosecond),
		}

		require.True(t, s.IsExpired())
	})

	t.Run("handles zero time expiration", func(t *testing.T) {
		t.Parallel()

		s := session.Session[TestData]{
			ID:        uuid.New(),
			Token:     "test-token",
			DeviceID:  uuid.New(),
			UserID:    uuid.Nil,
			Data:      TestData{},
			ExpiresAt: time.Time{}, // Zero time
		}

		require.True(t, s.IsExpired())
	})

	t.Run("edge case - very close to expiration", func(t *testing.T) {
		t.Parallel()

		// Test a session that expires in 1 millisecond from now
		s := session.Session[TestData]{
			ID:        uuid.New(),
			Token:     "test-token",
			DeviceID:  uuid.New(),
			UserID:    uuid.Nil,
			Data:      TestData{},
			ExpiresAt: time.Now().Add(1 * time.Millisecond),
		}

		require.False(t, s.IsExpired())

		// Wait and check again
		time.Sleep(2 * time.Millisecond)
		require.True(t, s.IsExpired())
	})
}
