// Package binder provides comprehensive HTTP request data binding utilities for Go web applications.
// It supports binding JSON, form data, query parameters, and path parameters to Go structs
// with built-in validation, sanitization, and security features.
//
// # Features
//
//   - JSON binding with strict parsing and size limits
//   - Form data binding supporting both URL-encoded and multipart forms
//   - Query parameter binding with multi-value support
//   - Path parameter binding compatible with popular routers
//   - Automatic input sanitization to prevent XSS and injection attacks
//   - Comprehensive error handling with descriptive messages
//   - Security hardening against DoS and malformed data attacks
//
// # Usage
//
// The package provides four main binding functions that can be used individually
// or combined in a middleware chain:
//
//	import "github.com/dmitrymomot/foundation/core/binder"
//
//	// JSON binding
//	jsonBinder := binder.JSON()
//
//	// Form binding (URL-encoded and multipart)
//	formBinder := binder.Form()
//
//	// Query parameter binding
//	queryBinder := binder.Query()
//
//	// Path parameter binding (requires router-specific extractor)
//	pathBinder := binder.Path(chi.URLParam) // for chi router
//
// # JSON Binding
//
// JSON binding supports automatic parsing of request bodies with Content-Type validation,
// size limits, and strict parsing to prevent malformed data:
//
//	type CreateUserRequest struct {
//		Name     string `json:"name"`
//		Email    string `json:"email"`
//		Age      int    `json:"age"`
//		Optional *bool  `json:"optional,omitempty"`
//	}
//
//	func handler(w http.ResponseWriter, r *http.Request) {
//		var req CreateUserRequest
//		if err := binder.JSON()(r, &req); err != nil {
//			http.Error(w, err.Error(), http.StatusBadRequest)
//			return
//		}
//		// req is now populated from JSON body
//	}
//
// # Form Binding
//
// Form binding handles both URL-encoded forms and multipart forms with file uploads.
// It supports comprehensive struct tags and type conversion:
//
//	type UploadRequest struct {
//		Title       string                  `form:"title"`
//		Category    string                  `form:"category"`
//		Tags        []string                `form:"tags"`     // Multi-value support
//		IsPublic    bool                    `form:"public"`   // String to bool conversion
//		Priority    int                     `form:"priority"` // String to int conversion
//		Avatar      *multipart.FileHeader   `file:"avatar"`   // Single file upload
//		Attachments []*multipart.FileHeader `file:"files"`    // Multiple file uploads
//		Internal    string                  `form:"-"`        // Ignored field
//	}
//
//	func uploadHandler(w http.ResponseWriter, r *http.Request) {
//		var req UploadRequest
//		if err := binder.Form()(r, &req); err != nil {
//			http.Error(w, err.Error(), http.StatusBadRequest)
//			return
//		}
//
//		// Process uploaded files
//		if req.Avatar != nil {
//			file, err := req.Avatar.Open()
//			if err != nil {
//				// Handle error
//			}
//			defer file.Close()
//			// Process file content
//		}
//	}
//
// # Query Parameter Binding
//
// Query parameter binding extracts data from URL query strings with support
// for multi-value parameters and type conversion:
//
//	type SearchRequest struct {
//		Query     string   `query:"q"`
//		Page      int      `query:"page"`
//		PageSize  int      `query:"page_size"`
//		Tags      []string `query:"tags"`     // ?tags=go&tags=web
//		Active    *bool    `query:"active"`   // Optional parameter
//		SortBy    string   `query:"sort"`
//		Ascending bool     `query:"asc"`
//	}
//
//	func searchHandler(w http.ResponseWriter, r *http.Request) {
//		var req SearchRequest
//		if err := binder.Query()(r, &req); err != nil {
//			http.Error(w, err.Error(), http.StatusBadRequest)
//			return
//		}
//		// req is populated from query parameters
//	}
//
// # Path Parameter Binding
//
// Path parameter binding extracts values from URL path segments using
// router-specific extractor functions:
//
//	type ProfileRequest struct {
//		UserID   string `path:"id"`
//		Username string `path:"username"`
//		Tab      string `path:"tab"`
//	}
//
//	// With chi router
//	func setupChiRoutes(r chi.Router) {
//		pathBinder := binder.Path(chi.URLParam)
//		r.Get("/users/{id}/profile/{username}/{tab}", func(w http.ResponseWriter, r *http.Request) {
//			var req ProfileRequest
//			if err := pathBinder(r, &req); err != nil {
//				http.Error(w, err.Error(), http.StatusBadRequest)
//				return
//			}
//			// req.UserID, req.Username, req.Tab are populated
//		})
//	}
//
//	// With gorilla/mux
//	func setupMuxRoutes(router *mux.Router) {
//		muxExtractor := func(r *http.Request, fieldName string) string {
//			vars := mux.Vars(r)
//			return vars[fieldName]
//		}
//		pathBinder := binder.Path(muxExtractor)
//		// Use pathBinder in handlers
//	}
//
// # Combining Multiple Binders
//
// Multiple binders can be combined to handle complex request structures
// that include data from different sources:
//
//	type ComplexRequest struct {
//		// From path parameters
//		UserID   string `path:"id"`
//
//		// From query parameters
//		Page     int    `query:"page"`
//		PageSize int    `query:"page_size"`
//
//		// From form data
//		Name     string                `form:"name"`
//		Avatar   *multipart.FileHeader `file:"avatar"`
//
//		// From JSON body (requires separate handling)
//		Metadata map[string]any `json:"metadata"`
//	}
//
//	func complexHandler(w http.ResponseWriter, r *http.Request) {
//		var req ComplexRequest
//
//		// Apply binders in sequence
//		binders := []func(*http.Request, any) error{
//			binder.Path(chi.URLParam),
//			binder.Query(),
//			binder.Form(),
//		}
//
//		for _, bind := range binders {
//			if err := bind(r, &req); err != nil {
//				http.Error(w, err.Error(), http.StatusBadRequest)
//				return
//			}
//		}
//
//		// req is now populated from all sources
//	}
//
// # Supported Types
//
// The binder package supports automatic type conversion for:
//
//   - string
//   - int, int8, int16, int32, int64
//   - uint, uint8, uint16, uint32, uint64
//   - float32, float64
//   - bool (recognizes: true, false, 1, 0, on, off, yes, no)
//   - Slices of any of the above types
//   - Pointers to any of the above types (for optional fields)
//   - *multipart.FileHeader and []*multipart.FileHeader (for file uploads)
//
// # Security Features
//
// The package includes several security hardening measures:
//
//   - Request size limits to prevent DoS attacks (1MB for JSON, 10MB for forms)
//   - Input sanitization to prevent XSS and injection attacks
//   - Filename sanitization for uploaded files to prevent path traversal
//   - Boundary validation for multipart forms
//   - Strict JSON parsing with unknown field rejection
//   - Context timeout handling to avoid processing cancelled requests
//
// # Error Handling
//
// The package provides comprehensive error types for different failure scenarios:
//
//	var (
//		ErrUnsupportedMediaType   // Wrong Content-Type header
//		ErrFailedToParseJSON     // JSON parsing failures
//		ErrFailedToParseForm     // Form parsing failures
//		ErrFailedToParseQuery    // Query parameter failures
//		ErrFailedToParsePath     // Path parameter failures
//		ErrMissingContentType    // Missing Content-Type header
//		ErrBinderNotApplicable   // Binder cannot process request
//	)
//
//	func handleBindingError(err error) {
//		switch {
//		case errors.Is(err, binder.ErrUnsupportedMediaType):
//			// Handle unsupported media type
//		case errors.Is(err, binder.ErrFailedToParseJSON):
//			// Handle JSON parsing error
//		default:
//			// Handle other binding errors
//		}
//	}
//
// # Best Practices
//
//   - Use struct tags to clearly specify parameter names and binding sources
//   - Combine multiple binders when handling complex request structures
//   - Validate bound data using a validation library after binding
//   - Handle binding errors appropriately and provide clear error messages
//   - Use pointer types for optional fields to distinguish between zero values and missing parameters
//   - Always close multipart form resources when done processing uploaded files
//
// # Performance Considerations
//
//   - JSON binding limits request body size to 1MB by default
//   - Form binding uses 10MB memory limit for multipart parsing
//   - Path extraction requires router-specific functions for optimal performance
//   - String sanitization is applied automatically but adds minimal overhead
//   - Reflection is used for struct field binding, so cache binder functions when possible
package binder
