// Package postmark provides production-ready Postmark email service integration for transactional emails in SaaS applications.
//
// This package implements the core email.EmailSender interface using Postmark's reliable transactional email API.
// It provides automatic error classification, email tracking configuration, and validation to ensure consistent
// email delivery in production environments.
//
// # Key Features
//
// The package provides Postmark-backed email sending with production-ready defaults:
//
//   - New: Creates a Postmark email client with configuration validation
//   - MustNewClient: Creates a client that panics on invalid configuration (for dependency injection)
//   - SendEmail: Implements the EmailSender interface with tracking and error handling
//
// All email sending includes automatic tracking configuration (opens and HTML link clicks) and proper
// reply-to headers to ensure customer responses reach the support team.
//
// # Configuration
//
// All configuration is handled through the Config struct with environment variable mapping:
//
//	type Config struct {
//		PostmarkServerToken  string `env:"POSTMARK_SERVER_TOKEN"`
//		PostmarkAccountToken string `env:"POSTMARK_ACCOUNT_TOKEN"`
//		SenderEmail          string `env:"SENDER_EMAIL,required"`
//		SupportEmail         string `env:"SUPPORT_EMAIL,required"`
//	}
//
// The Postmark tokens are optional to support development environments where email sending is disabled,
// but SenderEmail and SupportEmail are required as they establish sender identity and reply-to behavior.
//
// # Usage Example
//
//	package main
//
//	import (
//		"context"
//		"log"
//
//		"github.com/dmitrymomot/gokit/core/email"
//		"github.com/dmitrymomot/gokit/integration/email/postmark"
//	)
//
//	func main() {
//		// Load configuration from environment variables
//		cfg := postmark.Config{
//			PostmarkServerToken:  "your-server-token",
//			PostmarkAccountToken: "your-account-token",
//			SenderEmail:          "noreply@example.com",
//			SupportEmail:         "support@example.com",
//		}
//
//		// Create Postmark email client
//		emailSender, err := postmark.New(cfg)
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
// # Dependency Injection Pattern
//
// For applications using dependency injection, use MustNewClient to fail fast during initialization:
//
//	type Services struct {
//		EmailSender email.EmailSender
//	}
//
//	func NewServices(cfg postmark.Config) *Services {
//		return &Services{
//			EmailSender: postmark.MustNewClient(cfg),
//		}
//	}
//
// This pattern ensures that configuration errors are caught during application startup rather than
// at runtime when attempting to send emails.
//
// # Email Tracking and Analytics
//
// The package automatically configures email tracking for analytics and deliverability monitoring:
//
//   - TrackOpens: Enabled for all emails to track open rates
//   - TrackLinks: Set to "HtmlOnly" to track clicks only in HTML content (privacy-conscious)
//   - Reply-To: Automatically set to the configured support email address
//
// This tracking configuration balances useful analytics with privacy considerations by avoiding
// tracking in plain text emails.
//
// # Error Handling
//
// The package provides structured error handling that integrates with the core email error types:
//
//	if err := emailSender.SendEmail(ctx, params); err != nil {
//		if errors.Is(err, email.ErrInvalidConfig) {
//			log.Error("Email service configuration error")
//		} else if errors.Is(err, email.ErrFailedToSendEmail) {
//			log.Error("Email delivery failed", "error", err)
//			// Implement retry logic or alternative notification
//		}
//	}
//
// Postmark API errors are wrapped with context and error codes for debugging while maintaining
// consistent error types for application-level error handling.
//
// # Email Content Best Practices
//
// When sending emails through Postmark, follow these best practices:
//
//	// Use descriptive tags for tracking and filtering
//	params := email.SendEmailParams{
//		SendTo:   recipient,
//		Subject:  "Password Reset Request",
//		BodyHTML: htmlContent,
//		Tag:      "password-reset", // Clear, descriptive tag
//	}
//
//	// Include both plain text and HTML for better deliverability
//	// (Note: Current implementation focuses on HTML, consider extending for plain text)
//
//	// Use consistent sender identity
//	// The configured SenderEmail should match your domain's SPF/DKIM records
//
// # Configuration Validation
//
// The package performs comprehensive configuration validation during client creation:
//
//   - PostmarkServerToken and PostmarkAccountToken are required for runtime operation
//   - SenderEmail must be a valid email address format
//   - SupportEmail must be a valid email address format
//   - Email addresses are validated using a built-in regex pattern
//
// This validation prevents runtime failures and ensures proper email delivery configuration.
//
// # Development Environment Support
//
// The configuration supports development environments where Postmark tokens might not be available:
//
//	// Development configuration (will fail validation in New())
//	cfg := postmark.Config{
//		PostmarkServerToken:  "", // Empty in development
//		PostmarkAccountToken: "", // Empty in development
//		SenderEmail:          "dev@localhost",
//		SupportEmail:         "dev@localhost",
//	}
//
// Consider implementing a development email sender that logs emails instead of sending them,
// or use Postmark's sandbox environment for development testing.
//
// # Production Considerations
//
// For production deployments with Postmark:
//
//   - Use separate Postmark servers for different environments (staging, production)
//   - Configure proper SPF, DKIM, and DMARC records for your sending domain
//   - Monitor Postmark delivery statistics and bounce/complaint rates
//   - Implement proper email template management for consistent branding
//   - Use Postmark's suppression list management for bounce and unsubscribe handling
//   - Set up webhook endpoints to handle delivery notifications and bounce processing
//
// The package provides the foundation for reliable email delivery, but consider these additional
// infrastructure components for a complete production email system.
package postmark
