package gokit

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// streamResponse implements Response for custom streaming content.
// It executes a writer function that has direct access to the response writer.
type streamResponse struct {
	writer func(w io.Writer) error
}

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
func Stream(writer func(w io.Writer) error) Response {
	return &streamResponse{
		writer: writer,
	}
}

// Render implements the Response interface for streamResponse.
func (r *streamResponse) Render(w http.ResponseWriter, req *http.Request) error {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
		return nil
	}

	w.Header().Set("Transfer-Encoding", "chunked")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	w.WriteHeader(http.StatusOK)

	if err := r.writer(w); err != nil {
		// Can't change status after WriteHeader, but we can log the error
		// The error is returned to be handled by the framework
		return err
	}

	// Final flush to ensure all data is sent
	flusher.Flush()

	return nil
}

// streamJSONResponse implements Response for streaming JSON lines (NDJSON).
type streamJSONResponse struct {
	items   <-chan any
	onError func(context.Context, error) // Optional error handler
}

// StreamOption configures streaming behavior.
type StreamOption func(*streamJSONResponse)

// WithStreamErrorHandler sets an error handler for streaming errors.
// The handler receives the request context and error for logging or monitoring.
func WithStreamErrorHandler(handler func(context.Context, error)) StreamOption {
	return func(s *streamJSONResponse) {
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
func StreamJSON(items <-chan any, opts ...StreamOption) Response {
	r := &streamJSONResponse{
		items: items,
	}
	for _, opt := range opts {
		opt(r)
	}
	return r
}

// Render implements the Response interface for streamJSONResponse.
func (r *streamJSONResponse) Render(w http.ResponseWriter, req *http.Request) error {
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

	// Stream items from channel
	encoder := json.NewEncoder(w)

	for {
		select {
		case <-req.Context().Done():
			// Request cancelled
			return nil

		case item, ok := <-r.items:
			if !ok {
				// Channel closed, streaming complete
				return nil
			}

			// Encode and write the item
			if err := encoder.Encode(item); err != nil {
				if r.onError != nil {
					// Pass request context to error handler
					r.onError(req.Context(), fmt.Errorf("failed to encode item: %w", err))
				}
				// Continue streaming despite error
				continue
			}

			// Flush to send data immediately
			flusher.Flush()
		}
	}
}
