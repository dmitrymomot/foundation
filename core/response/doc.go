// Package response provides comprehensive HTTP response utilities including JSON, HTML,
// Server-Sent Events (SSE), WebSocket upgrades, file serving, and HTMX support.
// It offers a consistent API for generating various types of HTTP responses with
// proper headers, status codes, and error handling.
//
// # Features
//
//   - JSON responses with proper Content-Type headers
//   - HTML template rendering with Go templates and templ components
//   - Server-Sent Events (SSE) for real-time updates
//   - WebSocket connection upgrades
//   - File downloads and streaming responses
//   - HTMX response helpers for modern web applications
//   - HTTP redirections with proper status codes
//   - Comprehensive error handling with status code mapping
//   - Decorator pattern for response enhancement
//
// # Basic Usage
//
// The package provides functions that return handler.Response for use in HTTP handlers:
//
//	import "github.com/dmitrymomot/foundation/core/response"
//
//	// JSON responses
//	func getUserHandler(ctx handler.Context) handler.Response {
//		user := User{ID: 1, Name: "John Doe"}
//		return response.JSON(user)
//	}
//
//	// Error responses
//	func errorHandler(ctx handler.Context) handler.Response {
//		return response.Error(http.StatusBadRequest, "Invalid input")
//	}
//
//	// HTML responses
//	func homeHandler(ctx handler.Context) handler.Response {
//		data := PageData{Title: "Home", Content: "Welcome!"}
//		return response.HTML("home.html", data)
//	}
//
// # JSON Responses
//
// Create JSON responses with automatic serialization and proper headers:
//
//	// Simple JSON response
//	response.JSON(map[string]string{
//		"message": "Success",
//		"status":  "ok",
//	})
//
//	// JSON with custom status code
//	response.JSONWithStatus(http.StatusCreated, user)
//
//	// JSON error response
//	response.JSONError(http.StatusBadRequest, "Validation failed", map[string]string{
//		"email": "Invalid email format",
//		"age":   "Must be at least 18",
//	})
//
//	// Paginated JSON response
//	response.JSON(PaginatedResponse{
//		Data:       users,
//		Page:       1,
//		PerPage:    10,
//		Total:      100,
//		TotalPages: 10,
//	})
//
// # HTML Template Responses
//
// Render HTML templates with data:
//
//	// Basic template rendering
//	response.HTML("user-profile.html", UserData{
//		Name:  "John Doe",
//		Email: "john@example.com",
//	})
//
//	// Template with custom status
//	response.HTMLWithStatus(http.StatusNotFound, "404.html", ErrorData{
//		Message: "Page not found",
//	})
//
//	// Templ component rendering
//	component := userProfileComponent(user)
//	response.Templ(component)
//
// # Server-Sent Events (SSE)
//
// Create real-time streaming responses:
//
//	func liveUpdatesHandler(ctx handler.Context) handler.Response {
//		return response.SSE(func(writer *response.SSEWriter) error {
//			ticker := time.NewTicker(time.Second)
//			defer ticker.Stop()
//
//			for {
//				select {
//				case <-ctx.Done():
//					return nil
//				case <-ticker.C:
//					event := response.SSEEvent{
//						Event: "update",
//						Data:  fmt.Sprintf("Current time: %v", time.Now()),
//						ID:    fmt.Sprintf("%d", time.Now().Unix()),
//					}
//					if err := writer.WriteEvent(event); err != nil {
//						return err
//					}
//				}
//			}
//		})
//	}
//
//	// Simple SSE with predefined events
//	func notificationHandler(ctx handler.Context) handler.Response {
//		events := []response.SSEEvent{
//			{Event: "notification", Data: "New message received"},
//			{Event: "update", Data: "Status changed"},
//		}
//		return response.SSEEvents(events)
//	}
//
// # WebSocket Responses
//
// Upgrade HTTP connections to WebSocket:
//
//	func chatHandler(ctx handler.Context) handler.Response {
//		return response.WebSocket(func(conn *websocket.Conn) error {
//			defer conn.Close()
//
//			for {
//				var message ChatMessage
//				if err := conn.ReadJSON(&message); err != nil {
//					return err
//				}
//
//				// Process message
//				response := processMessage(message)
//
//				if err := conn.WriteJSON(response); err != nil {
//					return err
//				}
//			}
//		})
//	}
//
// # File Responses
//
// Serve files and handle downloads:
//
//	// File download
//	response.File("/path/to/document.pdf", "invoice.pdf")
//
//	// Inline file serving
//	response.FileInline("/path/to/image.jpg")
//
//	// Stream from io.Reader
//	response.Stream(fileReader, "application/pdf", "document.pdf")
//
//	// Byte array response
//	response.Bytes(imageData, "image/jpeg")
//
// # HTMX Responses
//
// Create responses optimized for HTMX applications:
//
//	// Basic HTMX response with triggers
//	response.WithHTMX(
//		response.HTML("partial.html", data),
//		response.TriggerEvent("userUpdated", user),
//		response.PushURL("/users/" + user.ID),
//	)
//
//	// HTMX redirect
//	response.WithHTMX(
//		response.NoContent(),
//		response.HTMXRedirect("/dashboard"),
//	)
//
//	// HTMX with multiple triggers
//	response.WithHTMX(
//		response.JSON(result),
//		response.Trigger(map[string]any{
//			"formSubmitted": map[string]any{
//				"success": true,
//				"message": "Data saved",
//			},
//			"updateUI": nil,
//		}),
//		response.Refresh(),
//	)
//
//	// HTMX swap and retarget
//	response.WithHTMX(
//		response.HTML("new-content.html", data),
//		response.Reswap("outerHTML"),
//		response.Retarget("#content-area"),
//	)
//
// # Redirect Responses
//
// Handle various types of HTTP redirects:
//
//	// Permanent redirect (301)
//	response.Redirect("/new-location", http.StatusMovedPermanently)
//
//	// Temporary redirect (302)
//	response.Redirect("/temporary-location", http.StatusFound)
//
//	// See other (303) - for POST-redirect-GET pattern
//	response.Redirect("/success", http.StatusSeeOther)
//
//	// Temporary redirect (307) - preserves request method
//	response.Redirect("/retry", http.StatusTemporaryRedirect)
//
// # Error Responses
//
// Generate consistent error responses:
//
//	// Simple error
//	response.Error(http.StatusNotFound, "User not found")
//
//	// Error with details
//	response.ErrorWithDetails(http.StatusBadRequest, "Validation failed", map[string]any{
//		"errors": []string{
//			"Email is required",
//			"Password must be at least 8 characters",
//		},
//	})
//
//	// JSON error response
//	response.JSONError(http.StatusUnauthorized, "Access denied", map[string]string{
//		"code": "INVALID_TOKEN",
//		"hint": "Please refresh your token",
//	})
//
// # Response Decorators
//
// Enhance responses with additional functionality:
//
//	// Add custom headers
//	response.WithHeaders(
//		response.JSON(data),
//		map[string]string{
//			"X-API-Version": "v1.2.3",
//			"X-Rate-Limit":  "100",
//		},
//	)
//
//	// Add cookies
//	response.WithCookies(
//		response.HTML("welcome.html", data),
//		&http.Cookie{
//			Name:  "session_id",
//			Value: sessionID,
//		},
//	)
//
//	// Combine decorators
//	response.WithHeaders(
//		response.WithCookies(
//			response.JSON(data),
//			sessionCookie,
//		),
//		headers,
//	)
//
// # Advanced Error Handling
//
// Use error handlers for consistent error processing:
//
//	func apiErrorHandler(ctx handler.Context, err error) {
//		// Log error
//		log.Error("Handler error", "error", err, "path", ctx.Request().URL.Path)
//
//		// Map error to HTTP status
//		status := mapErrorToStatus(err)
//
//		// Send appropriate response
//		if isAPIRequest(ctx.Request()) {
//			response.JSONError(status, err.Error(), nil)(
//				ctx.ResponseWriter(), ctx.Request())
//		} else {
//			response.HTMLWithStatus(status, "error.html", ErrorData{
//				Message: err.Error(),
//			})(ctx.ResponseWriter(), ctx.Request())
//		}
//	}
//
//	func mapErrorToStatus(err error) int {
//		switch {
//		case errors.Is(err, ErrNotFound):
//			return http.StatusNotFound
//		case errors.Is(err, ErrUnauthorized):
//			return http.StatusUnauthorized
//		case errors.Is(err, ErrValidation):
//			return http.StatusBadRequest
//		default:
//			return http.StatusInternalServerError
//		}
//	}
//
// # Content Negotiation
//
// Handle different content types based on Accept header:
//
//	func adaptiveResponse(ctx handler.Context, data any) handler.Response {
//		accept := ctx.Request().Header.Get("Accept")
//
//		switch {
//		case strings.Contains(accept, "application/json"):
//			return response.JSON(data)
//		case strings.Contains(accept, "text/html"):
//			return response.HTML("data.html", data)
//		case strings.Contains(accept, "text/xml"):
//			return response.XML(data)
//		default:
//			return response.JSON(data) // Default to JSON
//		}
//	}
//
// # Caching Headers
//
// Set appropriate caching headers:
//
//	// Cache for 1 hour
//	response.WithHeaders(
//		response.JSON(data),
//		map[string]string{
//			"Cache-Control": "public, max-age=3600",
//			"ETag":          generateETag(data),
//		},
//	)
//
//	// No cache for dynamic content
//	response.WithHeaders(
//		response.HTML("dashboard.html", userData),
//		map[string]string{
//			"Cache-Control": "no-cache, no-store, must-revalidate",
//			"Pragma":        "no-cache",
//			"Expires":       "0",
//		},
//	)
//
// # API Response Patterns
//
// Implement consistent API response patterns:
//
//	type APIResponse struct {
//		Success bool        `json:"success"`
//		Data    any         `json:"data,omitempty"`
//		Error   *APIError   `json:"error,omitempty"`
//		Meta    *APIMeta    `json:"meta,omitempty"`
//	}
//
//	func successResponse(data any) handler.Response {
//		return response.JSON(APIResponse{
//			Success: true,
//			Data:    data,
//		})
//	}
//
//	func errorResponse(code int, message string) handler.Response {
//		return response.JSONWithStatus(code, APIResponse{
//			Success: false,
//			Error: &APIError{
//				Code:    code,
//				Message: message,
//			},
//		})
//	}
//
//	func paginatedResponse(data any, page, limit, total int) handler.Response {
//		return response.JSON(APIResponse{
//			Success: true,
//			Data:    data,
//			Meta: &APIMeta{
//				Page:       page,
//				Limit:      limit,
//				Total:      total,
//				TotalPages: (total + limit - 1) / limit,
//			},
//		})
//	}
//
// # Best Practices
//
//   - Use appropriate HTTP status codes for different scenarios
//   - Set proper Content-Type headers for all responses
//   - Implement consistent error response formats
//   - Use HTMX helpers for modern web applications
//   - Handle content negotiation for API versioning
//   - Set appropriate caching headers for static content
//   - Log errors before sending error responses
//   - Use streaming responses for large data sets
//   - Implement proper CORS headers for API endpoints
//   - Use decorators to add cross-cutting concerns like headers and cookies
package response
