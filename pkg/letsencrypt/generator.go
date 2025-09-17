package letsencrypt

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-acme/lego/v4/certcrypto"
	"github.com/go-acme/lego/v4/certificate"
	"github.com/go-acme/lego/v4/challenge"
	"github.com/go-acme/lego/v4/challenge/http01"
	"github.com/go-acme/lego/v4/lego"
	"github.com/go-acme/lego/v4/registration"
)

// Option configures the certificate generator.
type Option func(*config) error

// WithCADirectoryURL overrides the ACME directory URL (defaults to Let's Encrypt production).
func WithCADirectoryURL(url string) Option {
	return func(cfg *config) error {
		cfg.caDirURL = strings.TrimSpace(url)
		return nil
	}
}

// WithHTTP01Address selects the bind address for the internal HTTP-01 challenge server (host:port).
// Leave empty to fall back to all interfaces on port 80.
func WithHTTP01Address(addr string) Option {
	return func(cfg *config) error {
		cfg.http01Address = strings.TrimSpace(addr)
		return nil
	}
}

// WithHTTP01ProxyHeader sets the header the challenge server inspects for host matching when behind a proxy (e.g. X-Forwarded-Host).
func WithHTTP01ProxyHeader(header string) Option {
	return func(cfg *config) error {
		cfg.proxyHeader = strings.TrimSpace(header)
		return nil
	}
}

// WithCertificateKeyType overrides the key type used for the issued certificate's private key.
func WithCertificateKeyType(keyType certcrypto.KeyType) Option {
	return func(cfg *config) error {
		cfg.certificateKeyType = keyType
		return nil
	}
}

// WithBundle toggles whether the returned certificate includes the issuer chain concatenated to the leaf cert (default true).
func WithBundle(bundle bool) Option {
	return func(cfg *config) error {
		cfg.bundle = bundle
		return nil
	}
}

// Generator issues certificates via Let's Encrypt and stores them on disk.
type Generator struct {
	cfg             config
	clientFactory   clientFactory
	accountKeyMaker func() (crypto.PrivateKey, error)
}

type config struct {
	domains            []string
	email              string
	outputDir          string
	caDirURL           string
	certificateKeyType certcrypto.KeyType
	bundle             bool
	http01Address      string
	http01Host         string
	http01Port         string
	proxyHeader        string
}

const (
	defaultDirectoryURL = lego.LEDirectoryProduction
	defaultHTTPPort     = "80"
)

// NewGenerator constructs a Generator for the provided domain list and account email.
// The first domain is used to derive the default filenames for the certificate artifacts.
func NewGenerator(domains []string, email, outputDir string, opts ...Option) (*Generator, error) {
	cfg := config{
		domains:            cloneStrings(domains),
		email:              strings.TrimSpace(email),
		outputDir:          strings.TrimSpace(outputDir),
		caDirURL:           defaultDirectoryURL,
		certificateKeyType: certcrypto.RSA2048,
		bundle:             true,
	}

	for _, opt := range opts {
		if opt == nil {
			continue
		}
		if err := opt(&cfg); err != nil {
			return nil, err
		}
	}

	if err := cfg.applyDefaults(); err != nil {
		return nil, err
	}

	gen := &Generator{
		cfg:           cfg,
		clientFactory: defaultClientFactory,
		accountKeyMaker: func() (crypto.PrivateKey, error) {
			return ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		},
	}

	return gen, nil
}

// Result captures the file paths of the generated certificate artifacts.
type Result struct {
	CertificatePath       string
	PrivateKeyPath        string
	IssuerCertificatePath string
}

// Generate obtains a fresh certificate from Let's Encrypt and writes it alongside the private key to disk.
func (g *Generator) Generate(ctx context.Context) (*Result, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	accountKey, err := g.accountKeyMaker()
	if err != nil {
		return nil, fmt.Errorf("generate account key: %w", err)
	}

	user := &accountUser{
		email: g.cfg.email,
		key:   accountKey,
	}

	legoCfg := lego.NewConfig(user)
	legoCfg.CADirURL = g.cfg.caDirURL
	legoCfg.Certificate.KeyType = g.cfg.certificateKeyType

	client, err := g.clientFactory(legoCfg)
	if err != nil {
		return nil, fmt.Errorf("create acme client: %w", err)
	}

	if err := ctx.Err(); err != nil {
		return nil, err
	}

	provider := http01.NewProviderServer(g.cfg.http01Host, g.cfg.http01Port)
	if g.cfg.proxyHeader != "" {
		provider.SetProxyHeader(g.cfg.proxyHeader)
	}

	if err := client.SetHTTP01Provider(provider); err != nil {
		return nil, fmt.Errorf("configure http-01 provider: %w", err)
	}

	registrationResource, err := client.Register(registration.RegisterOptions{TermsOfServiceAgreed: true})
	if err != nil {
		return nil, fmt.Errorf("register account: %w", err)
	}
	user.registration = registrationResource

	if err := ctx.Err(); err != nil {
		return nil, err
	}

	certRes, err := client.Obtain(certificate.ObtainRequest{
		Domains:        g.cfg.domains,
		Bundle:         g.cfg.bundle,
		EmailAddresses: []string{g.cfg.email},
	})
	if err != nil {
		return nil, fmt.Errorf("obtain certificate: %w", err)
	}

	result, err := g.writeCertificateArtifacts(certRes)
	if err != nil {
		return nil, err
	}

	return result, nil
}

func (g *Generator) writeCertificateArtifacts(certRes *certificate.Resource) (*Result, error) {
	if certRes == nil {
		return nil, errors.New("certificate resource is nil")
	}

	if err := os.MkdirAll(g.cfg.outputDir, 0o755); err != nil {
		return nil, fmt.Errorf("ensure output directory: %w", err)
	}

	baseName := safeFileSegment(g.cfg.domains[0])
	certPath := filepath.Join(g.cfg.outputDir, baseName+".crt")
	keyPath := filepath.Join(g.cfg.outputDir, baseName+".key")
	issuerPath := filepath.Join(g.cfg.outputDir, baseName+"-issuer.crt")

	if len(certRes.PrivateKey) == 0 {
		return nil, errors.New("empty private key received from ACME server")
	}

	if err := os.WriteFile(keyPath, certRes.PrivateKey, 0o600); err != nil {
		return nil, fmt.Errorf("write private key: %w", err)
	}

	if len(certRes.Certificate) == 0 {
		return nil, errors.New("empty certificate payload received from ACME server")
	}

	if err := os.WriteFile(certPath, certRes.Certificate, 0o644); err != nil {
		return nil, fmt.Errorf("write certificate: %w", err)
	}

	issuerWritten := false
	if len(certRes.IssuerCertificate) > 0 {
		if err := os.WriteFile(issuerPath, certRes.IssuerCertificate, 0o644); err != nil {
			return nil, fmt.Errorf("write issuer certificate: %w", err)
		}
		issuerWritten = true
	}

	result := &Result{
		CertificatePath: certPath,
		PrivateKeyPath:  keyPath,
	}
	if issuerWritten {
		result.IssuerCertificatePath = issuerPath
	}

	return result, nil
}

func (cfg *config) applyDefaults() error {
	if len(cfg.domains) == 0 {
		return errors.New("at least one domain is required")
	}

	for i := range cfg.domains {
		cfg.domains[i] = strings.TrimSpace(cfg.domains[i])
		if cfg.domains[i] == "" {
			return errors.New("domain entries cannot be empty")
		}
	}

	if cfg.email == "" {
		return errors.New("email is required")
	}

	if cfg.outputDir == "" {
		return errors.New("output directory is required")
	}

	if cfg.caDirURL == "" {
		cfg.caDirURL = defaultDirectoryURL
	}

	host, port, err := parseHTTPAddress(cfg.http01Address)
	if err != nil {
		return err
	}

	if port == "" {
		port = defaultHTTPPort
	}

	cfg.http01Host = host
	cfg.http01Port = port

	if cfg.certificateKeyType == "" {
		cfg.certificateKeyType = certcrypto.RSA2048
	}

	if cfg.proxyHeader != "" {
		cfg.proxyHeader = httpCanonicalHeader(cfg.proxyHeader)
	}

	return nil
}

func parseHTTPAddress(addr string) (string, string, error) {
	if strings.TrimSpace(addr) == "" {
		return "", "", nil
	}

	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return "", "", fmt.Errorf("invalid http-01 address %q: %w", addr, err)
	}

	return host, port, nil
}

func httpCanonicalHeader(header string) string {
	return textprotoCanonicalMIMEHeader(header)
}

func cloneStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}

	out := make([]string, len(values))
	copy(out, values)
	return out
}

func safeFileSegment(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	if value == "" {
		return "certificate"
	}

	var b strings.Builder
	b.Grow(len(value))

	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
		case r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '.' || r == '-' || r == '_':
			b.WriteRune(r)
		default:
			b.WriteRune('_')
		}
	}

	sanitized := strings.Trim(b.String(), "._-")
	if sanitized == "" {
		return "certificate"
	}

	return sanitized
}

type clientFactory func(*lego.Config) (acmeClient, error)

type acmeClient interface {
	Register(options registration.RegisterOptions) (*registration.Resource, error)
	SetHTTP01Provider(provider challenge.Provider) error
	Obtain(request certificate.ObtainRequest) (*certificate.Resource, error)
}

func defaultClientFactory(cfg *lego.Config) (acmeClient, error) {
	client, err := lego.NewClient(cfg)
	if err != nil {
		return nil, err
	}

	return &legoClientAdapter{client: client}, nil
}

type legoClientAdapter struct {
	client *lego.Client
}

func (l *legoClientAdapter) Register(options registration.RegisterOptions) (*registration.Resource, error) {
	return l.client.Registration.Register(options)
}

func (l *legoClientAdapter) SetHTTP01Provider(provider challenge.Provider) error {
	return l.client.Challenge.SetHTTP01Provider(provider)
}

func (l *legoClientAdapter) Obtain(request certificate.ObtainRequest) (*certificate.Resource, error) {
	return l.client.Certificate.Obtain(request)
}

type accountUser struct {
	email        string
	registration *registration.Resource
	key          crypto.PrivateKey
}

func (u *accountUser) GetEmail() string {
	return u.email
}

func (u *accountUser) GetRegistration() *registration.Resource {
	return u.registration
}

func (u *accountUser) GetPrivateKey() crypto.PrivateKey {
	return u.key
}

// textprotoCanonicalMIMEHeader is defined to prevent pulling the entire net/textproto package in tests.
var textprotoCanonicalMIMEHeader = func(v string) string {
	if v == "" {
		return ""
	}
	return http.CanonicalHeaderKey(v)
}
