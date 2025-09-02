// Package templates provides email template rendering using the templ templating engine.
//
// This package includes utilities for rendering templ components to HTML strings
// suitable for email bodies. The templates are designed to be responsive and work
// across email clients with proper HTML email best practices.
//
// # Basic Usage
//
// Use the Render function to convert any templ.Component to an HTML string:
//
// First, define your email template in a .templ file:
//
//	package myapp
//
//	import "github.com/dmitrymomot/foundation/core/email/templates/components"
//
//	templ WelcomeEmail(name string) {
//		@components.Layout() {
//			@components.Header("Welcome!", "Thank you for joining us")
//			@components.Text() {
//				Hello { name },
//				Welcome to our platform! We're excited to have you on board.
//			}
//			@components.ButtonGroup() {
//				@components.PrimaryButton("Get Started", "https://example.com/start")
//			}
//		}
//	}
//
// Then use it from your Go code:
//
//	import (
//		"context"
//		"github.com/dmitrymomot/foundation/core/email/templates"
//		// Import your package with the templ components
//		"myapp"
//	)
//
//	func sendEmail(ctx context.Context) error {
//		// Create an email template by calling your templ component
//		template := myapp.WelcomeEmail("John Doe")
//
//		// Render to HTML string
//		htmlBody, err := templates.Render(ctx, template)
//		if err != nil {
//			return err
//		}
//
//		// Use htmlBody in your email sender
//		return sendEmailWithHTML(htmlBody)
//	}
//
// # Template Composition
//
// Templates are designed to be composed from reusable components. The Layout
// component provides the base HTML structure, while other components handle
// specific content sections:
//
// Define a templ component that composes other components.
// In a .templ file:
//
//	package myapp
//
//	import "github.com/dmitrymomot/foundation/core/email/templates/components"
//
//	templ AlertEmail(verifyURL string) {
//		@components.Layout() {
//			@components.Header("Account Alert", "")
//			@components.TextWarning() {
//				Your account requires immediate attention.
//			}
//			@components.Text() {
//				Please click the button below to verify your account.
//			}
//			@components.ButtonGroup() {
//				@components.DangerButton("Verify Now", verifyURL)
//			}
//			@components.TextSecondary() {
//				This link expires in 24 hours.
//			}
//		}
//	}
//
// Then call it from Go:
//
//	template := myapp.AlertEmail("https://example.com/verify")
//	htmlBody, _ := templates.Render(ctx, template)
//
// # Performance
//
// The Render function uses strings.Builder for zero-allocation string construction
// during template rendering, which is critical for email throughput performance
// in high-volume applications.
package templates
