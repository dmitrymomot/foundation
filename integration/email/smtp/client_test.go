package smtp_test

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dmitrymomot/foundation/core/email"
	"github.com/dmitrymomot/foundation/integration/email/smtp"
)

func TestNewClient_Validation(t *testing.T) {
	t.Parallel()

	validConfig := smtp.Config{
		Host:         "smtp.example.com",
		Port:         587,
		Username:     "user@example.com",
		Password:     "password",
		TLSMode:      "starttls",
		SenderEmail:  "sender@example.com",
		SupportEmail: "support@example.com",
	}

	tests := []struct {
		name    string
		config  smtp.Config
		wantErr bool
		errMsg  string
	}{
		{
			name:    "valid config",
			config:  validConfig,
			wantErr: false,
		},
		{
			name: "empty host",
			config: func() smtp.Config {
				cfg := validConfig
				cfg.Host = ""
				return cfg
			}(),
			wantErr: true,
			errMsg:  "Host is required",
		},
		{
			name: "invalid port - zero",
			config: func() smtp.Config {
				cfg := validConfig
				cfg.Port = 0
				return cfg
			}(),
			wantErr: true,
			errMsg:  "Port must be between 1 and 65535",
		},
		{
			name: "invalid port - too high",
			config: func() smtp.Config {
				cfg := validConfig
				cfg.Port = 70000
				return cfg
			}(),
			wantErr: true,
			errMsg:  "Port must be between 1 and 65535",
		},
		{
			name: "empty username",
			config: func() smtp.Config {
				cfg := validConfig
				cfg.Username = ""
				return cfg
			}(),
			wantErr: true,
			errMsg:  "Username is required",
		},
		{
			name: "empty password",
			config: func() smtp.Config {
				cfg := validConfig
				cfg.Password = ""
				return cfg
			}(),
			wantErr: true,
			errMsg:  "Password is required",
		},
		{
			name: "invalid TLS mode",
			config: func() smtp.Config {
				cfg := validConfig
				cfg.TLSMode = "invalid"
				return cfg
			}(),
			wantErr: true,
			errMsg:  "TLSMode must be starttls, tls, or plain",
		},
		{
			name: "valid TLS mode - tls",
			config: func() smtp.Config {
				cfg := validConfig
				cfg.TLSMode = "tls"
				return cfg
			}(),
			wantErr: false,
		},
		{
			name: "valid TLS mode - plain",
			config: func() smtp.Config {
				cfg := validConfig
				cfg.TLSMode = "plain"
				return cfg
			}(),
			wantErr: false,
		},
		{
			name: "empty sender email",
			config: func() smtp.Config {
				cfg := validConfig
				cfg.SenderEmail = ""
				return cfg
			}(),
			wantErr: true,
			errMsg:  "SenderEmail must be a valid email address",
		},
		{
			name: "invalid sender email",
			config: func() smtp.Config {
				cfg := validConfig
				cfg.SenderEmail = "not-an-email"
				return cfg
			}(),
			wantErr: true,
			errMsg:  "SenderEmail must be a valid email address",
		},
		{
			name: "empty support email",
			config: func() smtp.Config {
				cfg := validConfig
				cfg.SupportEmail = ""
				return cfg
			}(),
			wantErr: true,
			errMsg:  "SupportEmail must be a valid email address",
		},
		{
			name: "invalid support email",
			config: func() smtp.Config {
				cfg := validConfig
				cfg.SupportEmail = "invalid@"
				return cfg
			}(),
			wantErr: true,
			errMsg:  "SupportEmail must be a valid email address",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			client, err := smtp.New(tt.config)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, client)
				assert.ErrorIs(t, err, email.ErrInvalidConfig)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, client)
			}
		})
	}
}

func TestMustNewClient(t *testing.T) {
	t.Parallel()

	t.Run("valid config", func(t *testing.T) {
		t.Parallel()

		config := smtp.Config{
			Host:         "smtp.example.com",
			Port:         587,
			Username:     "user@example.com",
			Password:     "password",
			TLSMode:      "starttls",
			SenderEmail:  "sender@example.com",
			SupportEmail: "support@example.com",
		}

		assert.NotPanics(t, func() {
			client := smtp.MustNewClient(config)
			assert.NotNil(t, client)
		})
	})

	t.Run("invalid config panics", func(t *testing.T) {
		t.Parallel()

		config := smtp.Config{
			Host: "", // Invalid
		}

		assert.Panics(t, func() {
			smtp.MustNewClient(config)
		})
	})
}

func TestClient_SendEmail_Validation(t *testing.T) {
	t.Parallel()

	config := smtp.Config{
		Host:         "smtp.example.com",
		Port:         587,
		Username:     "user@example.com",
		Password:     "password",
		TLSMode:      "starttls",
		SenderEmail:  "sender@example.com",
		SupportEmail: "support@example.com",
	}

	client, err := smtp.New(config)
	require.NoError(t, err)

	ctx := context.Background()

	tests := []struct {
		name    string
		params  email.SendEmailParams
		wantErr bool
		errType error
	}{
		{
			name: "invalid params - empty SendTo",
			params: email.SendEmailParams{
				SendTo:   "",
				Subject:  "Test",
				BodyHTML: "<p>Test</p>",
			},
			wantErr: true,
			errType: email.ErrInvalidParams,
		},
		{
			name: "invalid params - invalid email",
			params: email.SendEmailParams{
				SendTo:   "invalid-email",
				Subject:  "Test",
				BodyHTML: "<p>Test</p>",
			},
			wantErr: true,
			errType: email.ErrInvalidParams,
		},
		{
			name: "invalid params - empty subject",
			params: email.SendEmailParams{
				SendTo:   "user@example.com",
				Subject:  "",
				BodyHTML: "<p>Test</p>",
			},
			wantErr: true,
			errType: email.ErrInvalidParams,
		},
		{
			name: "invalid params - empty body",
			params: email.SendEmailParams{
				SendTo:   "user@example.com",
				Subject:  "Test",
				BodyHTML: "",
			},
			wantErr: true,
			errType: email.ErrInvalidParams,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := client.SendEmail(ctx, tt.params)
			if tt.wantErr {
				assert.Error(t, err)
				assert.ErrorIs(t, err, tt.errType)
			} else {
				// We can't test successful sending without a real SMTP server
				// but we ensure validation passes
				assert.NoError(t, err)
			}
		})
	}
}

func TestClient_SendEmail_ConnectionError(t *testing.T) {
	t.Parallel()

	// Find an unused port
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	port := listener.Addr().(*net.TCPAddr).Port
	listener.Close()

	config := smtp.Config{
		Host:         "127.0.0.1",
		Port:         port, // Use the unused port
		Username:     "user@example.com",
		Password:     "password",
		TLSMode:      "plain",
		SenderEmail:  "sender@example.com",
		SupportEmail: "support@example.com",
	}

	client, err := smtp.New(config)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	params := email.SendEmailParams{
		SendTo:   "user@example.com",
		Subject:  "Test Email",
		BodyHTML: "<p>Test content</p>",
		Tag:      "test",
	}

	// Should fail to connect
	err = client.SendEmail(ctx, params)
	assert.Error(t, err)
	assert.ErrorIs(t, err, email.ErrFailedToSendEmail)
	assert.Contains(t, err.Error(), "failed to connect to SMTP server")
}

func TestClient_BuildMessage(t *testing.T) {
	t.Parallel()

	config := smtp.Config{
		Host:         "smtp.example.com",
		Port:         587,
		Username:     "user@example.com",
		Password:     "password",
		TLSMode:      "starttls",
		SenderEmail:  "sender@example.com",
		SupportEmail: "support@example.com",
	}

	client, err := smtp.New(config)
	require.NoError(t, err)

	// We can't directly test buildMessage as it's private,
	// but we can verify the client is created properly
	assert.NotNil(t, client)

	// Test that the client implements the EmailSender interface
	var _ email.EmailSender = client
}

func TestClient_TLSModes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		tlsMode string
		valid   bool
	}{
		{"starttls", "starttls", true},
		{"tls", "tls", true},
		{"plain", "plain", true},
		{"invalid", "ssl", false},
		{"empty", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			config := smtp.Config{
				Host:         "smtp.example.com",
				Port:         587,
				Username:     "user@example.com",
				Password:     "password",
				TLSMode:      tt.tlsMode,
				SenderEmail:  "sender@example.com",
				SupportEmail: "support@example.com",
			}

			client, err := smtp.New(config)
			if tt.valid {
				assert.NoError(t, err)
				assert.NotNil(t, client)
			} else {
				assert.Error(t, err)
				assert.Nil(t, client)
				assert.ErrorIs(t, err, email.ErrInvalidConfig)
			}
		})
	}
}

func TestClient_EmailValidation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		email string
		valid bool
	}{
		{"valid simple", "user@example.com", true},
		{"valid with subdomain", "user@mail.example.com", true},
		{"valid with plus", "user+tag@example.com", true},
		{"valid with dots", "first.last@example.com", true},
		{"valid with numbers", "user123@example.com", true},
		{"invalid no @", "userexample.com", false},
		{"invalid no domain", "user@", false},
		{"invalid no local", "@example.com", false},
		{"invalid double @", "user@@example.com", false},
		{"invalid spaces", "user @example.com", false},
		{"empty", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			config := smtp.Config{
				Host:         "smtp.example.com",
				Port:         587,
				Username:     "user@example.com",
				Password:     "password",
				TLSMode:      "starttls",
				SenderEmail:  tt.email,
				SupportEmail: "support@example.com",
			}

			if !tt.valid && tt.email != "" {
				config.SenderEmail = "valid@example.com"
				config.SupportEmail = tt.email
			}

			_, err := smtp.New(config)
			if tt.valid || tt.email == "" {
				if tt.email == "" {
					assert.Error(t, err)
				} else {
					// Valid email in SenderEmail position
					config.SupportEmail = "support@example.com"
					config.SenderEmail = tt.email
					_, err = smtp.New(config)
					assert.NoError(t, err)
				}
			} else {
				assert.Error(t, err)
				assert.ErrorIs(t, err, email.ErrInvalidConfig)
			}
		})
	}
}
