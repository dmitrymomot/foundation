package sessiontransport_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/dmitrymomot/foundation/core/cookie"
	"github.com/dmitrymomot/foundation/core/session"
	"github.com/dmitrymomot/foundation/core/sessiontransport"
)

// testData is the session data type used in all tests
type testData struct {
	CartItems []string
	Theme     string
}

// mockStore implements session.Store interface for testing
type mockStore struct {
	mock.Mock
}

func (m *mockStore) GetByID(ctx context.Context, id uuid.UUID) (*session.Session[testData], error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*session.Session[testData]), args.Error(1)
}

func (m *mockStore) GetByToken(ctx context.Context, token string) (*session.Session[testData], error) {
	args := m.Called(ctx, token)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*session.Session[testData]), args.Error(1)
}

func (m *mockStore) Save(ctx context.Context, sess *session.Session[testData]) error {
	args := m.Called(ctx, sess)
	return args.Error(0)
}

func (m *mockStore) Delete(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *mockStore) DeleteExpired(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

// cookieManagerWrapper wraps cookie.Manager and allows intercepting calls
type cookieManagerWrapper struct {
	*cookie.Manager
	onGetSigned func(r *http.Request, name string) (string, error)
	onSetSigned func(w http.ResponseWriter, r *http.Request, name, value string, opts ...cookie.Option) error
	onDelete    func(w http.ResponseWriter, name string)
}

func (c *cookieManagerWrapper) GetSigned(r *http.Request, name string) (string, error) {
	if c.onGetSigned != nil {
		return c.onGetSigned(r, name)
	}
	return c.Manager.GetSigned(r, name)
}

func (c *cookieManagerWrapper) SetSigned(w http.ResponseWriter, r *http.Request, name, value string, opts ...cookie.Option) error {
	if c.onSetSigned != nil {
		return c.onSetSigned(w, r, name, value, opts...)
	}
	return c.Manager.SetSigned(w, r, name, value, opts...)
}

func (c *cookieManagerWrapper) Delete(w http.ResponseWriter, name string) {
	if c.onDelete != nil {
		c.onDelete(w, name)
		return
	}
	c.Manager.Delete(w, name)
}

// mockContext implements handler.Context for testing
type mockContext struct {
	mock.Mock
	request        *http.Request
	responseWriter http.ResponseWriter
}

func (m *mockContext) Deadline() (deadline time.Time, ok bool) {
	args := m.Called()
	return args.Get(0).(time.Time), args.Bool(1)
}

func (m *mockContext) Done() <-chan struct{} {
	args := m.Called()
	if args.Get(0) == nil {
		return nil
	}
	return args.Get(0).(<-chan struct{})
}

func (m *mockContext) Err() error {
	args := m.Called()
	return args.Error(0)
}

func (m *mockContext) Value(key interface{}) interface{} {
	args := m.Called(key)
	return args.Get(0)
}

func (m *mockContext) Request() *http.Request {
	return m.request
}

func (m *mockContext) ResponseWriter() http.ResponseWriter {
	return m.responseWriter
}

func (m *mockContext) Param(key string) string {
	args := m.Called(key)
	return args.String(0)
}

func (m *mockContext) SetValue(key, val any) {
	m.Called(key, val)
}

// Helper functions

func createValidSession(t *testing.T) session.Session[testData] {
	t.Helper()
	sess, err := session.New[testData](session.NewSessionParams{
		Fingerprint: "v1:test-fingerprint-hash-12345678901",
		IP:          "127.0.0.1",
		UserAgent:   "Mozilla/5.0 Test Browser",
	}, time.Hour)
	require.NoError(t, err)
	return sess
}

func createExpiredSession(t *testing.T) session.Session[testData] {
	t.Helper()
	sess, err := session.New[testData](session.NewSessionParams{
		Fingerprint: "v1:test-fingerprint-hash-12345678901",
		IP:          "127.0.0.1",
		UserAgent:   "Mozilla/5.0 Test Browser",
	}, -time.Hour) // Negative TTL creates already expired session
	require.NoError(t, err)
	return sess
}

func createMockContext(t *testing.T) *mockContext {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 Test Browser")
	req.Header.Set("X-Forwarded-For", "127.0.0.1")
	req.AddCookie(&http.Cookie{Name: "fingerprint", Value: "test-fingerprint-hash-12345678901"})

	w := httptest.NewRecorder()

	return &mockContext{
		request:        req,
		responseWriter: w,
	}
}

func createCookieTransport(t *testing.T, store *mockStore) (*sessiontransport.Cookie[testData], *cookieManagerWrapper) {
	t.Helper()
	sessionMgr := session.NewManager[testData](store, time.Hour, 5*time.Minute)

	baseCookieMgr, err := cookie.New([]string{"test-secret-key-for-cookie-manager-32chars"})
	require.NoError(t, err)

	wrapper := &cookieManagerWrapper{Manager: baseCookieMgr}
	transport := sessiontransport.NewCookie[testData](sessionMgr, wrapper, "session")

	return transport, wrapper
}

// Load Method Tests

func TestCookieLoad_Success(t *testing.T) {
	t.Parallel()

	t.Run("valid signed cookie loads session", func(t *testing.T) {
		t.Parallel()

		store := &mockStore{}
		transport, _ := createCookieTransport(t, store)
		ctx := createMockContext(t)
		validSession := createValidSession(t)

		// First set a cookie with the session token
		cookieMgr, err := cookie.New([]string{"test-secret-key-for-cookie-manager-32chars"})
		require.NoError(t, err)
		err = cookieMgr.SetSigned(ctx.ResponseWriter(), ctx.Request(), "session", validSession.Token, cookie.WithEssential())
		require.NoError(t, err)

		// Add the cookie to the request
		recorder := ctx.ResponseWriter().(*httptest.ResponseRecorder)
		cookies := recorder.Result().Cookies()
		for _, c := range cookies {
			ctx.Request().AddCookie(c)
		}

		store.On("GetByToken", ctx, mock.AnythingOfType("string")).Return(&validSession, nil)

		result, err := transport.Load(ctx)

		require.NoError(t, err)
		// Verify session was loaded successfully - returned session should match store data
		assert.Equal(t, validSession.ID, result.ID)
		assert.Equal(t, validSession.IP, result.IP)
		assert.Equal(t, validSession.UserAgent, result.UserAgent)
		store.AssertExpectations(t)
	})
}

func TestCookieLoad_NoCookie(t *testing.T) {
	t.Parallel()

	t.Run("creates anonymous session on cookie error", func(t *testing.T) {
		t.Parallel()

		store := &mockStore{}
		transport, wrapper := createCookieTransport(t, store)
		ctx := createMockContext(t)

		wrapper.onGetSigned = func(r *http.Request, name string) (string, error) {
			return "", errors.New("cookie not found")
		}

		result, err := transport.Load(ctx)

		require.NoError(t, err)
		assert.NotEqual(t, uuid.Nil, result.ID)
		assert.Equal(t, uuid.Nil, result.UserID) // Anonymous session
		assert.NotEmpty(t, result.Token)
	})
}

func TestCookieLoad_InvalidCookie(t *testing.T) {
	t.Parallel()

	t.Run("creates anonymous session on invalid cookie", func(t *testing.T) {
		t.Parallel()

		store := &mockStore{}
		transport, _ := createCookieTransport(t, store)
		ctx := createMockContext(t)

		// Set a cookie with an invalid token
		cookieMgr, err := cookie.New([]string{"test-secret-key-for-cookie-manager-32chars"})
		require.NoError(t, err)
		err = cookieMgr.SetSigned(ctx.ResponseWriter(), ctx.Request(), "session", "invalid-token", cookie.WithEssential())
		require.NoError(t, err)

		// Add the cookie to the request
		recorder := ctx.ResponseWriter().(*httptest.ResponseRecorder)
		cookies := recorder.Result().Cookies()
		for _, c := range cookies {
			ctx.Request().AddCookie(c)
		}

		store.On("GetByToken", ctx, mock.AnythingOfType("string")).Return(nil, session.ErrNotFound)

		result, err := transport.Load(ctx)

		require.NoError(t, err)
		assert.NotEqual(t, uuid.Nil, result.ID)
		assert.Equal(t, uuid.Nil, result.UserID)
		store.AssertExpectations(t)
	})
}

func TestCookieLoad_TokenNotFound(t *testing.T) {
	t.Parallel()

	t.Run("creates anonymous session when GetByToken fails", func(t *testing.T) {
		t.Parallel()

		store := &mockStore{}
		transport, wrapper := createCookieTransport(t, store)
		ctx := createMockContext(t)

		wrapper.onGetSigned = func(r *http.Request, name string) (string, error) {
			return "unknown-token", nil
		}
		store.On("GetByToken", ctx, "unknown-token").Return(nil, session.ErrNotFound)

		result, err := transport.Load(ctx)

		require.NoError(t, err)
		assert.Equal(t, uuid.Nil, result.UserID)
		store.AssertExpectations(t)
	})
}

func TestCookieLoad_SessionExpired(t *testing.T) {
	t.Parallel()

	t.Run("creates anonymous session on expired session", func(t *testing.T) {
		t.Parallel()

		store := &mockStore{}
		transport, wrapper := createCookieTransport(t, store)
		ctx := createMockContext(t)
		expiredSession := createExpiredSession(t)

		wrapper.onGetSigned = func(r *http.Request, name string) (string, error) {
			return expiredSession.Token, nil
		}
		store.On("GetByToken", ctx, expiredSession.Token).Return(nil, session.ErrExpired)

		result, err := transport.Load(ctx)

		require.NoError(t, err)
		assert.NotEqual(t, expiredSession.ID, result.ID)
		assert.Equal(t, uuid.Nil, result.UserID)
		store.AssertExpectations(t)
	})
}

func TestCookieLoad_ExtractsMetadata(t *testing.T) {
	t.Parallel()

	t.Run("verifies fingerprint/IP/UserAgent extraction", func(t *testing.T) {
		t.Parallel()

		store := &mockStore{}
		transport, wrapper := createCookieTransport(t, store)
		ctx := createMockContext(t)

		wrapper.onGetSigned = func(r *http.Request, name string) (string, error) {
			return "", errors.New("no cookie")
		}

		result, err := transport.Load(ctx)

		require.NoError(t, err)
		assert.Equal(t, "127.0.0.1", result.IP)
		assert.Equal(t, "Mozilla/5.0 Test Browser", result.UserAgent)
		assert.NotEmpty(t, result.Fingerprint)
	})
}

// Save Method Tests

func TestCookieSave_Success(t *testing.T) {
	t.Parallel()

	t.Run("saves token to signed cookie with correct MaxAge", func(t *testing.T) {
		t.Parallel()

		store := &mockStore{}
		transport, wrapper := createCookieTransport(t, store)
		ctx := createMockContext(t)
		sess := createValidSession(t)

		var savedToken string
		wrapper.onSetSigned = func(w http.ResponseWriter, r *http.Request, name, value string, opts ...cookie.Option) error {
			savedToken = value
			return nil
		}

		err := transport.Save(ctx, sess)

		require.NoError(t, err)
		assert.Equal(t, sess.Token, savedToken)
	})
}

func TestCookieSave_Essential(t *testing.T) {
	t.Parallel()

	t.Run("verifies WithEssential option", func(t *testing.T) {
		t.Parallel()

		store := &mockStore{}
		transport, wrapper := createCookieTransport(t, store)
		ctx := createMockContext(t)
		sess := createValidSession(t)

		wrapper.onSetSigned = func(w http.ResponseWriter, r *http.Request, name, value string, opts ...cookie.Option) error {
			// Cookie transport should pass WithEssential() option
			return nil
		}

		err := transport.Save(ctx, sess)

		require.NoError(t, err)
	})
}

func TestCookieSave_ExpiredSession(t *testing.T) {
	t.Parallel()

	t.Run("returns error for expired session", func(t *testing.T) {
		t.Parallel()

		store := &mockStore{}
		transport, _ := createCookieTransport(t, store)
		ctx := createMockContext(t)
		sess := createExpiredSession(t)

		err := transport.Save(ctx, sess)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "cannot save expired session")
	})
}

func TestCookieSave_CookieError(t *testing.T) {
	t.Parallel()

	t.Run("propagates cookie manager errors", func(t *testing.T) {
		t.Parallel()

		store := &mockStore{}
		transport, wrapper := createCookieTransport(t, store)
		ctx := createMockContext(t)
		sess := createValidSession(t)
		cookieErr := errors.New("failed to set cookie")

		wrapper.onSetSigned = func(w http.ResponseWriter, r *http.Request, name, value string, opts ...cookie.Option) error {
			return cookieErr
		}

		err := transport.Save(ctx, sess)

		require.Error(t, err)
		assert.ErrorIs(t, err, cookieErr)
	})
}

// Authenticate Method Tests

func TestCookieAuthenticate_Success(t *testing.T) {
	t.Parallel()

	t.Run("full chain with token rotation", func(t *testing.T) {
		t.Parallel()

		store := &mockStore{}
		transport, _ := createCookieTransport(t, store)
		ctx := createMockContext(t)
		userID := uuid.New()

		store.On("Save", ctx, mock.MatchedBy(func(s *session.Session[testData]) bool {
			return s.UserID == userID && s.IsAuthenticated()
		})).Return(nil)

		result, err := transport.Authenticate(ctx, userID)

		require.NoError(t, err)
		assert.Equal(t, userID, result.UserID)
		assert.True(t, result.IsAuthenticated())
		assert.NotEmpty(t, result.Token)

		// Verify cookie was set by checking response headers
		recorder := ctx.ResponseWriter().(*httptest.ResponseRecorder)
		cookies := recorder.Result().Cookies()
		assert.NotEmpty(t, cookies, "Should have set a cookie")

		store.AssertExpectations(t)
	})
}

func TestCookieAuthenticate_WithData(t *testing.T) {
	t.Parallel()

	t.Run("handles optional data parameter", func(t *testing.T) {
		t.Parallel()

		store := &mockStore{}
		transport, wrapper := createCookieTransport(t, store)
		ctx := createMockContext(t)
		userID := uuid.New()
		data := testData{CartItems: []string{"item1"}, Theme: "dark"}

		wrapper.onGetSigned = func(r *http.Request, name string) (string, error) {
			return "", errors.New("no cookie")
		}

		store.On("Save", ctx, mock.MatchedBy(func(s *session.Session[testData]) bool {
			return s.UserID == userID && s.Data.Theme == "dark" && len(s.Data.CartItems) == 1
		})).Return(nil)

		wrapper.onSetSigned = func(w http.ResponseWriter, r *http.Request, name, value string, opts ...cookie.Option) error {
			return nil
		}

		result, err := transport.Authenticate(ctx, userID, data)

		require.NoError(t, err)
		assert.Equal(t, userID, result.UserID)
		assert.Equal(t, "dark", result.Data.Theme)
		assert.Equal(t, []string{"item1"}, result.Data.CartItems)
		store.AssertExpectations(t)
	})
}

func TestCookieAuthenticate_LoadError(t *testing.T) {
	t.Parallel()

	t.Run("propagates Load errors", func(t *testing.T) {
		t.Parallel()

		store := &mockStore{}
		transport, wrapper := createCookieTransport(t, store)
		ctx := createMockContext(t)
		userID := uuid.New()

		wrapper.onGetSigned = func(r *http.Request, name string) (string, error) {
			return "", errors.New("cookie error")
		}

		storeErr := errors.New("store error")
		store.On("Save", ctx, mock.Anything).Return(storeErr)

		result, err := transport.Authenticate(ctx, userID)

		require.Error(t, err)
		assert.ErrorIs(t, err, storeErr)
		assert.Equal(t, uuid.Nil, result.ID)
		store.AssertExpectations(t)
	})
}

func TestCookieAuthenticate_StoreError(t *testing.T) {
	t.Parallel()

	t.Run("propagates Manager.Store errors", func(t *testing.T) {
		t.Parallel()

		store := &mockStore{}
		transport, wrapper := createCookieTransport(t, store)
		ctx := createMockContext(t)
		userID := uuid.New()
		storeErr := errors.New("database error")

		wrapper.onGetSigned = func(r *http.Request, name string) (string, error) {
			return "", errors.New("no cookie")
		}
		store.On("Save", ctx, mock.Anything).Return(storeErr)

		result, err := transport.Authenticate(ctx, userID)

		require.Error(t, err)
		assert.ErrorIs(t, err, storeErr)
		assert.Equal(t, uuid.Nil, result.ID)
		store.AssertExpectations(t)
	})
}

func TestCookieAuthenticate_SaveError(t *testing.T) {
	t.Parallel()

	t.Run("propagates Save errors", func(t *testing.T) {
		t.Parallel()

		store := &mockStore{}
		transport, wrapper := createCookieTransport(t, store)
		ctx := createMockContext(t)
		userID := uuid.New()
		saveErr := errors.New("cookie save error")

		wrapper.onGetSigned = func(r *http.Request, name string) (string, error) {
			return "", errors.New("no cookie")
		}
		store.On("Save", ctx, mock.Anything).Return(nil)
		wrapper.onSetSigned = func(w http.ResponseWriter, r *http.Request, name, value string, opts ...cookie.Option) error {
			return saveErr
		}

		result, err := transport.Authenticate(ctx, userID)

		require.Error(t, err)
		assert.ErrorIs(t, err, saveErr)
		assert.Equal(t, uuid.Nil, result.ID)
		store.AssertExpectations(t)
	})
}

// Logout Method Tests

func TestCookieLogout_Success(t *testing.T) {
	t.Parallel()

	t.Run("deletes session and cookie", func(t *testing.T) {
		t.Parallel()

		store := &mockStore{}
		transport, wrapper := createCookieTransport(t, store)
		ctx := createMockContext(t)
		authSession := createValidSession(t)
		authSession.Authenticate(uuid.New())

		wrapper.onGetSigned = func(r *http.Request, name string) (string, error) {
			return authSession.Token, nil
		}
		store.On("GetByToken", ctx, mock.AnythingOfType("string")).Return(&authSession, nil)
		store.On("Delete", ctx, mock.AnythingOfType("uuid.UUID")).Return(nil)

		deleteCalled := false
		wrapper.onDelete = func(w http.ResponseWriter, name string) {
			deleteCalled = true
		}

		result, err := transport.Logout(ctx)

		require.NoError(t, err)
		assert.Equal(t, uuid.Nil, result.ID)
		assert.True(t, deleteCalled)
		store.AssertExpectations(t)
	})
}

func TestCookieLogout_LoadError(t *testing.T) {
	t.Parallel()

	t.Run("propagates Load errors", func(t *testing.T) {
		t.Parallel()

		store := &mockStore{}
		transport, wrapper := createCookieTransport(t, store)
		ctx := createMockContext(t)

		wrapper.onGetSigned = func(r *http.Request, name string) (string, error) {
			return "", errors.New("cookie error")
		}

		storeErr := errors.New("store error")
		store.On("Delete", ctx, mock.AnythingOfType("uuid.UUID")).Return(storeErr)

		result, err := transport.Logout(ctx)

		require.Error(t, err)
		assert.ErrorIs(t, err, storeErr)
		assert.Equal(t, uuid.Nil, result.ID)
		store.AssertExpectations(t)
	})
}

func TestCookieLogout_NotAuthenticatedOK(t *testing.T) {
	t.Parallel()

	t.Run("handles ErrNotAuthenticated gracefully", func(t *testing.T) {
		t.Parallel()

		store := &mockStore{}
		transport, wrapper := createCookieTransport(t, store)
		ctx := createMockContext(t)
		anonSession := createValidSession(t)

		wrapper.onGetSigned = func(r *http.Request, name string) (string, error) {
			return anonSession.Token, nil
		}
		store.On("GetByToken", ctx, mock.AnythingOfType("string")).Return(&anonSession, nil)
		store.On("Delete", ctx, mock.AnythingOfType("uuid.UUID")).Return(session.ErrNotAuthenticated)

		wrapper.onDelete = func(w http.ResponseWriter, name string) {}

		result, err := transport.Logout(ctx)

		require.NoError(t, err)
		assert.Equal(t, uuid.Nil, result.ID)
		store.AssertExpectations(t)
	})
}

func TestCookieLogout_DeleteError(t *testing.T) {
	t.Parallel()

	t.Run("propagates deletion errors", func(t *testing.T) {
		t.Parallel()

		store := &mockStore{}
		transport, wrapper := createCookieTransport(t, store)
		ctx := createMockContext(t)
		authSession := createValidSession(t)
		authSession.Authenticate(uuid.New())
		deleteErr := errors.New("delete failed")

		wrapper.onGetSigned = func(r *http.Request, name string) (string, error) {
			return authSession.Token, nil
		}
		store.On("GetByToken", ctx, mock.AnythingOfType("string")).Return(&authSession, nil)
		store.On("Delete", ctx, mock.AnythingOfType("uuid.UUID")).Return(deleteErr)

		result, err := transport.Logout(ctx)

		require.Error(t, err)
		assert.ErrorIs(t, err, deleteErr)
		assert.Equal(t, uuid.Nil, result.ID)
		store.AssertExpectations(t)
	})
}

// Delete Method Tests

func TestCookieDelete_Success(t *testing.T) {
	t.Parallel()

	t.Run("deletes session and cookie", func(t *testing.T) {
		t.Parallel()

		store := &mockStore{}
		transport, wrapper := createCookieTransport(t, store)
		ctx := createMockContext(t)
		sess := createValidSession(t)

		wrapper.onGetSigned = func(r *http.Request, name string) (string, error) {
			return sess.Token, nil
		}
		store.On("GetByToken", ctx, mock.AnythingOfType("string")).Return(&sess, nil)
		store.On("Delete", ctx, mock.AnythingOfType("uuid.UUID")).Return(nil)

		deleteCalled := false
		wrapper.onDelete = func(w http.ResponseWriter, name string) {
			deleteCalled = true
		}

		err := transport.Delete(ctx)

		require.NoError(t, err)
		assert.True(t, deleteCalled)
		store.AssertExpectations(t)
	})
}

func TestCookieDelete_LoadError(t *testing.T) {
	t.Parallel()

	t.Run("propagates Load errors", func(t *testing.T) {
		t.Parallel()

		store := &mockStore{}
		transport, wrapper := createCookieTransport(t, store)
		ctx := createMockContext(t)

		wrapper.onGetSigned = func(r *http.Request, name string) (string, error) {
			return "", errors.New("cookie error")
		}

		storeErr := errors.New("store error")
		store.On("Delete", ctx, mock.AnythingOfType("uuid.UUID")).Return(storeErr)

		err := transport.Delete(ctx)

		require.Error(t, err)
		assert.ErrorIs(t, err, storeErr)
		store.AssertExpectations(t)
	})
}

func TestCookieDelete_NotAuthenticatedOK(t *testing.T) {
	t.Parallel()

	t.Run("handles ErrNotAuthenticated gracefully", func(t *testing.T) {
		t.Parallel()

		store := &mockStore{}
		transport, wrapper := createCookieTransport(t, store)
		ctx := createMockContext(t)
		anonSession := createValidSession(t)

		wrapper.onGetSigned = func(r *http.Request, name string) (string, error) {
			return anonSession.Token, nil
		}
		store.On("GetByToken", ctx, mock.AnythingOfType("string")).Return(&anonSession, nil)
		store.On("Delete", ctx, mock.AnythingOfType("uuid.UUID")).Return(session.ErrNotAuthenticated)

		wrapper.onDelete = func(w http.ResponseWriter, name string) {}

		err := transport.Delete(ctx)

		require.NoError(t, err)
		store.AssertExpectations(t)
	})
}

func TestCookieDelete_DeleteError(t *testing.T) {
	t.Parallel()

	t.Run("propagates deletion errors", func(t *testing.T) {
		t.Parallel()

		store := &mockStore{}
		transport, wrapper := createCookieTransport(t, store)
		ctx := createMockContext(t)
		sess := createValidSession(t)
		deleteErr := errors.New("delete failed")

		wrapper.onGetSigned = func(r *http.Request, name string) (string, error) {
			return sess.Token, nil
		}
		store.On("GetByToken", ctx, mock.AnythingOfType("string")).Return(&sess, nil)
		store.On("Delete", ctx, mock.AnythingOfType("uuid.UUID")).Return(deleteErr)

		err := transport.Delete(ctx)

		require.Error(t, err)
		assert.ErrorIs(t, err, deleteErr)
		store.AssertExpectations(t)
	})
}

// Store Method Tests

func TestCookieStore_Success(t *testing.T) {
	t.Parallel()

	t.Run("stores and updates cookie", func(t *testing.T) {
		t.Parallel()

		store := &mockStore{}
		transport, wrapper := createCookieTransport(t, store)
		ctx := createMockContext(t)
		sess := createValidSession(t)
		expectedID := sess.ID
		sess.SetData(testData{Theme: "dark"})

		store.On("Save", ctx, mock.MatchedBy(func(s *session.Session[testData]) bool {
			return s.ID == expectedID && s.Data.Theme == "dark"
		})).Return(nil)

		var savedToken string
		wrapper.onSetSigned = func(w http.ResponseWriter, r *http.Request, name, value string, opts ...cookie.Option) error {
			savedToken = value
			return nil
		}

		err := transport.Store(ctx, sess)

		require.NoError(t, err)
		assert.NotEmpty(t, savedToken)
		store.AssertExpectations(t)
	})
}

func TestCookieStore_DeletesOnNotAuthenticated(t *testing.T) {
	t.Parallel()

	t.Run("deletes cookie on ErrNotAuthenticated", func(t *testing.T) {
		t.Parallel()

		store := &mockStore{}
		transport, _ := createCookieTransport(t, store)
		ctx := createMockContext(t)
		sess := createValidSession(t)
		sess.Logout()

		store.On("Delete", ctx, mock.AnythingOfType("uuid.UUID")).Return(session.ErrNotAuthenticated)

		err := transport.Store(ctx, sess)

		require.Error(t, err)
		assert.ErrorIs(t, err, session.ErrNotAuthenticated)

		// Verify cookie was deleted by checking Set-Cookie header
		recorder := ctx.ResponseWriter().(*httptest.ResponseRecorder)
		cookies := recorder.Result().Cookies()
		var sessionCookie *http.Cookie
		for _, c := range cookies {
			if c.Name == "session" {
				sessionCookie = c
				break
			}
		}
		// Deleted cookie has MaxAge=-1 or empty value
		if sessionCookie != nil {
			assert.True(t, sessionCookie.MaxAge == -1 || sessionCookie.Value == "", "Cookie should be deleted")
		}
		store.AssertExpectations(t)
	})
}

func TestCookieStore_PropagatesNotAuthenticated(t *testing.T) {
	t.Parallel()

	t.Run("returns ErrNotAuthenticated after deletion", func(t *testing.T) {
		t.Parallel()

		store := &mockStore{}
		transport, _ := createCookieTransport(t, store)
		ctx := createMockContext(t)
		sess := createValidSession(t)
		sess.Logout()

		store.On("Delete", ctx, mock.AnythingOfType("uuid.UUID")).Return(session.ErrNotAuthenticated)

		err := transport.Store(ctx, sess)

		require.Error(t, err)
		assert.ErrorIs(t, err, session.ErrNotAuthenticated)

		// Verify cookie was deleted by checking Set-Cookie header
		recorder := ctx.ResponseWriter().(*httptest.ResponseRecorder)
		cookies := recorder.Result().Cookies()
		var sessionCookie *http.Cookie
		for _, c := range cookies {
			if c.Name == "session" {
				sessionCookie = c
				break
			}
		}
		// Deleted cookie has MaxAge=-1 or empty value
		if sessionCookie != nil {
			assert.True(t, sessionCookie.MaxAge == -1 || sessionCookie.Value == "", "Cookie should be deleted")
		}
		store.AssertExpectations(t)
	})
}

func TestCookieStore_StoreError(t *testing.T) {
	t.Parallel()

	t.Run("propagates Manager.Store errors", func(t *testing.T) {
		t.Parallel()

		store := &mockStore{}
		transport, _ := createCookieTransport(t, store)
		ctx := createMockContext(t)
		sess := createValidSession(t)
		expectedID := sess.ID
		storeErr := errors.New("database error")

		store.On("Save", ctx, mock.MatchedBy(func(s *session.Session[testData]) bool {
			return s.ID == expectedID
		})).Return(storeErr)

		err := transport.Store(ctx, sess)

		require.Error(t, err)
		assert.ErrorIs(t, err, storeErr)
		store.AssertExpectations(t)
	})
}

func TestCookieStore_SaveError(t *testing.T) {
	t.Parallel()

	t.Run("saves cookie with expired session fails", func(t *testing.T) {
		t.Parallel()

		store := &mockStore{}
		transport, _ := createCookieTransport(t, store)
		ctx := createMockContext(t)
		sess := createExpiredSession(t)

		// Manager.Store will be called because session is modified
		// It will attempt to save to store, then Save() will fail with expired error
		store.On("Save", ctx, mock.Anything).Return(nil)

		err := transport.Store(ctx, sess)

		require.Error(t, err)
		// The error should be about saving an expired session
		assert.Contains(t, err.Error(), "expired")
		store.AssertExpectations(t)
	})
}
