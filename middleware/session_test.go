package middleware_test

import (
	"bytes"
	"context"
	"errors"
	"log"
	"net/http"
	"net/http/httptest"
	"testing"

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

// MockTransport mocks the session transport interface
type MockTransport[Data any] struct {
	mock.Mock
}

func (m *MockTransport[Data]) Load(ctx context.Context, r *http.Request) (session.Session[Data], error) {
	args := m.Called(ctx, r)
	return args.Get(0).(session.Session[Data]), args.Error(1)
}

func (m *MockTransport[Data]) Touch(ctx context.Context, w http.ResponseWriter, sess session.Session[Data]) error {
	args := m.Called(ctx, w, sess)
	return args.Error(0)
}

type testSessionData struct {
	Theme    string
	Language string
}

func TestSession(t *testing.T) {
	t.Parallel()

	t.Run("loads session from transport and stores in context", func(t *testing.T) {
		t.Parallel()

		mockTransport := new(MockTransport[testSessionData])
		sess := session.Session[testSessionData]{
			ID:     uuid.New(),
			Token:  "test-token",
			UserID: uuid.New(),
			Data:   testSessionData{Theme: "dark", Language: "en"},
		}

		mockTransport.On("Load", mock.Anything, mock.Anything).Return(sess, nil)
		mockTransport.On("Touch", mock.Anything, mock.Anything, sess).Return(nil)

		r := router.New[*router.Context]()
		r.Use(middleware.Session[*router.Context, testSessionData](mockTransport))

		r.Get("/test", func(ctx *router.Context) handler.Response {
			retrieved, ok := middleware.GetSession[testSessionData](ctx)
			assert.True(t, ok, "Session should be available in context")
			assert.Equal(t, sess.ID, retrieved.ID)
			assert.Equal(t, sess.Token, retrieved.Token)
			assert.Equal(t, sess.UserID, retrieved.UserID)
			assert.Equal(t, "dark", retrieved.Data.Theme)
			assert.Equal(t, "en", retrieved.Data.Language)
			return response.JSON(map[string]string{"status": "ok"})
		})

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		mockTransport.AssertExpectations(t)
	})

	t.Run("calls next handler", func(t *testing.T) {
		t.Parallel()

		mockTransport := new(MockTransport[string])
		sess := session.Session[string]{
			ID:     uuid.New(),
			Token:  "token",
			UserID: uuid.Nil,
			Data:   "test-data",
		}

		mockTransport.On("Load", mock.Anything, mock.Anything).Return(sess, nil)
		mockTransport.On("Touch", mock.Anything, mock.Anything, sess).Return(nil)

		handlerCalled := false

		r := router.New[*router.Context]()
		r.Use(middleware.Session[*router.Context, string](mockTransport))

		r.Get("/test", func(ctx *router.Context) handler.Response {
			handlerCalled = true
			return response.JSON(map[string]string{"status": "ok"})
		})

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		assert.True(t, handlerCalled, "Next handler should be called")
		mockTransport.AssertExpectations(t)
	})

	t.Run("touches session after request", func(t *testing.T) {
		t.Parallel()

		mockTransport := new(MockTransport[string])
		sess := session.Session[string]{
			ID:     uuid.New(),
			Token:  "token",
			UserID: uuid.Nil,
			Data:   "test",
		}

		mockTransport.On("Load", mock.Anything, mock.Anything).Return(sess, nil)
		mockTransport.On("Touch", mock.Anything, mock.Anything, sess).Return(nil)

		r := router.New[*router.Context]()
		r.Use(middleware.Session[*router.Context, string](mockTransport))

		r.Get("/test", func(ctx *router.Context) handler.Response {
			return response.JSON(map[string]string{"status": "ok"})
		})

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		// Verify Touch was called with the loaded session
		mockTransport.AssertCalled(t, "Touch", mock.Anything, mock.Anything, sess)
		mockTransport.AssertExpectations(t)
	})

	t.Run("handles Load errors gracefully - logs and continues with empty session", func(t *testing.T) {
		// Note: Not parallel due to global log.SetOutput usage
		// Capture log output
		var logBuf bytes.Buffer
		log.SetOutput(&logBuf)
		defer log.SetOutput(nil)

		mockTransport := new(MockTransport[string])
		loadErr := errors.New("database connection failed")

		mockTransport.On("Load", mock.Anything, mock.Anything).Return(session.Session[string]{}, loadErr)
		mockTransport.On("Touch", mock.Anything, mock.Anything, mock.Anything).Return(nil)

		r := router.New[*router.Context]()
		r.Use(middleware.Session[*router.Context, string](mockTransport))

		r.Get("/test", func(ctx *router.Context) handler.Response {
			sess, ok := middleware.GetSession[string](ctx)
			assert.True(t, ok, "Empty session should still be available")
			assert.Equal(t, uuid.Nil, sess.ID, "Session ID should be nil UUID")
			assert.Equal(t, uuid.Nil, sess.UserID, "UserID should be nil UUID")
			assert.Equal(t, "", sess.Token, "Token should be empty")
			return response.JSON(map[string]string{"status": "ok"})
		})

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code, "Request should succeed despite Load error")
		assert.Contains(t, logBuf.String(), "failed to load session")
		assert.Contains(t, logBuf.String(), "database connection failed")
		mockTransport.AssertExpectations(t)
	})

	t.Run("handles Touch errors gracefully - logs and doesn't fail request", func(t *testing.T) {
		// Note: Not parallel due to global log.SetOutput usage
		// Capture log output
		var logBuf bytes.Buffer
		log.SetOutput(&logBuf)
		defer log.SetOutput(nil)

		mockTransport := new(MockTransport[string])
		sess := session.Session[string]{
			ID:     uuid.New(),
			Token:  "token",
			UserID: uuid.Nil,
			Data:   "test",
		}
		touchErr := errors.New("redis connection timeout")

		mockTransport.On("Load", mock.Anything, mock.Anything).Return(sess, nil)
		mockTransport.On("Touch", mock.Anything, mock.Anything, sess).Return(touchErr)

		r := router.New[*router.Context]()
		r.Use(middleware.Session[*router.Context, string](mockTransport))

		r.Get("/test", func(ctx *router.Context) handler.Response {
			return response.JSON(map[string]string{"status": "ok"})
		})

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code, "Request should succeed despite Touch error")
		assert.Contains(t, logBuf.String(), "failed to touch session")
		assert.Contains(t, logBuf.String(), "redis connection timeout")
		mockTransport.AssertExpectations(t)
	})

	t.Run("works with different data types", func(t *testing.T) {
		t.Parallel()

		type complexData struct {
			UserPrefs map[string]string
			Cart      []string
			Count     int
		}

		mockTransport := new(MockTransport[complexData])
		sess := session.Session[complexData]{
			ID:     uuid.New(),
			Token:  "token",
			UserID: uuid.New(),
			Data: complexData{
				UserPrefs: map[string]string{"theme": "light"},
				Cart:      []string{"item1", "item2"},
				Count:     42,
			},
		}

		mockTransport.On("Load", mock.Anything, mock.Anything).Return(sess, nil)
		mockTransport.On("Touch", mock.Anything, mock.Anything, sess).Return(nil)

		r := router.New[*router.Context]()
		r.Use(middleware.Session[*router.Context, complexData](mockTransport))

		r.Get("/test", func(ctx *router.Context) handler.Response {
			retrieved, ok := middleware.GetSession[complexData](ctx)
			assert.True(t, ok)
			assert.Equal(t, "light", retrieved.Data.UserPrefs["theme"])
			assert.Equal(t, []string{"item1", "item2"}, retrieved.Data.Cart)
			assert.Equal(t, 42, retrieved.Data.Count)
			return response.JSON(map[string]string{"status": "ok"})
		})

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		mockTransport.AssertExpectations(t)
	})
}

func TestGetSession(t *testing.T) {
	t.Parallel()

	t.Run("returns session and true when session exists in context", func(t *testing.T) {
		t.Parallel()

		mockTransport := new(MockTransport[string])
		sess := session.Session[string]{
			ID:     uuid.New(),
			Token:  "test-token",
			UserID: uuid.New(),
			Data:   "test-data",
		}

		mockTransport.On("Load", mock.Anything, mock.Anything).Return(sess, nil)
		mockTransport.On("Touch", mock.Anything, mock.Anything, sess).Return(nil)

		r := router.New[*router.Context]()
		r.Use(middleware.Session[*router.Context, string](mockTransport))

		r.Get("/test", func(ctx *router.Context) handler.Response {
			retrieved, ok := middleware.GetSession[string](ctx)
			require.True(t, ok, "Session should be available")
			assert.Equal(t, sess.ID, retrieved.ID)
			assert.Equal(t, sess.Token, retrieved.Token)
			assert.Equal(t, sess.UserID, retrieved.UserID)
			assert.Equal(t, "test-data", retrieved.Data)
			return response.JSON(map[string]string{"status": "ok"})
		})

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		mockTransport.AssertExpectations(t)
	})

	t.Run("returns empty session and false when no session in context", func(t *testing.T) {
		t.Parallel()

		r := router.New[*router.Context]()
		// No session middleware

		r.Get("/test", func(ctx *router.Context) handler.Response {
			sess, ok := middleware.GetSession[string](ctx)
			assert.False(t, ok, "Session should not be available")
			assert.Equal(t, uuid.Nil, sess.ID)
			assert.Equal(t, "", sess.Token)
			assert.Equal(t, uuid.Nil, sess.UserID)
			assert.Equal(t, "", sess.Data)
			return response.JSON(map[string]string{"status": "ok"})
		})

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("returns false when context is nil", func(t *testing.T) {
		t.Parallel()

		sess, ok := middleware.GetSession[string](nil)
		assert.False(t, ok, "Should return false for nil context")
		assert.Equal(t, session.Session[string]{}, sess)
	})
}

func TestMustGetSession(t *testing.T) {
	t.Parallel()

	t.Run("returns session when exists in context", func(t *testing.T) {
		t.Parallel()

		mockTransport := new(MockTransport[string])
		sess := session.Session[string]{
			ID:     uuid.New(),
			Token:  "test-token",
			UserID: uuid.New(),
			Data:   "test-data",
		}

		mockTransport.On("Load", mock.Anything, mock.Anything).Return(sess, nil)
		mockTransport.On("Touch", mock.Anything, mock.Anything, sess).Return(nil)

		r := router.New[*router.Context]()
		r.Use(middleware.Session[*router.Context, string](mockTransport))

		r.Get("/test", func(ctx *router.Context) handler.Response {
			retrieved := middleware.MustGetSession[string](ctx)
			assert.Equal(t, sess.ID, retrieved.ID)
			assert.Equal(t, sess.Token, retrieved.Token)
			assert.Equal(t, sess.UserID, retrieved.UserID)
			assert.Equal(t, "test-data", retrieved.Data)
			return response.JSON(map[string]string{"status": "ok"})
		})

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		mockTransport.AssertExpectations(t)
	})

	t.Run("panics when no session in context", func(t *testing.T) {
		t.Parallel()

		r := router.New[*router.Context]()
		// No session middleware

		r.Get("/test", func(ctx *router.Context) handler.Response {
			assert.Panics(t, func() {
				middleware.MustGetSession[string](ctx)
			}, "Should panic when session not in context")
			return response.JSON(map[string]string{"status": "ok"})
		})

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)
	})
}

func TestSetSession(t *testing.T) {
	t.Parallel()

	t.Run("updates session in context", func(t *testing.T) {
		t.Parallel()

		mockTransport := new(MockTransport[string])
		originalSess := session.Session[string]{
			ID:     uuid.New(),
			Token:  "original-token",
			UserID: uuid.New(),
			Data:   "original",
		}

		mockTransport.On("Load", mock.Anything, mock.Anything).Return(originalSess, nil)
		mockTransport.On("Touch", mock.Anything, mock.Anything, mock.Anything).Return(nil)

		r := router.New[*router.Context]()
		r.Use(middleware.Session[*router.Context, string](mockTransport))

		r.Get("/test", func(ctx *router.Context) handler.Response {
			// Get original session
			original, ok := middleware.GetSession[string](ctx)
			require.True(t, ok)
			assert.Equal(t, "original", original.Data)

			// Update session
			updated := original
			updated.Data = "modified"
			middleware.SetSession(ctx, updated)

			// Verify update
			retrieved, ok := middleware.GetSession[string](ctx)
			require.True(t, ok)
			assert.Equal(t, "modified", retrieved.Data)
			assert.Equal(t, original.ID, retrieved.ID, "ID should remain same")
			assert.Equal(t, original.Token, retrieved.Token, "Token should remain same")

			return response.JSON(map[string]string{"status": "ok"})
		})

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		mockTransport.AssertExpectations(t)
	})

	t.Run("subsequent GetSession returns updated session", func(t *testing.T) {
		t.Parallel()

		mockTransport := new(MockTransport[testSessionData])
		originalSess := session.Session[testSessionData]{
			ID:     uuid.New(),
			Token:  "token",
			UserID: uuid.New(),
			Data:   testSessionData{Theme: "light", Language: "fr"},
		}

		mockTransport.On("Load", mock.Anything, mock.Anything).Return(originalSess, nil)
		mockTransport.On("Touch", mock.Anything, mock.Anything, mock.Anything).Return(nil)

		r := router.New[*router.Context]()
		r.Use(middleware.Session[*router.Context, testSessionData](mockTransport))

		r.Get("/test", func(ctx *router.Context) handler.Response {
			sess, _ := middleware.GetSession[testSessionData](ctx)
			sess.Data.Theme = "dark"
			sess.Data.Language = "en"
			middleware.SetSession(ctx, sess)

			// Verify in same handler
			updated, ok := middleware.GetSession[testSessionData](ctx)
			require.True(t, ok)
			assert.Equal(t, "dark", updated.Data.Theme)
			assert.Equal(t, "en", updated.Data.Language)

			return response.JSON(map[string]string{"status": "ok"})
		})

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		mockTransport.AssertExpectations(t)
	})
}

func TestRequireAuth(t *testing.T) {
	t.Parallel()

	t.Run("allows authenticated users to proceed", func(t *testing.T) {
		t.Parallel()

		mockTransport := new(MockTransport[string])
		authenticatedUserID := uuid.New()
		sess := session.Session[string]{
			ID:     uuid.New(),
			Token:  "token",
			UserID: authenticatedUserID, // Authenticated
			Data:   "data",
		}

		mockTransport.On("Load", mock.Anything, mock.Anything).Return(sess, nil)
		mockTransport.On("Touch", mock.Anything, mock.Anything, sess).Return(nil)

		r := router.New[*router.Context]()
		r.Use(middleware.Session[*router.Context, string](mockTransport))
		r.Use(middleware.RequireAuth[*router.Context, string]())

		r.Get("/protected", func(ctx *router.Context) handler.Response {
			return response.JSON(map[string]string{"status": "authenticated"})
		})

		req := httptest.NewRequest(http.MethodGet, "/protected", nil)
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), "authenticated")
		mockTransport.AssertExpectations(t)
	})

	t.Run("returns ErrUnauthorized for anonymous users", func(t *testing.T) {
		t.Parallel()

		mockTransport := new(MockTransport[string])
		sess := session.Session[string]{
			ID:     uuid.New(),
			Token:  "token",
			UserID: uuid.Nil, // Anonymous
			Data:   "data",
		}

		mockTransport.On("Load", mock.Anything, mock.Anything).Return(sess, nil)
		mockTransport.On("Touch", mock.Anything, mock.Anything, sess).Return(nil)

		r := router.New[*router.Context]()
		r.Use(middleware.Session[*router.Context, string](mockTransport))
		r.Use(middleware.RequireAuth[*router.Context, string]())

		r.Get("/protected", func(ctx *router.Context) handler.Response {
			return response.JSON(map[string]string{"status": "should not reach"})
		})

		req := httptest.NewRequest(http.MethodGet, "/protected", nil)
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
		mockTransport.AssertExpectations(t)
	})

	t.Run("returns ErrUnauthorized when no session in context", func(t *testing.T) {
		t.Parallel()

		r := router.New[*router.Context]()
		// No session middleware
		r.Use(middleware.RequireAuth[*router.Context, string]())

		r.Get("/protected", func(ctx *router.Context) handler.Response {
			return response.JSON(map[string]string{"status": "should not reach"})
		})

		req := httptest.NewRequest(http.MethodGet, "/protected", nil)
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})
}

func TestRequireGuest(t *testing.T) {
	t.Parallel()

	t.Run("allows anonymous users to proceed", func(t *testing.T) {
		t.Parallel()

		mockTransport := new(MockTransport[string])
		sess := session.Session[string]{
			ID:     uuid.New(),
			Token:  "token",
			UserID: uuid.Nil, // Anonymous
			Data:   "data",
		}

		mockTransport.On("Load", mock.Anything, mock.Anything).Return(sess, nil)
		mockTransport.On("Touch", mock.Anything, mock.Anything, sess).Return(nil)

		r := router.New[*router.Context]()
		r.Use(middleware.Session[*router.Context, string](mockTransport))
		r.Use(middleware.RequireGuest[*router.Context, string]())

		r.Get("/login", func(ctx *router.Context) handler.Response {
			return response.JSON(map[string]string{"status": "guest allowed"})
		})

		req := httptest.NewRequest(http.MethodGet, "/login", nil)
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), "guest allowed")
		mockTransport.AssertExpectations(t)
	})

	t.Run("redirects authenticated users to /dashboard", func(t *testing.T) {
		t.Parallel()

		mockTransport := new(MockTransport[string])
		sess := session.Session[string]{
			ID:     uuid.New(),
			Token:  "token",
			UserID: uuid.New(), // Authenticated
			Data:   "data",
		}

		mockTransport.On("Load", mock.Anything, mock.Anything).Return(sess, nil)
		mockTransport.On("Touch", mock.Anything, mock.Anything, sess).Return(nil)

		r := router.New[*router.Context]()
		r.Use(middleware.Session[*router.Context, string](mockTransport))
		r.Use(middleware.RequireGuest[*router.Context, string]())

		r.Get("/login", func(ctx *router.Context) handler.Response {
			return response.JSON(map[string]string{"status": "should not reach"})
		})

		req := httptest.NewRequest(http.MethodGet, "/login", nil)
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusFound, w.Code)
		assert.Equal(t, "/dashboard", w.Header().Get("Location"))
		mockTransport.AssertExpectations(t)
	})

	t.Run("allows when no session in context", func(t *testing.T) {
		t.Parallel()

		r := router.New[*router.Context]()
		// No session middleware
		r.Use(middleware.RequireGuest[*router.Context, string]())

		r.Get("/login", func(ctx *router.Context) handler.Response {
			return response.JSON(map[string]string{"status": "no session ok"})
		})

		req := httptest.NewRequest(http.MethodGet, "/login", nil)
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), "no session ok")
	})
}
