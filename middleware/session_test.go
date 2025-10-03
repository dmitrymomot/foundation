package middleware_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/dmitrymomot/foundation/core/handler"
	"github.com/dmitrymomot/foundation/core/response"
	"github.com/dmitrymomot/foundation/core/router"
	"github.com/dmitrymomot/foundation/core/session"
	"github.com/dmitrymomot/foundation/middleware"
)

// Test session data type
type SessionData struct {
	Theme    string `json:"theme"`
	Language string `json:"language"`
}

// MockStore is a mock implementation of session.Store interface
type MockStore struct {
	mock.Mock
}

func (m *MockStore) Get(ctx context.Context, tokenHash string) (session.Session[SessionData], error) {
	args := m.Called(ctx, tokenHash)
	return args.Get(0).(session.Session[SessionData]), args.Error(1)
}

func (m *MockStore) Store(ctx context.Context, sess session.Session[SessionData]) error {
	args := m.Called(ctx, sess)
	return args.Error(0)
}

func (m *MockStore) Delete(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

// MockTransport is a mock implementation of session.Transport interface
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

func createTestManager(t *testing.T, store session.Store[SessionData], transport session.Transport) *session.Manager[SessionData] {
	manager, err := session.New[SessionData](
		session.WithStore[SessionData](store),
		session.WithTransport[SessionData](transport),
		session.WithConfig[SessionData](
			session.WithTTL(time.Hour),
			session.WithTouchInterval(5*time.Minute),
		),
	)
	require.NoError(t, err)
	return manager
}

func TestSessionBasicFlow(t *testing.T) {
	t.Parallel()

	store := new(MockStore)
	transport := new(MockTransport)
	manager := createTestManager(t, store, transport)

	// Expect Extract to be called - no existing token
	transport.On("Extract", mock.Anything).Return("", nil).Once()

	// Expect Store to be called when saving new session
	store.On("Store", mock.Anything, mock.MatchedBy(func(sess session.Session[SessionData]) bool {
		return sess.Data.Theme == "dark" && sess.Data.Language == "en"
	})).Return(nil).Once()

	// Expect Embed to be called when saving session
	transport.On("Embed", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Once()

	r := router.New[*router.Context]()
	r.Use(middleware.Session[*router.Context, SessionData](manager))

	var sessionID uuid.UUID

	r.Get("/test", func(ctx *router.Context) handler.Response {
		sess, ok := middleware.GetSession[SessionData](ctx)
		assert.True(t, ok, "Session should be available in handler")

		sessionID = sess.ID

		// Modify session data
		sess.Data.Theme = "dark"
		sess.Data.Language = "en"
		middleware.SetSession(ctx, sess)

		return response.JSON(map[string]any{
			"session_id": sess.ID.String(),
			"theme":      sess.Data.Theme,
		})
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "dark")
	assert.NotEqual(t, uuid.Nil, sessionID)

	store.AssertExpectations(t)
	transport.AssertExpectations(t)
}

func TestSessionLoadExisting(t *testing.T) {
	t.Parallel()

	store := new(MockStore)
	transport := new(MockTransport)
	manager := createTestManager(t, store, transport)

	// Pre-create a session
	now := time.Now()
	testToken := "test-token"

	// Compute the hash of the token (same logic as session manager)
	hash := sha256.Sum256([]byte(testToken))
	tokenHash := hex.EncodeToString(hash[:])

	existingSession := session.Session[SessionData]{
		ID:        uuid.New(),
		Token:     testToken,
		TokenHash: tokenHash,
		DeviceID:  uuid.New(),
		UserID:    uuid.New(),
		Data: SessionData{
			Theme:    "light",
			Language: "fr",
		},
		ExpiresAt: now.Add(time.Hour),
		CreatedAt: now.Add(-time.Hour),
		UpdatedAt: now.Add(-30 * time.Minute),
	}

	// Setup expectations
	transport.On("Extract", mock.Anything).Return(testToken, nil).Once()
	store.On("Get", mock.Anything, tokenHash).Return(existingSession, nil).Once()

	// Auto-touch will be triggered (because UpdatedAt is 30 min ago and TouchInterval is 5 min)
	// This will call Store and Embed
	store.On("Store", mock.Anything, mock.Anything).Return(nil).Once()
	transport.On("Embed", mock.Anything, mock.Anything, testToken, mock.Anything).Return(nil).Once()

	r := router.New[*router.Context]()
	r.Use(middleware.Session[*router.Context, SessionData](manager))

	r.Get("/profile", func(ctx *router.Context) handler.Response {
		sess, ok := middleware.GetSession[SessionData](ctx)
		assert.True(t, ok)

		return response.JSON(map[string]any{
			"session_id": sess.ID.String(),
			"user_id":    sess.UserID.String(),
			"theme":      sess.Data.Theme,
			"language":   sess.Data.Language,
		})
	})

	req := httptest.NewRequest(http.MethodGet, "/profile", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), existingSession.ID.String())
	assert.Contains(t, w.Body.String(), existingSession.UserID.String())
	assert.Contains(t, w.Body.String(), "light")
	assert.Contains(t, w.Body.String(), "fr")

	store.AssertExpectations(t)
	transport.AssertExpectations(t)
}

func TestSessionAuthentication(t *testing.T) {
	t.Parallel()

	store := new(MockStore)
	transport := new(MockTransport)
	manager := createTestManager(t, store, transport)

	r := router.New[*router.Context]()
	r.Use(middleware.Session[*router.Context, SessionData](manager))

	r.Get("/status", func(ctx *router.Context) handler.Response {
		sess, ok := middleware.GetSession[SessionData](ctx)
		assert.True(t, ok)

		return response.JSON(map[string]any{
			"authenticated": sess.IsAuthenticated(),
			"user_id":       sess.UserID.String(),
		})
	})

	t.Run("anonymous session", func(t *testing.T) {
		// Expect Extract to be called - no existing token
		transport.On("Extract", mock.Anything).Return("", nil).Once()

		// Note: Store is NOT called because the session data is not modified
		// The middleware only saves if data changes or auto-save is enabled
		// Since we're not modifying the session, no Store call is expected

		req := httptest.NewRequest(http.MethodGet, "/status", nil)
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), `"authenticated":false`)

		transport.AssertExpectations(t)
	})
}

func TestSessionWithSkip(t *testing.T) {
	t.Parallel()

	t.Run("skip health check", func(t *testing.T) {
		store := new(MockStore)
		transport := new(MockTransport)
		manager := createTestManager(t, store, transport)

		r := router.New[*router.Context]()
		r.Use(middleware.SessionWithConfig[*router.Context, SessionData](middleware.SessionConfig[SessionData]{
			Manager: manager,
			Skip: func(ctx handler.Context) bool {
				return ctx.Request().URL.Path == "/health"
			},
			AutoSave: true,
		}))

		r.Get("/health", func(ctx *router.Context) handler.Response {
			_, ok := middleware.GetSession[SessionData](ctx)
			assert.False(t, ok, "Session should not be available for skipped routes")
			return response.JSON(map[string]string{"status": "healthy"})
		})

		req := httptest.NewRequest(http.MethodGet, "/health", nil)
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), "healthy")

		// No expectations - skip should prevent middleware execution
	})

	t.Run("session loaded for app route", func(t *testing.T) {
		store := new(MockStore)
		transport := new(MockTransport)
		manager := createTestManager(t, store, transport)

		r := router.New[*router.Context]()
		r.Use(middleware.SessionWithConfig[*router.Context, SessionData](middleware.SessionConfig[SessionData]{
			Manager: manager,
			Skip: func(ctx handler.Context) bool {
				return ctx.Request().URL.Path == "/health"
			},
			AutoSave: true,
		}))

		r.Get("/app", func(ctx *router.Context) handler.Response {
			sess, ok := middleware.GetSession[SessionData](ctx)
			assert.True(t, ok, "Session should be available for non-skipped routes")
			return response.JSON(map[string]string{"session_id": sess.ID.String()})
		})

		// For /app route, expect Extract to be called
		transport.On("Extract", mock.Anything).Return("", nil).Once()
		// Note: Store is NOT called because the session is not modified
		// But Embed IS called to set the cookie even when unchanged
		transport.On("Embed", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Once()

		req := httptest.NewRequest(http.MethodGet, "/app", nil)
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), "session_id")

		// Store should NOT be called (session not modified)
		store.AssertNotCalled(t, "Store", mock.Anything, mock.Anything)
		transport.AssertExpectations(t)
	})
}

func TestSessionLoadError(t *testing.T) {
	t.Parallel()

	store := new(MockStore)
	transport := new(MockTransport)
	manager := createTestManager(t, store, transport)

	// Setup expectations - transport returns token, but store Get fails
	transport.On("Extract", mock.Anything).Return("some-token", nil).Once()

	// Compute hash for "some-token"
	hash := sha256.Sum256([]byte("some-token"))
	tokenHash := hex.EncodeToString(hash[:])

	store.On("Get", mock.Anything, tokenHash).Return(
		session.Session[SessionData]{},
		errors.New("database connection failed"),
	).Once()

	r := router.New[*router.Context]()
	r.Use(middleware.Session[*router.Context, SessionData](manager))

	r.Get("/test", func(ctx *router.Context) handler.Response {
		t.Fatal("Handler should not be called when session loading fails")
		return response.JSON(map[string]string{"status": "ok"})
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)

	store.AssertExpectations(t)
	transport.AssertExpectations(t)
}

func TestSessionCustomErrorHandler(t *testing.T) {
	t.Parallel()

	store := new(MockStore)
	transport := new(MockTransport)
	manager := createTestManager(t, store, transport)

	// Setup expectations - transport returns token, but store Get fails
	transport.On("Extract", mock.Anything).Return("some-token", nil).Once()

	// Compute hash for "some-token"
	hash := sha256.Sum256([]byte("some-token"))
	tokenHash := hex.EncodeToString(hash[:])

	store.On("Get", mock.Anything, tokenHash).Return(
		session.Session[SessionData]{},
		errors.New("critical error"),
	).Once()

	customErrorHandled := false

	r := router.New[*router.Context]()
	r.Use(middleware.SessionWithConfig[*router.Context, SessionData](middleware.SessionConfig[SessionData]{
		Manager: manager,
		ErrorHandler: func(ctx handler.Context, err error) handler.Response {
			customErrorHandled = true
			return response.JSONWithStatus(
				map[string]string{"error": "session unavailable"},
				http.StatusServiceUnavailable,
			)
		},
	}))

	r.Get("/test", func(ctx *router.Context) handler.Response {
		return response.JSON(map[string]string{"status": "ok"})
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
	assert.True(t, customErrorHandled)
	assert.Contains(t, w.Body.String(), "session unavailable")

	store.AssertExpectations(t)
	transport.AssertExpectations(t)
}

func TestSessionAutoSaveDisabled(t *testing.T) {
	t.Parallel()

	store := new(MockStore)
	transport := new(MockTransport)
	manager := createTestManager(t, store, transport)

	// Expect Extract to be called - no existing token
	transport.On("Extract", mock.Anything).Return("", nil).Once()

	// When auto-save is disabled, we should NOT expect Store to be called

	r := router.New[*router.Context]()
	r.Use(middleware.SessionWithConfig[*router.Context, SessionData](middleware.SessionConfig[SessionData]{
		Manager:  manager,
		AutoSave: false, // Disable auto-save
	}))

	r.Get("/test", func(ctx *router.Context) handler.Response {
		sess, ok := middleware.GetSession[SessionData](ctx)
		assert.True(t, ok)

		// Modify session but don't call Save
		sess.Data.Theme = "dark"
		middleware.SetSession(ctx, sess)

		return response.JSON(map[string]string{"status": "ok"})
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	// Store should NOT have been called (auto-save disabled)
	store.AssertNotCalled(t, "Store", mock.Anything, mock.Anything)
	transport.AssertExpectations(t)
}

func TestSessionGetSet(t *testing.T) {
	t.Parallel()

	store := new(MockStore)
	transport := new(MockTransport)
	manager := createTestManager(t, store, transport)

	// Expect Extract to be called - no existing token
	transport.On("Extract", mock.Anything).Return("", nil).Once()

	// Expect Store to be called when saving modified session
	store.On("Store", mock.Anything, mock.MatchedBy(func(sess session.Session[SessionData]) bool {
		return sess.Data.Theme == "dark" && sess.Data.Language == "es"
	})).Return(nil).Once()

	transport.On("Embed", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Once()

	r := router.New[*router.Context]()
	r.Use(middleware.Session[*router.Context, SessionData](manager))

	r.Get("/update", func(ctx *router.Context) handler.Response {
		// Get session
		sess, ok := middleware.GetSession[SessionData](ctx)
		assert.True(t, ok)
		assert.Equal(t, "", sess.Data.Theme)

		// Modify and set back
		sess.Data.Theme = "dark"
		sess.Data.Language = "es"
		middleware.SetSession(ctx, sess)

		// Get again to verify
		updatedSess, ok := middleware.GetSession[SessionData](ctx)
		assert.True(t, ok)
		assert.Equal(t, "dark", updatedSess.Data.Theme)
		assert.Equal(t, "es", updatedSess.Data.Language)

		// Use sess to avoid unused variable error
		_ = sess

		return response.JSON(map[string]string{"status": "updated"})
	})

	req := httptest.NewRequest(http.MethodGet, "/update", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	store.AssertExpectations(t)
	transport.AssertExpectations(t)
}

func TestSessionNotFoundInContext(t *testing.T) {
	t.Parallel()

	// Create router without session middleware
	r := router.New[*router.Context]()

	r.Get("/test", func(ctx *router.Context) handler.Response {
		sess, ok := middleware.GetSession[SessionData](ctx)
		assert.False(t, ok, "Session should not be available without middleware")
		assert.Equal(t, uuid.Nil, sess.ID)

		return response.JSON(map[string]string{"status": "ok"})
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestSessionMultipleRequests(t *testing.T) {
	t.Parallel()

	store := new(MockStore)
	transport := new(MockTransport)
	manager := createTestManager(t, store, transport)

	r := router.New[*router.Context]()
	r.Use(middleware.Session[*router.Context, SessionData](manager))

	r.Get("/first", func(ctx *router.Context) handler.Response {
		sess, ok := middleware.GetSession[SessionData](ctx)
		assert.True(t, ok)

		sess.Data.Theme = "light"
		middleware.SetSession(ctx, sess)

		return response.JSON(map[string]any{
			"session_id": sess.ID.String(),
			"theme":      sess.Data.Theme,
		})
	})

	r.Get("/second", func(ctx *router.Context) handler.Response {
		sess, ok := middleware.GetSession[SessionData](ctx)
		assert.True(t, ok)

		return response.JSON(map[string]any{
			"session_id": sess.ID.String(),
			"theme":      sess.Data.Theme,
		})
	})

	// First request
	t.Run("first request", func(t *testing.T) {
		// First request - no existing token
		transport.On("Extract", mock.Anything).Return("", nil).Once()

		// Store called for first request
		store.On("Store", mock.Anything, mock.MatchedBy(func(sess session.Session[SessionData]) bool {
			return sess.Data.Theme == "light"
		})).Return(nil).Once()

		transport.On("Embed", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Once()

		req1 := httptest.NewRequest(http.MethodGet, "/first", nil)
		w1 := httptest.NewRecorder()
		r.ServeHTTP(w1, req1)

		assert.Equal(t, http.StatusOK, w1.Code)
		assert.Contains(t, w1.Body.String(), "light")

		store.AssertExpectations(t)
		transport.AssertExpectations(t)
	})

	// Second request (independent - new session)
	t.Run("second request", func(t *testing.T) {
		// Second request - no existing token (new session)
		transport.On("Extract", mock.Anything).Return("", nil).Once()

		// Note: Store is NOT called because the session data is not modified
		// The second handler doesn't modify the session, so no Store call happens

		req2 := httptest.NewRequest(http.MethodGet, "/second", nil)
		w2 := httptest.NewRecorder()
		r.ServeHTTP(w2, req2)

		assert.Equal(t, http.StatusOK, w2.Code)

		transport.AssertExpectations(t)
	})
}

func TestSessionPanicOnNilManager(t *testing.T) {
	t.Parallel()

	assert.Panics(t, func() {
		middleware.SessionWithConfig[*router.Context, SessionData](middleware.SessionConfig[SessionData]{
			Manager: nil, // This should panic
		})
	}, "Should panic when manager is nil")
}
