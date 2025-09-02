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
//   - Logo() - Logo placeholder component
//   - Footer() - Email footer component
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
