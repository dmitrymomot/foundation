// Package response provides HTTP response utilities for web applications.
// It offers a consistent API for generating various types of HTTP responses
// including JSON, HTML templates, files, redirects, streaming responses, WebSockets,
// Server-Sent Events, and HTMX-enhanced responses.
//
// # Basic Usage
//
// All functions return handler.Response which can be used in HTTP handlers:
//
//	import "github.com/dmitrymomot/foundation/core/response"
//
//	func getUserHandler(ctx handler.Context) handler.Response {
//		user := User{ID: 1, Name: "John Doe"}
//		return response.JSON(user)
//	}
//
//	func homeHandler(ctx handler.Context) handler.Response {
//		return response.HTML("<h1>Welcome!</h1>")
//	}
//
// # JSON Responses
//
// Create JSON responses with automatic serialization:
//
//	// JSON with 200 OK status
//	response.JSON(map[string]string{
//		"message": "Success",
//		"status":  "ok",
//	})
//
//	// JSON with custom status code
//	response.JSONWithStatus(user, http.StatusCreated)
//
// # Basic Response Types
//
// Create simple text and HTML responses:
//
//	// Plain text response
//	response.String("Hello, World!")
//
//	// HTML content
//	response.HTML("<h1>Welcome</h1>")
//
//	// Raw bytes with content type
//	response.Bytes(imageData, "image/jpeg")
//
//	// Empty responses
//	response.NoContent()           // 204 No Content
//	response.Status(http.StatusOK) // Custom status with no body
//
// # Template Responses
//
// Render Go html/template templates:
//
//	tmpl := template.Must(template.ParseFiles("user.html"))
//	response.Template(tmpl, userData)
//
//	// Named templates from parsed files
//	response.TemplateName(tmpl, "user-profile", userData)
//
//	// Streaming templates (more memory efficient)
//	response.TemplateStream(tmpl, userData)
//
// # Templ Component Responses
//
// Render templ components:
//
//	component := userProfile(user)
//	response.Templ(component)
//
//	// With custom status
//	response.TemplWithStatus(component, http.StatusCreated)
//
// # File Responses
//
// Serve files and handle downloads:
//
//	// Serve static files
//	response.File("/path/to/image.jpg")
//
//	// Force download with custom filename
//	response.Download("/path/to/document.pdf", "invoice.pdf")
//
//	// Download in-memory data
//	response.Attachment(pdfData, "report.pdf", "application/pdf")
//
//	// Stream from io.Reader
//	response.FileReader(reader, "data.csv", "text/csv")
//
//	// Generate CSV downloads
//	records := [][]string{{"Name", "Age"}, {"John", "30"}}
//	response.CSV(records, "users.csv")
//
//	// CSV with headers
//	response.CSVWithHeaders([]string{"Name", "Age"}, userRows, "users.csv")
//
// # Redirects
//
// Handle HTTP redirections (with automatic HTMX support):
//
//	// Temporary redirect (302)
//	response.Redirect("/dashboard")
//
//	// Permanent redirect (301)
//	response.RedirectPermanent("/new-location")
//
//	// See Other (303) - POST-redirect-GET pattern
//	response.RedirectSeeOther("/success")
//
//	// Temporary redirect preserving method (307)
//	response.RedirectTemporary("/retry")
//
//	// Custom redirect status
//	response.RedirectWithStatus("/custom", http.StatusFound)
//
// # Server-Sent Events (SSE)
//
// Create real-time streaming responses:
//
//	events := make(chan any)
//	go func() {
//		defer close(events)
//		for i := 0; i < 10; i++ {
//			events <- fmt.Sprintf("Event %d", i)
//			time.Sleep(time.Second)
//		}
//	}()
//
//	response.SSE(events)
//
//	// With custom event configuration
//	response.SSE(events,
//		response.WithEventName("update"),
//		response.WithKeepAlive(30*time.Second),
//		response.WithEventIDGenerator(func(data any) string {
//			return fmt.Sprintf("msg-%d", time.Now().Unix())
//		}),
//	)
//
// # WebSocket Responses
//
// Upgrade HTTP connections to WebSocket:
//
//	response.WebSocket(func(ctx context.Context, conn *websocket.Conn) error {
//		defer conn.Close()
//		for {
//			var message map[string]any
//			if err := conn.ReadJSON(&message); err != nil {
//				return err
//			}
//			// Echo message back
//			return conn.WriteJSON(message)
//		}
//	})
//
//	// Simple echo WebSocket
//	response.EchoWebSocket()
//
//	// Channel-based WebSocket
//	incoming := make(chan response.WebSocketMessage)
//	outgoing := make(chan response.WebSocketMessage)
//	response.WebSocketWithChannels(incoming, outgoing)
//
// # Streaming Responses
//
// Create streaming responses for large data:
//
//	// Custom streaming
//	response.Stream(func(w io.Writer) error {
//		for i := 0; i < 1000; i++ {
//			fmt.Fprintf(w, "Line %d\n", i)
//			if f, ok := w.(http.Flusher); ok {
//				f.Flush()
//			}
//		}
//		return nil
//	})
//
//	// Newline-delimited JSON streaming
//	items := make(chan any)
//	go func() {
//		defer close(items)
//		for _, user := range users {
//			items <- user
//		}
//	}()
//	response.StreamJSON(items)
//
// # HTMX Support
//
// Enhanced responses for HTMX applications:
//
//	// Basic HTMX response with triggers
//	response.WithHTMX(
//		response.HTML("<div>Updated</div>"),
//		response.TriggerEvent("userUpdated", userData),
//		response.PushURL("/users/1"),
//	)
//
//	// HTMX redirect
//	response.WithHTMX(
//		response.NoContent(),
//		response.HTMXRedirect("/dashboard"),
//	)
//
//	// Complex HTMX behavior
//	response.WithHTMX(
//		response.Templ(component),
//		response.Trigger(map[string]any{
//			"formSubmitted": map[string]any{"success": true},
//			"updateUI":      nil,
//		}),
//		response.Reswap("outerHTML"),
//		response.Retarget("#content"),
//	)
//
//	// Check for HTMX requests
//	if response.IsHTMXRequest(request) {
//		// Return partial HTML
//	}
//
// # Response Decorators
//
// Enhance responses with headers, cookies, and caching:
//
//	// Add custom headers
//	response.WithHeaders(
//		response.JSON(data),
//		map[string]string{
//			"X-API-Version": "v1.0.0",
//			"X-Rate-Limit":  "100",
//		},
//	)
//
//	// Add cookies
//	response.WithCookie(
//		response.HTML("<h1>Welcome</h1>"),
//		&http.Cookie{
//			Name:  "session_id",
//			Value: sessionID,
//		},
//	)
//
//	// Cache control
//	response.WithCache(
//		response.JSON(publicData),
//		time.Hour, // Cache for 1 hour
//	)
//
//	// Disable caching
//	response.WithCache(response.HTML(dynamicContent), 0)
//
// # Error Handling
//
// The package provides structured error handling with HTTPError types:
//
//	// Return an error to be handled by error middleware
//	response.Error(errors.New("something went wrong"))
//
//	// Use predefined HTTP errors
//	response.Error(response.ErrNotFound)
//	response.Error(response.ErrUnauthorized.WithMessage("Invalid token"))
//
//	// Custom HTTP error
//	httpErr := response.HTTPError{
//		Status:  http.StatusBadRequest,
//		Code:    "validation_failed",
//		Message: "Invalid input data",
//		Details: map[string]any{
//			"field_errors": []string{"email is required"},
//		},
//	}
//	response.Error(httpErr)
//
//	// Use error handlers for consistent error processing
//	response.ErrorHandler(ctx, err)     // Plain text error response
//	response.JSONErrorHandler(ctx, err) // JSON error response
//
// # Rendering Responses
//
// Use the Render function to execute responses in handlers:
//
//	func handler(ctx handler.Context) {
//		resp := response.JSON(data)
//		response.Render(ctx, resp)
//	}
package response
