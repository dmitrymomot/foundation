package letsencrypt_test

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"math/big"
	"net/http"
	"sync"
	"time"

	"golang.org/x/crypto/acme/autocert"
)

// mockACMEProvider is a test implementation of ACMEProvider
type mockACMEProvider struct {
	mu          sync.Mutex
	getCertFunc func(*tls.ClientHelloInfo) (*tls.Certificate, error)
	httpHandler http.Handler
	callCount   int
}

func (m *mockACMEProvider) GetCertificate(hello *tls.ClientHelloInfo) (*tls.Certificate, error) {
	m.mu.Lock()
	m.callCount++
	m.mu.Unlock()

	if m.getCertFunc != nil {
		return m.getCertFunc(hello)
	}
	return nil, errors.New("mock: GetCertificate not implemented")
}

func (m *mockACMEProvider) HTTPHandler(fallback http.Handler) http.Handler {
	if m.httpHandler != nil {
		return m.httpHandler
	}
	if fallback != nil {
		return fallback
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
}

func (m *mockACMEProvider) CallCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.callCount
}

// mockCache is a test implementation of autocert.Cache
type mockCache struct {
	mu         sync.Mutex
	data       map[string][]byte
	getFunc    func(ctx context.Context, key string) ([]byte, error)
	putFunc    func(ctx context.Context, key string, data []byte) error
	deleteFunc func(ctx context.Context, key string) error
}

func newMockCache() *mockCache {
	return &mockCache{
		data: make(map[string][]byte),
	}
}

func (m *mockCache) Get(ctx context.Context, key string) ([]byte, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.getFunc != nil {
		return m.getFunc(ctx, key)
	}

	data, ok := m.data[key]
	if !ok {
		return nil, autocert.ErrCacheMiss
	}
	return data, nil
}

func (m *mockCache) Put(ctx context.Context, key string, data []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.putFunc != nil {
		return m.putFunc(ctx, key, data)
	}

	m.data[key] = data
	return nil
}

func (m *mockCache) Delete(ctx context.Context, key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.deleteFunc != nil {
		return m.deleteFunc(ctx, key)
	}

	delete(m.data, key)
	return nil
}

// generateTestCertificate creates a valid self-signed certificate for testing
func generateTestCertificate(domain string) ([]byte, error) {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, err
	}

	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			CommonName: domain,
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(365 * 24 * time.Hour),
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		DNSNames:              []string{domain},
	}

	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	if err != nil {
		return nil, err
	}

	// Encode certificate
	certPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: certDER,
	})

	// Encode private key
	privDER, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		return nil, err
	}

	keyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: privDER,
	})

	// Combine cert and key (as autocert does)
	combined := append(certPEM, keyPEM...)
	return combined, nil
}
