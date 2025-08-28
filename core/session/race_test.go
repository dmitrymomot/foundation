package session_test

import (
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/dmitrymomot/foundation/core/session"
	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// TestValueSemanticsPreventRace verifies that value semantics prevent race conditions
func TestValueSemanticsPreventRace(t *testing.T) {
	t.Parallel()

	store := &MockStore[TestData]{}
	transport := &MockTransport{}

	// Create a session that will be returned by the store
	testSession := session.Session[TestData]{
		ID:        uuid.New(),
		Token:     "test-token",
		DeviceID:  uuid.New(),
		UserID:    uuid.Nil,
		Data:      TestData{},
		ExpiresAt: time.Now().Add(1 * time.Hour),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now().Add(-10 * time.Minute), // Old enough to trigger touch
	}

	// Store.Get returns a copy each time (value semantics)
	store.On("Get", mock.Anything, "test-token").Return(testSession, nil)

	// Store.Store receives a copy each time (value semantics)
	store.On("Store", mock.Anything, mock.MatchedBy(func(s session.Session[TestData]) bool {
		// Just verify it's a valid session
		return s.Token == "test-token"
	})).Return(nil)

	transport.On("Extract", mock.Anything).Return("test-token", nil)
	transport.On("Embed", mock.Anything, mock.Anything, "test-token", mock.Anything).Return(nil)

	manager, err := session.New(
		session.WithStore[TestData](store),
		session.WithTransport[TestData](transport),
		session.WithConfig[TestData](
			session.WithTouchInterval(0), // No throttling for this test
		),
	)
	require.NoError(t, err)

	// Run concurrent Touch operations
	const numGoroutines = 100
	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()

			w := httptest.NewRecorder()
			r := httptest.NewRequest("GET", "/", nil)

			// Each goroutine calls Touch, which internally:
			// 1. Gets a copy of the session from store
			// 2. Modifies its own copy
			// 3. Stores its copy
			// No shared state = no race condition!
			err := manager.Touch(w, r)
			require.NoError(t, err)
		}()
	}

	wg.Wait()

	// Verify all operations completed without race
	store.AssertExpectations(t)
	transport.AssertExpectations(t)

	// The key insight: with value semantics, each goroutine worked with its own
	// copy of the session, preventing any race conditions by design!
}

// TestConcurrentLoadWithValueSemantics verifies concurrent Load operations are safe
func TestConcurrentLoadWithValueSemantics(t *testing.T) {
	t.Parallel()

	store := &MockStore[TestData]{}
	transport := &MockTransport{}

	// Each Load creates a new session (anonymous)
	transport.On("Extract", mock.Anything).Return("", nil)

	manager, err := session.New(
		session.WithStore[TestData](store),
		session.WithTransport[TestData](transport),
	)
	require.NoError(t, err)

	// Run concurrent Load operations
	const numGoroutines = 50
	sessions := make([]session.Session[TestData], numGoroutines)
	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(idx int) {
			defer wg.Done()

			w := httptest.NewRecorder()
			r := httptest.NewRequest("GET", "/", nil)

			// Each Load returns a value (copy), not a pointer
			sess, err := manager.Load(w, r)
			require.NoError(t, err)

			// Store the session - each goroutine has its own copy
			sessions[idx] = sess
		}(i)
	}

	wg.Wait()

	// Verify each goroutine got its own unique session (different IDs)
	seenIDs := make(map[uuid.UUID]bool)
	for _, sess := range sessions {
		require.NotEqual(t, uuid.Nil, sess.ID)
		require.False(t, seenIDs[sess.ID], "Duplicate session ID found - sessions should be unique")
		seenIDs[sess.ID] = true
	}

	store.AssertExpectations(t)
	transport.AssertExpectations(t)
}
