package response

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/dmitrymomot/gokit/core/handler"
)

// DefaultSSEKeepAlive is the default keep-alive interval for SSE connections.
const DefaultSSEKeepAlive = 30 * time.Second

// sseConfig holds configuration for Server-Sent Events responses.
type sseConfig struct {
	events      <-chan any
	eventName   string
	eventID     string
	idGen       func(any) string
	reconnect   int
	keepAlive   time.Duration
	noKeepAlive bool
	onError     func(context.Context, error) // Optional error handler
}

// EventOption configures Server-Sent Events behavior.
type EventOption func(*sseConfig)

// WithEventName sets the event name for SSE events.
func WithEventName(name string) EventOption {
	return func(s *sseConfig) {
		s.eventName = name
	}
}

// WithEventID sets a fixed event ID for all SSE events.
func WithEventID(id string) EventOption {
	return func(s *sseConfig) {
		s.eventID = id
	}
}

// WithEventIDGenerator sets a function to generate event IDs dynamically based on data.
func WithEventIDGenerator(fn func(data any) string) EventOption {
	return func(s *sseConfig) {
		s.idGen = fn
	}
}

// WithReconnectTime sets the client reconnection time in milliseconds for SSE.
func WithReconnectTime(milliseconds int) EventOption {
	return func(s *sseConfig) {
		s.reconnect = milliseconds
	}
}

// WithKeepAlive sets the keep-alive interval for SSE connections.
func WithKeepAlive(interval time.Duration) EventOption {
	return func(s *sseConfig) {
		s.keepAlive = interval
	}
}

// WithoutKeepAlive disables keep-alive messages for SSE connections.
func WithoutKeepAlive() EventOption {
	return func(s *sseConfig) {
		s.noKeepAlive = true
	}
}

// WithSSEErrorHandler sets an error handler for SSE streaming errors.
// The handler receives the request context and error for logging or monitoring.
func WithSSEErrorHandler(handler func(context.Context, error)) EventOption {
	return func(s *sseConfig) {
		s.onError = handler
	}
}

// SSE creates a Server-Sent Events response from a channel of data.
func SSE(events <-chan any, opts ...EventOption) handler.Response {
	cfg := &sseConfig{
		events:    events,
		keepAlive: DefaultSSEKeepAlive,
	}

	for _, opt := range opts {
		opt(cfg)
	}

	return func(w http.ResponseWriter, req *http.Request) error {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.Header().Set("X-Accel-Buffering", "no")

		if cfg.reconnect > 0 {
			w.Header().Set("Retry", fmt.Sprintf("%d", cfg.reconnect))
		}

		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
			return nil
		}

		w.WriteHeader(http.StatusOK)

		// Write initial connection message
		if _, err := fmt.Fprintf(w, ": connected\n\n"); err != nil {
			if cfg.onError != nil {
				cfg.onError(req.Context(), fmt.Errorf("failed to write connection message: %w", err))
			}
			return nil
		}
		flusher.Flush()

		var keepAliveTicker *time.Ticker
		var keepAliveChan <-chan time.Time

		if !cfg.noKeepAlive && cfg.keepAlive > 0 {
			keepAliveTicker = time.NewTicker(cfg.keepAlive)
			keepAliveChan = keepAliveTicker.C
			defer keepAliveTicker.Stop()
		}

		for {
			select {
			case <-req.Context().Done():
				return nil

			case <-keepAliveChan:
				if _, err := fmt.Fprintf(w, ": keepalive\n\n"); err != nil {
					if cfg.onError != nil {
						cfg.onError(req.Context(), fmt.Errorf("failed to send keepalive: %w", err))
					}
					return nil // Stop on keepalive failure
				}
				flusher.Flush()

			case data, ok := <-cfg.events:
				if !ok {
					return nil
				}

				if keepAliveTicker != nil {
					keepAliveTicker.Reset(cfg.keepAlive)
				}

				if err := writeSSEEvent(w, data, cfg.eventName, cfg.eventID, cfg.idGen); err != nil {
					if cfg.onError != nil {
						cfg.onError(req.Context(), fmt.Errorf("failed to write event: %w", err))
					}
					// Continue streaming despite write error
					continue
				}
				flusher.Flush()
			}
		}
	}
}

// writeSSEEvent writes a Server-Sent Event to the writer.
func writeSSEEvent(w io.Writer, data any, eventName, eventID string, idGen func(any) string) error {
	if eventName != "" {
		if _, err := fmt.Fprintf(w, "event: %s\n", eventName); err != nil {
			return err
		}
	}

	resolvedEventID := eventID
	if idGen != nil {
		resolvedEventID = idGen(data)
	}
	if resolvedEventID != "" {
		if _, err := fmt.Fprintf(w, "id: %s\n", resolvedEventID); err != nil {
			return err
		}
	}

	var dataStr string
	switch v := data.(type) {
	case string:
		dataStr = v
	case []byte:
		dataStr = string(v)
	default:
		jsonData, err := json.Marshal(data)
		if err != nil {
			return err
		}
		dataStr = string(jsonData)
	}

	if _, err := fmt.Fprintf(w, "data: %s\n\n", dataStr); err != nil {
		return err
	}

	return nil
}
