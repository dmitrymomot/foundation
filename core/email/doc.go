// Package email provides email sending functionality with support for different
// providers and development mode. It includes validation, template rendering capabilities,
// and a flexible interface design that enables easy testing and provider switching.
//
// # Features
//
//   - Abstract EmailSender interface for provider flexibility
//   - Built-in development mode that saves emails to disk
//   - Comprehensive parameter validation with detailed error messages
//   - Template rendering support using templ components
//   - Safe filename generation for development mode
//   - Context-aware operations for timeout and cancellation handling
//   - JSON metadata export for email debugging and analytics
//
// # Usage
//
// The package centers around the EmailSender interface, which can be implemented
// by different email providers:
//
//	import "github.com/dmitrymomot/gokit/core/email"
//
//	// For development
//	sender := email.NewDevSender("./dev_emails")
//
//	// Send email
//	params := email.SendEmailParams{
//		SendTo:   "user@example.com",
//		Subject:  "Welcome to our service",
//		BodyHTML: "<h1>Welcome!</h1><p>Thanks for joining us.</p>",
//		Tag:      "welcome_email",
//	}
//
//	err := sender.SendEmail(context.Background(), params)
//	if err != nil {
//		log.Error("Failed to send email:", err)
//	}
//
// # EmailSender Interface
//
// The EmailSender interface provides a clean abstraction for different email providers:
//
//	type EmailSender interface {
//		SendEmail(ctx context.Context, params SendEmailParams) error
//	}
//
//	// Example implementation for a third-party provider
//	type SMTPSender struct {
//		host     string
//		port     int
//		username string
//		password string
//	}
//
//	func (s *SMTPSender) SendEmail(ctx context.Context, params email.SendEmailParams) error {
//		if err := params.Validate(); err != nil {
//			return err
//		}
//
//		// Implement SMTP sending logic
//		return nil
//	}
//
// # Development Mode
//
// The DevSender implementation saves emails as HTML and JSON files for local development:
//
//	// Create development sender
//	devSender := email.NewDevSender("./dev_emails")
//
//	// Send email (saves to disk instead of sending)
//	params := email.SendEmailParams{
//		SendTo:   "user@example.com",
//		Subject:  "Account Verification",
//		BodyHTML: "<h1>Please verify your account</h1>",
//		Tag:      "verification",
//	}
//
//	err := devSender.SendEmail(context.Background(), params)
//	if err != nil {
//		log.Error("Failed to save development email:", err)
//	}
//
//	// Files created:
//	// ./dev_emails/2024_01_15_143052_verification.html
//	// ./dev_emails/2024_01_15_143052_verification.json
//
// # Email Parameters
//
// The SendEmailParams struct defines the email content and metadata:
//
//	type SendEmailParams struct {
//		SendTo   string // Recipient email address (required)
//		Subject  string // Email subject line (required)
//		BodyHTML string // HTML email body (required)
//		Tag      string // Optional tag for analytics and tracking
//	}
//
//	// Example with all fields
//	params := email.SendEmailParams{
//		SendTo:   "customer@example.com",
//		Subject:  "Order Confirmation #12345",
//		BodyHTML: generateOrderConfirmationHTML(order),
//		Tag:      "order_confirmation",
//	}
//
// # Template Integration
//
// Use the templates subpackage for rendering templ components to HTML:
//
//	import (
//		"github.com/dmitrymomot/gokit/core/email"
//		"github.com/dmitrymomot/gokit/core/email/templates"
//	)
//
//	// Define your email template
//	func WelcomeEmail(userName, activationLink string) templ.Component {
//		return templates.Layout(
//			templates.Header("Welcome!", "Thank you for joining us"),
//			templates.Text(fmt.Sprintf("Hi %s,", userName)),
//			templates.Text("Welcome to our platform. Click the button below to activate your account:"),
//			templates.PrimaryButton("Activate Account", activationLink),
//			templates.Footer(),
//		)
//	}
//
//	// Render and send
//	func sendWelcomeEmail(sender email.EmailSender, userEmail, userName, activationLink string) error {
//		template := WelcomeEmail(userName, activationLink)
//		html, err := templates.Render(context.Background(), template)
//		if err != nil {
//			return fmt.Errorf("failed to render template: %w", err)
//		}
//
//		params := email.SendEmailParams{
//			SendTo:   userEmail,
//			Subject:  "Welcome - Please activate your account",
//			BodyHTML: html,
//			Tag:      "welcome_activation",
//		}
//
//		return sender.SendEmail(context.Background(), params)
//	}
//
// # Email Template Components
//
// The package includes pre-built components for common email patterns:
//
//	import "github.com/dmitrymomot/gokit/core/email/templates"
//
//	// Build email with components
//	func buildNotificationEmail(title, message, actionURL string) templ.Component {
//		return templates.Layout(
//			templates.Header(title, ""),
//			templates.Text(message),
//			templates.ButtonGroup(
//				templates.PrimaryButton("Take Action", actionURL),
//				templates.Link("View in Browser", actionURL),
//			),
//			templates.Footer(),
//		)
//	}
//
//	// OTP email
//	func buildOTPEmail(otp string) templ.Component {
//		return templates.Layout(
//			templates.Header("Verification Code", ""),
//			templates.Text("Your verification code is:"),
//			templates.OTP(otp),
//			templates.TextSecondary("This code expires in 10 minutes."),
//			templates.Footer(),
//		)
//	}
//
// # Error Handling
//
// The package provides specific error types for different failure scenarios:
//
//	import "errors"
//
//	err := sender.SendEmail(ctx, params)
//	if err != nil {
//		switch {
//		case errors.Is(err, email.ErrInvalidParams):
//			// Handle parameter validation errors
//			log.Error("Invalid email parameters:", err)
//		case errors.Is(err, email.ErrFailedToSendEmail):
//			// Handle sending failures (network, provider issues, etc.)
//			log.Error("Email delivery failed:", err)
//		case errors.Is(err, email.ErrInvalidConfig):
//			// Handle configuration errors
//			log.Error("Email configuration invalid:", err)
//		default:
//			// Handle unexpected errors
//			log.Error("Unexpected email error:", err)
//		}
//	}
//
// # Production Email Service
//
// Implement production email services using popular providers:
//
//	// Example with SendGrid
//	type SendGridSender struct {
//		apiKey string
//		client *sendgrid.Client
//	}
//
//	func NewSendGridSender(apiKey string) email.EmailSender {
//		return &SendGridSender{
//			apiKey: apiKey,
//			client: sendgrid.NewSendClient(apiKey),
//		}
//	}
//
//	func (s *SendGridSender) SendEmail(ctx context.Context, params email.SendEmailParams) error {
//		if err := params.Validate(); err != nil {
//			return err
//		}
//
//		message := mail.NewV3Mail()
//		message.SetFrom(mail.NewEmail("noreply", "noreply@yourapp.com"))
//		message.AddTo(mail.NewEmail("", params.SendTo))
//		message.Subject = params.Subject
//		message.AddContent(mail.NewContent("text/html", params.BodyHTML))
//
//		if params.Tag != "" {
//			message.SetCustomArg("tag", params.Tag)
//		}
//
//		response, err := s.client.SendWithContext(ctx, message)
//		if err != nil {
//			return fmt.Errorf("%w: SendGrid error: %v", email.ErrFailedToSendEmail, err)
//		}
//
//		if response.StatusCode >= 400 {
//			return fmt.Errorf("%w: SendGrid HTTP %d", email.ErrFailedToSendEmail, response.StatusCode)
//		}
//
//		return nil
//	}
//
// # Email Service Factory
//
// Create a factory function to switch between different email senders based on environment:
//
//	func NewEmailSender(config EmailConfig) email.EmailSender {
//		switch config.Provider {
//		case "dev":
//			return email.NewDevSender(config.DevDirectory)
//		case "sendgrid":
//			return NewSendGridSender(config.SendGridAPIKey)
//		case "smtp":
//			return NewSMTPSender(config.SMTPConfig)
//		default:
//			return email.NewDevSender("./emails")
//		}
//	}
//
//	// Usage in application startup
//	func initializeEmailService() email.EmailSender {
//		config := loadEmailConfig()
//		return NewEmailSender(config)
//	}
//
// # Batch Email Processing
//
// Handle bulk email operations with proper error handling and rate limiting:
//
//	type EmailJob struct {
//		UserEmail string
//		UserName  string
//		Template  string
//		Data      map[string]any
//	}
//
//	func processBatchEmails(sender email.EmailSender, jobs []EmailJob) error {
//		var errors []error
//
//		for _, job := range jobs {
//			html, err := renderTemplate(job.Template, job.Data)
//			if err != nil {
//				errors = append(errors, fmt.Errorf("template render failed for %s: %w", job.UserEmail, err))
//				continue
//			}
//
//			params := email.SendEmailParams{
//				SendTo:   job.UserEmail,
//				Subject:  getSubjectForTemplate(job.Template),
//				BodyHTML: html,
//				Tag:      job.Template,
//			}
//
//			if err := sender.SendEmail(context.Background(), params); err != nil {
//				errors = append(errors, fmt.Errorf("send failed for %s: %w", job.UserEmail, err))
//				continue
//			}
//
//			// Rate limiting
//			time.Sleep(100 * time.Millisecond)
//		}
//
//		if len(errors) > 0 {
//			return fmt.Errorf("batch processing failed with %d errors: %v", len(errors), errors)
//		}
//
//		return nil
//	}
//
// # Testing
//
// The package design facilitates easy testing with mock implementations:
//
//	type MockEmailSender struct {
//		SentEmails []email.SendEmailParams
//		ShouldFail bool
//	}
//
//	func (m *MockEmailSender) SendEmail(ctx context.Context, params email.SendEmailParams) error {
//		if m.ShouldFail {
//			return email.ErrFailedToSendEmail
//		}
//		m.SentEmails = append(m.SentEmails, params)
//		return nil
//	}
//
//	// Use in tests
//	func TestUserRegistration(t *testing.T) {
//		mockSender := &MockEmailSender{}
//		userService := NewUserService(mockSender)
//
//		err := userService.RegisterUser("test@example.com", "John Doe")
//		require.NoError(t, err)
//
//		// Verify email was sent
//		require.Len(t, mockSender.SentEmails, 1)
//		assert.Equal(t, "test@example.com", mockSender.SentEmails[0].SendTo)
//		assert.Contains(t, mockSender.SentEmails[0].Subject, "Welcome")
//	}
//
// # Best Practices
//
//   - Always validate email parameters before sending
//   - Use descriptive tags for analytics and debugging
//   - Implement proper error handling and retry logic for production
//   - Use development mode during local development and testing
//   - Keep email templates in version control and use template rendering
//   - Implement rate limiting for bulk email operations
//   - Log email failures for monitoring and debugging
//   - Use context with timeouts for email operations
//   - Test email templates across different email clients
//   - Monitor email delivery rates and provider-specific metrics
package email
