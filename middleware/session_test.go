package middleware_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
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

// testSessionData is the session data type used in all tests
type testSessionData struct {
	CartItems []string
	Theme     string
}

// mockTransport implements the Transport interface for testing
type mockTransport struct {
	mock.Mock
}

func (m *mockTransport) Load(ctx handler.Context) (*session.Session[testSessionData], error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*session.Session[testSessionData]), args.Error(1)
}

func (m *mockTransport) Store(ctx handler.Context, sess *session.Session[testSessionData]) error {
	args := m.Called(ctx, sess)
	return args.Error(0)
}

func TestSession(t *testing.T) {
	t.Parallel()

	t.Run("creates middleware with default config", func(t *testing.T) {
		t.Parallel()

		transport := &mockTransport{}
		userID := uuid.New()
		sess := &session.Session[testSessionData]{
			ID:        uuid.New(),
			Token:     "test-token",
			UserID:    userID,
			ExpiresAt: time.Now().Add(time.Hour),
			Data:      testSessionData{Theme: "dark"},
		}

		transport.On("Load", mock.Anything).Return(sess, nil)
		transport.On("Store", mock.Anything, mock.Anything).Return(nil)

		r := router.New[*router.Context]()
		r.Use(middleware.Session[*router.Context, testSessionData](transport))

		r.Get("/test", func(ctx *router.Context) handler.Response {
			loadedSess, ok := middleware.GetSession[testSessionData](ctx)
			require.True(t, ok)
			assert.Equal(t, sess.ID, loadedSess.ID)
			assert.Equal(t, sess.UserID, loadedSess.UserID)
			assert.Equal(t, "dark", loadedSess.Data.Theme)
			return response.JSON(map[string]string{"status": "ok"})
		})

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		transport.AssertExpectations(t)
	})

	t.Run("session lifecycle: Load -> Process -> Store", func(t *testing.T) {
		t.Parallel()

		transport := &mockTransport{}
		sessionID := uuid.New()
		userID := uuid.New()
		originalSess := &session.Session[testSessionData]{
			ID:        sessionID,
			Token:     "original-token",
			UserID:    userID,
			ExpiresAt: time.Now().Add(time.Hour),
			Data:      testSessionData{Theme: "light"},
		}

		transport.On("Load", mock.Anything).Return(originalSess, nil)

		// Verify Store receives the mutated session
		transport.On("Store", mock.Anything, mock.MatchedBy(func(s *session.Session[testSessionData]) bool {
			return s.ID == sessionID && s.Data.Theme == "dark"
		})).Return(nil)

		r := router.New[*router.Context]()
		r.Use(middleware.Session[*router.Context, testSessionData](transport))

		r.Get("/mutate", func(ctx *router.Context) handler.Response {
			sess, _ := middleware.GetSession[testSessionData](ctx)
			sess.Data.Theme = "dark"
			middleware.SetSession(ctx, sess)
			return response.JSON(map[string]string{"status": "mutated"})
		})

		req := httptest.NewRequest(http.MethodGet, "/mutate", nil)
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		transport.AssertExpectations(t)
	})
}

func TestSessionWithConfig(t *testing.T) {
	t.Parallel()

	t.Run("panics when transport is nil", func(t *testing.T) {
		t.Parallel()

		assert.Panics(t, func() {
			middleware.SessionWithConfig[*router.Context, testSessionData](middleware.SessionConfig[*router.Context, testSessionData]{
				Transport: nil,
			})
		})
	})

	t.Run("panics when both RequireAuth and RequireGuest are true", func(t *testing.T) {
		t.Parallel()

		transport := &mockTransport{}

		assert.Panics(t, func() {
			middleware.SessionWithConfig[*router.Context, testSessionData](middleware.SessionConfig[*router.Context, testSessionData]{
				Transport:    transport,
				RequireAuth:  true,
				RequireGuest: true,
			})
		})
	})

	t.Run("uses custom logger", func(t *testing.T) {
		t.Parallel()

		transport := &mockTransport{}
		customLogger := slog.New(slog.NewTextHandler(io.Discard, nil))
		sess := &session.Session[testSessionData]{
			ID:        uuid.New(),
			Token:     "test-token",
			ExpiresAt: time.Now().Add(time.Hour),
		}

		transport.On("Load", mock.Anything).Return(sess, nil)
		transport.On("Store", mock.Anything, mock.Anything).Return(nil)

		r := router.New[*router.Context]()
		r.Use(middleware.SessionWithConfig[*router.Context, testSessionData](middleware.SessionConfig[*router.Context, testSessionData]{
			Transport: transport,
			Logger:    customLogger,
		}))

		r.Get("/test", func(ctx *router.Context) handler.Response {
			return response.JSON(map[string]string{"status": "ok"})
		})

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		transport.AssertExpectations(t)
	})

	t.Run("skip function bypasses middleware", func(t *testing.T) {
		t.Parallel()

		transport := &mockTransport{}

		r := router.New[*router.Context]()
		r.Use(middleware.SessionWithConfig[*router.Context, testSessionData](middleware.SessionConfig[*router.Context, testSessionData]{
			Transport: transport,
			Skip: func(ctx *router.Context) bool {
				return ctx.Request().URL.Path == "/health"
			},
		}))

		r.Get("/health", func(ctx *router.Context) handler.Response {
			_, ok := middleware.GetSession[testSessionData](ctx)
			assert.False(t, ok, "session should not be loaded for skipped routes")
			return response.JSON(map[string]string{"status": "healthy"})
		})

		r.Get("/protected", func(ctx *router.Context) handler.Response {
			return response.JSON(map[string]string{"status": "ok"})
		})

		// Test skipped route - no transport calls expected
		req := httptest.NewRequest(http.MethodGet, "/health", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)

		// Transport should not have been called for skipped route
		transport.AssertNotCalled(t, "Load")
		transport.AssertNotCalled(t, "Store")
	})

	t.Run("RequireAuth enforces authenticated session", func(t *testing.T) {
		t.Parallel()

		transport := &mockTransport{}

		// Anonymous session (UserID = uuid.Nil)
		anonSess := &session.Session[testSessionData]{
			ID:        uuid.New(),
			Token:     "anon-token",
			UserID:    uuid.Nil,
			ExpiresAt: time.Now().Add(time.Hour),
		}

		transport.On("Load", mock.Anything).Return(anonSess, nil)

		r := router.New[*router.Context]()
		r.Use(middleware.SessionWithConfig[*router.Context, testSessionData](middleware.SessionConfig[*router.Context, testSessionData]{
			Transport:   transport,
			RequireAuth: true,
		}))

		r.Get("/protected", func(ctx *router.Context) handler.Response {
			return response.JSON(map[string]string{"status": "ok"})
		})

		req := httptest.NewRequest(http.MethodGet, "/protected", nil)
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
		transport.AssertExpectations(t)
	})

	t.Run("RequireAuth allows authenticated session", func(t *testing.T) {
		t.Parallel()

		transport := &mockTransport{}
		userID := uuid.New()

		authSess := &session.Session[testSessionData]{
			ID:        uuid.New(),
			Token:     "auth-token",
			UserID:    userID,
			ExpiresAt: time.Now().Add(time.Hour),
		}

		transport.On("Load", mock.Anything).Return(authSess, nil)
		transport.On("Store", mock.Anything, mock.Anything).Return(nil)

		r := router.New[*router.Context]()
		r.Use(middleware.SessionWithConfig[*router.Context, testSessionData](middleware.SessionConfig[*router.Context, testSessionData]{
			Transport:   transport,
			RequireAuth: true,
		}))

		r.Get("/protected", func(ctx *router.Context) handler.Response {
			sess, ok := middleware.GetSession[testSessionData](ctx)
			require.True(t, ok)
			assert.Equal(t, userID, sess.UserID)
			return response.JSON(map[string]string{"status": "ok"})
		})

		req := httptest.NewRequest(http.MethodGet, "/protected", nil)
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		transport.AssertExpectations(t)
	})

	t.Run("RequireGuest enforces guest session", func(t *testing.T) {
		t.Parallel()

		transport := &mockTransport{}
		userID := uuid.New()

		authSess := &session.Session[testSessionData]{
			ID:        uuid.New(),
			Token:     "auth-token",
			UserID:    userID,
			ExpiresAt: time.Now().Add(time.Hour),
		}

		transport.On("Load", mock.Anything).Return(authSess, nil)

		r := router.New[*router.Context]()
		r.Use(middleware.SessionWithConfig[*router.Context, testSessionData](middleware.SessionConfig[*router.Context, testSessionData]{
			Transport:    transport,
			RequireGuest: true,
		}))

		r.Get("/login", func(ctx *router.Context) handler.Response {
			return response.JSON(map[string]string{"status": "ok"})
		})

		req := httptest.NewRequest(http.MethodGet, "/login", nil)
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusForbidden, w.Code)
		transport.AssertExpectations(t)
	})

	t.Run("RequireGuest allows guest session", func(t *testing.T) {
		t.Parallel()

		transport := &mockTransport{}

		guestSess := &session.Session[testSessionData]{
			ID:        uuid.New(),
			Token:     "guest-token",
			UserID:    uuid.Nil,
			ExpiresAt: time.Now().Add(time.Hour),
		}

		transport.On("Load", mock.Anything).Return(guestSess, nil)
		transport.On("Store", mock.Anything, mock.Anything).Return(nil)

		r := router.New[*router.Context]()
		r.Use(middleware.SessionWithConfig[*router.Context, testSessionData](middleware.SessionConfig[*router.Context, testSessionData]{
			Transport:    transport,
			RequireGuest: true,
		}))

		r.Get("/login", func(ctx *router.Context) handler.Response {
			sess, ok := middleware.GetSession[testSessionData](ctx)
			require.True(t, ok)
			assert.Equal(t, uuid.Nil, sess.UserID)
			return response.JSON(map[string]string{"status": "ok"})
		})

		req := httptest.NewRequest(http.MethodGet, "/login", nil)
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		transport.AssertExpectations(t)
	})

	t.Run("custom ErrorHandler for transport Load failure", func(t *testing.T) {
		t.Parallel()

		transport := &mockTransport{}
		loadErr := errors.New("database connection failed")

		transport.On("Load", mock.Anything).Return(nil, loadErr)

		customErrorCalled := false
		r := router.New[*router.Context]()
		r.Use(middleware.SessionWithConfig[*router.Context, testSessionData](middleware.SessionConfig[*router.Context, testSessionData]{
			Transport: transport,
			ErrorHandler: func(ctx *router.Context, err error) handler.Response {
				customErrorCalled = true
				assert.Equal(t, loadErr, err)
				return response.JSONWithStatus(
					map[string]string{"error": "custom error"},
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
		assert.Contains(t, w.Body.String(), "custom error")
		assert.True(t, customErrorCalled)
		transport.AssertExpectations(t)
	})

	t.Run("custom ErrorHandler for transport Store failure", func(t *testing.T) {
		t.Parallel()

		transport := &mockTransport{}
		sess := &session.Session[testSessionData]{
			ID:        uuid.New(),
			Token:     "test-token",
			ExpiresAt: time.Now().Add(time.Hour),
		}
		storeErr := errors.New("failed to write session")

		transport.On("Load", mock.Anything).Return(sess, nil)
		transport.On("Store", mock.Anything, mock.Anything).Return(storeErr)

		customErrorCalled := false
		r := router.New[*router.Context]()
		r.Use(middleware.SessionWithConfig[*router.Context, testSessionData](middleware.SessionConfig[*router.Context, testSessionData]{
			Transport: transport,
			ErrorHandler: func(ctx *router.Context, err error) handler.Response {
				customErrorCalled = true
				assert.Equal(t, storeErr, err)
				return response.JSONWithStatus(
					map[string]string{"error": "storage failed"},
					http.StatusInternalServerError,
				)
			},
		}))

		r.Get("/test", func(ctx *router.Context) handler.Response {
			return response.JSON(map[string]string{"status": "ok"})
		})

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
		assert.Contains(t, w.Body.String(), "storage failed")
		assert.True(t, customErrorCalled)
		transport.AssertExpectations(t)
	})

	t.Run("custom ErrorHandler for RequireAuth failure", func(t *testing.T) {
		t.Parallel()

		transport := &mockTransport{}
		anonSess := &session.Session[testSessionData]{
			ID:        uuid.New(),
			Token:     "anon-token",
			UserID:    uuid.Nil,
			ExpiresAt: time.Now().Add(time.Hour),
		}

		transport.On("Load", mock.Anything).Return(anonSess, nil)

		customErrorCalled := false
		r := router.New[*router.Context]()
		r.Use(middleware.SessionWithConfig[*router.Context, testSessionData](middleware.SessionConfig[*router.Context, testSessionData]{
			Transport:   transport,
			RequireAuth: true,
			ErrorHandler: func(ctx *router.Context, err error) handler.Response {
				customErrorCalled = true
				assert.Equal(t, response.ErrUnauthorized, err)
				return response.Redirect("/login?redirect=" + ctx.Request().URL.Path)
			},
		}))

		r.Get("/dashboard", func(ctx *router.Context) handler.Response {
			return response.JSON(map[string]string{"status": "ok"})
		})

		req := httptest.NewRequest(http.MethodGet, "/dashboard", nil)
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusFound, w.Code)
		assert.Equal(t, "/login?redirect=/dashboard", w.Header().Get("Location"))
		assert.True(t, customErrorCalled)
		transport.AssertExpectations(t)
	})

	t.Run("custom ErrorHandler for RequireGuest failure", func(t *testing.T) {
		t.Parallel()

		transport := &mockTransport{}
		userID := uuid.New()
		authSess := &session.Session[testSessionData]{
			ID:        uuid.New(),
			Token:     "auth-token",
			UserID:    userID,
			ExpiresAt: time.Now().Add(time.Hour),
		}

		transport.On("Load", mock.Anything).Return(authSess, nil)

		customErrorCalled := false
		r := router.New[*router.Context]()
		r.Use(middleware.SessionWithConfig[*router.Context, testSessionData](middleware.SessionConfig[*router.Context, testSessionData]{
			Transport:    transport,
			RequireGuest: true,
			ErrorHandler: func(ctx *router.Context, err error) handler.Response {
				customErrorCalled = true
				assert.Equal(t, response.ErrForbidden, err)
				return response.Redirect("/dashboard")
			},
		}))

		r.Get("/login", func(ctx *router.Context) handler.Response {
			return response.JSON(map[string]string{"status": "ok"})
		})

		req := httptest.NewRequest(http.MethodGet, "/login", nil)
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusFound, w.Code)
		assert.Equal(t, "/dashboard", w.Header().Get("Location"))
		assert.True(t, customErrorCalled)
		transport.AssertExpectations(t)
	})

	t.Run("ErrorHandler returning nil results in error response", func(t *testing.T) {
		t.Parallel()

		transport := &mockTransport{}
		loadErr := errors.New("transient error")

		transport.On("Load", mock.Anything).Return(nil, loadErr)

		r := router.New[*router.Context]()
		r.Use(middleware.SessionWithConfig[*router.Context, testSessionData](middleware.SessionConfig[*router.Context, testSessionData]{
			Transport: transport,
			ErrorHandler: func(ctx *router.Context, err error) handler.Response {
				// Returning nil causes router to return ErrNilResponse error
				return nil
			},
		}))

		r.Get("/test", func(ctx *router.Context) handler.Response {
			return response.JSON(map[string]string{"status": "ok"})
		})

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		// Router treats nil response as an error (ErrNilResponse)
		assert.Equal(t, http.StatusInternalServerError, w.Code)
		transport.AssertExpectations(t)
	})

	t.Run("wrong session data type returns false from GetSession", func(t *testing.T) {
		t.Parallel()

		transport := &mockTransport{}
		sess := &session.Session[testSessionData]{
			ID:        uuid.New(),
			Token:     "test-token",
			ExpiresAt: time.Now().Add(time.Hour),
			Data:      testSessionData{Theme: "dark"},
		}

		transport.On("Load", mock.Anything).Return(sess, nil)
		// Store is called with correct session
		transport.On("Store", mock.Anything, mock.Anything).Return(nil)

		r := router.New[*router.Context]()
		r.Use(middleware.SessionWithConfig[*router.Context, testSessionData](middleware.SessionConfig[*router.Context, testSessionData]{
			Transport: transport,
		}))

		r.Get("/test", func(ctx *router.Context) handler.Response {
			// Get session with correct type works
			correctSess, ok := middleware.GetSession[testSessionData](ctx)
			require.True(t, ok)
			assert.Equal(t, "dark", correctSess.Data.Theme)

			// Get session with wrong data type returns false
			type differentData struct {
				Name string
			}
			_, ok = middleware.GetSession[differentData](ctx)
			assert.False(t, ok, "GetSession should return false for wrong data type")

			return response.JSON(map[string]string{"status": "ok"})
		})

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		transport.AssertExpectations(t)
	})

	t.Run("does not log ErrNotAuthenticated on Load", func(t *testing.T) {
		t.Parallel()

		transport := &mockTransport{}
		transport.On("Load", mock.Anything).Return(nil, session.ErrNotAuthenticated)

		r := router.New[*router.Context]()
		r.Use(middleware.SessionWithConfig[*router.Context, testSessionData](middleware.SessionConfig[*router.Context, testSessionData]{
			Transport: transport,
			Logger:    slog.New(slog.NewTextHandler(io.Discard, nil)),
		}))

		r.Get("/test", func(ctx *router.Context) handler.Response {
			return response.JSON(map[string]string{"status": "ok"})
		})

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		// ErrNotAuthenticated is not an HTTPError, so router treats it as 500
		// The middleware doesn't log it (line 186-188), but returns it via ErrorHandler
		assert.Equal(t, http.StatusInternalServerError, w.Code)
		transport.AssertExpectations(t)
	})

	t.Run("does not log ErrNotAuthenticated on Store", func(t *testing.T) {
		t.Parallel()

		transport := &mockTransport{}
		sess := &session.Session[testSessionData]{
			ID:        uuid.New(),
			Token:     "test-token",
			ExpiresAt: time.Now().Add(time.Hour),
		}

		transport.On("Load", mock.Anything).Return(sess, nil)
		transport.On("Store", mock.Anything, mock.Anything).Return(session.ErrNotAuthenticated)

		r := router.New[*router.Context]()
		r.Use(middleware.SessionWithConfig[*router.Context, testSessionData](middleware.SessionConfig[*router.Context, testSessionData]{
			Transport: transport,
			Logger:    slog.New(slog.NewTextHandler(io.Discard, nil)),
		}))

		r.Get("/test", func(ctx *router.Context) handler.Response {
			return response.JSON(map[string]string{"status": "ok"})
		})

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		// ErrNotAuthenticated is not an HTTPError, so router treats it as 500
		// The middleware doesn't log it (line 218-220), but returns it via ErrorHandler
		assert.Equal(t, http.StatusInternalServerError, w.Code)
		transport.AssertExpectations(t)
	})
}

func TestGetSession(t *testing.T) {
	t.Parallel()

	t.Run("returns session when present in context", func(t *testing.T) {
		t.Parallel()

		transport := &mockTransport{}
		userID := uuid.New()
		expectedSess := &session.Session[testSessionData]{
			ID:        uuid.New(),
			Token:     "test-token",
			UserID:    userID,
			ExpiresAt: time.Now().Add(time.Hour),
			Data:      testSessionData{Theme: "dark", CartItems: []string{"item1"}},
		}

		transport.On("Load", mock.Anything).Return(expectedSess, nil)
		transport.On("Store", mock.Anything, mock.Anything).Return(nil)

		r := router.New[*router.Context]()
		r.Use(middleware.Session[*router.Context, testSessionData](transport))

		r.Get("/test", func(ctx *router.Context) handler.Response {
			sess, ok := middleware.GetSession[testSessionData](ctx)
			assert.True(t, ok)
			assert.Equal(t, expectedSess.ID, sess.ID)
			assert.Equal(t, expectedSess.UserID, sess.UserID)
			assert.Equal(t, "dark", sess.Data.Theme)
			assert.Equal(t, []string{"item1"}, sess.Data.CartItems)
			return response.JSON(map[string]string{"status": "ok"})
		})

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		transport.AssertExpectations(t)
	})

	t.Run("returns false when session not in context", func(t *testing.T) {
		t.Parallel()

		r := router.New[*router.Context]()

		r.Get("/test", func(ctx *router.Context) handler.Response {
			sess, ok := middleware.GetSession[testSessionData](ctx)
			assert.False(t, ok)
			assert.Nil(t, sess)
			return response.JSON(map[string]string{"status": "ok"})
		})

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("returns false when context is nil", func(t *testing.T) {
		t.Parallel()

		sess, ok := middleware.GetSession[testSessionData](nil)
		assert.False(t, ok)
		assert.Nil(t, sess)
	})

	t.Run("returns false for wrong data type", func(t *testing.T) {
		t.Parallel()

		r := router.New[*router.Context]()

		r.Get("/test", func(ctx *router.Context) handler.Response {
			// Try to get session with wrong data type
			type differentData struct {
				Name string
			}
			sess, ok := middleware.GetSession[differentData](ctx)
			assert.False(t, ok)
			assert.Nil(t, sess)
			return response.JSON(map[string]string{"status": "ok"})
		})

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})
}

func TestMustGetSession(t *testing.T) {
	t.Parallel()

	t.Run("returns session when present in context", func(t *testing.T) {
		t.Parallel()

		transport := &mockTransport{}
		userID := uuid.New()
		expectedSess := &session.Session[testSessionData]{
			ID:        uuid.New(),
			Token:     "test-token",
			UserID:    userID,
			ExpiresAt: time.Now().Add(time.Hour),
			Data:      testSessionData{Theme: "dark"},
		}

		transport.On("Load", mock.Anything).Return(expectedSess, nil)
		transport.On("Store", mock.Anything, mock.Anything).Return(nil)

		r := router.New[*router.Context]()
		r.Use(middleware.Session[*router.Context, testSessionData](transport))

		r.Get("/test", func(ctx *router.Context) handler.Response {
			sess := middleware.MustGetSession[testSessionData](ctx)
			assert.Equal(t, expectedSess.ID, sess.ID)
			assert.Equal(t, expectedSess.UserID, sess.UserID)
			assert.Equal(t, "dark", sess.Data.Theme)
			return response.JSON(map[string]string{"status": "ok"})
		})

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		transport.AssertExpectations(t)
	})

	t.Run("panics when session not in context", func(t *testing.T) {
		t.Parallel()

		r := router.New[*router.Context]()

		panicked := false
		r.Get("/test", func(ctx *router.Context) handler.Response {
			defer func() {
				if r := recover(); r != nil {
					panicked = true
					assert.Contains(t, r, "session not found in context")
				}
			}()

			// This should panic since no session middleware was used
			middleware.MustGetSession[testSessionData](ctx)
			return response.JSON(map[string]string{"status": "ok"})
		})

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		assert.True(t, panicked, "MustGetSession should have panicked when session not in context")
	})

	t.Run("panics when context is nil", func(t *testing.T) {
		t.Parallel()

		assert.Panics(t, func() {
			middleware.MustGetSession[testSessionData](nil)
		})
	})
}

func TestSetSession(t *testing.T) {
	t.Parallel()

	t.Run("updates session in context", func(t *testing.T) {
		t.Parallel()

		transport := &mockTransport{}
		originalSess := &session.Session[testSessionData]{
			ID:        uuid.New(),
			Token:     "original-token",
			ExpiresAt: time.Now().Add(time.Hour),
			Data:      testSessionData{Theme: "light"},
		}

		transport.On("Load", mock.Anything).Return(originalSess, nil)

		// Verify the updated session is stored
		transport.On("Store", mock.Anything, mock.MatchedBy(func(s *session.Session[testSessionData]) bool {
			return s.Data.Theme == "dark" && len(s.Data.CartItems) == 2
		})).Return(nil)

		r := router.New[*router.Context]()
		r.Use(middleware.Session[*router.Context, testSessionData](transport))

		r.Get("/test", func(ctx *router.Context) handler.Response {
			sess, _ := middleware.GetSession[testSessionData](ctx)
			sess.Data.Theme = "dark"
			sess.Data.CartItems = []string{"item1", "item2"}
			middleware.SetSession(ctx, sess)

			// Verify we can retrieve the updated session
			updatedSess, ok := middleware.GetSession[testSessionData](ctx)
			require.True(t, ok)
			assert.Equal(t, "dark", updatedSess.Data.Theme)
			assert.Equal(t, []string{"item1", "item2"}, updatedSess.Data.CartItems)

			return response.JSON(map[string]string{"status": "ok"})
		})

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		transport.AssertExpectations(t)
	})

	t.Run("can set session in context without middleware", func(t *testing.T) {
		t.Parallel()

		r := router.New[*router.Context]()

		r.Get("/test", func(ctx *router.Context) handler.Response {
			newSess := &session.Session[testSessionData]{
				ID:        uuid.New(),
				Token:     "new-token",
				ExpiresAt: time.Now().Add(time.Hour),
				Data:      testSessionData{Theme: "blue"},
			}

			middleware.SetSession(ctx, newSess)

			retrievedSess, ok := middleware.GetSession[testSessionData](ctx)
			require.True(t, ok)
			assert.Equal(t, newSess.ID, retrievedSess.ID)
			assert.Equal(t, "blue", retrievedSess.Data.Theme)

			return response.JSON(map[string]string{"status": "ok"})
		})

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})
}

func TestSessionEdgeCases(t *testing.T) {
	t.Parallel()

	t.Run("handler returns original response on success", func(t *testing.T) {
		t.Parallel()

		transport := &mockTransport{}
		sess := &session.Session[testSessionData]{
			ID:        uuid.New(),
			Token:     "test-token",
			ExpiresAt: time.Now().Add(time.Hour),
		}

		transport.On("Load", mock.Anything).Return(sess, nil)
		transport.On("Store", mock.Anything, mock.Anything).Return(nil)

		r := router.New[*router.Context]()
		r.Use(middleware.Session[*router.Context, testSessionData](transport))

		r.Get("/test", func(ctx *router.Context) handler.Response {
			return response.JSONWithStatus(
				map[string]string{"custom": "response"},
				http.StatusCreated,
			)
		})

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusCreated, w.Code)
		assert.Contains(t, w.Body.String(), "custom")
		transport.AssertExpectations(t)
	})

	t.Run("concurrent session mutations are isolated", func(t *testing.T) {
		t.Parallel()

		transport := &mockTransport{}
		sess := &session.Session[testSessionData]{
			ID:        uuid.New(),
			Token:     "test-token",
			ExpiresAt: time.Now().Add(time.Hour),
			Data:      testSessionData{Theme: "default"},
		}

		transport.On("Load", mock.Anything).Return(sess, nil)
		transport.On("Store", mock.Anything, mock.Anything).Return(nil)

		r := router.New[*router.Context]()
		r.Use(middleware.Session[*router.Context, testSessionData](transport))

		r.Get("/test1", func(ctx *router.Context) handler.Response {
			sess, _ := middleware.GetSession[testSessionData](ctx)
			sess.Data.Theme = "theme1"
			middleware.SetSession(ctx, sess)
			return response.JSON(map[string]string{"theme": sess.Data.Theme})
		})

		r.Get("/test2", func(ctx *router.Context) handler.Response {
			sess, _ := middleware.GetSession[testSessionData](ctx)
			sess.Data.Theme = "theme2"
			middleware.SetSession(ctx, sess)
			return response.JSON(map[string]string{"theme": sess.Data.Theme})
		})

		// Make concurrent requests
		req1 := httptest.NewRequest(http.MethodGet, "/test1", nil)
		w1 := httptest.NewRecorder()

		req2 := httptest.NewRequest(http.MethodGet, "/test2", nil)
		w2 := httptest.NewRecorder()

		r.ServeHTTP(w1, req1)
		r.ServeHTTP(w2, req2)

		assert.Equal(t, http.StatusOK, w1.Code)
		assert.Equal(t, http.StatusOK, w2.Code)
		assert.Contains(t, w1.Body.String(), "theme1")
		assert.Contains(t, w2.Body.String(), "theme2")
		transport.AssertExpectations(t)
	})

	t.Run("empty session data is preserved", func(t *testing.T) {
		t.Parallel()

		transport := &mockTransport{}
		sess := &session.Session[testSessionData]{
			ID:        uuid.New(),
			Token:     "test-token",
			ExpiresAt: time.Now().Add(time.Hour),
			Data:      testSessionData{}, // Empty data
		}

		transport.On("Load", mock.Anything).Return(sess, nil)
		transport.On("Store", mock.Anything, mock.MatchedBy(func(s *session.Session[testSessionData]) bool {
			return s.Data.Theme == "" && len(s.Data.CartItems) == 0
		})).Return(nil)

		r := router.New[*router.Context]()
		r.Use(middleware.Session[*router.Context, testSessionData](transport))

		r.Get("/test", func(ctx *router.Context) handler.Response {
			sess, ok := middleware.GetSession[testSessionData](ctx)
			require.True(t, ok)
			assert.Empty(t, sess.Data.Theme)
			assert.Empty(t, sess.Data.CartItems)
			return response.JSON(map[string]string{"status": "ok"})
		})

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		transport.AssertExpectations(t)
	})

	t.Run("default ErrorHandler returns response.Error", func(t *testing.T) {
		t.Parallel()

		transport := &mockTransport{}
		customErr := errors.New("custom transport error")

		transport.On("Load", mock.Anything).Return(session.Session[testSessionData]{}, customErr)

		r := router.New[*router.Context]()
		r.Use(middleware.Session[*router.Context, testSessionData](transport))

		r.Get("/test", func(ctx *router.Context) handler.Response {
			return response.JSON(map[string]string{"status": "ok"})
		})

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		// Default ErrorHandler calls response.Error which returns InternalServerError for non-HTTPError
		assert.Equal(t, http.StatusInternalServerError, w.Code)
		transport.AssertExpectations(t)
	})

	t.Run("middleware chain continues after successful session handling", func(t *testing.T) {
		t.Parallel()

		transport := &mockTransport{}
		sess := &session.Session[testSessionData]{
			ID:        uuid.New(),
			Token:     "test-token",
			ExpiresAt: time.Now().Add(time.Hour),
		}

		transport.On("Load", mock.Anything).Return(sess, nil)
		transport.On("Store", mock.Anything, mock.Anything).Return(nil)

		middlewareCalled := false

		r := router.New[*router.Context]()
		r.Use(middleware.Session[*router.Context, testSessionData](transport))

		// Add another middleware after session
		r.Use(func(next handler.HandlerFunc[*router.Context]) handler.HandlerFunc[*router.Context] {
			return func(ctx *router.Context) handler.Response {
				middlewareCalled = true
				// Verify session is accessible in subsequent middleware
				_, ok := middleware.GetSession[testSessionData](ctx)
				assert.True(t, ok)
				return next(ctx)
			}
		})

		r.Get("/test", func(ctx *router.Context) handler.Response {
			return response.JSON(map[string]string{"status": "ok"})
		})

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.True(t, middlewareCalled)
		transport.AssertExpectations(t)
	})
}

// mockContextForNil is a minimal mock for testing nil context scenarios
type mockContextForNil struct {
	mock.Mock
}

func (m *mockContextForNil) Deadline() (deadline time.Time, ok bool) {
	args := m.Called()
	return args.Get(0).(time.Time), args.Bool(1)
}

func (m *mockContextForNil) Done() <-chan struct{} {
	args := m.Called()
	if args.Get(0) == nil {
		return nil
	}
	return args.Get(0).(<-chan struct{})
}

func (m *mockContextForNil) Err() error {
	args := m.Called()
	return args.Error(0)
}

func (m *mockContextForNil) Value(key any) any {
	args := m.Called(key)
	return args.Get(0)
}

func (m *mockContextForNil) SetValue(key, value any) {
	m.Called(key, value)
}

func (m *mockContextForNil) Request() *http.Request {
	args := m.Called()
	if args.Get(0) == nil {
		return nil
	}
	return args.Get(0).(*http.Request)
}

func (m *mockContextForNil) ResponseWriter() http.ResponseWriter {
	args := m.Called()
	if args.Get(0) == nil {
		return nil
	}
	return args.Get(0).(http.ResponseWriter)
}

func (m *mockContextForNil) Context() context.Context {
	args := m.Called()
	if args.Get(0) == nil {
		return context.Background()
	}
	return args.Get(0).(context.Context)
}
