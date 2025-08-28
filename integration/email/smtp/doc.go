// Package smtp provides an SMTP-based implementation of the email.EmailSender interface.
//
// This package enables sending HTML emails through any SMTP server with support for
// STARTTLS, direct TLS, and plain connections. It validates configuration and email
// parameters to prevent runtime failures.
//
// Basic usage:
//
//	import (
//		"context"
//		"github.com/dmitrymomot/foundation/core/email"
//		"github.com/dmitrymomot/foundation/integration/email/smtp"
//	)
//
//	cfg := smtp.Config{
//		Host:         "smtp.gmail.com",
//		Port:         587,
//		Username:     "your-email@gmail.com",
//		Password:     "your-app-password",
//		TLSMode:      "starttls",
//		SenderEmail:  "noreply@example.com",
//		SupportEmail: "support@example.com",
//	}
//
//	sender, err := smtp.New(cfg)
//	if err != nil {
//		// Handle configuration error
//	}
//
//	params := email.SendEmailParams{
//		SendTo:   "user@example.com",
//		Subject:  "Welcome!",
//		BodyHTML: "<h1>Welcome to our service</h1>",
//		Tag:      "welcome",
//	}
//
//	err = sender.SendEmail(context.Background(), params)
//	if err != nil {
//		// Handle sending error
//	}
//
// # Configuration
//
// The Config struct defines all required SMTP settings:
//
//   - Host: SMTP server hostname (required)
//   - Port: SMTP server port, typically 587 for STARTTLS or 465 for TLS (required)
//   - Username: SMTP authentication username (required)
//   - Password: SMTP authentication password (required)
//   - TLSMode: Connection security mode - "starttls", "tls", or "plain" (required)
//   - SenderEmail: From address for outgoing emails (required, validated)
//   - SupportEmail: Reply-To address for email responses (required, validated)
//
// All fields are validated during client creation to catch configuration errors early.
//
// # TLS Modes
//
// Three TLS modes are supported:
//
//   - "starttls": Start with plain connection, upgrade to TLS (recommended, port 587)
//   - "tls": Direct TLS connection (port 465)
//   - "plain": No encryption (development only, port 25)
//
// # Dependency Injection
//
// Use MustNewClient for dependency injection patterns:
//
//	type Services struct {
//		EmailSender email.EmailSender
//	}
//
//	func NewServices(cfg smtp.Config) *Services {
//		return &Services{
//			EmailSender: smtp.MustNewClient(cfg), // Panics on invalid config
//		}
//	}
package smtp
