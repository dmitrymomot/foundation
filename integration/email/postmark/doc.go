// Package postmark provides Postmark email service integration implementing the core email.EmailSender interface.
//
// This package sends transactional emails through Postmark's API with automatic tracking
// configuration and proper error handling.
//
// Basic usage:
//
//	import (
//		"context"
//		"log"
//
//		"github.com/dmitrymomot/foundation/core/email"
//		"github.com/dmitrymomot/foundation/integration/email/postmark"
//	)
//
//	func main() {
//		cfg := postmark.Config{
//			PostmarkServerToken:  "your-server-token",
//			PostmarkAccountToken: "your-account-token",
//			SenderEmail:          "noreply@example.com",
//			SupportEmail:         "support@example.com",
//		}
//
//		emailSender, err := postmark.New(cfg)
//		if err != nil {
//			log.Fatal("Failed to create email client:", err)
//		}
//
//		params := email.SendEmailParams{
//			SendTo:   "user@example.com",
//			Subject:  "Welcome to Our Service!",
//			BodyHTML: "<h1>Welcome!</h1><p>Thank you for joining.</p>",
//			Tag:      "welcome",
//		}
//
//		if err := emailSender.SendEmail(context.Background(), params); err != nil {
//			log.Printf("Failed to send email: %v", err)
//		}
//	}
//
// # Configuration
//
// The Config struct requires all four fields for successful client creation:
//
//	type Config struct {
//		PostmarkServerToken  string `env:"POSTMARK_SERVER_TOKEN"`
//		PostmarkAccountToken string `env:"POSTMARK_ACCOUNT_TOKEN"`
//		SenderEmail          string `env:"SENDER_EMAIL,required"`
//		SupportEmail         string `env:"SUPPORT_EMAIL,required"`
//	}
//
// All fields are validated during client creation. Email addresses must match a basic regex pattern.
//
// # Dependency Injection
//
// Use MustNewClient for initialization that panics on configuration errors:
//
//	func NewServices(cfg postmark.Config) *Services {
//		return &Services{
//			EmailSender: postmark.MustNewClient(cfg),
//		}
//	}
//
// # Email Features
//
// All emails sent through this package include:
//
//   - TrackOpens: true (tracks email opens)
//   - TrackLinks: "HtmlOnly" (tracks clicks in HTML content only)
//   - From: Uses configured SenderEmail
//   - ReplyTo: Uses configured SupportEmail
//
// # Error Handling
//
// The package wraps Postmark API errors with core email error types:
//
//	if err := emailSender.SendEmail(ctx, params); err != nil {
//		if errors.Is(err, email.ErrInvalidConfig) {
//			// Configuration validation failed
//		} else if errors.Is(err, email.ErrFailedToSendEmail) {
//			// Email sending failed (API error or network issue)
//		}
//	}
package postmark
