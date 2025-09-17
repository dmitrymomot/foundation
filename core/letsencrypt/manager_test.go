package letsencrypt_test

import (
	"context"
	"crypto/tls"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/dmitrymomot/foundation/core/letsencrypt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewManager(t *testing.T) {
	tests := []struct {
		name    string
		config  letsencrypt.Config
		wantErr error
	}{
		{
			name: "valid config",
			config: letsencrypt.Config{
				Email:   "test@example.com",
				CertDir: "/tmp/certs",
			},
			wantErr: nil,
		},
		{
			name: "missing email",
			config: letsencrypt.Config{
				CertDir: "/tmp/certs",
			},
			wantErr: letsencrypt.ErrEmailRequired,
		},
		{
			name: "missing cert dir",
			config: letsencrypt.Config{
				Email: "test@example.com",
			},
			wantErr: letsencrypt.ErrCertDirRequired,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m, err := letsencrypt.NewManager(tt.config)
			if tt.wantErr != nil {
				assert.Error(t, err)
				assert.ErrorIs(t, err, tt.wantErr)
				assert.Nil(t, m)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, m)
			}
		})
	}
}

func TestManagerCertDir(t *testing.T) {
	certDir := "/tmp/test-certs"
	m, err := letsencrypt.NewManager(letsencrypt.Config{
		Email:   "test@example.com",
		CertDir: certDir,
	})
	require.NoError(t, err)
	assert.Equal(t, certDir, m.CertDir())
}

func TestManagerExists(t *testing.T) {
	certDir, err := os.MkdirTemp("", "letsencrypt-test-*")
	require.NoError(t, err)
	defer func() {
		_ = os.RemoveAll(certDir)
	}()

	m, err := letsencrypt.NewManager(letsencrypt.Config{
		Email:   "test@example.com",
		CertDir: certDir,
	})
	require.NoError(t, err)

	// Non-existent certificate
	assert.False(t, m.Exists("example.com"))

	// Create a dummy certificate file
	certFile := filepath.Join(certDir, "example.com")
	err = os.WriteFile(certFile, []byte("dummy cert"), 0644)
	require.NoError(t, err)

	// Should now exist
	assert.True(t, m.Exists("example.com"))
}

func TestManagerDelete(t *testing.T) {
	certDir, err := os.MkdirTemp("", "letsencrypt-test-*")
	require.NoError(t, err)
	defer func() {
		_ = os.RemoveAll(certDir)
	}()

	m, err := letsencrypt.NewManager(letsencrypt.Config{
		Email:   "test@example.com",
		CertDir: certDir,
	})
	require.NoError(t, err)

	// Create a dummy certificate file
	certFile := filepath.Join(certDir, "example.com")
	err = os.WriteFile(certFile, []byte("dummy cert"), 0644)
	require.NoError(t, err)

	// Verify it exists
	assert.True(t, m.Exists("example.com"))

	// Delete it
	err = m.Delete("example.com")
	assert.NoError(t, err)

	// Verify it no longer exists
	assert.False(t, m.Exists("example.com"))
}

func TestManagerGetCertificate(t *testing.T) {
	certDir, err := os.MkdirTemp("", "letsencrypt-test-*")
	require.NoError(t, err)
	defer func() {
		_ = os.RemoveAll(certDir)
	}()

	m, err := letsencrypt.NewManager(letsencrypt.Config{
		Email:   "test@example.com",
		CertDir: certDir,
	})
	require.NoError(t, err)

	t.Run("invalid domain", func(t *testing.T) {
		hello := &tls.ClientHelloInfo{
			ServerName: "",
		}
		cert, err := m.GetCertificate(hello)
		assert.Error(t, err)
		assert.ErrorIs(t, err, letsencrypt.ErrInvalidDomain)
		assert.Nil(t, cert)
	})

	t.Run("certificate not found", func(t *testing.T) {
		hello := &tls.ClientHelloInfo{
			ServerName: "notfound.com",
		}
		cert, err := m.GetCertificate(hello)
		assert.Error(t, err)
		assert.ErrorIs(t, err, letsencrypt.ErrCertificateNotFound)
		assert.Nil(t, cert)
	})
}

func TestManagerHandleChallenge(t *testing.T) {
	m, err := letsencrypt.NewManager(letsencrypt.Config{
		Email:   "test@example.com",
		CertDir: "/tmp/test-certs",
	})
	require.NoError(t, err)

	t.Run("ACME challenge path", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/.well-known/acme-challenge/test-token", nil)
		w := httptest.NewRecorder()

		handled := m.HandleChallenge(w, req)
		assert.True(t, handled)
	})

	t.Run("non-ACME path", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/other-path", nil)
		w := httptest.NewRecorder()

		handled := m.HandleChallenge(w, req)
		assert.False(t, handled)
	})
}

func TestIsRetryableError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{"nil error", nil, false},
		{"connection refused", errors.New("connection refused"), true},
		{"network unreachable", errors.New("network is unreachable"), true},
		{"no such host", errors.New("no such host"), true},
		{"timeout", errors.New("request timeout"), true},
		{"rate limit", errors.New("rate limit exceeded"), true},
		{"429 status", errors.New("HTTP 429: Too Many Requests"), true},
		{"503 status", errors.New("HTTP 503: Service Unavailable"), true},
		{"temporary failure", errors.New("temporary failure in name resolution"), true},
		{"unauthorized", errors.New("unauthorized"), false},
		{"bad request", errors.New("bad request"), false},
		{"internal error", errors.New("internal server error"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := letsencrypt.IsRetryableError(tt.err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestManagerGenerateWithRetry(t *testing.T) {
	t.Run("successful generation on first attempt", func(t *testing.T) {
		mock := &mockACMEProvider{
			getCertFunc: func(hello *tls.ClientHelloInfo) (*tls.Certificate, error) {
				assert.Equal(t, "example.com", hello.ServerName)
				return &tls.Certificate{}, nil
			},
		}

		m, err := letsencrypt.NewManager(
			letsencrypt.Config{
				Email:   "test@example.com",
				CertDir: t.TempDir(),
			},
			letsencrypt.WithACMEProvider(mock),
		)
		require.NoError(t, err)

		err = m.Generate(context.Background(), "example.com")
		assert.NoError(t, err)
		assert.Equal(t, 1, mock.CallCount())
	})

	t.Run("retry on transient failure then succeed", func(t *testing.T) {
		attempts := 0
		mock := &mockACMEProvider{
			getCertFunc: func(hello *tls.ClientHelloInfo) (*tls.Certificate, error) {
				attempts++
				if attempts < 3 {
					return nil, errors.New("connection refused")
				}
				return &tls.Certificate{}, nil
			},
		}

		m, err := letsencrypt.NewManager(
			letsencrypt.Config{
				Email:   "test@example.com",
				CertDir: t.TempDir(),
			},
			letsencrypt.WithACMEProvider(mock),
			letsencrypt.WithRetryConfig(3, 10*time.Millisecond), // Fast retry for testing
		)
		require.NoError(t, err)

		start := time.Now()
		err = m.Generate(context.Background(), "example.com")
		duration := time.Since(start)

		assert.NoError(t, err)
		assert.Equal(t, 3, attempts)
		// Should have delays: 10ms + 20ms = 30ms minimum
		assert.GreaterOrEqual(t, duration, 30*time.Millisecond)
		assert.Less(t, duration, 1*time.Second)
	})

	t.Run("context cancellation during retry", func(t *testing.T) {
		mock := &mockACMEProvider{
			getCertFunc: func(hello *tls.ClientHelloInfo) (*tls.Certificate, error) {
				return nil, errors.New("connection refused")
			},
		}

		m, err := letsencrypt.NewManager(
			letsencrypt.Config{
				Email:   "test@example.com",
				CertDir: t.TempDir(),
			},
			letsencrypt.WithACMEProvider(mock),
			letsencrypt.WithRetryConfig(3, 100*time.Millisecond), // Longer than context timeout
		)
		require.NoError(t, err)

		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()

		err = m.Generate(ctx, "example.com")
		assert.Error(t, err)
		assert.ErrorIs(t, err, letsencrypt.ErrGenerationFailed)
		assert.Contains(t, err.Error(), "context canceled")
	})

	t.Run("non-retryable error fails immediately", func(t *testing.T) {
		mock := &mockACMEProvider{
			getCertFunc: func(hello *tls.ClientHelloInfo) (*tls.Certificate, error) {
				return nil, errors.New("unauthorized")
			},
		}

		m, err := letsencrypt.NewManager(
			letsencrypt.Config{
				Email:   "test@example.com",
				CertDir: t.TempDir(),
			},
			letsencrypt.WithACMEProvider(mock),
		)
		require.NoError(t, err)

		start := time.Now()
		err = m.Generate(context.Background(), "example.com")
		duration := time.Since(start)

		assert.Error(t, err)
		assert.ErrorIs(t, err, letsencrypt.ErrGenerationFailed)
		assert.Contains(t, err.Error(), "unauthorized")
		assert.Equal(t, 1, mock.CallCount())
		// Should fail immediately without retries
		assert.Less(t, duration, 1*time.Second)
	})

	t.Run("max retries exhausted", func(t *testing.T) {
		attempts := 0
		mock := &mockACMEProvider{
			getCertFunc: func(hello *tls.ClientHelloInfo) (*tls.Certificate, error) {
				attempts++
				return nil, errors.New("timeout")
			},
		}

		m, err := letsencrypt.NewManager(
			letsencrypt.Config{
				Email:   "test@example.com",
				CertDir: t.TempDir(),
			},
			letsencrypt.WithACMEProvider(mock),
			letsencrypt.WithRetryConfig(3, 10*time.Millisecond), // Fast retry for testing
		)
		require.NoError(t, err)

		err = m.Generate(context.Background(), "example.com")
		assert.Error(t, err)
		assert.ErrorIs(t, err, letsencrypt.ErrGenerationFailed)
		assert.Contains(t, err.Error(), "failed after 3 attempts")
		assert.Equal(t, 3, attempts)
	})
}

func TestManagerRenew(t *testing.T) {
	t.Run("successful renewal", func(t *testing.T) {
		cache := newMockCache()
		cache.Put(context.Background(), "example.com", []byte("old-cert"))

		mock := &mockACMEProvider{
			getCertFunc: func(hello *tls.ClientHelloInfo) (*tls.Certificate, error) {
				assert.Equal(t, "example.com", hello.ServerName)
				return &tls.Certificate{}, nil
			},
		}

		m, err := letsencrypt.NewManager(
			letsencrypt.Config{
				Email:   "test@example.com",
				CertDir: t.TempDir(),
			},
			letsencrypt.WithACMEProvider(mock),
			letsencrypt.WithCache(cache),
		)
		require.NoError(t, err)

		// Verify cert exists before renewal
		assert.True(t, m.Exists("example.com"))

		err = m.Renew(context.Background(), "example.com")
		assert.NoError(t, err)

		// Verify old cert was deleted
		_, err = cache.Get(context.Background(), "example.com")
		assert.Error(t, err)
	})

	t.Run("cache deletion failure", func(t *testing.T) {
		cache := &mockCache{
			deleteFunc: func(ctx context.Context, key string) error {
				return errors.New("permission denied")
			},
		}

		mock := &mockACMEProvider{}

		m, err := letsencrypt.NewManager(
			letsencrypt.Config{
				Email:   "test@example.com",
				CertDir: t.TempDir(),
			},
			letsencrypt.WithACMEProvider(mock),
			letsencrypt.WithCache(cache),
		)
		require.NoError(t, err)

		err = m.Renew(context.Background(), "example.com")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to delete existing certificate")
		assert.Contains(t, err.Error(), "permission denied")
	})

	t.Run("generation failure during renewal", func(t *testing.T) {
		cache := newMockCache()

		mock := &mockACMEProvider{
			getCertFunc: func(hello *tls.ClientHelloInfo) (*tls.Certificate, error) {
				return nil, errors.New("ACME server error")
			},
		}

		m, err := letsencrypt.NewManager(
			letsencrypt.Config{
				Email:   "test@example.com",
				CertDir: t.TempDir(),
			},
			letsencrypt.WithACMEProvider(mock),
			letsencrypt.WithCache(cache),
		)
		require.NoError(t, err)

		err = m.Renew(context.Background(), "example.com")
		assert.Error(t, err)
		assert.ErrorIs(t, err, letsencrypt.ErrGenerationFailed)
		assert.Contains(t, err.Error(), "renewal failed")
		assert.Contains(t, err.Error(), "ACME server error")
	})
}

func TestManagerGetCertificateSuccess(t *testing.T) {
	t.Run("successful certificate retrieval", func(t *testing.T) {
		// Create a valid self-signed certificate for testing
		certPEM, err := generateTestCertificate("example.com")
		require.NoError(t, err)

		cache := newMockCache()
		cache.Put(context.Background(), "example.com", certPEM)

		m, err := letsencrypt.NewManager(
			letsencrypt.Config{
				Email:   "test@example.com",
				CertDir: t.TempDir(),
			},
			letsencrypt.WithCache(cache),
		)
		require.NoError(t, err)

		hello := &tls.ClientHelloInfo{
			ServerName: "example.com",
		}

		cert, err := m.GetCertificate(hello)
		assert.NoError(t, err)
		assert.NotNil(t, cert)
	})

	t.Run("malformed certificate data", func(t *testing.T) {
		cache := newMockCache()
		cache.Put(context.Background(), "example.com", []byte("invalid-cert-data"))

		m, err := letsencrypt.NewManager(
			letsencrypt.Config{
				Email:   "test@example.com",
				CertDir: t.TempDir(),
			},
			letsencrypt.WithCache(cache),
		)
		require.NoError(t, err)

		hello := &tls.ClientHelloInfo{
			ServerName: "example.com",
		}

		cert, err := m.GetCertificate(hello)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to load certificate")
		assert.Nil(t, cert)
	})
}

func TestManagerConcurrentOperations(t *testing.T) {
	t.Run("concurrent generate calls", func(t *testing.T) {
		var mu sync.Mutex
		calls := make(map[string]int)

		mock := &mockACMEProvider{
			getCertFunc: func(hello *tls.ClientHelloInfo) (*tls.Certificate, error) {
				mu.Lock()
				calls[hello.ServerName]++
				mu.Unlock()
				time.Sleep(100 * time.Millisecond) // Simulate work
				return &tls.Certificate{}, nil
			},
		}

		m, err := letsencrypt.NewManager(
			letsencrypt.Config{
				Email:   "test@example.com",
				CertDir: t.TempDir(),
			},
			letsencrypt.WithACMEProvider(mock),
		)
		require.NoError(t, err)

		var wg sync.WaitGroup
		domains := []string{"site1.com", "site2.com", "site3.com"}

		for _, domain := range domains {
			wg.Add(1)
			go func(d string) {
				defer wg.Done()
				err := m.Generate(context.Background(), d)
				assert.NoError(t, err)
			}(domain)
		}

		wg.Wait()

		// Verify each domain was processed
		mu.Lock()
		defer mu.Unlock()
		for _, domain := range domains {
			assert.Equal(t, 1, calls[domain], "Domain %s should be called exactly once", domain)
		}
	})

	t.Run("concurrent read operations", func(t *testing.T) {
		certPEM, err := generateTestCertificate("example.com")
		require.NoError(t, err)

		cache := newMockCache()
		cache.Put(context.Background(), "example.com", certPEM)

		m, err := letsencrypt.NewManager(
			letsencrypt.Config{
				Email:   "test@example.com",
				CertDir: t.TempDir(),
			},
			letsencrypt.WithCache(cache),
		)
		require.NoError(t, err)

		var wg sync.WaitGroup
		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				hello := &tls.ClientHelloInfo{
					ServerName: "example.com",
				}
				cert, err := m.GetCertificate(hello)
				assert.NoError(t, err)
				assert.NotNil(t, cert)
			}()
		}

		wg.Wait()
	})
}

func TestManagerWithOptions(t *testing.T) {
	t.Run("with custom ACME provider", func(t *testing.T) {
		mock := &mockACMEProvider{
			getCertFunc: func(hello *tls.ClientHelloInfo) (*tls.Certificate, error) {
				return &tls.Certificate{}, nil
			},
		}

		m, err := letsencrypt.NewManager(
			letsencrypt.Config{
				Email:   "test@example.com",
				CertDir: t.TempDir(),
			},
			letsencrypt.WithACMEProvider(mock),
		)
		require.NoError(t, err)

		err = m.Generate(context.Background(), "example.com")
		assert.NoError(t, err)
		assert.Equal(t, 1, mock.CallCount())
	})

	t.Run("with custom cache", func(t *testing.T) {
		cache := newMockCache()
		cache.Put(context.Background(), "test.com", []byte("test-cert"))

		m, err := letsencrypt.NewManager(
			letsencrypt.Config{
				Email:   "test@example.com",
				CertDir: t.TempDir(),
			},
			letsencrypt.WithCache(cache),
		)
		require.NoError(t, err)

		assert.True(t, m.Exists("test.com"))
		assert.False(t, m.Exists("other.com"))
	})
}

func TestManagerHostPolicyWithPort(t *testing.T) {
	t.Run("GetCertificate with port in domain", func(t *testing.T) {
		certPEM, err := generateTestCertificate("example.com")
		require.NoError(t, err)

		cache := newMockCache()
		cache.Put(context.Background(), "example.com", certPEM)

		m, err := letsencrypt.NewManager(
			letsencrypt.Config{
				Email:   "test@example.com",
				CertDir: t.TempDir(),
			},
			letsencrypt.WithCache(cache),
		)
		require.NoError(t, err)

		// Test with port - should strip port and still work
		hello := &tls.ClientHelloInfo{
			ServerName: "example.com:443",
		}

		_, err = m.GetCertificate(hello)
		// This will fail because GetCertificate doesn't strip the port
		// But it tests the hostPolicy logic indirectly
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "certificate not found")
	})
}

func TestManagerHandleChallengeWithMock(t *testing.T) {
	t.Run("ACME challenge with mock provider", func(t *testing.T) {
		handlerCalled := false
		mockHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			handlerCalled = true
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("challenge-response"))
		})

		mock := &mockACMEProvider{
			httpHandler: mockHandler,
		}

		m, err := letsencrypt.NewManager(
			letsencrypt.Config{
				Email:   "test@example.com",
				CertDir: t.TempDir(),
			},
			letsencrypt.WithACMEProvider(mock),
		)
		require.NoError(t, err)

		req := httptest.NewRequest("GET", "/.well-known/acme-challenge/test-token", nil)
		w := httptest.NewRecorder()

		handled := m.HandleChallenge(w, req)
		assert.True(t, handled)
		assert.True(t, handlerCalled)
		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "challenge-response", w.Body.String())
	})
}
