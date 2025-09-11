package vectorizer

import "errors"

var (
	// ErrInvalidDimensions indicates invalid dimensions for the model.
	ErrInvalidDimensions = errors.New("invalid dimensions for model")

	// ErrModelNotSupported indicates the model is not supported.
	ErrModelNotSupported = errors.New("model not supported")

	// ErrRateLimitExceeded indicates the API rate limit was exceeded.
	ErrRateLimitExceeded = errors.New("rate limit exceeded")

	// ErrTextTooLong indicates the input text exceeds the token limit.
	ErrTextTooLong = errors.New("input text exceeds token limit")

	// ErrBatchTooLarge indicates the batch size exceeds the limit.
	ErrBatchTooLarge = errors.New("batch size exceeds limit")

	// ErrInvalidAPIKey indicates an invalid or missing API key.
	ErrInvalidAPIKey = errors.New("invalid or missing API key")
)
