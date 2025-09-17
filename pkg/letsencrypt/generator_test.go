package letsencrypt

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"os"
	"path/filepath"
	"testing"

	"github.com/go-acme/lego/v4/certificate"
	"github.com/go-acme/lego/v4/challenge"
	"github.com/go-acme/lego/v4/lego"
	"github.com/go-acme/lego/v4/registration"
)

func TestNewGeneratorValidation(t *testing.T) {
	_, err := NewGenerator(nil, "", "")
	if err == nil {
		t.Fatalf("expected error when mandatory config missing")
	}

	_, err = NewGenerator([]string{""}, "admin@example.com", "/tmp")
	if err == nil {
		t.Fatalf("expected error for empty domain entry")
	}

	_, err = NewGenerator([]string{"example.com"}, "", "/tmp")
	if err == nil {
		t.Fatalf("expected error when email missing")
	}

	_, err = NewGenerator([]string{"example.com"}, "admin@example.com", "")
	if err == nil {
		t.Fatalf("expected error when output dir missing")
	}

	_, err = NewGenerator([]string{"example.com"}, "admin@example.com", "/tmp", WithHTTP01Address("bad-address"))
	if err == nil {
		t.Fatalf("expected error for malformed http-01 address")
	}
}

func TestGenerateWritesArtifacts(t *testing.T) {
	gen, err := NewGenerator([]string{"example.com"}, "admin@example.com", t.TempDir(), WithCADirectoryURL("https://example.test/directory"))
	if err != nil {
		t.Fatalf("NewGenerator: %v", err)
	}

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate test key: %v", err)
	}

	stub := &stubClient{}
	gen.clientFactory = func(*lego.Config) (acmeClient, error) {
		return stub, nil
	}
	gen.accountKeyMaker = func() (crypto.PrivateKey, error) {
		return key, nil
	}

	result, err := gen.Generate(context.Background())
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	if !stub.providerConfigured {
		t.Fatalf("expected http-01 provider to be configured")
	}

	if !stub.registered {
		t.Fatalf("expected ACME registration to occur")
	}

	if result.CertificatePath == "" || result.PrivateKeyPath == "" {
		t.Fatalf("unexpected empty result paths: %+v", result)
	}

	assertFileContents(t, result.PrivateKeyPath, stub.lastResource.PrivateKey)
	assertFileContents(t, result.CertificatePath, stub.lastResource.Certificate)

	if stub.lastResource.IssuerCertificate != nil {
		if result.IssuerCertificatePath == "" {
			t.Fatalf("expected issuer certificate path to be set")
		}
		assertFileContents(t, result.IssuerCertificatePath, stub.lastResource.IssuerCertificate)
	}

	name := filepath.Base(result.CertificatePath)
	if name != "example.com.crt" {
		t.Fatalf("unexpected certificate filename: %s", name)
	}
}

func assertFileContents(t *testing.T, path string, want []byte) {
	t.Helper()

	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}

	if string(b) != string(want) {
		t.Fatalf("unexpected file contents for %s: got %q want %q", path, string(b), string(want))
	}
}

type stubClient struct {
	providerConfigured bool
	registered         bool
	lastResource       *certificate.Resource
}

func (s *stubClient) Register(registration.RegisterOptions) (*registration.Resource, error) {
	s.registered = true
	return &registration.Resource{}, nil
}

func (s *stubClient) SetHTTP01Provider(challenge.Provider) error {
	s.providerConfigured = true
	return nil
}

func (s *stubClient) Obtain(certificate.ObtainRequest) (*certificate.Resource, error) {
	s.lastResource = &certificate.Resource{
		Certificate:       []byte("cert-data"),
		PrivateKey:        []byte("key-data"),
		IssuerCertificate: []byte("issuer-data"),
	}
	return s.lastResource, nil
}
