package response

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/dmitrymomot/gokit/core/handler"
)

// Stream creates a streaming response that gives direct access to the response writer.
// The writer function should write data in chunks and return any errors.
// The response will automatically be flushed after the writer function completes.
//
// Example:
//
//	Stream(func(w io.Writer) error {
//	    for i := range 100 {
//	        fmt.Fprintf(w, "Data chunk %d\n", i)
//	        if f, ok := w.(http.Flusher); ok {
//	            f.Flush() // Flush for real-time streaming
//	        }
//	    }
//	    return nil
//	})
func Stream(writer func(w io.Writer) error) handler.Response {
	return func(w http.ResponseWriter, r *http.Request) error {
		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
			return nil
		}

		w.Header().Set("Transfer-Encoding", "chunked")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")

		w.WriteHeader(http.StatusOK)

		if err := writer(w); err != nil {
			// Status code cannot be changed after WriteHeader - error goes to framework
			return err
		}

		flusher.Flush()
		return nil
	}
}

// streamJSONConfig holds configuration for streaming JSON responses.
type streamJSONConfig struct {
	items   <-chan any
	onError func(context.Context, error)
}

// StreamOption configures streaming behavior.
type StreamOption func(*streamJSONConfig)

// WithStreamErrorHandler sets an error handler for streaming errors.
func WithStreamErrorHandler(handler func(context.Context, error)) StreamOption {
	return func(s *streamJSONConfig) {
		s.onError = handler
	}
}

// StreamJSON creates a newline-delimited JSON streaming response.
// Each item from the channel is marshaled to JSON and written as a separate line.
// This format is compatible with tools like jq and is ideal for streaming large datasets.
//
// The response uses Content-Type: application/x-ndjson
//
// Example:
//
//	items := make(chan any)
//	go func() {
//	    defer close(items)
//	    for _, user := range users {
//	        items <- user
//	    }
//	}()
//	return StreamJSON(items)
//
// With error handling:
//
//	return StreamJSON(items, WithStreamErrorHandler(func(ctx context.Context, err error) {
//	    logger := ctx.Value("logger").(*slog.Logger)
//	    logger.Error("stream error", "error", err)
//	}))
func StreamJSON(items <-chan any, opts ...StreamOption) handler.Response {
	cfg := &streamJSONConfig{
		items: items,
	}
	for _, opt := range opts {
		opt(cfg)
	}

	return func(w http.ResponseWriter, req *http.Request) error {
		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
			return nil
		}

		w.Header().Set("Content-Type", "application/x-ndjson")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.Header().Set("X-Content-Type-Options", "nosniff")

		w.WriteHeader(http.StatusOK)

		encoder := json.NewEncoder(w)

		for {
			select {
			case <-req.Context().Done():
				return nil

			case item, ok := <-cfg.items:
				if !ok {
					return nil
				}

				if err := encoder.Encode(item); err != nil {
					if cfg.onError != nil {
						cfg.onError(req.Context(), fmt.Errorf("failed to encode item: %w", err))
					}
					continue
				}

				flusher.Flush()
			}
		}
	}
}
