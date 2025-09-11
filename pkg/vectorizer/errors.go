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

	// ErrEmbeddingFailed indicates a failure in creating embeddings.
	ErrEmbeddingFailed = errors.New("failed to create embedding")

	// ErrNoEmbeddingReturned indicates no embedding was returned by the API.
	ErrNoEmbeddingReturned = errors.New("no embedding returned")

	// ErrEmbeddingCountMismatch indicates the number of embeddings returned doesn't match the input.
	ErrEmbeddingCountMismatch = errors.New("embedding count mismatch")

	// ErrEmptyEmbedding indicates an empty embedding was returned.
	ErrEmptyEmbedding = errors.New("empty embedding returned")

	// ErrClientCreationFailed indicates a failure in creating the API client.
	ErrClientCreationFailed = errors.New("failed to create API client")
)
