package s3

import (
	"context"
	"errors"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/smithy-go"

	"github.com/dmitrymomot/foundation/core/storage"
)

// classifyS3Error converts S3 errors to domain-specific errors.
// Provides consistent error handling across all S3 operations with proper
// classification for retry logic and user-facing error messages.
func classifyS3Error(err error, operation string) error {
	if err == nil {
		return nil
	}

	// Context errors have highest priority for proper cancellation handling
	if errors.Is(err, context.DeadlineExceeded) {
		return fmt.Errorf("%w: %s operation", storage.ErrOperationTimeout, operation)
	}
	if errors.Is(err, context.Canceled) {
		return fmt.Errorf("%w: %s operation", storage.ErrOperationCanceled, operation)
	}

	// Specific S3 error types for type-safe error checking
	var nsk *types.NoSuchKey
	if errors.As(err, &nsk) {
		return fmt.Errorf("%w: %s", storage.ErrFileNotFound, err)
	}

	var nsb *types.NoSuchBucket
	if errors.As(err, &nsb) {
		return storage.ErrBucketNotFound
	}

	// Generic API errors with proper retry classification
	var apiErr smithy.APIError
	if errors.As(err, &apiErr) {
		code := apiErr.ErrorCode()
		switch code {
		case "AccessDenied":
			return fmt.Errorf("%w: %s operation", storage.ErrAccessDenied, operation)
		case "RequestTimeout":
			return fmt.Errorf("%w: %s operation", storage.ErrRequestTimeout, operation)
		case "SlowDown", "ServiceUnavailable":
			return fmt.Errorf("%w: %s operation", storage.ErrServiceUnavailable, operation) // Retryable
		case "InvalidObjectState":
			return fmt.Errorf("%w: %s operation", storage.ErrInvalidObjectState, operation)
		case "NoSuchKey":
			return fmt.Errorf("%w: %s", storage.ErrFileNotFound, err)
		case "NoSuchBucket":
			return storage.ErrBucketNotFound
		default:
			// Include error code for debugging while preserving original error
			return fmt.Errorf("%s operation failed (code: %s): %w", operation, code, err)
		}
	}

	// Default fallback with context preservation
	return fmt.Errorf("%s operation failed: %w", operation, err)
}
