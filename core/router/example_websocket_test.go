package router_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"

	"github.com/dmitrymomot/foundation/core/handler"
	"github.com/dmitrymomot/foundation/core/router"
)

// Example_webSocketUpgrade demonstrates how to handle WebSocket upgrades with the router.
// The router's responseWriter now implements http.Hijacker interface, enabling WebSocket support.
func Example_webSocketUpgrade() {
	r := router.New[*router.Context]()

	// WebSocket endpoint
	r.Get("/ws", func(ctx *router.Context) handler.Response {
		return func(w http.ResponseWriter, r *http.Request) error {
			// Check if this is a WebSocket upgrade request
			if r.Header.Get("Upgrade") != "websocket" {
				http.Error(w, "Not a WebSocket request", http.StatusBadRequest)
				return nil
			}

			// Verify that our response writer supports hijacking
			hijacker, ok := w.(http.Hijacker)
			if !ok {
				http.Error(w, "WebSocket upgrade not supported", http.StatusInternalServerError)
				return nil
			}

			// At this point, you would typically:
			// 1. Use a WebSocket library (e.g., gorilla/websocket) to upgrade the connection
			// 2. The library would use hijacker.Hijack() to take over the connection
			// 3. Handle WebSocket communication

			// For this example, we just demonstrate that the interface is available
			fmt.Printf("WebSocket upgrade ready - Hijacker interface available: %v\n", hijacker != nil)

			// In production, you would upgrade the connection here
			// upgrader.Upgrade(w, r, nil)

			w.WriteHeader(http.StatusSwitchingProtocols)
			return nil
		}
	})

	// Create a test request with WebSocket headers
	req := httptest.NewRequest(http.MethodGet, "/ws", nil)
	req.Header.Set("Upgrade", "websocket")
	req.Header.Set("Connection", "Upgrade")
	w := httptest.NewRecorder()

	// Serve the request
	r.ServeHTTP(w, req)

	// Output: WebSocket upgrade ready - Hijacker interface available: true
}

// Example_http2Push demonstrates HTTP/2 server push capability with the router.
// The router's responseWriter now implements http.Pusher interface for HTTP/2 push support.
func Example_http2Push() {
	r := router.New[*router.Context]()

	r.Get("/", func(ctx *router.Context) handler.Response {
		return func(w http.ResponseWriter, r *http.Request) error {
			// Check if HTTP/2 push is supported
			pusher, ok := w.(http.Pusher)
			if ok {
				// In a real HTTP/2 connection, we would push resources
				// For this example, we just verify the interface is available
				fmt.Printf("HTTP/2 Pusher interface available: %v\n", pusher != nil)
			}

			// Send the main HTML response
			w.Header().Set("Content-Type", "text/html")
			w.Write([]byte(`<!DOCTYPE html>
<html>
<head>
    <link rel="stylesheet" href="/static/style.css">
    <script src="/static/app.js"></script>
</head>
<body>
    <h1>HTTP/2 Push Example</h1>
</body>
</html>`))

			return nil
		}
	})

	// Create a test request
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()

	// Serve the request
	r.ServeHTTP(w, req)

	// Output: HTTP/2 Pusher interface available: true
}
