package postmark

import (
	"context"
	"errors"
	"fmt"
	"regexp"

	"github.com/dmitrymomot/gokit/core/email"
	"github.com/mrz1836/postmark"
)

type Client struct {
	client *postmark.Client
	config Config
}

// New creates a Postmark-backed email sender.
// Both tokens are required for runtime operation - this enforces
// explicit configuration rather than silent failures in production.
func New(cfg Config) (email.EmailSender, error) {
	if cfg.PostmarkServerToken == "" {
		return nil, fmt.Errorf("%w: PostmarkServerToken is required", email.ErrInvalidConfig)
	}
	if cfg.PostmarkAccountToken == "" {
		return nil, fmt.Errorf("%w: PostmarkAccountToken is required", email.ErrInvalidConfig)
	}
	if cfg.SenderEmail == "" {
		return nil, fmt.Errorf("%w: SenderEmail is required", email.ErrInvalidConfig)
	}
	if cfg.SenderEmail == "" || !isValidEmail(cfg.SenderEmail) {
		return nil, fmt.Errorf("%w: SenderEmail must be a valid email address", email.ErrInvalidConfig)
	}
	if cfg.SupportEmail == "" {
		return nil, fmt.Errorf("%w: SupportEmail is required", email.ErrInvalidConfig)
	}
	if cfg.SupportEmail == "" || !isValidEmail(cfg.SupportEmail) {
		return nil, fmt.Errorf("%w: SupportEmail must be a valid email address", email.ErrInvalidConfig)
	}

	return &Client{
		client: postmark.NewClient(cfg.PostmarkServerToken, cfg.PostmarkAccountToken),
		config: cfg,
	}, nil
}

// MustNewClient creates a Postmark client that panics on invalid config.
// Follows framework pattern of failing fast during initialization rather than
// allowing broken services to start.
func MustNewClient(cfg Config) email.EmailSender {
	client, err := New(cfg)
	if err != nil {
		panic(err)
	}
	return client
}

// SendEmail implements EmailSender using Postmark's transactional API.
// Tracking is enabled by default for analytics - opens and HTML link clicks only
// to avoid privacy issues with plain text. Reply-To is set to support email
// to ensure customer responses reach the right team.
func (c *Client) SendEmail(ctx context.Context, params email.SendEmailParams) error {
	if err := params.Validate(); err != nil {
		return err
	}

	resp, err := c.client.SendEmail(ctx, postmark.Email{
		From:       c.config.SenderEmail,
		ReplyTo:    c.config.SupportEmail,
		To:         params.SendTo,
		Subject:    params.Subject,
		Tag:        params.Tag,
		HTMLBody:   params.BodyHTML,
		TrackOpens: true,
		TrackLinks: "HtmlOnly",
	})
	if err != nil {
		return errors.Join(email.ErrFailedToSendEmail, err)
	}
	if resp.ErrorCode > 0 {
		return errors.Join(
			email.ErrFailedToSendEmail,
			fmt.Errorf("postmark error: %d - %s", resp.ErrorCode, resp.Message),
		)
	}
	return nil
}

// emailRegex is a simple regex for validating email addresses.
var emailRegex = regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)

// isValidEmail checks if the provided string is a valid email address.
func isValidEmail(email string) bool {
	return emailRegex.MatchString(email)
}
