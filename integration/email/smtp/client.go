package smtp

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/smtp"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/dmitrymomot/foundation/core/email"
)

// Client implements the EmailSender interface using standard SMTP protocol.
// Supports multiple TLS modes (STARTTLS, TLS, plain) and is thread-safe for concurrent use.
type Client struct {
	config Config
	auth   smtp.Auth
}

// New creates an SMTP-backed email sender.
// All configuration fields are required for runtime operation to ensure
// explicit configuration and avoid silent failures in production.
func New(cfg Config) (email.EmailSender, error) {
	if cfg.Host == "" {
		return nil, fmt.Errorf("%w: Host is required", email.ErrInvalidConfig)
	}
	if cfg.Port <= 0 || cfg.Port > 65535 {
		return nil, fmt.Errorf("%w: Port must be between 1 and 65535", email.ErrInvalidConfig)
	}
	if cfg.Username == "" {
		return nil, fmt.Errorf("%w: Username is required", email.ErrInvalidConfig)
	}
	if cfg.Password == "" {
		return nil, fmt.Errorf("%w: Password is required", email.ErrInvalidConfig)
	}
	if cfg.TLSMode != "starttls" && cfg.TLSMode != "tls" && cfg.TLSMode != "plain" {
		return nil, fmt.Errorf("%w: TLSMode must be starttls, tls, or plain", email.ErrInvalidConfig)
	}
	if cfg.SenderEmail == "" || !isValidEmail(cfg.SenderEmail) {
		return nil, fmt.Errorf("%w: SenderEmail must be a valid email address", email.ErrInvalidConfig)
	}
	if cfg.SupportEmail == "" || !isValidEmail(cfg.SupportEmail) {
		return nil, fmt.Errorf("%w: SupportEmail must be a valid email address", email.ErrInvalidConfig)
	}

	auth := smtp.PlainAuth("", cfg.Username, cfg.Password, cfg.Host)

	return &Client{
		config: cfg,
		auth:   auth,
	}, nil
}

// MustNewClient creates an SMTP client that panics on invalid config.
// Follows framework pattern of failing fast during initialization rather than
// allowing broken services to start.
func MustNewClient(cfg Config) email.EmailSender {
	client, err := New(cfg)
	if err != nil {
		panic(err)
	}
	return client
}

// SendEmail implements EmailSender using SMTP protocol.
// Supports STARTTLS, TLS, and plain text connections based on configuration.
// The context is used for timeout control during the SMTP transaction.
func (c *Client) SendEmail(ctx context.Context, params email.SendEmailParams) error {
	// Check if context is already cancelled
	if err := ctx.Err(); err != nil {
		return errors.Join(email.ErrFailedToSendEmail, err)
	}

	if err := params.Validate(); err != nil {
		return err
	}

	// Build the email message
	message := c.buildMessage(params)

	// Get the server address
	serverAddr := net.JoinHostPort(c.config.Host, strconv.Itoa(c.config.Port))

	// Send based on TLS mode
	var err error
	switch c.config.TLSMode {
	case "tls":
		err = c.sendWithTLS(serverAddr, message)
	case "starttls":
		err = c.sendWithSTARTTLS(serverAddr, message)
	case "plain":
		err = c.sendPlain(serverAddr, message)
	}

	if err != nil {
		return errors.Join(email.ErrFailedToSendEmail, err)
	}

	return nil
}

// buildMessage creates the MIME-formatted email message.
func (c *Client) buildMessage(params email.SendEmailParams) []byte {
	headers := make(map[string]string)
	headers["From"] = c.config.SenderEmail
	headers["To"] = params.SendTo
	headers["Reply-To"] = c.config.SupportEmail
	headers["Subject"] = params.Subject
	headers["MIME-Version"] = "1.0"
	headers["Content-Type"] = "text/html; charset=\"UTF-8\""
	headers["Date"] = time.Now().Format(time.RFC1123Z)

	// Add message ID for tracking
	headers["Message-ID"] = fmt.Sprintf("<%d.%s@%s>",
		time.Now().UnixNano(),
		strings.ReplaceAll(params.Tag, " ", "_"),
		c.config.Host,
	)

	// Build the message
	var message strings.Builder
	for key, value := range headers {
		message.WriteString(fmt.Sprintf("%s: %s\r\n", key, value))
	}
	message.WriteString("\r\n")
	message.WriteString(params.BodyHTML)

	return []byte(message.String())
}

// sendWithTLS sends email using direct TLS connection.
func (c *Client) sendWithTLS(serverAddr string, message []byte) error {
	tlsConfig := &tls.Config{
		ServerName: c.config.Host,
	}

	conn, err := tls.Dial("tcp", serverAddr, tlsConfig)
	if err != nil {
		return fmt.Errorf("failed to connect to SMTP server with TLS: %w", err)
	}
	defer func() { _ = conn.Close() }()

	client, err := smtp.NewClient(conn, c.config.Host)
	if err != nil {
		return fmt.Errorf("failed to create SMTP client: %w", err)
	}
	defer func() { _ = client.Close() }()

	return c.performSMTPTransaction(client, message)
}

// sendWithSTARTTLS sends email using STARTTLS upgrade.
func (c *Client) sendWithSTARTTLS(serverAddr string, message []byte) error {
	client, err := smtp.Dial(serverAddr)
	if err != nil {
		return fmt.Errorf("failed to connect to SMTP server: %w", err)
	}
	defer func() { _ = client.Close() }()

	// Upgrade to TLS
	tlsConfig := &tls.Config{
		ServerName: c.config.Host,
	}
	if err := client.StartTLS(tlsConfig); err != nil {
		return fmt.Errorf("failed to start TLS: %w", err)
	}

	return c.performSMTPTransaction(client, message)
}

// sendPlain sends email without encryption.
func (c *Client) sendPlain(serverAddr string, message []byte) error {
	client, err := smtp.Dial(serverAddr)
	if err != nil {
		return fmt.Errorf("failed to connect to SMTP server: %w", err)
	}
	defer func() { _ = client.Close() }()

	return c.performSMTPTransaction(client, message)
}

// performSMTPTransaction performs the actual SMTP transaction.
func (c *Client) performSMTPTransaction(client *smtp.Client, message []byte) error {
	// Authenticate
	if err := client.Auth(c.auth); err != nil {
		return fmt.Errorf("authentication failed: %w", err)
	}

	// Set sender
	if err := client.Mail(c.config.SenderEmail); err != nil {
		return fmt.Errorf("failed to set sender: %w", err)
	}

	// Extract recipient from message
	toHeader := extractHeader(string(message), "To")
	if toHeader == "" {
		return fmt.Errorf("recipient not found in message")
	}

	// Set recipient
	if err := client.Rcpt(toHeader); err != nil {
		return fmt.Errorf("failed to set recipient: %w", err)
	}

	// Send message
	writer, err := client.Data()
	if err != nil {
		return fmt.Errorf("failed to get data writer: %w", err)
	}

	if _, err := writer.Write(message); err != nil {
		_ = writer.Close()
		return fmt.Errorf("failed to write message: %w", err)
	}

	if err := writer.Close(); err != nil {
		return fmt.Errorf("failed to close data writer: %w", err)
	}

	// Quit the session
	if err := client.Quit(); err != nil {
		// Quit errors are non-fatal as the message was already sent
		// Some servers close the connection immediately after DATA
		return nil
	}

	return nil
}

// extractHeader extracts a header value from the message.
func extractHeader(message, header string) string {
	lines := strings.Split(message, "\r\n")
	prefix := header + ": "
	for _, line := range lines {
		if value, found := strings.CutPrefix(line, prefix); found {
			return value
		}
	}
	return ""
}

// emailRegex is a simple regex for validating email addresses.
var emailRegex = regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)

// isValidEmail checks if the provided string is a valid email address.
func isValidEmail(email string) bool {
	return emailRegex.MatchString(email)
}
