package letsencrypt_test

import (
	"crypto/tls"
	"errors"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

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
