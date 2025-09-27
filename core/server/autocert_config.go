package server

// AutoCertConfig extends the base Config with AutoCert-specific settings.
// IMPORTANT: The embedded Config.Addr field is ignored - use HTTPAddr/HTTPSAddr instead.
type AutoCertConfig struct {
	// Embed the base Config to get all timeout settings
	// We'll ignore the Addr field and use HTTPAddr/HTTPSAddr instead
	Config

	// AutoCert-specific addresses (instead of single Addr)
	HTTPAddr  string `env:"AUTOCERT_HTTP_ADDR" envDefault:":80"`
	HTTPSAddr string `env:"AUTOCERT_HTTPS_ADDR" envDefault:":443"`
}

// DefaultAutoCertConfig returns config with sensible defaults.
func DefaultAutoCertConfig() AutoCertConfig {
	baseConfig := DefaultConfig()
	// Clear the Addr since we don't use it
	baseConfig.Addr = ""

	return AutoCertConfig{
		Config:    baseConfig,
		HTTPAddr:  ":80",
		HTTPSAddr: ":443",
	}
}

// WithCertManager sets the certificate manager (REQUIRED).
func WithCertManager(cm CertificateManager) AutoCertOption {
	return func(s *AutoCertServer) {
		s.certManager = cm
	}
}

// WithDomainStore sets the domain store (REQUIRED).
func WithDomainStore(ds DomainStore) AutoCertOption {
	return func(s *AutoCertServer) {
		s.domainStore = ds
	}
}

// WithHTTPServer replaces the HTTP server.
func WithHTTPServer(server *Server) AutoCertOption {
	return func(s *AutoCertServer) {
		s.httpServer = server
	}
}

// WithHTTPSServer replaces the HTTPS server.
func WithHTTPSServer(server *Server) AutoCertOption {
	return func(s *AutoCertServer) {
		s.httpsServer = server
	}
}

// WithServerOptions applies options to both HTTP and HTTPS servers.
func WithServerOptions(opts ...Option) AutoCertOption {
	return func(s *AutoCertServer) {
		for _, opt := range opts {
			opt(s.httpServer)
			opt(s.httpsServer)
		}
	}
}

// WithProvisioningHandler sets custom provisioning handler.
func WithProvisioningHandler(handler ProvisioningHandler) AutoCertOption {
	return func(s *AutoCertServer) {
		s.provisioningHandler = handler
	}
}

// WithFailedHandler sets custom failed handler.
func WithFailedHandler(handler FailedHandler) AutoCertOption {
	return func(s *AutoCertServer) {
		s.failedHandler = handler
	}
}

// WithNotFoundHandler sets custom not found handler.
func WithNotFoundHandler(handler NotFoundHandler) AutoCertOption {
	return func(s *AutoCertServer) {
		s.notFoundHandler = handler
	}
}

// NewAutoCertFromConfig creates AutoCertServer from environment config.
// We DON'T use NewFromConfig because we need two servers with different addresses.
func NewAutoCertFromConfig(
	cfg AutoCertConfig,
	certManager CertificateManager,
	domainStore DomainStore,
	opts ...AutoCertOption,
) (*AutoCertServer, error) {
	// Create HTTP server directly with all config values EXCEPT Addr
	httpServer := New(cfg.HTTPAddr,
		WithReadTimeout(cfg.ReadTimeout),         // From embedded Config
		WithWriteTimeout(cfg.WriteTimeout),       // From embedded Config
		WithIdleTimeout(cfg.IdleTimeout),         // From embedded Config
		WithShutdownTimeout(cfg.ShutdownTimeout), // From embedded Config
		WithMaxHeaderBytes(cfg.MaxHeaderBytes),   // From embedded Config
	)

	// Create HTTPS server with same timeouts but different address
	httpsServer := New(cfg.HTTPSAddr,
		WithReadTimeout(cfg.ReadTimeout),
		WithWriteTimeout(cfg.WriteTimeout),
		WithIdleTimeout(cfg.IdleTimeout),
		WithShutdownTimeout(cfg.ShutdownTimeout),
		WithMaxHeaderBytes(cfg.MaxHeaderBytes),
	)

	// Build options
	configOpts := []AutoCertOption{
		WithCertManager(certManager),
		WithDomainStore(domainStore),
		WithHTTPServer(httpServer),
		WithHTTPSServer(httpsServer),
	}

	// Append user options (can override)
	configOpts = append(configOpts, opts...)

	return NewAutoCertServer(configOpts...)
}
