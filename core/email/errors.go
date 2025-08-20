package email

import "errors"

// Error variables define email operation failures that can be wrapped with
// detailed context using errors.Join() for comprehensive error reporting.
var (
	ErrFailedToSendEmail = errors.New("failed to send email")
	ErrInvalidConfig     = errors.New("invalid email configuration")
	ErrInvalidParams     = errors.New("invalid email parameters")
)
