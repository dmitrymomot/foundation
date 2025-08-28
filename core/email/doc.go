// Package email provides a simple, flexible email sending interface with built-in
// development mode and template rendering support.
//
// The package centers around the EmailSender interface, which enables easy testing
// and provider switching. It includes parameter validation, development mode that
// saves emails to disk, and integration with templ components for HTML rendering.
//
// Basic usage:
//
//	import "github.com/dmitrymomot/foundation/core/email"
//
//	// For development (saves emails to disk)
//	sender := email.NewDevSender("./dev_emails")
//
//	params := email.SendEmailParams{
//		SendTo:   "user@example.com",
//		Subject:  "Welcome!",
//		BodyHTML: "<h1>Welcome to our service</h1>",
//		Tag:      "welcome", // optional, for analytics
//	}
//
//	err := sender.SendEmail(context.Background(), params)
//	if err != nil {
//		// Handle error (validation, file system, etc.)
//	}
//
// # Template Integration
//
// Use the templates subpackage to render templ components to HTML:
//
//	import (
//		"github.com/dmitrymomot/foundation/core/email"
//		"github.com/dmitrymomot/foundation/core/email/templates"
//		"github.com/dmitrymomot/foundation/core/email/templates/components"
//	)
//
//	// Create a template using available components
//	func createWelcomeEmail(userName, activationURL string) (string, error) {
//		template := components.Layout(
//			components.Header("Welcome!", "Thanks for joining us"),
//			components.Text(),  // Add text content between tags
//			components.PrimaryButton("Activate Account", activationURL),
//			components.Footer(),
//		)
//
//		return templates.Render(context.Background(), template)
//	}
//
//	// Send the templated email
//	html, err := createWelcomeEmail("John", "https://example.com/activate")
//	if err != nil {
//		return err
//	}
//
//	params := email.SendEmailParams{
//		SendTo:   "john@example.com",
//		Subject:  "Welcome - Please activate your account",
//		BodyHTML: html,
//		Tag:      "welcome",
//	}
//
//	return sender.SendEmail(context.Background(), params)
//
// # Available Template Components
//
// The components subpackage provides these templ components:
//
//   - Layout() - Main HTML structure for emails
//   - Header(title, subtitle) - Email header section
//   - Footer() - Email footer section
//   - Text(), TextWarning(), TextSecondary() - Text content areas
//   - PrimaryButton(text, url), SuccessButton(text, url), DangerButton(text, url) - Action buttons
//   - ButtonGroup() - Container for multiple buttons
//   - Link(text, url) - Text links
//   - Logo(logoURL, alt) - Logo images
//   - OTP(code) - Styled one-time password display
//   - FooterLink(text, url) - Links for footer area
//
// # Development Mode
//
// DevSender saves emails as HTML and JSON files instead of sending them:
//
//	sender := email.NewDevSender("./dev_emails")
//
//	// This creates two files:
//	// ./dev_emails/2024_01_15_143052_welcome.html (HTML content)
//	// ./dev_emails/2024_01_15_143052_welcome.json (metadata)
//
// Files are timestamped and use the Tag field (or sanitized Subject) for naming.
//
// # Error Handling
//
// The package provides specific error types:
//
//	err := sender.SendEmail(ctx, params)
//	if err != nil {
//		switch {
//		case errors.Is(err, email.ErrInvalidParams):
//			// Parameter validation failed (empty fields, invalid email, etc.)
//		case errors.Is(err, email.ErrFailedToSendEmail):
//			// Sending failed (filesystem issues for DevSender, network for real senders)
//		case errors.Is(err, email.ErrInvalidConfig):
//			// Configuration problem (invalid directories, missing credentials, etc.)
//		}
//	}
//
// # Custom Implementations
//
// Implement EmailSender interface for production email services:
//
//	type ProductionSender struct {
//		// your email service client
//	}
//
//	func (p *ProductionSender) SendEmail(ctx context.Context, params email.SendEmailParams) error {
//		if err := params.Validate(); err != nil {
//			return err
//		}
//		// implement sending logic
//		return nil
//	}
//
// # Testing
//
// Use mock implementations for testing:
//
//	type MockEmailSender struct {
//		SentEmails []email.SendEmailParams
//	}
//
//	func (m *MockEmailSender) SendEmail(ctx context.Context, params email.SendEmailParams) error {
//		m.SentEmails = append(m.SentEmails, params)
//		return nil
//	}
package email
