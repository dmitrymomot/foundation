// Package smtp provides production-ready SMTP email service integration for transactional emails in SaaS applications.
//
// This package implements the core email.EmailSender interface using standard SMTP protocol with support for
// multiple TLS modes (STARTTLS, TLS, plain). It provides proper MIME formatting, authentication, and error
// handling for reliable email delivery through any SMTP-compatible email service.
//
// # Key Features
//
// The package provides SMTP-backed email sending with comprehensive TLS support:
//
//   - New: Creates an SMTP email client with configuration validation
//   - MustNewClient: Creates a client that panics on invalid configuration (for dependency injection)
//   - SendEmail: Implements the EmailSender interface with proper MIME formatting and error handling
//
// The implementation supports three TLS modes: direct TLS connection, STARTTLS upgrade, and plain text
// (for development or internal networks). All emails are formatted as proper MIME messages with HTML content.
//
// # Configuration
//
// All configuration is handled through the Config struct with environment variable mapping:
//
//	type Config struct {
//		Host         string `env:"SMTP_HOST,required"`
//		Port         int    `env:"SMTP_PORT" envDefault:"587"`
//		Username     string `env:"SMTP_USERNAME,required"`
//		Password     string `env:"SMTP_PASSWORD,required"`
//		TLSMode      string `env:"SMTP_TLS_MODE" envDefault:"starttls"` // starttls, tls, or plain
//		SenderEmail  string `env:"SENDER_EMAIL,required"`
//		SupportEmail string `env:"SUPPORT_EMAIL,required"`
//	}
//
// All configuration fields are required for runtime operation to ensure explicit configuration
// and avoid silent failures in production environments.
//
// # Usage Example
//
//	package main
//
//	import (
//		"context"
//		"log"
//
//		"github.com/dmitrymomot/foundation/core/email"
//		"github.com/dmitrymomot/foundation/integration/email/smtp"
//	)
//
//	func main() {
//		// Load configuration from environment variables
//		cfg := smtp.Config{
//			Host:         "smtp.gmail.com",
//			Port:         587,
//			Username:     "your-email@gmail.com",
//			Password:     "your-app-password",
//			TLSMode:      "starttls",
//			SenderEmail:  "noreply@example.com",
//			SupportEmail: "support@example.com",
//		}
//
//		// Create SMTP email client
//		emailSender, err := smtp.New(cfg)
//		if err != nil {
//			log.Fatal("Failed to create email client:", err)
//		}
//
//		// Send a welcome email
//		ctx := context.Background()
//		params := email.SendEmailParams{
//			SendTo:   "user@example.com",
//			Subject:  "Welcome to Our Service!",
//			BodyHTML: "<h1>Welcome!</h1><p>Thank you for joining our service.</p>",
//			Tag:      "welcome-email",
//		}
//
//		if err := emailSender.SendEmail(ctx, params); err != nil {
//			log.Printf("Failed to send email: %v", err)
//		} else {
//			log.Println("Email sent successfully")
//		}
//	}
//
// # TLS Configuration Modes
//
// The package supports three TLS modes for different deployment scenarios:
//
//	// STARTTLS (recommended for most providers like Gmail, Outlook)
//	cfg.TLSMode = "starttls"
//	cfg.Port = 587
//
//	// Direct TLS (for providers that require immediate TLS)
//	cfg.TLSMode = "tls"
//	cfg.Port = 465
//
//	// Plain text (only for development or secure internal networks)
//	cfg.TLSMode = "plain"
//	cfg.Port = 25
//
// STARTTLS is the recommended mode for most email providers as it starts with a plain connection
// and upgrades to TLS, providing better compatibility and security.
//
// # Provider-Specific Configuration
//
// Common SMTP provider configurations:
//
//	// Gmail (requires App Password, not regular password)
//	cfg := smtp.Config{
//		Host:     "smtp.gmail.com",
//		Port:     587,
//		TLSMode:  "starttls",
//		Username: "your-email@gmail.com",
//		Password: "your-16-char-app-password",
//	}
//
//	// Outlook/Hotmail
//	cfg := smtp.Config{
//		Host:     "smtp-mail.outlook.com",
//		Port:     587,
//		TLSMode:  "starttls",
//		Username: "your-email@outlook.com",
//		Password: "your-password",
//	}
//
//	// AWS SES
//	cfg := smtp.Config{
//		Host:     "email-smtp.us-east-1.amazonaws.com",
//		Port:     587,
//		TLSMode:  "starttls",
//		Username: "your-ses-smtp-username",
//		Password: "your-ses-smtp-password",
//	}
//
//	// SendGrid
//	cfg := smtp.Config{
//		Host:     "smtp.sendgrid.net",
//		Port:     587,
//		TLSMode:  "starttls",
//		Username: "apikey",
//		Password: "your-sendgrid-api-key",
//	}
//
// # Dependency Injection Pattern
//
// For applications using dependency injection, use MustNewClient to fail fast during initialization:
//
//	type Services struct {
//		EmailSender email.EmailSender
//	}
//
//	func NewServices(cfg smtp.Config) *Services {
//		return &Services{
//			EmailSender: smtp.MustNewClient(cfg),
//		}
//	}
//
// This pattern ensures that SMTP configuration errors are caught during application startup
// rather than at runtime when attempting to send emails.
//
// # MIME Message Format
//
// The package automatically formats emails as proper MIME messages with:
//
//   - Proper headers (From, To, Reply-To, Subject, Date, Message-ID)
//   - HTML content type with UTF-8 encoding
//   - Message-ID generation for tracking and debugging
//   - Reply-To header set to the configured support email
//
// The Message-ID includes a timestamp and tag for easy identification in email logs.
//
// # Error Handling
//
// The package provides structured error handling that integrates with the core email error types:
//
//	if err := emailSender.SendEmail(ctx, params); err != nil {
//		if errors.Is(err, email.ErrInvalidConfig) {
//			log.Error("SMTP configuration error")
//		} else if errors.Is(err, email.ErrFailedToSendEmail) {
//			log.Error("SMTP delivery failed", "error", err)
//			// Implement retry logic or alternative notification
//		}
//	}
//
// SMTP protocol errors are wrapped with context for debugging while maintaining consistent
// error types for application-level error handling.
//
// # Context and Timeout Handling
//
// The SendEmail method respects context cancellation and timeouts:
//
//	// Set a timeout for email sending
//	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
//	defer cancel()
//
//	if err := emailSender.SendEmail(ctx, params); err != nil {
//		if errors.Is(err, context.DeadlineExceeded) {
//			log.Error("Email sending timed out")
//		}
//	}
//
// This allows applications to control email sending timeouts and handle cancellation appropriately.
//
// # Security Considerations
//
// For production SMTP deployments:
//
//   - Always use TLS modes ("starttls" or "tls") for credential protection
//   - Store SMTP credentials securely using environment variables or secret management
//   - Use application-specific passwords when available (e.g., Gmail App Passwords)
//   - Configure proper SPF, DKIM, and DMARC records for your sending domain
//   - Monitor failed authentication attempts and implement rate limiting if needed
//   - Consider using OAuth2 authentication where supported by the provider
//
// # Performance and Reliability
//
// For high-volume email sending:
//
//   - Implement connection pooling at the application level for better performance
//   - Use provider-specific rate limiting to avoid throttling
//   - Implement exponential backoff retry logic for transient failures
//   - Monitor delivery rates and bounce handling
//   - Consider using dedicated email services (like Postmark) for better deliverability
//
// The current implementation creates a new connection for each email, which is suitable for
// moderate volumes but may need optimization for high-throughput scenarios.
//
// # Development and Testing
//
// For development environments, consider using:
//
//   - Local SMTP servers like MailHog or Mailcatcher for testing
//   - Provider sandbox environments where available
//   - Mock implementations of the EmailSender interface for unit testing
//
// The plain TLS mode can be useful for development with local SMTP servers, but should
// never be used in production environments handling real email traffic.
package smtp
