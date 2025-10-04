package sessiontransport_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/dmitrymomot/foundation/core/cookie"
	"github.com/dmitrymomot/foundation/core/handler"
	"github.com/dmitrymomot/foundation/core/session"
	"github.com/dmitrymomot/foundation/core/sessiontransport"
	"github.com/dmitrymomot/foundation/pkg/jwt"
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

func (m *MockStore[Data]) DeleteExpired(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(1)
}

// Test data type
type testData struct {
	Key   string
	Value int
}

// Helper to create a valid cookie manager
func newTestCookieManager(t *testing.T) *cookie.Manager {
	t.Helper()
	mgr, err := cookie.New([]string{"test-secret-key-exactly-32-char!"})
	require.NoError(t, err)
	return mgr
}

// Helper to create a valid JWT service
func newTestJWTService(t *testing.T) *jwt.Service {
	t.Helper()
	svc, err := jwt.NewFromString("test-jwt-secret-key-32-chars!!")
	require.NoError(t, err)
	return svc
}

// newTestContext creates a test handler.Context for testing
func newTestContext(w http.ResponseWriter, r *http.Request) handler.Context {
	return &testContext{
		w: w,
		r: r,
	}
}

type testContext struct {
	w http.ResponseWriter
	r *http.Request
}

func (c *testContext) Deadline() (time.Time, bool) { return c.r.Context().Deadline() }
func (c *testContext) Done() <-chan struct{}       { return c.r.Context().Done() }
func (c *testContext) Err() error                  { return c.r.Context().Err() }
func (c *testContext) Value(key any) any           { return c.r.Context().Value(key) }
func (c *testContext) SetValue(key, val any) {
	ctx := context.WithValue(c.r.Context(), key, val)
	c.r = c.r.WithContext(ctx)
}
func (c *testContext) Request() *http.Request              { return c.r }
func (c *testContext) ResponseWriter() http.ResponseWriter { return c.w }
func (c *testContext) Param(key string) string             { return "" }

// ============================================================================
// Cookie Transport Tests
// ============================================================================

func TestNewCookie(t *testing.T) {
	t.Parallel()

	t.Run("creates cookie transport with valid parameters", func(t *testing.T) {
		t.Parallel()

		store := &MockStore[string]{}
		sessionMgr := session.NewManager(store, 1*time.Hour, 5*time.Minute)
		cookieMgr := newTestCookieManager(t)
		transport := sessiontransport.NewCookie(sessionMgr, cookieMgr, "session")

		assert.NotNil(t, transport)
	})

	t.Run("works with different data types", func(t *testing.T) {
		t.Parallel()

		cookieMgr := newTestCookieManager(t)

		// String data
		storeString := &MockStore[string]{}
		sessionMgrString := session.NewManager(storeString, 1*time.Hour, 5*time.Minute)
		transportString := sessiontransport.NewCookie(sessionMgrString, cookieMgr, "session")
		assert.NotNil(t, transportString)

		// Struct data
		storeStruct := &MockStore[testData]{}
		sessionMgrStruct := session.NewManager(storeStruct, 1*time.Hour, 5*time.Minute)
		transportStruct := sessiontransport.NewCookie(sessionMgrStruct, cookieMgr, "session")
		assert.NotNil(t, transportStruct)

		// Map data
		storeMap := &MockStore[map[string]interface{}]{}
		sessionMgrMap := session.NewManager(storeMap, 1*time.Hour, 5*time.Minute)
		transportMap := sessiontransport.NewCookie(sessionMgrMap, cookieMgr, "session")
		assert.NotNil(t, transportMap)
	})
}

func TestCookie_Load(t *testing.T) {
	t.Parallel()

	t.Run("loads session from valid cookie", func(t *testing.T) {
		t.Parallel()

		store := &MockStore[string]{}
		sessionMgr := session.NewManager(store, 1*time.Hour, 5*time.Minute)
		cookieMgr := newTestCookieManager(t)
		transport := sessiontransport.NewCookie(sessionMgr, cookieMgr, "session")

		// ctx not needed with handler.Context
		sessionID := uuid.New()
		token := "valid-session-token"
		expected := session.Session[string]{
			ID:        sessionID,
			Token:     token,
			UserID:    uuid.New(),
			Data:      "test-data",
			ExpiresAt: time.Now().Add(1 * time.Hour),
			CreatedAt: time.Now().Add(-30 * time.Minute),
			UpdatedAt: time.Now().Add(-30 * time.Minute),
		}

		// Create request with signed cookie
		r := httptest.NewRequest("GET", "/", nil)
		w := httptest.NewRecorder()
		require.NoError(t, cookieMgr.SetSigned(w, r, "session", token, cookie.WithEssential()))

		// Copy cookie from response to request
		for _, c := range w.Result().Cookies() {
			r.AddCookie(c)
		}

		store.On("GetByToken", mock.Anything, token).Return(expected, nil)
		// GetByToken triggers touch/save since UpdatedAt is old
		store.On("Save", mock.Anything, mock.AnythingOfType("*session.Session[string]")).Return(nil).Maybe()

		sess, err := transport.Load(newTestContext(w, r))

		require.NoError(t, err)
		assert.Equal(t, expected.ID, sess.ID)
		assert.Equal(t, expected.Token, sess.Token)
		assert.Equal(t, expected.Data, sess.Data)

		store.AssertExpectations(t)
	})

	t.Run("creates anonymous session when no cookie present", func(t *testing.T) {
		t.Parallel()

		store := &MockStore[string]{}
		sessionMgr := session.NewManager(store, 1*time.Hour, 5*time.Minute)
		cookieMgr := newTestCookieManager(t)
		transport := sessiontransport.NewCookie(sessionMgr, cookieMgr, "session")

		// ctx not needed with handler.Context
		r := httptest.NewRequest("GET", "/", nil)
		w := httptest.NewRecorder()

		store.On("Save", mock.Anything, mock.AnythingOfType("*session.Session[string]")).Return(nil)

		sess, err := transport.Load(newTestContext(w, r))

		require.NoError(t, err)
		assert.Equal(t, uuid.Nil, sess.UserID)
		assert.NotEmpty(t, sess.Token)

		store.AssertExpectations(t)
	})

	t.Run("creates anonymous session when cookie has invalid signature", func(t *testing.T) {
		t.Parallel()

		store := &MockStore[string]{}
		sessionMgr := session.NewManager(store, 1*time.Hour, 5*time.Minute)
		cookieMgr := newTestCookieManager(t)
		transport := sessiontransport.NewCookie(sessionMgr, cookieMgr, "session")

		// ctx not needed with handler.Context
		r := httptest.NewRequest("GET", "/", nil)
		w := httptest.NewRecorder()

		// Add cookie with invalid signature (not signed properly)
		r.AddCookie(&http.Cookie{
			Name:  "session",
			Value: "invalid-signature-value",
		})

		store.On("Save", mock.Anything, mock.AnythingOfType("*session.Session[string]")).Return(nil)

		sess, err := transport.Load(newTestContext(w, r))

		require.NoError(t, err)
		assert.Equal(t, uuid.Nil, sess.UserID)

		store.AssertExpectations(t)
	})

	t.Run("creates anonymous session when session not found in store", func(t *testing.T) {
		t.Parallel()

		store := &MockStore[string]{}
		sessionMgr := session.NewManager(store, 1*time.Hour, 5*time.Minute)
		cookieMgr := newTestCookieManager(t)
		transport := sessiontransport.NewCookie(sessionMgr, cookieMgr, "session")

		// ctx not needed with handler.Context
		token := "missing-session-token"

		// Create request with valid signed cookie
		r := httptest.NewRequest("GET", "/", nil)
		w := httptest.NewRecorder()
		require.NoError(t, cookieMgr.SetSigned(w, r, "session", token, cookie.WithEssential()))

		for _, c := range w.Result().Cookies() {
			r.AddCookie(c)
		}

		store.On("GetByToken", mock.Anything, token).Return(nil, session.ErrNotFound)
		store.On("Save", mock.Anything, mock.AnythingOfType("*session.Session[string]")).Return(nil)

		sess, err := transport.Load(newTestContext(w, r))

		require.NoError(t, err)
		assert.Equal(t, uuid.Nil, sess.UserID)

		store.AssertExpectations(t)
	})

	t.Run("creates anonymous session when session expired", func(t *testing.T) {
		t.Parallel()

		store := &MockStore[string]{}
		sessionMgr := session.NewManager(store, 1*time.Hour, 5*time.Minute)
		cookieMgr := newTestCookieManager(t)
		transport := sessiontransport.NewCookie(sessionMgr, cookieMgr, "session")

		// ctx not needed with handler.Context
		token := "expired-session-token"

		// Create request with valid signed cookie
		r := httptest.NewRequest("GET", "/", nil)
		w := httptest.NewRecorder()
		require.NoError(t, cookieMgr.SetSigned(w, r, "session", token, cookie.WithEssential()))

		for _, c := range w.Result().Cookies() {
			r.AddCookie(c)
		}

		store.On("GetByToken", mock.Anything, token).Return(nil, session.ErrExpired)
		store.On("Save", mock.Anything, mock.AnythingOfType("*session.Session[string]")).Return(nil)

		sess, err := transport.Load(newTestContext(w, r))

		require.NoError(t, err)
		assert.Equal(t, uuid.Nil, sess.UserID)

		store.AssertExpectations(t)
	})
}

func TestCookie_Save(t *testing.T) {
	t.Parallel()

	t.Run("sets signed cookie with session token", func(t *testing.T) {
		t.Parallel()

		store := &MockStore[string]{}
		sessionMgr := session.NewManager(store, 1*time.Hour, 5*time.Minute)
		cookieMgr := newTestCookieManager(t)
		transport := sessiontransport.NewCookie(sessionMgr, cookieMgr, "session")

		w := httptest.NewRecorder()
		r := httptest.NewRequest("GET", "/", nil)

		sess := session.Session[string]{
			ID:        uuid.New(),
			Token:     "session-token-to-save",
			UserID:    uuid.New(),
			Data:      "test-data",
			ExpiresAt: time.Now().Add(1 * time.Hour),
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}

		err := transport.Save(newTestContext(w, r), sess)

		require.NoError(t, err)

		// Verify cookie was set
		cookies := w.Result().Cookies()
		require.Len(t, cookies, 1)
		assert.Equal(t, "session", cookies[0].Name)
		assert.NotEmpty(t, cookies[0].Value)
		assert.True(t, cookies[0].HttpOnly)
		assert.Equal(t, http.SameSiteLaxMode, cookies[0].SameSite)
	})

	t.Run("sets cookie MaxAge based on session expiration", func(t *testing.T) {
		t.Parallel()

		store := &MockStore[string]{}
		sessionMgr := session.NewManager(store, 1*time.Hour, 5*time.Minute)
		cookieMgr := newTestCookieManager(t)
		transport := sessiontransport.NewCookie(sessionMgr, cookieMgr, "session")

		w := httptest.NewRecorder()
		r := httptest.NewRequest("GET", "/", nil)

		expiresIn := 30 * time.Minute
		sess := session.Session[string]{
			ID:        uuid.New(),
			Token:     "session-token",
			UserID:    uuid.New(),
			Data:      "test-data",
			ExpiresAt: time.Now().Add(expiresIn),
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}

		err := transport.Save(newTestContext(w, r), sess)

		require.NoError(t, err)

		cookies := w.Result().Cookies()
		require.Len(t, cookies, 1)
		// MaxAge should be approximately 30 minutes (allow 5 second tolerance)
		assert.InDelta(t, int(expiresIn.Seconds()), cookies[0].MaxAge, 5)
	})

	t.Run("returns error for expired session", func(t *testing.T) {
		t.Parallel()

		store := &MockStore[string]{}
		sessionMgr := session.NewManager(store, 1*time.Hour, 5*time.Minute)
		cookieMgr := newTestCookieManager(t)
		transport := sessiontransport.NewCookie(sessionMgr, cookieMgr, "session")

		w := httptest.NewRecorder()
		r := httptest.NewRequest("GET", "/", nil)

		sess := session.Session[string]{
			ID:        uuid.New(),
			Token:     "expired-token",
			UserID:    uuid.New(),
			Data:      "test-data",
			ExpiresAt: time.Now().Add(-10 * time.Minute), // Already expired
			CreatedAt: time.Now().Add(-1 * time.Hour),
			UpdatedAt: time.Now().Add(-1 * time.Hour),
		}

		err := transport.Save(newTestContext(w, r), sess)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "cannot save expired session")
	})
}

func TestCookie_Authenticate(t *testing.T) {
	t.Parallel()

	t.Run("authenticates session and sets new cookie", func(t *testing.T) {
		t.Parallel()

		store := &MockStore[string]{}
		sessionMgr := session.NewManager(store, 1*time.Hour, 5*time.Minute)
		cookieMgr := newTestCookieManager(t)
		transport := sessiontransport.NewCookie(sessionMgr, cookieMgr, "session")

		// ctx not needed with handler.Context
		userID := uuid.New()
		oldToken := "old-anonymous-token"
		oldID := uuid.New()

		// Create request with old cookie
		r := httptest.NewRequest("POST", "/login", nil)
		w := httptest.NewRecorder()
		require.NoError(t, cookieMgr.SetSigned(w, r, "session", oldToken, cookie.WithEssential()))
		for _, c := range w.Result().Cookies() {
			r.AddCookie(c)
		}

		currentSess := session.Session[string]{
			ID:        oldID,
			Token:     oldToken,
			UserID:    uuid.Nil,
			Data:      "cart-data",
			ExpiresAt: time.Now().Add(1 * time.Hour),
			CreatedAt: time.Now().Add(-30 * time.Minute),
			UpdatedAt: time.Now().Add(-30 * time.Minute),
		}

		store.On("GetByToken", mock.Anything, oldToken).Return(currentSess, nil)
		// GetByToken triggers touch/save since UpdatedAt is old
		store.On("Save", mock.Anything, mock.AnythingOfType("*session.Session[string]")).Return(nil).Maybe()
		store.On("Delete", mock.Anything, mock.AnythingOfType("uuid.UUID")).Return(nil)
		store.On("Save", mock.Anything, mock.MatchedBy(func(s *session.Session[string]) bool {
			return s.ID != oldID && s.Token != oldToken && s.UserID == userID
		})).Return(nil)

		w = httptest.NewRecorder() // Fresh recorder for authenticate response
		sess, err := transport.Authenticate(newTestContext(w, r), userID)

		require.NoError(t, err)
		assert.NotEqual(t, oldID, sess.ID)
		assert.NotEqual(t, oldToken, sess.Token)
		assert.Equal(t, userID, sess.UserID)
		assert.Equal(t, "cart-data", sess.Data)

		// Verify new cookie was set
		cookies := w.Result().Cookies()
		require.Len(t, cookies, 1)
		assert.Equal(t, "session", cookies[0].Name)

		store.AssertExpectations(t)
	})

	t.Run("returns error when load fails", func(t *testing.T) {
		t.Parallel()

		store := &MockStore[string]{}
		sessionMgr := session.NewManager(store, 1*time.Hour, 5*time.Minute)
		cookieMgr := newTestCookieManager(t)
		transport := sessiontransport.NewCookie(sessionMgr, cookieMgr, "session")

		// ctx not needed with handler.Context
		userID := uuid.New()
		r := httptest.NewRequest("POST", "/login", nil)
		w := httptest.NewRecorder()

		expectedErr := errors.New("failed to create anonymous session")
		store.On("Save", mock.Anything, mock.AnythingOfType("*session.Session[string]")).Return(expectedErr)

		_, err := transport.Authenticate(newTestContext(w, r), userID)

		assert.Error(t, err)
		assert.ErrorIs(t, err, expectedErr)

		store.AssertExpectations(t)
	})

	t.Run("returns error when authenticate fails", func(t *testing.T) {
		t.Parallel()

		store := &MockStore[string]{}
		sessionMgr := session.NewManager(store, 1*time.Hour, 5*time.Minute)
		cookieMgr := newTestCookieManager(t)
		transport := sessiontransport.NewCookie(sessionMgr, cookieMgr, "session")

		// ctx not needed with handler.Context
		userID := uuid.New()
		r := httptest.NewRequest("POST", "/login", nil)
		w := httptest.NewRecorder()

		expectedErr := errors.New("authentication failed")
		store.On("Save", mock.Anything, mock.AnythingOfType("*session.Session[string]")).Return(nil)
		store.On("Delete", mock.Anything, mock.AnythingOfType("uuid.UUID")).Return(expectedErr)

		_, err := transport.Authenticate(newTestContext(w, r), userID)

		assert.Error(t, err)
		assert.ErrorIs(t, err, expectedErr)

		store.AssertExpectations(t)
	})
}

func TestCookie_Logout(t *testing.T) {
	t.Parallel()

	t.Run("logs out session and sets new anonymous cookie", func(t *testing.T) {
		t.Parallel()

		store := &MockStore[string]{}
		sessionMgr := session.NewManager(store, 1*time.Hour, 5*time.Minute)
		cookieMgr := newTestCookieManager(t)
		transport := sessiontransport.NewCookie(sessionMgr, cookieMgr, "session")

		// ctx not needed with handler.Context
		userID := uuid.New()
		oldToken := "authenticated-token"
		oldID := uuid.New()

		// Create request with authenticated cookie
		r := httptest.NewRequest("POST", "/logout", nil)
		w := httptest.NewRecorder()
		require.NoError(t, cookieMgr.SetSigned(w, r, "session", oldToken, cookie.WithEssential()))
		for _, c := range w.Result().Cookies() {
			r.AddCookie(c)
		}

		authSess := session.Session[string]{
			ID:        oldID,
			Token:     oldToken,
			UserID:    userID,
			Data:      "user-data",
			ExpiresAt: time.Now().Add(1 * time.Hour),
			CreatedAt: time.Now().Add(-30 * time.Minute),
			UpdatedAt: time.Now().Add(-30 * time.Minute),
		}

		store.On("GetByToken", mock.Anything, oldToken).Return(authSess, nil)
		// GetByToken triggers touch/save since UpdatedAt is old
		store.On("Save", mock.Anything, mock.AnythingOfType("*session.Session[string]")).Return(nil).Maybe()
		store.On("Delete", mock.Anything, mock.AnythingOfType("uuid.UUID")).Return(nil)
		store.On("Save", mock.Anything, mock.MatchedBy(func(s *session.Session[string]) bool {
			return s.ID != oldID && s.Token != oldToken && s.UserID == uuid.Nil
		})).Return(nil)

		w = httptest.NewRecorder() // Fresh recorder
		sess, err := transport.Logout(newTestContext(w, r))

		require.NoError(t, err)
		assert.NotEqual(t, oldID, sess.ID)
		assert.NotEqual(t, oldToken, sess.Token)
		assert.Equal(t, uuid.Nil, sess.UserID)
		assert.Equal(t, "", sess.Data)

		// Verify new cookie was set
		cookies := w.Result().Cookies()
		require.Len(t, cookies, 1)
		assert.Equal(t, "session", cookies[0].Name)

		store.AssertExpectations(t)
	})

	t.Run("returns error when load fails", func(t *testing.T) {
		t.Parallel()

		store := &MockStore[string]{}
		sessionMgr := session.NewManager(store, 1*time.Hour, 5*time.Minute)
		cookieMgr := newTestCookieManager(t)
		transport := sessiontransport.NewCookie(sessionMgr, cookieMgr, "session")

		// ctx not needed with handler.Context
		r := httptest.NewRequest("POST", "/logout", nil)
		w := httptest.NewRecorder()

		expectedErr := errors.New("failed to create anonymous session")
		store.On("Save", mock.Anything, mock.AnythingOfType("*session.Session[string]")).Return(expectedErr)

		_, err := transport.Logout(newTestContext(w, r))

		assert.Error(t, err)
		assert.ErrorIs(t, err, expectedErr)

		store.AssertExpectations(t)
	})

	t.Run("returns error when logout fails", func(t *testing.T) {
		t.Parallel()

		store := &MockStore[string]{}
		sessionMgr := session.NewManager(store, 1*time.Hour, 5*time.Minute)
		cookieMgr := newTestCookieManager(t)
		transport := sessiontransport.NewCookie(sessionMgr, cookieMgr, "session")

		// ctx not needed with handler.Context
		r := httptest.NewRequest("POST", "/logout", nil)
		w := httptest.NewRecorder()

		expectedErr := errors.New("logout failed")
		store.On("Save", mock.Anything, mock.AnythingOfType("*session.Session[string]")).Return(nil)
		store.On("Delete", mock.Anything, mock.AnythingOfType("uuid.UUID")).Return(expectedErr)

		_, err := transport.Logout(newTestContext(w, r))

		assert.Error(t, err)
		assert.ErrorIs(t, err, expectedErr)

		store.AssertExpectations(t)
	})
}

func TestCookie_Delete(t *testing.T) {
	t.Parallel()

	t.Run("deletes cookie and session from store", func(t *testing.T) {
		t.Parallel()

		store := &MockStore[string]{}
		sessionMgr := session.NewManager(store, 1*time.Hour, 5*time.Minute)
		cookieMgr := newTestCookieManager(t)
		transport := sessiontransport.NewCookie(sessionMgr, cookieMgr, "session")

		// ctx not needed with handler.Context
		sessionID := uuid.New()
		token := "session-to-delete"

		// Create request with cookie
		r := httptest.NewRequest("DELETE", "/session", nil)
		w := httptest.NewRecorder()
		require.NoError(t, cookieMgr.SetSigned(w, r, "session", token, cookie.WithEssential()))
		for _, c := range w.Result().Cookies() {
			r.AddCookie(c)
		}

		sess := session.Session[string]{
			ID:        sessionID,
			Token:     token,
			UserID:    uuid.New(),
			Data:      "test-data",
			ExpiresAt: time.Now().Add(1 * time.Hour),
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}

		store.On("GetByToken", mock.Anything, token).Return(sess, nil)
		store.On("Delete", mock.Anything, sessionID).Return(nil)

		w = httptest.NewRecorder() // Fresh recorder
		err := transport.Delete(newTestContext(w, r))

		require.NoError(t, err)

		// Verify cookie was deleted (MaxAge=-1)
		cookies := w.Result().Cookies()
		require.Len(t, cookies, 1)
		assert.Equal(t, "session", cookies[0].Name)
		assert.Equal(t, -1, cookies[0].MaxAge)

		store.AssertExpectations(t)
	})

	t.Run("returns error when load fails", func(t *testing.T) {
		t.Parallel()

		store := &MockStore[string]{}
		sessionMgr := session.NewManager(store, 1*time.Hour, 5*time.Minute)
		cookieMgr := newTestCookieManager(t)
		transport := sessiontransport.NewCookie(sessionMgr, cookieMgr, "session")

		// ctx not needed with handler.Context
		r := httptest.NewRequest("DELETE", "/session", nil)
		w := httptest.NewRecorder()

		expectedErr := errors.New("failed to load session")
		store.On("Save", mock.Anything, mock.AnythingOfType("*session.Session[string]")).Return(expectedErr)

		err := transport.Delete(newTestContext(w, r))

		assert.Error(t, err)
		assert.ErrorIs(t, err, expectedErr)

		store.AssertExpectations(t)
	})

	t.Run("returns error when delete fails", func(t *testing.T) {
		t.Parallel()

		store := &MockStore[string]{}
		sessionMgr := session.NewManager(store, 1*time.Hour, 5*time.Minute)
		cookieMgr := newTestCookieManager(t)
		transport := sessiontransport.NewCookie(sessionMgr, cookieMgr, "session")

		// ctx not needed with handler.Context
		r := httptest.NewRequest("DELETE", "/session", nil)
		w := httptest.NewRecorder()

		expectedErr := errors.New("database error")
		store.On("Save", mock.Anything, mock.AnythingOfType("*session.Session[string]")).Return(nil)
		store.On("Delete", mock.Anything, mock.AnythingOfType("uuid.UUID")).Return(expectedErr)

		err := transport.Delete(newTestContext(w, r))

		assert.Error(t, err)
		assert.ErrorIs(t, err, expectedErr)

		store.AssertExpectations(t)
	})
}

func TestCookie_Touch(t *testing.T) {
	t.Parallel()

	t.Run("extends session when interval passed and updates cookie", func(t *testing.T) {
		t.Parallel()

		store := &MockStore[string]{}
		sessionMgr := session.NewManager(store, 1*time.Hour, 5*time.Minute)
		cookieMgr := newTestCookieManager(t)
		transport := sessiontransport.NewCookie(sessionMgr, cookieMgr, "session")

		w := httptest.NewRecorder()
		r := httptest.NewRequest("GET", "/", nil)
		sessionID := uuid.New()

		oldUpdateTime := time.Now().Add(-20 * time.Minute)
		newUpdateTime := time.Now()

		currentSess := session.Session[string]{
			ID:        sessionID,
			Token:     "session-token",
			UserID:    uuid.New(),
			Data:      "test-data",
			ExpiresAt: time.Now().Add(40 * time.Minute),
			CreatedAt: time.Now().Add(-1 * time.Hour),
			UpdatedAt: oldUpdateTime,
		}

		refreshedSess := session.Session[string]{
			ID:        sessionID,
			Token:     "session-token",
			UserID:    currentSess.UserID,
			Data:      "test-data",
			ExpiresAt: time.Now().Add(1 * time.Hour),
			CreatedAt: currentSess.CreatedAt,
			UpdatedAt: newUpdateTime,
		}

		store.On("GetByID", mock.Anything, sessionID).Return(refreshedSess, nil)

		err := transport.Touch(newTestContext(w, r), currentSess)

		require.NoError(t, err)

		// Verify cookie was updated
		cookies := w.Result().Cookies()
		require.Len(t, cookies, 1)
		assert.Equal(t, "session", cookies[0].Name)

		store.AssertExpectations(t)
	})

	t.Run("does not update cookie when session not refreshed", func(t *testing.T) {
		t.Parallel()

		store := &MockStore[string]{}
		sessionMgr := session.NewManager(store, 1*time.Hour, 5*time.Minute)
		cookieMgr := newTestCookieManager(t)
		transport := sessiontransport.NewCookie(sessionMgr, cookieMgr, "session")

		w := httptest.NewRecorder()
		r := httptest.NewRequest("GET", "/", nil)
		sessionID := uuid.New()

		recentUpdateTime := time.Now().Add(-2 * time.Minute)

		currentSess := session.Session[string]{
			ID:        sessionID,
			Token:     "session-token",
			UserID:    uuid.New(),
			Data:      "test-data",
			ExpiresAt: time.Now().Add(1 * time.Hour),
			CreatedAt: time.Now().Add(-30 * time.Minute),
			UpdatedAt: recentUpdateTime,
		}

		// GetByID returns same session (not touched)
		store.On("GetByID", mock.Anything, sessionID).Return(currentSess, nil)

		err := transport.Touch(newTestContext(w, r), currentSess)

		require.NoError(t, err)

		// Verify no cookie was set (response has no cookies)
		cookies := w.Result().Cookies()
		assert.Empty(t, cookies)

		store.AssertExpectations(t)
	})

	t.Run("returns error when GetByID fails", func(t *testing.T) {
		t.Parallel()

		store := &MockStore[string]{}
		sessionMgr := session.NewManager(store, 1*time.Hour, 5*time.Minute)
		cookieMgr := newTestCookieManager(t)
		transport := sessiontransport.NewCookie(sessionMgr, cookieMgr, "session")

		w := httptest.NewRecorder()
		r := httptest.NewRequest("GET", "/", nil)
		sessionID := uuid.New()

		currentSess := session.Session[string]{
			ID:        sessionID,
			Token:     "session-token",
			UserID:    uuid.New(),
			Data:      "test-data",
			ExpiresAt: time.Now().Add(1 * time.Hour),
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}

		expectedErr := session.ErrExpired
		store.On("GetByID", mock.Anything, sessionID).Return(nil, expectedErr)

		err := transport.Touch(newTestContext(w, r), currentSess)

		assert.Error(t, err)
		assert.ErrorIs(t, err, expectedErr)

		store.AssertExpectations(t)
	})
}

// ============================================================================
// JWT Transport Tests
// ============================================================================

func TestNewJWT(t *testing.T) {
	t.Parallel()

	t.Run("creates JWT transport with valid secret", func(t *testing.T) {
		t.Parallel()

		store := &MockStore[string]{}
		sessionMgr := session.NewManager(store, 24*time.Hour, 5*time.Minute)
		transport, err := sessiontransport.NewJWT(sessionMgr, "valid-secret-key-32-characters!", 15*time.Minute, "test-app")

		require.NoError(t, err)
		assert.NotNil(t, transport)
	})

	t.Run("returns error with invalid secret", func(t *testing.T) {
		t.Parallel()

		store := &MockStore[string]{}
		sessionMgr := session.NewManager(store, 24*time.Hour, 5*time.Minute)
		transport, err := sessiontransport.NewJWT(sessionMgr, "", 15*time.Minute, "test-app")

		assert.Error(t, err)
		assert.Nil(t, transport)
	})

	t.Run("works with different data types", func(t *testing.T) {
		t.Parallel()

		secret := "test-secret-key-32-characters!!!"

		// String data
		storeString := &MockStore[string]{}
		sessionMgrString := session.NewManager(storeString, 24*time.Hour, 5*time.Minute)
		transportString, err := sessiontransport.NewJWT(sessionMgrString, secret, 15*time.Minute, "app")
		require.NoError(t, err)
		assert.NotNil(t, transportString)

		// Struct data
		storeStruct := &MockStore[testData]{}
		sessionMgrStruct := session.NewManager(storeStruct, 24*time.Hour, 5*time.Minute)
		transportStruct, err := sessiontransport.NewJWT(sessionMgrStruct, secret, 15*time.Minute, "app")
		require.NoError(t, err)
		assert.NotNil(t, transportStruct)

		// Map data
		storeMap := &MockStore[map[string]interface{}]{}
		sessionMgrMap := session.NewManager(storeMap, 24*time.Hour, 5*time.Minute)
		transportMap, err := sessiontransport.NewJWT(sessionMgrMap, secret, 15*time.Minute, "app")
		require.NoError(t, err)
		assert.NotNil(t, transportMap)
	})
}

func TestJWT_Load(t *testing.T) {
	t.Parallel()

	t.Run("loads session from valid Bearer token", func(t *testing.T) {
		t.Parallel()

		store := &MockStore[string]{}
		sessionMgr := session.NewManager(store, 24*time.Hour, 5*time.Minute)
		jwtSvc := newTestJWTService(t)
		transport, err := sessiontransport.NewJWT(sessionMgr, "test-jwt-secret-key-32-chars!!", 15*time.Minute, "test-app")
		require.NoError(t, err)

		// ctx not needed with handler.Context
		sessionID := uuid.New()
		sessionToken := "session-token-in-jti"
		userID := uuid.New()

		sess := session.Session[string]{
			ID:        sessionID,
			Token:     sessionToken,
			UserID:    userID,
			Data:      "test-data",
			ExpiresAt: time.Now().Add(24 * time.Hour),
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}

		// Generate valid JWT with session token in JTI
		claims := jwt.StandardClaims{
			ID:        sessionToken,
			Subject:   userID.String(),
			Issuer:    "test-app",
			IssuedAt:  time.Now().Unix(),
			ExpiresAt: time.Now().Add(15 * time.Minute).Unix(),
		}
		jwtToken, err := jwtSvc.Generate(claims)
		require.NoError(t, err)

		r := httptest.NewRequest("GET", "/", nil)
		w := httptest.NewRecorder()
		r.Header.Set("Authorization", "Bearer "+jwtToken)

		store.On("GetByToken", mock.Anything, sessionToken).Return(sess, nil)

		loadedSess, err := transport.Load(newTestContext(w, r))

		require.NoError(t, err)
		assert.Equal(t, sess.ID, loadedSess.ID)
		assert.Equal(t, sess.Token, loadedSess.Token)
		assert.Equal(t, sess.UserID, loadedSess.UserID)

		store.AssertExpectations(t)
	})

	t.Run("returns ErrNoToken when no Authorization header", func(t *testing.T) {
		t.Parallel()

		store := &MockStore[string]{}
		sessionMgr := session.NewManager(store, 24*time.Hour, 5*time.Minute)
		transport, err := sessiontransport.NewJWT(sessionMgr, "test-jwt-secret-key-32-chars!!", 15*time.Minute, "test-app")
		require.NoError(t, err)

		_ = context.Background() // ctx not needed with handler.Context
		r := httptest.NewRequest("GET", "/", nil)
		w := httptest.NewRecorder()

		_, err = transport.Load(newTestContext(w, r))

		assert.Error(t, err)
		assert.ErrorIs(t, err, sessiontransport.ErrNoToken)
	})

	t.Run("returns ErrNoToken when Authorization header is empty", func(t *testing.T) {
		t.Parallel()

		store := &MockStore[string]{}
		sessionMgr := session.NewManager(store, 24*time.Hour, 5*time.Minute)
		transport, err := sessiontransport.NewJWT(sessionMgr, "test-jwt-secret-key-32-chars!!", 15*time.Minute, "test-app")
		require.NoError(t, err)

		_ = context.Background() // ctx not needed with handler.Context
		r := httptest.NewRequest("GET", "/", nil)
		w := httptest.NewRecorder()
		r.Header.Set("Authorization", "")

		_, err = transport.Load(newTestContext(w, r))

		assert.Error(t, err)
		assert.ErrorIs(t, err, sessiontransport.ErrNoToken)
	})

	t.Run("returns ErrNoToken when Authorization is not Bearer format", func(t *testing.T) {
		t.Parallel()

		store := &MockStore[string]{}
		sessionMgr := session.NewManager(store, 24*time.Hour, 5*time.Minute)
		transport, err := sessiontransport.NewJWT(sessionMgr, "test-jwt-secret-key-32-chars!!", 15*time.Minute, "test-app")
		require.NoError(t, err)

		_ = context.Background() // ctx not needed with handler.Context
		r := httptest.NewRequest("GET", "/", nil)
		w := httptest.NewRecorder()
		r.Header.Set("Authorization", "Basic dXNlcjpwYXNz")

		_, err = transport.Load(newTestContext(w, r))

		assert.Error(t, err)
		assert.ErrorIs(t, err, sessiontransport.ErrNoToken)
	})

	t.Run("creates anonymous session for invalid JWT", func(t *testing.T) {
		t.Parallel()

		store := &MockStore[string]{}
		sessionMgr := session.NewManager(store, 24*time.Hour, 5*time.Minute)
		transport, err := sessiontransport.NewJWT(sessionMgr, "test-jwt-secret-key-32-chars!!", 15*time.Minute, "test-app")
		require.NoError(t, err)

		// ctx not needed with handler.Context
		r := httptest.NewRequest("GET", "/", nil)
		w := httptest.NewRecorder()
		r.Header.Set("Authorization", "Bearer invalid.jwt.token")

		store.On("Save", mock.Anything, mock.AnythingOfType("*session.Session[string]")).Return(nil)

		sess, err := transport.Load(newTestContext(w, r))

		require.NoError(t, err)
		assert.Equal(t, uuid.Nil, sess.UserID)

		store.AssertExpectations(t)
	})

	t.Run("creates anonymous session when session not found", func(t *testing.T) {
		t.Parallel()

		store := &MockStore[string]{}
		sessionMgr := session.NewManager(store, 24*time.Hour, 5*time.Minute)
		jwtSvc := newTestJWTService(t)
		transport, err := sessiontransport.NewJWT(sessionMgr, "test-jwt-secret-key-32-chars!!", 15*time.Minute, "test-app")
		require.NoError(t, err)

		// ctx not needed with handler.Context
		sessionToken := "non-existent-session-token"

		claims := jwt.StandardClaims{
			ID:        sessionToken,
			Subject:   uuid.New().String(),
			Issuer:    "test-app",
			IssuedAt:  time.Now().Unix(),
			ExpiresAt: time.Now().Add(15 * time.Minute).Unix(),
		}
		jwtToken, err := jwtSvc.Generate(claims)
		require.NoError(t, err)

		r := httptest.NewRequest("GET", "/", nil)
		w := httptest.NewRecorder()
		r.Header.Set("Authorization", "Bearer "+jwtToken)

		store.On("GetByToken", mock.Anything, sessionToken).Return(nil, session.ErrNotFound)
		store.On("Save", mock.Anything, mock.AnythingOfType("*session.Session[string]")).Return(nil)

		sess, err := transport.Load(newTestContext(w, r))

		require.NoError(t, err)
		assert.Equal(t, uuid.Nil, sess.UserID)

		store.AssertExpectations(t)
	})

	t.Run("creates anonymous session when session expired", func(t *testing.T) {
		t.Parallel()

		store := &MockStore[string]{}
		sessionMgr := session.NewManager(store, 24*time.Hour, 5*time.Minute)
		jwtSvc := newTestJWTService(t)
		transport, err := sessiontransport.NewJWT(sessionMgr, "test-jwt-secret-key-32-chars!!", 15*time.Minute, "test-app")
		require.NoError(t, err)

		// ctx not needed with handler.Context
		sessionToken := "expired-session-token"

		claims := jwt.StandardClaims{
			ID:        sessionToken,
			Subject:   uuid.New().String(),
			Issuer:    "test-app",
			IssuedAt:  time.Now().Unix(),
			ExpiresAt: time.Now().Add(15 * time.Minute).Unix(),
		}
		jwtToken, err := jwtSvc.Generate(claims)
		require.NoError(t, err)

		r := httptest.NewRequest("GET", "/", nil)
		w := httptest.NewRecorder()
		r.Header.Set("Authorization", "Bearer "+jwtToken)

		store.On("GetByToken", mock.Anything, sessionToken).Return(nil, session.ErrExpired)
		store.On("Save", mock.Anything, mock.AnythingOfType("*session.Session[string]")).Return(nil)

		sess, err := transport.Load(newTestContext(w, r))

		require.NoError(t, err)
		assert.Equal(t, uuid.Nil, sess.UserID)

		store.AssertExpectations(t)
	})
}

func TestJWT_Save(t *testing.T) {
	t.Parallel()

	t.Run("persists session data to store", func(t *testing.T) {
		t.Parallel()

		store := &MockStore[string]{}
		sessionMgr := session.NewManager(store, 24*time.Hour, 5*time.Minute)
		transport, err := sessiontransport.NewJWT(sessionMgr, "test-jwt-secret-key-32-chars!!", 15*time.Minute, "test-app")
		require.NoError(t, err)

		w := httptest.NewRecorder()
		r := httptest.NewRequest("GET", "/", nil)

		sess := session.Session[string]{
			ID:        uuid.New(),
			Token:     "session-token",
			UserID:    uuid.New(),
			Data:      "test-data",
			ExpiresAt: time.Now().Add(24 * time.Hour),
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}

		// Mock expects Save to be called
		store.On("Save", mock.Anything, mock.AnythingOfType("*session.Session[string]")).Return(nil)

		err = transport.Save(newTestContext(w, r), sess)

		require.NoError(t, err)
		store.AssertExpectations(t)

		// Verify no response was written (JWT tokens are immutable)
		assert.Empty(t, w.Result().Cookies())
		assert.Empty(t, w.Body.Bytes())
	})
}

func TestJWT_Authenticate(t *testing.T) {
	t.Parallel()

	t.Run("authenticates and returns token pair with Session.Token in JTI", func(t *testing.T) {
		t.Parallel()

		store := &MockStore[string]{}
		sessionMgr := session.NewManager(store, 24*time.Hour, 5*time.Minute)
		transport, err := sessiontransport.NewJWT(sessionMgr, "test-jwt-secret-key-32-chars!!", 15*time.Minute, "test-app")
		require.NoError(t, err)

		// ctx not needed with handler.Context
		w := httptest.NewRecorder()
		r := httptest.NewRequest("POST", "/login", nil)
		w = httptest.NewRecorder()
		userID := uuid.New()

		store.On("Save", mock.Anything, mock.AnythingOfType("*session.Session[string]")).Return(nil)
		store.On("Delete", mock.Anything, mock.AnythingOfType("uuid.UUID")).Return(nil).Once()

		sess, tokens, err := transport.Authenticate(newTestContext(w, r), userID)

		require.NoError(t, err)
		assert.Equal(t, userID, sess.UserID)

		// Verify token pair
		assert.NotEmpty(t, tokens.AccessToken)
		assert.NotEmpty(t, tokens.RefreshToken)
		assert.Equal(t, "Bearer", tokens.TokenType)
		assert.Equal(t, int(15*time.Minute.Seconds()), tokens.ExpiresIn)
		assert.NotZero(t, tokens.ExpiresAt)

		// Verify Session.Token is in JWT JTI claim
		jwtSvc := newTestJWTService(t)
		var accessClaims jwt.StandardClaims
		err = jwtSvc.Parse(tokens.AccessToken, &accessClaims)
		require.NoError(t, err)
		assert.Equal(t, sess.Token, accessClaims.ID)

		var refreshClaims jwt.StandardClaims
		err = jwtSvc.Parse(tokens.RefreshToken, &refreshClaims)
		require.NoError(t, err)
		assert.Equal(t, sess.Token, refreshClaims.ID)

		store.AssertExpectations(t)
	})

	t.Run("authenticates from existing bearer token", func(t *testing.T) {
		t.Parallel()

		store := &MockStore[string]{}
		sessionMgr := session.NewManager(store, 24*time.Hour, 5*time.Minute)
		jwtSvc := newTestJWTService(t)
		transport, err := sessiontransport.NewJWT(sessionMgr, "test-jwt-secret-key-32-chars!!", 15*time.Minute, "test-app")
		require.NoError(t, err)

		// ctx not needed with handler.Context
		w := httptest.NewRecorder()
		userID := uuid.New()
		oldSessionToken := "old-session-token"
		oldID := uuid.New()

		// Create request with existing JWT
		claims := jwt.StandardClaims{
			ID:        oldSessionToken,
			Subject:   uuid.Nil.String(),
			Issuer:    "test-app",
			IssuedAt:  time.Now().Unix(),
			ExpiresAt: time.Now().Add(15 * time.Minute).Unix(),
		}
		oldJWT, err := jwtSvc.Generate(claims)
		require.NoError(t, err)

		r := httptest.NewRequest("POST", "/login", nil)
		r.Header.Set("Authorization", "Bearer "+oldJWT)

		currentSess := session.Session[string]{
			ID:        oldID,
			Token:     oldSessionToken,
			UserID:    uuid.Nil,
			Data:      "",
			ExpiresAt: time.Now().Add(24 * time.Hour),
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}

		store.On("GetByToken", mock.Anything, oldSessionToken).Return(currentSess, nil)
		store.On("Delete", mock.Anything, mock.AnythingOfType("uuid.UUID")).Return(nil)
		store.On("Save", mock.Anything, mock.MatchedBy(func(s *session.Session[string]) bool {
			return s.UserID == userID && s.Token != oldSessionToken
		})).Return(nil)

		sess, tokens, err := transport.Authenticate(newTestContext(w, r), userID)

		require.NoError(t, err)
		assert.NotEqual(t, oldSessionToken, sess.Token)
		assert.NotEmpty(t, tokens.AccessToken)

		store.AssertExpectations(t)
	})

	t.Run("returns error when authenticate fails", func(t *testing.T) {
		t.Parallel()

		store := &MockStore[string]{}
		sessionMgr := session.NewManager(store, 24*time.Hour, 5*time.Minute)
		transport, err := sessiontransport.NewJWT(sessionMgr, "test-jwt-secret-key-32-chars!!", 15*time.Minute, "test-app")
		require.NoError(t, err)

		// ctx not needed with handler.Context
		w := httptest.NewRecorder()
		r := httptest.NewRequest("POST", "/login", nil)
		w = httptest.NewRecorder()
		userID := uuid.New()

		expectedErr := errors.New("authentication failed")
		store.On("Save", mock.Anything, mock.AnythingOfType("*session.Session[string]")).Return(nil).Maybe()
		store.On("Delete", mock.Anything, mock.AnythingOfType("uuid.UUID")).Return(expectedErr).Maybe()

		_, _, err = transport.Authenticate(newTestContext(w, r), userID)

		assert.Error(t, err)
		assert.ErrorIs(t, err, expectedErr)

		store.AssertExpectations(t)
	})
}

func TestJWT_Refresh(t *testing.T) {
	t.Parallel()

	t.Run("refreshes tokens and rotates session token", func(t *testing.T) {
		t.Parallel()

		store := &MockStore[string]{}
		sessionMgr := session.NewManager(store, 24*time.Hour, 5*time.Minute)
		jwtSvc := newTestJWTService(t)
		transport, err := sessiontransport.NewJWT(sessionMgr, "test-jwt-secret-key-32-chars!!", 15*time.Minute, "test-app")
		require.NoError(t, err)

		ctx := context.Background()
		userID := uuid.New()
		oldSessionToken := "old-session-token"
		sessionID := uuid.New()

		// Create old refresh token
		oldClaims := jwt.StandardClaims{
			ID:        oldSessionToken,
			Subject:   userID.String(),
			Issuer:    "test-app",
			IssuedAt:  time.Now().Add(-1 * time.Hour).Unix(),
			ExpiresAt: time.Now().Add(23 * time.Hour).Unix(),
		}
		oldRefreshToken, err := jwtSvc.Generate(oldClaims)
		require.NoError(t, err)

		oldSess := session.Session[string]{
			ID:        sessionID,
			Token:     oldSessionToken,
			UserID:    userID,
			Data:      "user-data",
			ExpiresAt: time.Now().Add(23 * time.Hour),
			CreatedAt: time.Now().Add(-1 * time.Hour),
			UpdatedAt: time.Now().Add(-1 * time.Hour),
		}

		// New implementation: GetByToken (triggers touch/save) + Save (refresh)
		store.On("GetByToken", mock.Anything, oldSessionToken).Return(oldSess, nil)
		// First Save is from touch (extends expiration, keeps same token)
		store.On("Save", mock.Anything, mock.MatchedBy(func(s *session.Session[string]) bool {
			return s.ID == sessionID && s.Token == oldSessionToken && s.UserID == userID
		})).Return(nil).Once()
		// Second Save is from refresh (rotates token, extends expiration)
		store.On("Save", mock.Anything, mock.MatchedBy(func(s *session.Session[string]) bool {
			return s.ID == sessionID && s.Token != oldSessionToken && s.UserID == userID
		})).Return(nil).Once()

		sess, tokens, err := transport.Refresh(ctx, oldRefreshToken)

		require.NoError(t, err)

		// Verify session ID stays the same (critical for audit logs)
		assert.Equal(t, sessionID, sess.ID)

		// Verify token is rotated
		assert.NotEqual(t, oldSessionToken, sess.Token)
		assert.Equal(t, userID, sess.UserID)

		// Verify new token pair
		assert.NotEmpty(t, tokens.AccessToken)
		assert.NotEmpty(t, tokens.RefreshToken)
		assert.NotEqual(t, oldRefreshToken, tokens.RefreshToken)

		// Verify new Session.Token is in JWT JTI
		var newAccessClaims jwt.StandardClaims
		err = jwtSvc.Parse(tokens.AccessToken, &newAccessClaims)
		require.NoError(t, err)
		assert.Equal(t, sess.Token, newAccessClaims.ID)

		var newRefreshClaims jwt.StandardClaims
		err = jwtSvc.Parse(tokens.RefreshToken, &newRefreshClaims)
		require.NoError(t, err)
		assert.Equal(t, sess.Token, newRefreshClaims.ID)

		store.AssertExpectations(t)
	})

	t.Run("returns ErrInvalidToken for malformed refresh token", func(t *testing.T) {
		t.Parallel()

		store := &MockStore[string]{}
		sessionMgr := session.NewManager(store, 24*time.Hour, 5*time.Minute)
		transport, err := sessiontransport.NewJWT(sessionMgr, "test-jwt-secret-key-32-chars!!", 15*time.Minute, "test-app")
		require.NoError(t, err)

		ctx := context.Background()

		_, _, err = transport.Refresh(ctx, "invalid.jwt.token")

		assert.Error(t, err)
		assert.ErrorIs(t, err, sessiontransport.ErrInvalidToken)
	})

	t.Run("returns error when session not found", func(t *testing.T) {
		t.Parallel()

		store := &MockStore[string]{}
		sessionMgr := session.NewManager(store, 24*time.Hour, 5*time.Minute)
		jwtSvc := newTestJWTService(t)
		transport, err := sessiontransport.NewJWT(sessionMgr, "test-jwt-secret-key-32-chars!!", 15*time.Minute, "test-app")
		require.NoError(t, err)

		ctx := context.Background()
		sessionToken := "non-existent-session"

		claims := jwt.StandardClaims{
			ID:        sessionToken,
			Subject:   uuid.New().String(),
			Issuer:    "test-app",
			IssuedAt:  time.Now().Unix(),
			ExpiresAt: time.Now().Add(24 * time.Hour).Unix(),
		}
		refreshToken, err := jwtSvc.Generate(claims)
		require.NoError(t, err)

		store.On("GetByToken", mock.Anything, sessionToken).Return(nil, session.ErrNotFound)

		_, _, err = transport.Refresh(ctx, refreshToken)

		assert.Error(t, err)
		assert.ErrorIs(t, err, session.ErrNotFound)

		store.AssertExpectations(t)
	})

	t.Run("returns error when refresh fails", func(t *testing.T) {
		t.Parallel()

		store := &MockStore[string]{}
		sessionMgr := session.NewManager(store, 24*time.Hour, 5*time.Minute)
		jwtSvc := newTestJWTService(t)
		transport, err := sessiontransport.NewJWT(sessionMgr, "test-jwt-secret-key-32-chars!!", 15*time.Minute, "test-app")
		require.NoError(t, err)

		ctx := context.Background()
		userID := uuid.New()
		sessionToken := "session-token"
		sessionID := uuid.New()

		claims := jwt.StandardClaims{
			ID:        sessionToken,
			Subject:   userID.String(),
			Issuer:    "test-app",
			IssuedAt:  time.Now().Unix(),
			ExpiresAt: time.Now().Add(24 * time.Hour).Unix(),
		}
		refreshToken, err := jwtSvc.Generate(claims)
		require.NoError(t, err)

		sess := session.Session[string]{
			ID:        sessionID,
			Token:     sessionToken,
			UserID:    userID,
			Data:      "user-data",
			ExpiresAt: time.Now().Add(24 * time.Hour),
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}

		expectedErr := errors.New("save failed")
		store.On("GetByToken", mock.Anything, sessionToken).Return(sess, nil)
		store.On("Save", mock.Anything, mock.Anything).Return(expectedErr)

		_, _, err = transport.Refresh(ctx, refreshToken)

		assert.Error(t, err)
		assert.ErrorIs(t, err, expectedErr)

		store.AssertExpectations(t)
	})

	t.Run("returns ErrNotAuthenticated for anonymous session", func(t *testing.T) {
		t.Parallel()

		store := &MockStore[string]{}
		sessionMgr := session.NewManager(store, 24*time.Hour, 5*time.Minute)
		jwtSvc := newTestJWTService(t)
		transport, err := sessiontransport.NewJWT(sessionMgr, "test-jwt-secret-key-32-chars!!", 15*time.Minute, "test-app")
		require.NoError(t, err)

		ctx := context.Background()
		sessionToken := "anonymous-session-token"

		claims := jwt.StandardClaims{
			ID:        sessionToken,
			Subject:   uuid.Nil.String(), // Anonymous
			Issuer:    "test-app",
			IssuedAt:  time.Now().Unix(),
			ExpiresAt: time.Now().Add(24 * time.Hour).Unix(),
		}
		refreshToken, err := jwtSvc.Generate(claims)
		require.NoError(t, err)

		anonymousSess := session.Session[string]{
			ID:        uuid.New(),
			Token:     sessionToken,
			UserID:    uuid.Nil, // Anonymous session
			Data:      "cart-data",
			ExpiresAt: time.Now().Add(24 * time.Hour),
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}

		store.On("GetByToken", mock.Anything, sessionToken).Return(anonymousSess, nil)

		_, _, err = transport.Refresh(ctx, refreshToken)

		assert.Error(t, err)
		assert.ErrorIs(t, err, session.ErrNotAuthenticated)

		store.AssertExpectations(t)
		// Save should not be called for anonymous sessions
		store.AssertNotCalled(t, "Save", mock.Anything, mock.Anything)
	})
}

func TestJWT_Logout(t *testing.T) {
	t.Parallel()

	t.Run("deletes session", func(t *testing.T) {
		t.Parallel()

		store := &MockStore[string]{}
		sessionMgr := session.NewManager(store, 24*time.Hour, 5*time.Minute)
		jwtSvc := newTestJWTService(t)
		transport, err := sessiontransport.NewJWT(sessionMgr, "test-jwt-secret-key-32-chars!!", 15*time.Minute, "test-app")
		require.NoError(t, err)

		w := httptest.NewRecorder()
		r := httptest.NewRequest("GET", "/", nil)
		sessionID := uuid.New()
		sessionToken := "session-to-delete"
		userID := uuid.New()

		// Create JWT
		claims := jwt.StandardClaims{
			ID:        sessionToken,
			Subject:   userID.String(),
			Issuer:    "test-app",
			IssuedAt:  time.Now().Unix(),
			ExpiresAt: time.Now().Add(15 * time.Minute).Unix(),
		}
		jwtToken, err := jwtSvc.Generate(claims)
		require.NoError(t, err)

		_ = context.Background() // ctx not needed with handler.Context
		r = httptest.NewRequest("POST", "/logout", nil)
		r.Header.Set("Authorization", "Bearer "+jwtToken)

		sess := session.Session[string]{
			ID:        sessionID,
			Token:     sessionToken,
			UserID:    userID,
			Data:      "user-data",
			ExpiresAt: time.Now().Add(24 * time.Hour),
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}

		store.On("GetByToken", mock.Anything, sessionToken).Return(sess, nil)
		store.On("Delete", mock.Anything, sessionID).Return(nil)

		err = transport.Logout(newTestContext(w, r))

		require.NoError(t, err)

		store.AssertExpectations(t)
	})

	t.Run("returns nil when no token present", func(t *testing.T) {
		t.Parallel()

		store := &MockStore[string]{}
		sessionMgr := session.NewManager(store, 24*time.Hour, 5*time.Minute)
		transport, err := sessiontransport.NewJWT(sessionMgr, "test-jwt-secret-key-32-chars!!", 15*time.Minute, "test-app")
		require.NoError(t, err)

		_ = context.Background() // ctx not needed with handler.Context
		w := httptest.NewRecorder()
		r := httptest.NewRequest("POST", "/logout", nil)
		w = httptest.NewRecorder()

		err = transport.Logout(newTestContext(w, r))

		require.NoError(t, err) // No error when already logged out
	})

	t.Run("returns error when delete fails", func(t *testing.T) {
		t.Parallel()

		store := &MockStore[string]{}
		sessionMgr := session.NewManager(store, 24*time.Hour, 5*time.Minute)
		jwtSvc := newTestJWTService(t)
		transport, err := sessiontransport.NewJWT(sessionMgr, "test-jwt-secret-key-32-chars!!", 15*time.Minute, "test-app")
		require.NoError(t, err)

		w := httptest.NewRecorder()
		r := httptest.NewRequest("GET", "/", nil)
		sessionID := uuid.New()
		sessionToken := "session-token"

		claims := jwt.StandardClaims{
			ID:        sessionToken,
			Subject:   uuid.New().String(),
			Issuer:    "test-app",
			IssuedAt:  time.Now().Unix(),
			ExpiresAt: time.Now().Add(15 * time.Minute).Unix(),
		}
		jwtToken, err := jwtSvc.Generate(claims)
		require.NoError(t, err)

		_ = context.Background() // ctx not needed with handler.Context
		r = httptest.NewRequest("POST", "/logout", nil)
		r.Header.Set("Authorization", "Bearer "+jwtToken)

		sess := session.Session[string]{
			ID:        sessionID,
			Token:     sessionToken,
			UserID:    uuid.New(),
			Data:      "user-data",
			ExpiresAt: time.Now().Add(24 * time.Hour),
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}

		expectedErr := errors.New("database error")
		store.On("GetByToken", mock.Anything, sessionToken).Return(sess, nil)
		store.On("Delete", mock.Anything, sessionID).Return(expectedErr)

		err = transport.Logout(newTestContext(w, r))

		assert.Error(t, err)
		assert.ErrorIs(t, err, expectedErr)

		store.AssertExpectations(t)
	})

	t.Run("returns error when delete fails", func(t *testing.T) {
		t.Parallel()

		store := &MockStore[string]{}
		sessionMgr := session.NewManager(store, 24*time.Hour, 5*time.Minute)
		jwtSvc := newTestJWTService(t)
		transport, err := sessiontransport.NewJWT(sessionMgr, "test-jwt-secret-key-32-chars!!", 15*time.Minute, "test-app")
		require.NoError(t, err)

		w := httptest.NewRecorder()
		r := httptest.NewRequest("GET", "/", nil)
		sessionID := uuid.New()
		sessionToken := "session-token"

		claims := jwt.StandardClaims{
			ID:        sessionToken,
			Subject:   uuid.New().String(),
			Issuer:    "test-app",
			IssuedAt:  time.Now().Unix(),
			ExpiresAt: time.Now().Add(15 * time.Minute).Unix(),
		}
		jwtToken, err := jwtSvc.Generate(claims)
		require.NoError(t, err)

		_ = context.Background() // ctx not needed with handler.Context
		r = httptest.NewRequest("POST", "/logout", nil)
		r.Header.Set("Authorization", "Bearer "+jwtToken)

		sess := session.Session[string]{
			ID:        sessionID,
			Token:     sessionToken,
			UserID:    uuid.New(),
			Data:      "user-data",
			ExpiresAt: time.Now().Add(24 * time.Hour),
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}

		expectedErr := errors.New("delete failed")
		store.On("GetByToken", mock.Anything, sessionToken).Return(sess, nil)
		store.On("Delete", mock.Anything, sessionID).Return(expectedErr)

		err = transport.Logout(newTestContext(w, r))

		assert.Error(t, err)
		assert.ErrorIs(t, err, expectedErr)

		store.AssertExpectations(t)
	})
}

func TestJWT_Delete(t *testing.T) {
	t.Parallel()

	t.Run("deletes session from store", func(t *testing.T) {
		t.Parallel()

		store := &MockStore[string]{}
		sessionMgr := session.NewManager(store, 24*time.Hour, 5*time.Minute)
		jwtSvc := newTestJWTService(t)
		transport, err := sessiontransport.NewJWT(sessionMgr, "test-jwt-secret-key-32-chars!!", 15*time.Minute, "test-app")
		require.NoError(t, err)

		w := httptest.NewRecorder()
		r := httptest.NewRequest("GET", "/", nil)
		sessionID := uuid.New()
		sessionToken := "session-to-delete"

		claims := jwt.StandardClaims{
			ID:        sessionToken,
			Subject:   uuid.New().String(),
			Issuer:    "test-app",
			IssuedAt:  time.Now().Unix(),
			ExpiresAt: time.Now().Add(15 * time.Minute).Unix(),
		}
		jwtToken, err := jwtSvc.Generate(claims)
		require.NoError(t, err)

		_ = context.Background() // ctx not needed with handler.Context
		r = httptest.NewRequest("DELETE", "/session", nil)
		r.Header.Set("Authorization", "Bearer "+jwtToken)

		sess := session.Session[string]{
			ID:        sessionID,
			Token:     sessionToken,
			UserID:    uuid.New(),
			Data:      "user-data",
			ExpiresAt: time.Now().Add(24 * time.Hour),
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}

		store.On("GetByToken", mock.Anything, sessionToken).Return(sess, nil)
		store.On("Delete", mock.Anything, sessionID).Return(nil)

		err = transport.Delete(newTestContext(w, r))

		require.NoError(t, err)

		store.AssertExpectations(t)
	})

	t.Run("returns nil when no token present", func(t *testing.T) {
		t.Parallel()

		store := &MockStore[string]{}
		sessionMgr := session.NewManager(store, 24*time.Hour, 5*time.Minute)
		transport, err := sessiontransport.NewJWT(sessionMgr, "test-jwt-secret-key-32-chars!!", 15*time.Minute, "test-app")
		require.NoError(t, err)

		_ = context.Background() // ctx not needed with handler.Context
		w := httptest.NewRecorder()
		r := httptest.NewRequest("DELETE", "/session", nil)
		w = httptest.NewRecorder()

		err = transport.Delete(newTestContext(w, r))

		require.NoError(t, err) // No error when no session to delete
	})

	t.Run("returns error when delete fails", func(t *testing.T) {
		t.Parallel()

		store := &MockStore[string]{}
		sessionMgr := session.NewManager(store, 24*time.Hour, 5*time.Minute)
		jwtSvc := newTestJWTService(t)
		transport, err := sessiontransport.NewJWT(sessionMgr, "test-jwt-secret-key-32-chars!!", 15*time.Minute, "test-app")
		require.NoError(t, err)

		w := httptest.NewRecorder()
		r := httptest.NewRequest("GET", "/", nil)
		sessionID := uuid.New()
		sessionToken := "session-token"

		claims := jwt.StandardClaims{
			ID:        sessionToken,
			Subject:   uuid.New().String(),
			Issuer:    "test-app",
			IssuedAt:  time.Now().Unix(),
			ExpiresAt: time.Now().Add(15 * time.Minute).Unix(),
		}
		jwtToken, err := jwtSvc.Generate(claims)
		require.NoError(t, err)

		_ = context.Background() // ctx not needed with handler.Context
		r = httptest.NewRequest("DELETE", "/session", nil)
		r.Header.Set("Authorization", "Bearer "+jwtToken)

		sess := session.Session[string]{
			ID:        sessionID,
			Token:     sessionToken,
			UserID:    uuid.New(),
			Data:      "user-data",
			ExpiresAt: time.Now().Add(24 * time.Hour),
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}

		expectedErr := errors.New("database error")
		store.On("GetByToken", mock.Anything, sessionToken).Return(sess, nil)
		store.On("Delete", mock.Anything, sessionID).Return(expectedErr)

		err = transport.Delete(newTestContext(w, r))

		assert.Error(t, err)
		assert.ErrorIs(t, err, expectedErr)

		store.AssertExpectations(t)
	})
}

func TestJWT_Touch(t *testing.T) {
	t.Parallel()

	t.Run("extends session when interval passed", func(t *testing.T) {
		t.Parallel()

		store := &MockStore[string]{}
		sessionMgr := session.NewManager(store, 24*time.Hour, 5*time.Minute)
		transport, err := sessiontransport.NewJWT(sessionMgr, "test-jwt-secret-key-32-chars!!", 15*time.Minute, "test-app")
		require.NoError(t, err)

		w := httptest.NewRecorder()
		r := httptest.NewRequest("GET", "/", nil)
		sessionID := uuid.New()

		oldUpdateTime := time.Now().Add(-20 * time.Minute)
		newUpdateTime := time.Now()

		currentSess := session.Session[string]{
			ID:        sessionID,
			Token:     "session-token",
			UserID:    uuid.New(),
			Data:      "test-data",
			ExpiresAt: time.Now().Add(4 * time.Hour),
			CreatedAt: time.Now().Add(-1 * time.Hour),
			UpdatedAt: oldUpdateTime,
		}

		refreshedSess := session.Session[string]{
			ID:        sessionID,
			Token:     "session-token",
			UserID:    currentSess.UserID,
			Data:      "test-data",
			ExpiresAt: time.Now().Add(24 * time.Hour),
			CreatedAt: currentSess.CreatedAt,
			UpdatedAt: newUpdateTime,
		}

		store.On("GetByID", mock.Anything, sessionID).Return(refreshedSess, nil)

		err = transport.Touch(newTestContext(w, r), currentSess)

		require.NoError(t, err)

		// JWT is immutable, so no response should be written
		assert.Empty(t, w.Body.Bytes())

		store.AssertExpectations(t)
	})

	t.Run("does nothing when session not refreshed", func(t *testing.T) {
		t.Parallel()

		store := &MockStore[string]{}
		sessionMgr := session.NewManager(store, 24*time.Hour, 5*time.Minute)
		transport, err := sessiontransport.NewJWT(sessionMgr, "test-jwt-secret-key-32-chars!!", 15*time.Minute, "test-app")
		require.NoError(t, err)

		w := httptest.NewRecorder()
		r := httptest.NewRequest("GET", "/", nil)
		sessionID := uuid.New()

		sess := session.Session[string]{
			ID:        sessionID,
			Token:     "session-token",
			UserID:    uuid.New(),
			Data:      "test-data",
			ExpiresAt: time.Now().Add(24 * time.Hour),
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}

		store.On("GetByID", mock.Anything, sessionID).Return(sess, nil)

		err = transport.Touch(newTestContext(w, r), sess)

		require.NoError(t, err)

		store.AssertExpectations(t)
	})

	t.Run("returns error when GetByID fails", func(t *testing.T) {
		t.Parallel()

		store := &MockStore[string]{}
		sessionMgr := session.NewManager(store, 24*time.Hour, 5*time.Minute)
		transport, err := sessiontransport.NewJWT(sessionMgr, "test-jwt-secret-key-32-chars!!", 15*time.Minute, "test-app")
		require.NoError(t, err)

		w := httptest.NewRecorder()
		r := httptest.NewRequest("GET", "/", nil)
		sessionID := uuid.New()

		sess := session.Session[string]{
			ID:        sessionID,
			Token:     "session-token",
			UserID:    uuid.New(),
			Data:      "test-data",
			ExpiresAt: time.Now().Add(24 * time.Hour),
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}

		expectedErr := session.ErrExpired
		store.On("GetByID", mock.Anything, sessionID).Return(nil, expectedErr)

		err = transport.Touch(newTestContext(w, r), sess)

		assert.Error(t, err)
		assert.ErrorIs(t, err, expectedErr)

		store.AssertExpectations(t)
	})
}

func TestExtractBearerToken(t *testing.T) {
	t.Parallel()

	t.Run("extracts token from valid Bearer format", func(t *testing.T) {
		t.Parallel()

		store := &MockStore[string]{}
		sessionMgr := session.NewManager(store, 24*time.Hour, 5*time.Minute)
		jwtSvc := newTestJWTService(t)
		transport, err := sessiontransport.NewJWT(sessionMgr, "test-jwt-secret-key-32-chars!!", 15*time.Minute, "test-app")
		require.NoError(t, err)

		// ctx not needed with handler.Context
		sessionToken := "valid-session-token"

		claims := jwt.StandardClaims{
			ID:        sessionToken,
			Subject:   uuid.New().String(),
			Issuer:    "test-app",
			IssuedAt:  time.Now().Unix(),
			ExpiresAt: time.Now().Add(15 * time.Minute).Unix(),
		}
		jwtToken, err := jwtSvc.Generate(claims)
		require.NoError(t, err)

		r := httptest.NewRequest("GET", "/", nil)
		w := httptest.NewRecorder()
		r.Header.Set("Authorization", "Bearer "+jwtToken)

		sess := session.Session[string]{
			ID:        uuid.New(),
			Token:     sessionToken,
			UserID:    uuid.New(),
			Data:      "test-data",
			ExpiresAt: time.Now().Add(24 * time.Hour),
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}

		store.On("GetByToken", mock.Anything, sessionToken).Return(sess, nil)

		loadedSess, err := transport.Load(newTestContext(w, r))

		require.NoError(t, err)
		assert.Equal(t, sess.Token, loadedSess.Token)

		store.AssertExpectations(t)
	})

	t.Run("returns empty string for missing Authorization header", func(t *testing.T) {
		t.Parallel()

		store := &MockStore[string]{}
		sessionMgr := session.NewManager(store, 24*time.Hour, 5*time.Minute)
		transport, err := sessiontransport.NewJWT(sessionMgr, "test-jwt-secret-key-32-chars!!", 15*time.Minute, "test-app")
		require.NoError(t, err)

		_ = context.Background() // ctx not needed with handler.Context
		r := httptest.NewRequest("GET", "/", nil)
		w := httptest.NewRecorder()

		_, err = transport.Load(newTestContext(w, r))

		assert.ErrorIs(t, err, sessiontransport.ErrNoToken)
	})

	t.Run("returns empty string for invalid format", func(t *testing.T) {
		t.Parallel()

		store := &MockStore[string]{}
		sessionMgr := session.NewManager(store, 24*time.Hour, 5*time.Minute)
		transport, err := sessiontransport.NewJWT(sessionMgr, "test-jwt-secret-key-32-chars!!", 15*time.Minute, "test-app")
		require.NoError(t, err)

		// ctx not needed with handler.Context

		testCases := []string{
			"NoBearer token-value",
			"Bearer",      // Missing token
			"token-value", // Missing Bearer prefix
		}

		// When token extraction fails, Load creates new anonymous session
		store.On("Save", mock.Anything, mock.AnythingOfType("*session.Session[string]")).Return(nil).Maybe()

		for _, tc := range testCases {
			r := httptest.NewRequest("GET", "/", nil)
			w := httptest.NewRecorder()
			r.Header.Set("Authorization", tc)

			_, err := transport.Load(newTestContext(w, r))

			assert.ErrorIs(t, err, sessiontransport.ErrNoToken, "Failed for: %s", tc)
		}
	})

	t.Run("handles case-insensitive Bearer prefix", func(t *testing.T) {
		t.Parallel()

		store := &MockStore[string]{}
		sessionMgr := session.NewManager(store, 24*time.Hour, 5*time.Minute)
		jwtSvc := newTestJWTService(t)
		transport, err := sessiontransport.NewJWT(sessionMgr, "test-jwt-secret-key-32-chars!!", 15*time.Minute, "test-app")
		require.NoError(t, err)

		// ctx not needed with handler.Context
		sessionToken := "session-token"

		claims := jwt.StandardClaims{
			ID:        sessionToken,
			Subject:   uuid.New().String(),
			Issuer:    "test-app",
			IssuedAt:  time.Now().Unix(),
			ExpiresAt: time.Now().Add(15 * time.Minute).Unix(),
		}
		jwtToken, err := jwtSvc.Generate(claims)
		require.NoError(t, err)

		testCases := []string{
			"Bearer " + jwtToken,
			"bearer " + jwtToken,
			"BEARER " + jwtToken,
			"BeArEr " + jwtToken,
		}

		for _, tc := range testCases {
			r := httptest.NewRequest("GET", "/", nil)
			w := httptest.NewRecorder()
			r.Header.Set("Authorization", tc)

			sess := session.Session[string]{
				ID:        uuid.New(),
				Token:     sessionToken,
				UserID:    uuid.New(),
				Data:      "test-data",
				ExpiresAt: time.Now().Add(24 * time.Hour),
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			}

			store.On("GetByToken", mock.Anything, sessionToken).Return(sess, nil).Once()

			loadedSess, err := transport.Load(newTestContext(w, r))

			require.NoError(t, err, "Failed for: %s", tc)
			assert.Equal(t, sessionToken, loadedSess.Token)
		}

		store.AssertExpectations(t)
	})

	t.Run("extracts token with spaces in value", func(t *testing.T) {
		t.Parallel()

		store := &MockStore[string]{}
		sessionMgr := session.NewManager(store, 24*time.Hour, 5*time.Minute)
		jwtSvc := newTestJWTService(t)
		transport, err := sessiontransport.NewJWT(sessionMgr, "test-jwt-secret-key-32-chars!!", 15*time.Minute, "test-app")
		require.NoError(t, err)

		// ctx not needed with handler.Context
		sessionToken := "session-token"

		claims := jwt.StandardClaims{
			ID:        sessionToken,
			Subject:   uuid.New().String(),
			Issuer:    "test-app",
			IssuedAt:  time.Now().Unix(),
			ExpiresAt: time.Now().Add(15 * time.Minute).Unix(),
		}
		jwtToken, err := jwtSvc.Generate(claims)
		require.NoError(t, err)

		// JWT might contain spaces in value portion (unlikely but possible in malformed tokens)
		r := httptest.NewRequest("GET", "/", nil)
		w := httptest.NewRecorder()
		r.Header.Set("Authorization", "Bearer "+jwtToken+" extra")

		// Split on first space only, so "jwtToken extra" is the extracted value
		tokenWithExtra := jwtToken + " extra"

		// Parse will fail for invalid JWT, creating anonymous session
		store.On("Save", mock.Anything, mock.AnythingOfType("*session.Session[string]")).Return(nil)

		sess, err := transport.Load(newTestContext(w, r))

		require.NoError(t, err)
		assert.Equal(t, uuid.Nil, sess.UserID)

		// Verify the extracted token was the full value after "Bearer "
		_ = tokenWithExtra // Not directly testable without exposing extractBearerToken

		store.AssertExpectations(t)
	})
}

// ============================================================================
// Integration Tests - Token Rotation and Real Implementations
// ============================================================================

func TestCookieTransport_Integration_TokenRotation(t *testing.T) {
	t.Parallel()

	t.Run("verifies token rotation on authenticate", func(t *testing.T) {
		t.Parallel()

		// Use mock store but real cookie and session managers
		store := &MockStore[string]{}
		sessionMgr := session.NewManager(store, 1*time.Hour, 5*time.Minute)
		cookieMgr := newTestCookieManager(t)
		transport := sessiontransport.NewCookie(sessionMgr, cookieMgr, "session")

		// ctx not needed with handler.Context
		userID := uuid.New()

		// Create initial anonymous session
		anonID := uuid.New()
		anonToken := "anonymous-token-12345"
		anonSess := session.Session[string]{
			ID:        anonID,
			Token:     anonToken,
			UserID:    uuid.Nil,
			Data:      "",
			ExpiresAt: time.Now().Add(1 * time.Hour),
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}

		// Setup request with anonymous cookie
		r := httptest.NewRequest("POST", "/login", nil)
		w := httptest.NewRecorder()
		require.NoError(t, cookieMgr.SetSigned(w, r, "session", anonToken, cookie.WithEssential()))
		for _, c := range w.Result().Cookies() {
			r.AddCookie(c)
		}

		store.On("GetByToken", mock.Anything, anonToken).Return(anonSess, nil)
		store.On("Delete", mock.Anything, anonID).Return(nil)
		store.On("Save", mock.Anything, mock.MatchedBy(func(s *session.Session[string]) bool {
			// Verify new session has different ID and token
			return s.ID != anonID && s.Token != anonToken && s.UserID == userID
		})).Return(nil)

		w = httptest.NewRecorder()
		authSess, err := transport.Authenticate(newTestContext(w, r), userID)

		require.NoError(t, err)
		assert.NotEqual(t, anonID, authSess.ID)
		assert.NotEqual(t, anonToken, authSess.Token)
		assert.Equal(t, userID, authSess.UserID)

		// Verify new cookie was set with new token
		cookies := w.Result().Cookies()
		require.Len(t, cookies, 1)

		store.AssertExpectations(t)
	})

	t.Run("verifies token rotation on logout", func(t *testing.T) {
		t.Parallel()

		store := &MockStore[string]{}
		sessionMgr := session.NewManager(store, 1*time.Hour, 5*time.Minute)
		cookieMgr := newTestCookieManager(t)
		transport := sessiontransport.NewCookie(sessionMgr, cookieMgr, "session")

		// ctx not needed with handler.Context
		authID := uuid.New()
		authToken := "authenticated-token-12345"
		userID := uuid.New()

		authSess := session.Session[string]{
			ID:        authID,
			Token:     authToken,
			UserID:    userID,
			Data:      "user-data",
			ExpiresAt: time.Now().Add(1 * time.Hour),
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}

		// Setup request with authenticated cookie
		r := httptest.NewRequest("POST", "/logout", nil)
		w := httptest.NewRecorder()
		require.NoError(t, cookieMgr.SetSigned(w, r, "session", authToken, cookie.WithEssential()))
		for _, c := range w.Result().Cookies() {
			r.AddCookie(c)
		}

		store.On("GetByToken", mock.Anything, authToken).Return(authSess, nil)
		store.On("Delete", mock.Anything, authID).Return(nil)
		store.On("Save", mock.Anything, mock.MatchedBy(func(s *session.Session[string]) bool {
			return s.ID != authID && s.Token != authToken && s.UserID == uuid.Nil
		})).Return(nil)

		w = httptest.NewRecorder()
		anonSess, err := transport.Logout(newTestContext(w, r))

		require.NoError(t, err)
		assert.NotEqual(t, authID, anonSess.ID)
		assert.NotEqual(t, authToken, anonSess.Token)
		assert.Equal(t, uuid.Nil, anonSess.UserID)
		assert.Equal(t, "", anonSess.Data)

		store.AssertExpectations(t)
	})
}

func TestJWTTransport_Integration_TokenRotation(t *testing.T) {
	t.Parallel()

	t.Run("verifies Session.Token in JWT JTI claim", func(t *testing.T) {
		t.Parallel()

		store := &MockStore[string]{}
		sessionMgr := session.NewManager(store, 24*time.Hour, 5*time.Minute)
		transport, err := sessiontransport.NewJWT(sessionMgr, "test-jwt-secret-key-32-chars!!", 15*time.Minute, "test-app")
		require.NoError(t, err)

		// ctx not needed with handler.Context
		w := httptest.NewRecorder()
		r := httptest.NewRequest("POST", "/login", nil)
		w = httptest.NewRecorder()
		userID := uuid.New()

		store.On("Save", mock.Anything, mock.AnythingOfType("*session.Session[string]")).Return(nil)
		store.On("Delete", mock.Anything, mock.AnythingOfType("uuid.UUID")).Return(nil).Once()

		sess, tokens, err := transport.Authenticate(newTestContext(w, r), userID)

		require.NoError(t, err)
		assert.NotEmpty(t, tokens.AccessToken)
		assert.NotEmpty(t, tokens.RefreshToken)

		// Parse JWT and verify Session.Token is in JTI
		jwtSvc := newTestJWTService(t)

		var accessClaims jwt.StandardClaims
		err = jwtSvc.Parse(tokens.AccessToken, &accessClaims)
		require.NoError(t, err)
		assert.Equal(t, sess.Token, accessClaims.ID, "Access token JTI should equal Session.Token")
		assert.Equal(t, userID.String(), accessClaims.Subject)

		var refreshClaims jwt.StandardClaims
		err = jwtSvc.Parse(tokens.RefreshToken, &refreshClaims)
		require.NoError(t, err)
		assert.Equal(t, sess.Token, refreshClaims.ID, "Refresh token JTI should equal Session.Token")
		assert.Equal(t, userID.String(), refreshClaims.Subject)

		store.AssertExpectations(t)
	})

	t.Run("verifies token rotation on refresh", func(t *testing.T) {
		t.Parallel()

		store := &MockStore[string]{}
		sessionMgr := session.NewManager(store, 24*time.Hour, 5*time.Minute)
		jwtSvc := newTestJWTService(t)
		transport, err := sessiontransport.NewJWT(sessionMgr, "test-jwt-secret-key-32-chars!!", 15*time.Minute, "test-app")
		require.NoError(t, err)

		ctx := context.Background()
		userID := uuid.New()
		sessionID := uuid.New()
		oldSessionToken := "old-session-token-12345"

		// Create old refresh token
		oldClaims := jwt.StandardClaims{
			ID:        oldSessionToken,
			Subject:   userID.String(),
			Issuer:    "test-app",
			IssuedAt:  time.Now().Add(-1 * time.Hour).Unix(),
			ExpiresAt: time.Now().Add(23 * time.Hour).Unix(),
		}
		oldRefreshToken, err := jwtSvc.Generate(oldClaims)
		require.NoError(t, err)

		oldSess := session.Session[string]{
			ID:        sessionID,
			Token:     oldSessionToken,
			UserID:    userID,
			Data:      "user-data",
			ExpiresAt: time.Now().Add(23 * time.Hour),
			CreatedAt: time.Now().Add(-1 * time.Hour),
			UpdatedAt: time.Now().Add(-1 * time.Hour),
		}

		// New implementation: GetByToken (triggers touch/save) + Save (refresh)
		store.On("GetByToken", mock.Anything, oldSessionToken).Return(oldSess, nil)
		// First Save is from touch (extends expiration, keeps same token)
		store.On("Save", mock.Anything, mock.MatchedBy(func(s *session.Session[string]) bool {
			return s.ID == sessionID && s.Token == oldSessionToken && s.UserID == userID
		})).Return(nil).Once()
		// Second Save is from refresh (rotates token, extends expiration)
		store.On("Save", mock.Anything, mock.MatchedBy(func(s *session.Session[string]) bool {
			return s.ID == sessionID && s.Token != oldSessionToken && s.UserID == userID
		})).Return(nil).Once()

		newSess, newTokens, err := transport.Refresh(ctx, oldRefreshToken)

		require.NoError(t, err)

		// Session ID should stay the same (critical for audit logs)
		assert.Equal(t, sessionID, newSess.ID)
		assert.NotEqual(t, oldSessionToken, newSess.Token)
		assert.Equal(t, userID, newSess.UserID)

		// Verify new tokens are different
		assert.NotEqual(t, oldRefreshToken, newTokens.RefreshToken)

		// Verify new Session.Token is in both JTIs
		var newAccessClaims jwt.StandardClaims
		err = jwtSvc.Parse(newTokens.AccessToken, &newAccessClaims)
		require.NoError(t, err)
		assert.Equal(t, newSess.Token, newAccessClaims.ID)

		var newRefreshClaims jwt.StandardClaims
		err = jwtSvc.Parse(newTokens.RefreshToken, &newRefreshClaims)
		require.NoError(t, err)
		assert.Equal(t, newSess.Token, newRefreshClaims.ID)

		// Verify tokens have same JTI but different issuance times
		assert.Equal(t, newAccessClaims.ID, newRefreshClaims.ID)

		store.AssertExpectations(t)
	})
}

func TestTokenPairStructure(t *testing.T) {
	t.Parallel()

	t.Run("verifies TokenPair has all required fields", func(t *testing.T) {
		t.Parallel()

		now := time.Now()
		pair := sessiontransport.TokenPair{
			AccessToken:  "access-token-value",
			RefreshToken: "refresh-token-value",
			TokenType:    "Bearer",
			ExpiresIn:    900,
			ExpiresAt:    now,
		}

		assert.Equal(t, "access-token-value", pair.AccessToken)
		assert.Equal(t, "refresh-token-value", pair.RefreshToken)
		assert.Equal(t, "Bearer", pair.TokenType)
		assert.Equal(t, 900, pair.ExpiresIn)
		assert.Equal(t, now, pair.ExpiresAt)
	})
}

func TestCookieTransport_WithRealCookieManager(t *testing.T) {
	t.Parallel()

	t.Run("cookie signature is verified correctly", func(t *testing.T) {
		t.Parallel()

		store := &MockStore[string]{}
		sessionMgr := session.NewManager(store, 1*time.Hour, 5*time.Minute)
		cookieMgr := newTestCookieManager(t)
		transport := sessiontransport.NewCookie(sessionMgr, cookieMgr, "session")

		// ctx not needed with handler.Context
		sessionToken := "signed-session-token"
		sessionID := uuid.New()

		sess := session.Session[string]{
			ID:        sessionID,
			Token:     sessionToken,
			UserID:    uuid.New(),
			Data:      "test-data",
			ExpiresAt: time.Now().Add(1 * time.Hour),
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}

		// Set cookie in first response
		r1 := httptest.NewRequest("GET", "/", nil)
		w1 := httptest.NewRecorder()
		// Store consent first
		require.NoError(t, cookieMgr.StoreConsent(w1, r1, cookie.ConsentAll))
		err := cookieMgr.SetSigned(w1, r1, "session", sessionToken, cookie.WithEssential())
		require.NoError(t, err)

		// Extract cookie and add to new request
		r2 := httptest.NewRequest("GET", "/", nil)
		w2 := httptest.NewRecorder()
		for _, c := range w1.Result().Cookies() {
			r2.AddCookie(c)
		}

		// Verify cookie is properly signed and can be retrieved
		store.On("GetByToken", mock.Anything, sessionToken).Return(sess, nil)

		loadedSess, err := transport.Load(newTestContext(w2, r2))

		require.NoError(t, err)
		assert.Equal(t, sessionToken, loadedSess.Token)
		assert.Equal(t, sessionID, loadedSess.ID)

		store.AssertExpectations(t)
	})

	t.Run("tampered cookie is rejected", func(t *testing.T) {
		t.Parallel()

		store := &MockStore[string]{}
		sessionMgr := session.NewManager(store, 1*time.Hour, 5*time.Minute)
		cookieMgr := newTestCookieManager(t)
		transport := sessiontransport.NewCookie(sessionMgr, cookieMgr, "session")

		// ctx not needed with handler.Context

		// Create request with tampered cookie (unsigned)
		r := httptest.NewRequest("GET", "/", nil)
		w := httptest.NewRecorder()
		r.AddCookie(&http.Cookie{
			Name:  "session",
			Value: "tampered-value-without-signature",
		})

		store.On("Save", mock.Anything, mock.AnythingOfType("*session.Session[string]")).Return(nil)

		sess, err := transport.Load(newTestContext(w, r))

		require.NoError(t, err)
		assert.Equal(t, uuid.Nil, sess.UserID) // Should create anonymous session

		store.AssertExpectations(t)
	})
}

func TestJWTTransport_WithRealJWTService(t *testing.T) {
	t.Parallel()

	t.Run("JWT signature is verified correctly", func(t *testing.T) {
		t.Parallel()

		store := &MockStore[string]{}
		sessionMgr := session.NewManager(store, 24*time.Hour, 5*time.Minute)
		secret := "test-jwt-secret-key-32-chars!!"
		transport, err := sessiontransport.NewJWT(sessionMgr, secret, 15*time.Minute, "test-app")
		require.NoError(t, err)

		jwtSvc, err := jwt.NewFromString(secret)
		require.NoError(t, err)

		// ctx not needed with handler.Context
		sessionToken := "valid-session-token"
		sessionID := uuid.New()
		userID := uuid.New()

		sess := session.Session[string]{
			ID:        sessionID,
			Token:     sessionToken,
			UserID:    userID,
			Data:      "test-data",
			ExpiresAt: time.Now().Add(24 * time.Hour),
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}

		// Generate valid JWT
		claims := jwt.StandardClaims{
			ID:        sessionToken,
			Subject:   userID.String(),
			Issuer:    "test-app",
			IssuedAt:  time.Now().Unix(),
			ExpiresAt: time.Now().Add(15 * time.Minute).Unix(),
		}
		jwtToken, err := jwtSvc.Generate(claims)
		require.NoError(t, err)

		r := httptest.NewRequest("GET", "/", nil)
		w := httptest.NewRecorder()
		r.Header.Set("Authorization", "Bearer "+jwtToken)

		store.On("GetByToken", mock.Anything, sessionToken).Return(sess, nil)

		loadedSess, err := transport.Load(newTestContext(w, r))

		require.NoError(t, err)
		assert.Equal(t, sessionToken, loadedSess.Token)
		assert.Equal(t, sessionID, loadedSess.ID)

		store.AssertExpectations(t)
	})

	t.Run("JWT with wrong signature is rejected", func(t *testing.T) {
		t.Parallel()

		store := &MockStore[string]{}
		sessionMgr := session.NewManager(store, 24*time.Hour, 5*time.Minute)
		transport, err := sessiontransport.NewJWT(sessionMgr, "correct-secret-key-32-chars!!!", 15*time.Minute, "test-app")
		require.NoError(t, err)

		// Sign with different secret
		wrongJWTSvc, err := jwt.NewFromString("wrong-secret-key-32-characters!")
		require.NoError(t, err)

		// ctx not needed with handler.Context
		sessionToken := "session-token"

		claims := jwt.StandardClaims{
			ID:        sessionToken,
			Subject:   uuid.New().String(),
			Issuer:    "test-app",
			IssuedAt:  time.Now().Unix(),
			ExpiresAt: time.Now().Add(15 * time.Minute).Unix(),
		}
		wrongJWT, err := wrongJWTSvc.Generate(claims)
		require.NoError(t, err)

		r := httptest.NewRequest("GET", "/", nil)
		w := httptest.NewRecorder()
		r.Header.Set("Authorization", "Bearer "+wrongJWT)

		store.On("Save", mock.Anything, mock.AnythingOfType("*session.Session[string]")).Return(nil)

		sess, err := transport.Load(newTestContext(w, r))

		require.NoError(t, err)
		assert.Equal(t, uuid.Nil, sess.UserID) // Should create anonymous session

		store.AssertExpectations(t)
	})

	t.Run("expired JWT is rejected", func(t *testing.T) {
		t.Parallel()

		store := &MockStore[string]{}
		sessionMgr := session.NewManager(store, 24*time.Hour, 5*time.Minute)
		secret := "test-jwt-secret-key-32-chars!!"
		transport, err := sessiontransport.NewJWT(sessionMgr, secret, 15*time.Minute, "test-app")
		require.NoError(t, err)

		jwtSvc, err := jwt.NewFromString(secret)
		require.NoError(t, err)

		// ctx not needed with handler.Context
		sessionToken := "expired-session-token"

		// Generate expired JWT
		claims := jwt.StandardClaims{
			ID:        sessionToken,
			Subject:   uuid.New().String(),
			Issuer:    "test-app",
			IssuedAt:  time.Now().Add(-1 * time.Hour).Unix(),
			ExpiresAt: time.Now().Add(-30 * time.Minute).Unix(), // Expired 30 min ago
		}
		expiredJWT, err := jwtSvc.Generate(claims)
		require.NoError(t, err)

		r := httptest.NewRequest("GET", "/", nil)
		w := httptest.NewRecorder()
		r.Header.Set("Authorization", "Bearer "+expiredJWT)

		store.On("Save", mock.Anything, mock.AnythingOfType("*session.Session[string]")).Return(nil)

		sess, err := transport.Load(newTestContext(w, r))

		require.NoError(t, err)
		assert.Equal(t, uuid.Nil, sess.UserID) // Should create anonymous session

		store.AssertExpectations(t)
	})
}

func TestCookieTransport_EdgeCases(t *testing.T) {
	t.Parallel()

	t.Run("handles multiple cookies with same name", func(t *testing.T) {
		t.Parallel()

		store := &MockStore[string]{}
		sessionMgr := session.NewManager(store, 1*time.Hour, 5*time.Minute)
		cookieMgr := newTestCookieManager(t)
		transport := sessiontransport.NewCookie(sessionMgr, cookieMgr, "session")

		// ctx not needed with handler.Context
		validToken := "valid-token"

		sess := session.Session[string]{
			ID:        uuid.New(),
			Token:     validToken,
			UserID:    uuid.New(),
			Data:      "test-data",
			ExpiresAt: time.Now().Add(1 * time.Hour),
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}

		// Create request with properly signed cookie
		r := httptest.NewRequest("GET", "/", nil)
		w := httptest.NewRecorder()
		// Store consent first (cookies are non-essential by default)
		require.NoError(t, cookieMgr.StoreConsent(w, r, cookie.ConsentAll))
		require.NoError(t, cookieMgr.SetSigned(w, r, "session", validToken, cookie.WithEssential()))
		for _, c := range w.Result().Cookies() {
			r.AddCookie(c)
		}

		// Add a duplicate cookie with different value (shouldn't happen, but test it)
		r.AddCookie(&http.Cookie{Name: "session", Value: "tampered-value"})

		store.On("GetByToken", mock.Anything, mock.AnythingOfType("string")).Return(sess, nil).Maybe()
		store.On("Save", mock.Anything, mock.AnythingOfType("*session.Session[string]")).Return(nil).Maybe()

		// Should handle gracefully (first cookie wins in http.Request)
		_, err := transport.Load(newTestContext(w, r))

		require.NoError(t, err)
	})

	t.Run("handles extremely long cookie values gracefully", func(t *testing.T) {
		t.Parallel()

		store := &MockStore[string]{}
		sessionMgr := session.NewManager(store, 1*time.Hour, 5*time.Minute)
		cookieMgr := newTestCookieManager(t)
		transport := sessiontransport.NewCookie(sessionMgr, cookieMgr, "session")

		w := httptest.NewRecorder()
		r := httptest.NewRequest("GET", "/", nil)

		// Create session with very long token (base64 of 32 bytes = 43 chars is normal)
		longToken := strings.Repeat("a", 500)
		sess := session.Session[string]{
			ID:        uuid.New(),
			Token:     longToken,
			UserID:    uuid.New(),
			Data:      "test-data",
			ExpiresAt: time.Now().Add(1 * time.Hour),
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}

		// Cookie manager should handle long values (with signing, may exceed 4KB limit)
		err := transport.Save(newTestContext(w, r), sess)

		// May succeed or fail depending on cookie size limit, just ensure it doesn't panic
		_ = err
	})
}

func TestJWTTransport_EdgeCases(t *testing.T) {
	t.Parallel()

	t.Run("handles malformed JWT gracefully", func(t *testing.T) {
		t.Parallel()

		store := &MockStore[string]{}
		sessionMgr := session.NewManager(store, 24*time.Hour, 5*time.Minute)
		transport, err := sessiontransport.NewJWT(sessionMgr, "test-jwt-secret-key-32-chars!!", 15*time.Minute, "test-app")
		require.NoError(t, err)

		// ctx not needed with handler.Context

		testCases := []string{
			"not.a.jwt",
			"only-one-part",
			"two.parts",
			"eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..", // Missing signature
			".eyJzdWIiOiIxMjM0NTY3ODkwIn0.",          // Missing header
		}

		for _, tc := range testCases {
			r := httptest.NewRequest("GET", "/", nil)
			w := httptest.NewRecorder()
			r.Header.Set("Authorization", "Bearer "+tc)

			store.On("Save", mock.Anything, mock.AnythingOfType("*session.Session[string]")).Return(nil).Once()

			sess, err := transport.Load(newTestContext(w, r))

			require.NoError(t, err, "Failed for: %s", tc)
			assert.Equal(t, uuid.Nil, sess.UserID, "Should create anonymous session for: %s", tc)
		}

		store.AssertExpectations(t)
	})

	t.Run("handles JWT with missing claims", func(t *testing.T) {
		t.Parallel()

		store := &MockStore[string]{}
		sessionMgr := session.NewManager(store, 24*time.Hour, 5*time.Minute)
		jwtSvc := newTestJWTService(t)
		transport, err := sessiontransport.NewJWT(sessionMgr, "test-jwt-secret-key-32-chars!!", 15*time.Minute, "test-app")
		require.NoError(t, err)

		// ctx not needed with handler.Context

		// Generate JWT with empty/missing ID (JTI)
		claims := jwt.StandardClaims{
			ID:        "", // Empty JTI
			Subject:   uuid.New().String(),
			Issuer:    "test-app",
			IssuedAt:  time.Now().Unix(),
			ExpiresAt: time.Now().Add(15 * time.Minute).Unix(),
		}
		jwtToken, err := jwtSvc.Generate(claims)
		require.NoError(t, err)

		r := httptest.NewRequest("GET", "/", nil)
		w := httptest.NewRecorder()
		r.Header.Set("Authorization", "Bearer "+jwtToken)

		// Empty token should fail GetByToken, creating anonymous session
		store.On("GetByToken", mock.Anything, "").Return(nil, session.ErrNotFound)
		store.On("Save", mock.Anything, mock.AnythingOfType("*session.Session[string]")).Return(nil)

		sess, err := transport.Load(newTestContext(w, r))

		require.NoError(t, err)
		assert.Equal(t, uuid.Nil, sess.UserID)

		store.AssertExpectations(t)
	})

	t.Run("handles very long bearer token", func(t *testing.T) {
		t.Parallel()

		store := &MockStore[string]{}
		sessionMgr := session.NewManager(store, 24*time.Hour, 5*time.Minute)
		transport, err := sessiontransport.NewJWT(sessionMgr, "test-jwt-secret-key-32-chars!!", 15*time.Minute, "test-app")
		require.NoError(t, err)

		// ctx not needed with handler.Context

		// Create extremely long authorization header
		longToken := strings.Repeat("a", 10000)
		r := httptest.NewRequest("GET", "/", nil)
		w := httptest.NewRecorder()
		r.Header.Set("Authorization", "Bearer "+longToken)

		store.On("Save", mock.Anything, mock.AnythingOfType("*session.Session[string]")).Return(nil)

		sess, err := transport.Load(newTestContext(w, r))

		require.NoError(t, err)
		assert.Equal(t, uuid.Nil, sess.UserID)

		store.AssertExpectations(t)
	})
}

// ============================================================================
// IP and UserAgent Extraction Tests
// ============================================================================

func TestCookie_Load_IPExtraction(t *testing.T) {
	t.Parallel()

	t.Run("extracts IP from CF-Connecting-IP header", func(t *testing.T) {
		t.Parallel()

		store := &MockStore[string]{}
		sessionMgr := session.NewManager(store, 1*time.Hour, 5*time.Minute)
		cookieMgr := newTestCookieManager(t)
		transport := sessiontransport.NewCookie(sessionMgr, cookieMgr, "session")

		r := httptest.NewRequest("GET", "/", nil)
		w := httptest.NewRecorder()
		r.Header.Set("CF-Connecting-IP", "203.0.113.42")
		r.Header.Set("X-Forwarded-For", "192.168.1.1")
		r.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0) Chrome/120.0")

		store.On("Save", mock.Anything, mock.MatchedBy(func(s *session.Session[string]) bool {
			return s.IP == "203.0.113.42" // CF-Connecting-IP has priority
		})).Return(nil)

		sess, err := transport.Load(newTestContext(w, r))

		require.NoError(t, err)
		assert.Equal(t, "203.0.113.42", sess.IP)

		store.AssertExpectations(t)
	})

	t.Run("extracts IP from X-Forwarded-For header", func(t *testing.T) {
		t.Parallel()

		store := &MockStore[string]{}
		sessionMgr := session.NewManager(store, 1*time.Hour, 5*time.Minute)
		cookieMgr := newTestCookieManager(t)
		transport := sessiontransport.NewCookie(sessionMgr, cookieMgr, "session")

		r := httptest.NewRequest("GET", "/", nil)
		w := httptest.NewRecorder()
		r.Header.Set("X-Forwarded-For", "198.51.100.5, 192.168.1.1")
		r.Header.Set("User-Agent", "Mozilla/5.0")

		store.On("Save", mock.Anything, mock.MatchedBy(func(s *session.Session[string]) bool {
			return s.IP == "198.51.100.5" // Leftmost IP from X-Forwarded-For
		})).Return(nil)

		sess, err := transport.Load(newTestContext(w, r))

		require.NoError(t, err)
		assert.Equal(t, "198.51.100.5", sess.IP)

		store.AssertExpectations(t)
	})

	t.Run("extracts IP from X-Real-IP header", func(t *testing.T) {
		t.Parallel()

		store := &MockStore[string]{}
		sessionMgr := session.NewManager(store, 1*time.Hour, 5*time.Minute)
		cookieMgr := newTestCookieManager(t)
		transport := sessiontransport.NewCookie(sessionMgr, cookieMgr, "session")

		r := httptest.NewRequest("GET", "/", nil)
		w := httptest.NewRecorder()
		r.Header.Set("X-Real-IP", "198.51.100.10")
		r.Header.Set("User-Agent", "Mozilla/5.0")

		store.On("Save", mock.Anything, mock.MatchedBy(func(s *session.Session[string]) bool {
			return s.IP == "198.51.100.10"
		})).Return(nil)

		sess, err := transport.Load(newTestContext(w, r))

		require.NoError(t, err)
		assert.Equal(t, "198.51.100.10", sess.IP)

		store.AssertExpectations(t)
	})

	t.Run("extracts IPv6 address", func(t *testing.T) {
		t.Parallel()

		store := &MockStore[string]{}
		sessionMgr := session.NewManager(store, 1*time.Hour, 5*time.Minute)
		cookieMgr := newTestCookieManager(t)
		transport := sessiontransport.NewCookie(sessionMgr, cookieMgr, "session")

		r := httptest.NewRequest("GET", "/", nil)
		w := httptest.NewRecorder()
		r.Header.Set("CF-Connecting-IP", "2001:db8::1")
		r.Header.Set("User-Agent", "Mozilla/5.0")

		store.On("Save", mock.Anything, mock.MatchedBy(func(s *session.Session[string]) bool {
			return s.IP == "2001:db8::1"
		})).Return(nil)

		sess, err := transport.Load(newTestContext(w, r))

		require.NoError(t, err)
		assert.Equal(t, "2001:db8::1", sess.IP)

		store.AssertExpectations(t)
	})

	t.Run("falls back to RemoteAddr when no proxy headers", func(t *testing.T) {
		t.Parallel()

		store := &MockStore[string]{}
		sessionMgr := session.NewManager(store, 1*time.Hour, 5*time.Minute)
		cookieMgr := newTestCookieManager(t)
		transport := sessiontransport.NewCookie(sessionMgr, cookieMgr, "session")

		r := httptest.NewRequest("GET", "/", nil)
		w := httptest.NewRecorder()
		r.RemoteAddr = "192.168.1.100:12345"
		r.Header.Set("User-Agent", "Mozilla/5.0")

		store.On("Save", mock.Anything, mock.AnythingOfType("*session.Session[string]")).Return(nil)

		sess, err := transport.Load(newTestContext(w, r))

		require.NoError(t, err)
		// clientip.GetIP will return the RemoteAddr when no headers present
		assert.Contains(t, sess.IP, "192.168.1.100")

		store.AssertExpectations(t)
	})
}

func TestCookie_Load_UserAgentExtraction(t *testing.T) {
	t.Parallel()

	t.Run("extracts User-Agent header", func(t *testing.T) {
		t.Parallel()

		store := &MockStore[string]{}
		sessionMgr := session.NewManager(store, 1*time.Hour, 5*time.Minute)
		cookieMgr := newTestCookieManager(t)
		transport := sessiontransport.NewCookie(sessionMgr, cookieMgr, "session")

		testUA := "Mozilla/5.0 (Windows NT 10.0; Win64; x64) Chrome/120.0.0.0"
		r := httptest.NewRequest("GET", "/", nil)
		w := httptest.NewRecorder()
		r.Header.Set("User-Agent", testUA)
		r.Header.Set("CF-Connecting-IP", "203.0.113.1")

		store.On("Save", mock.Anything, mock.MatchedBy(func(s *session.Session[string]) bool {
			return s.UserAgent == testUA
		})).Return(nil)

		sess, err := transport.Load(newTestContext(w, r))

		require.NoError(t, err)
		assert.Equal(t, testUA, sess.UserAgent)

		store.AssertExpectations(t)
	})

	t.Run("allows empty User-Agent", func(t *testing.T) {
		t.Parallel()

		store := &MockStore[string]{}
		sessionMgr := session.NewManager(store, 1*time.Hour, 5*time.Minute)
		cookieMgr := newTestCookieManager(t)
		transport := sessiontransport.NewCookie(sessionMgr, cookieMgr, "session")

		r := httptest.NewRequest("GET", "/", nil)
		w := httptest.NewRecorder()
		r.Header.Set("CF-Connecting-IP", "203.0.113.1")
		// No User-Agent header set

		store.On("Save", mock.Anything, mock.MatchedBy(func(s *session.Session[string]) bool {
			return s.UserAgent == ""
		})).Return(nil)

		sess, err := transport.Load(newTestContext(w, r))

		require.NoError(t, err)
		assert.Equal(t, "", sess.UserAgent)

		store.AssertExpectations(t)
	})

	t.Run("handles bot User-Agent", func(t *testing.T) {
		t.Parallel()

		store := &MockStore[string]{}
		sessionMgr := session.NewManager(store, 1*time.Hour, 5*time.Minute)
		cookieMgr := newTestCookieManager(t)
		transport := sessiontransport.NewCookie(sessionMgr, cookieMgr, "session")

		botUA := "Mozilla/5.0 (compatible; Googlebot/2.1; +http://www.google.com/bot.html)"
		r := httptest.NewRequest("GET", "/", nil)
		w := httptest.NewRecorder()
		r.Header.Set("User-Agent", botUA)
		r.Header.Set("CF-Connecting-IP", "66.249.66.1")

		store.On("Save", mock.Anything, mock.MatchedBy(func(s *session.Session[string]) bool {
			return s.UserAgent == botUA
		})).Return(nil)

		sess, err := transport.Load(newTestContext(w, r))

		require.NoError(t, err)
		assert.Equal(t, botUA, sess.UserAgent)

		store.AssertExpectations(t)
	})
}

func TestCookie_Load_IPAndUserAgentPersistence(t *testing.T) {
	t.Parallel()

	t.Run("preserves IP and UserAgent in existing session", func(t *testing.T) {
		t.Parallel()

		store := &MockStore[string]{}
		sessionMgr := session.NewManager(store, 1*time.Hour, 5*time.Minute)
		cookieMgr := newTestCookieManager(t)
		transport := sessiontransport.NewCookie(sessionMgr, cookieMgr, "session")

		token := "existing-session-token"
		originalIP := "198.51.100.100"
		originalUA := "Mozilla/5.0 (Original) Firefox/110.0"

		// Create request with signed cookie
		r := httptest.NewRequest("GET", "/", nil)
		w := httptest.NewRecorder()
		require.NoError(t, cookieMgr.SetSigned(w, r, "session", token, cookie.WithEssential()))
		for _, c := range w.Result().Cookies() {
			r.AddCookie(c)
		}

		// Set different IP/UA in headers (should not override existing session)
		r.Header.Set("CF-Connecting-IP", "203.0.113.99")
		r.Header.Set("User-Agent", "Mozilla/5.0 (Different) Chrome/120.0")

		existingSession := session.Session[string]{
			ID:        uuid.New(),
			Token:     token,
			UserID:    uuid.New(),
			IP:        originalIP,
			UserAgent: originalUA,
			Data:      "test-data",
			ExpiresAt: time.Now().Add(1 * time.Hour),
			CreatedAt: time.Now().Add(-30 * time.Minute),
			UpdatedAt: time.Now().Add(-1 * time.Minute), // Recent update, no touch
		}

		store.On("GetByToken", mock.Anything, token).Return(existingSession, nil)

		sess, err := transport.Load(newTestContext(w, r))

		require.NoError(t, err)
		// Should preserve original IP/UA from stored session
		assert.Equal(t, originalIP, sess.IP)
		assert.Equal(t, originalUA, sess.UserAgent)

		store.AssertExpectations(t)
	})
}
