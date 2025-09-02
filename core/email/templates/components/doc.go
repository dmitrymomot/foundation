// Package components provides reusable email template components for building HTML emails.
//
// This package contains pre-built templ components that follow email HTML best practices
// for maximum compatibility across email clients. Components are designed to be composed
// into complete email templates using the Layout component as a base structure.
//
// # Basic Usage
//
// Import the components package and use components within a Layout:
//
// Create your own email templates in .templ files:
//
//	package emails
//
//	import "github.com/dmitrymomot/foundation/core/email/templates/components"
//
//	templ HelloEmail() {
//		@components.Layout() {
//			@components.Header("Hello World", "Welcome message")
//			@components.Text() {
//				This is a paragraph of text in the email body.
//			}
//			@components.ButtonGroup() {
//				@components.PrimaryButton("Click Me", "https://example.com")
//			}
//		}
//	}
//
// # Available Components
//
// Layout Components:
//   - Layout() - Base HTML structure with responsive styles and email-safe CSS
//
// Content Components:
//   - Header(title, subtitle) - Main heading with optional subtitle
//   - Text() - Standard paragraph text with proper email styling
//   - TextWarning() - Warning text with yellow background styling
//   - TextSecondary() - Secondary text with muted color
//
// Interactive Components:
//   - ButtonGroup() - Container for button components
//   - PrimaryButton(text, url) - Blue primary action button
//   - SuccessButton(text, url) - Green success button
//   - DangerButton(text, url) - Red danger/alert button
//
// Utility Components:
//   - OTP(otp) - Styled one-time password display
//   - Logo(logoURL, alt) - Logo component with image URL and alt text
//   - Footer() - Email footer component (accepts children content)
//   - FooterLink(text, url) - Footer link with proper styling and separator
//
// # Button Usage
//
// Buttons should be wrapped in a ButtonGroup component:
//
// In a .templ file:
//
//	templ EmailWithButtons(confirmURL, cancelURL string) {
//		@components.Layout() {
//			@components.ButtonGroup() {
//				@components.PrimaryButton("Confirm Email", confirmURL)
//				@components.DangerButton("Cancel", cancelURL)
//			}
//		}
//	}
//
// # OTP Display
//
// For authentication emails with one-time passwords:
//
// In a .templ file:
//
//	templ OTPEmail(otp string) {
//		@components.Layout() {
//			@components.Header("Verification Code", "")
//			@components.Text() {
//				Please enter the following code to verify your account:
//			}
//			@components.OTP(otp)
//			@components.TextSecondary() {
//				This code expires in 10 minutes.
//			}
//		}
//	}
//
// # Complete Example
//
// Here's a comprehensive welcome email template that demonstrates maximum component usage:
//
// Create a welcome.templ file:
//
//	package emails
//
//	import "github.com/dmitrymomot/foundation/core/email/templates/components"
//
//	templ WelcomeEmail(userName, verificationCode, dashboardURL, supportURL string) {
//		@components.Layout() {
//			@components.Logo("https://example.com/logo.png", "Company Logo")
//			@components.Header("Welcome to Our Platform!", "You're all set to get started")
//
//			@components.Text() {
//				Hi { userName },
//			}
//			@components.Text() {
//				Welcome to our platform! We're excited to have you on board. Your account has been
//				successfully created and you're just one step away from accessing all features.
//			}
//
//			@components.TextWarning() {
//				Important: Please verify your email address to activate your account.
//			}
//
//			@components.Text() {
//				Use the verification code below to complete your account setup:
//			}
//			@components.OTP(verificationCode)
//			@components.TextSecondary() {
//				This verification code will expire in 10 minutes for security reasons.
//			}
//
//			@components.Text() {
//				Once verified, you can access your dashboard and start exploring:
//			}
//
//			@components.ButtonGroup() {
//				@components.PrimaryButton("Go to Dashboard", dashboardURL)
//				@components.SuccessButton("Get Started Guide", dashboardURL + "/guide")
//			}
//
//			@components.Text() {
//				If you didn't create this account or have any questions, please contact our support team.
//			}
//
//			@components.ButtonGroup() {
//				@components.DangerButton("Report Issue", supportURL)
//			}
//
//			@components.TextSecondary() {
//				Thanks for choosing our platform. We're here to help you succeed!
//			}
//
//			@components.Footer() {
//				Copyright © 2024 Company Name. All rights reserved.
//				@components.FooterLink("Privacy Policy", "https://example.com/privacy")
//				@components.FooterLink("Terms of Service", "https://example.com/terms")
//			}
//		}
//	}
//
// This example demonstrates:
//   - Complete email structure with Layout wrapper
//   - Logo placement at the top
//   - Header with title and subtitle for clear messaging
//   - Multiple Text components with different semantic purposes
//   - TextWarning for important notices
//   - TextSecondary for less prominent information
//   - OTP component for verification codes
//   - Multiple ButtonGroup sections with different button types
//   - Footer component for consistent email closure
//   - Proper content flow for a welcome email user journey
//
// # Email Client Compatibility
//
// All components use:
//   - Table-based layouts for older email clients
//   - Inline CSS styles for maximum compatibility
//   - Email-safe fonts (Arial, sans-serif)
//   - Proper spacing and padding for consistent rendering
//   - Responsive design with mobile-friendly breakpoints
//
// # Styling Guidelines
//
// Components follow these email HTML best practices:
//   - All styles are inline to avoid CSS stripping
//   - Colors use full hex codes for maximum compatibility
//   - Font sizes and line heights are explicitly set
//   - Background colors are set on table cells, not divs
//   - Links use proper color inheritance and text decoration
package components
