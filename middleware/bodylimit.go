package middleware

import (
	"fmt"
	"io"
	"mime"
	"strconv"

	"github.com/dmitrymomot/gokit/core/handler"
	"github.com/dmitrymomot/gokit/core/response"
)

// BodyLimitConfig configures the request body limit middleware.
// It provides fine-grained control over request body size restrictions.
type BodyLimitConfig struct {
	// Skip defines a function to skip middleware execution for specific requests
	Skip func(ctx handler.Context) bool

	// MaxSize is the maximum allowed size in bytes (default: 4MB)
	MaxSize int64

	// ContentTypeLimit allows setting different limits per content type
	// Example: {"application/json": 1MB, "multipart/form-data": 10MB}
	ContentTypeLimit map[string]int64

	// ErrorHandler handles requests that exceed the size limit
	ErrorHandler func(ctx handler.Context, contentLength int64, maxSize int64) handler.Response

	// DisableContentLengthCheck skips the Content-Length header check
	// and only enforces the limit during body reading
	DisableContentLengthCheck bool
}

// BodyLimit creates a body limit middleware with default configuration (4MB limit).
// It prevents processing of requests with excessively large bodies.
func BodyLimit[C handler.Context]() handler.Middleware[C] {
	return BodyLimitWithConfig[C](BodyLimitConfig{})
}

// BodyLimitWithSize creates a body limit middleware with a specified size limit.
func BodyLimitWithSize[C handler.Context](maxSize int64) handler.Middleware[C] {
	return BodyLimitWithConfig[C](BodyLimitConfig{
		MaxSize: maxSize,
	})
}

// BodyLimitWithConfig creates a body limit middleware with custom configuration.
// It restricts the size of incoming request bodies to prevent resource exhaustion.
func BodyLimitWithConfig[C handler.Context](cfg BodyLimitConfig) handler.Middleware[C] {
	// Set defaults
	if cfg.MaxSize <= 0 {
		cfg.MaxSize = 4 * 1024 * 1024 // 4MB default
	}

	if cfg.ErrorHandler == nil {
		cfg.ErrorHandler = func(ctx handler.Context, contentLength int64, maxSize int64) handler.Response {
			message := fmt.Sprintf("Request body too large. Maximum allowed: %s", formatBytes(maxSize))
			details := map[string]any{
				"limit": maxSize,
			}
			if contentLength > 0 {
				message = fmt.Sprintf("Request body too large. Size: %s, Maximum allowed: %s",
					formatBytes(contentLength), formatBytes(maxSize))
				details["size"] = contentLength
			}
			return response.Error(response.ErrRequestEntityTooLarge.WithMessage(message).WithDetails(details))
		}
	}

	return func(next handler.HandlerFunc[C]) handler.HandlerFunc[C] {
		return func(ctx C) handler.Response {
			if cfg.Skip != nil && cfg.Skip(ctx) {
				return next(ctx)
			}

			req := ctx.Request()

			// Determine the size limit based on content type
			maxSize := cfg.MaxSize
			if cfg.ContentTypeLimit != nil {
				contentType := req.Header.Get("Content-Type")
				// Extract the main content type using mime.ParseMediaType
				mediaType, _, err := mime.ParseMediaType(contentType)
				if err == nil {
					if limit, ok := cfg.ContentTypeLimit[mediaType]; ok {
						maxSize = limit
					}
				}
			}

			// Check Content-Length header if not disabled
			if !cfg.DisableContentLengthCheck {
				if contentLengthStr := req.Header.Get("Content-Length"); contentLengthStr != "" {
					contentLength, err := strconv.ParseInt(contentLengthStr, 10, 64)
					if err == nil && contentLength > maxSize {
						return cfg.ErrorHandler(ctx, contentLength, maxSize)
					}
				}
			}

			// Wrap the request body with a limited reader
			originalBody := req.Body
			if originalBody != nil {
				req.Body = &limitedReader{
					reader:  originalBody,
					limit:   maxSize,
					read:    0,
					ctx:     ctx,
					handler: cfg.ErrorHandler,
				}
			}

			return next(ctx)
		}
	}
}

// limitedReader wraps an io.ReadCloser to enforce a size limit
type limitedReader struct {
	reader  io.ReadCloser
	limit   int64
	read    int64
	ctx     handler.Context
	handler func(handler.Context, int64, int64) handler.Response
}

// Read implements io.Reader
func (lr *limitedReader) Read(p []byte) (int, error) {
	if lr.read >= lr.limit {
		return 0, fmt.Errorf("request body size exceeds limit of %d bytes (read: %d)", lr.limit, lr.read)
	}

	// Calculate how much we can read without exceeding the limit
	remaining := lr.limit - lr.read
	if int64(len(p)) > remaining {
		p = p[:remaining]
	}

	n, err := lr.reader.Read(p)
	lr.read += int64(n)

	// Check if we've exceeded the limit after reading
	if lr.read > lr.limit {
		return n, fmt.Errorf("request body size exceeds limit of %d bytes (read: %d)", lr.limit, lr.read)
	}

	return n, err
}

// Close implements io.Closer
func (lr *limitedReader) Close() error {
	return lr.reader.Close()
}

// formatBytes formats bytes into a human-readable string
func formatBytes(bytes int64) string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
	)

	switch {
	case bytes >= GB:
		return fmt.Sprintf("%.2f GB", float64(bytes)/float64(GB))
	case bytes >= MB:
		return fmt.Sprintf("%.2f MB", float64(bytes)/float64(MB))
	case bytes >= KB:
		return fmt.Sprintf("%.2f KB", float64(bytes)/float64(KB))
	default:
		return fmt.Sprintf("%d bytes", bytes)
	}
}

// Common size constants for convenience
const (
	// KB represents 1 kilobyte
	KB int64 = 1024
	// MB represents 1 megabyte
	MB = 1024 * KB
	// GB represents 1 gigabyte
	GB = 1024 * MB
)
