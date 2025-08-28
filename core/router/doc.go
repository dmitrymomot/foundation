// Package router provides a high-performance HTTP router with middleware support,
// context management, and flexible routing patterns. It offers both programmatic
// route definition and chainable middleware composition for building robust web applications.
//
// # Features
//
//   - High-performance radix tree-based routing
//   - Type-safe middleware composition
//   - Context-aware request handling
//   - Path parameter extraction
//   - Method-based routing (GET, POST, PUT, DELETE, etc.)
//   - Mount support for sub-routers
//   - Error handling with custom error handlers
//   - Middleware chaining with proper execution order
//   - Compatible with standard http.Handler interface
//
// # Basic Usage
//
// Create a router and define routes with handlers:
//
//	import "github.com/dmitrymomot/foundation/core/router"
//
//	// Create a new router
//	r := router.New()
//
//	// Define routes
//	r.GET("/users", listUsersHandler)
//	r.POST("/users", createUserHandler)
//	r.GET("/users/{id}", getUserHandler)
//	r.PUT("/users/{id}", updateUserHandler)
//	r.DELETE("/users/{id}", deleteUserHandler)
//
//	// Start server
//	http.ListenAndServe(":8080", r)
//
// # Path Parameters
//
// Extract path parameters from URLs:
//
//	func getUserHandler(ctx router.Context) router.Response {
//		userID := ctx.Param("id")
//		user, err := userService.GetByID(userID)
//		if err != nil {
//			return response.Error(404, "User not found")
//		}
//		return response.JSON(user)
//	}
//
//	// Multiple parameters
//	r.GET("/users/{userID}/posts/{postID}", func(ctx router.Context) router.Response {
//		userID := ctx.Param("userID")
//		postID := ctx.Param("postID")
//		// Handle request
//	})
//
// # Middleware
//
// Add middleware for cross-cutting concerns:
//
//	// Logging middleware
//	func loggingMiddleware(next router.HandlerFunc) router.HandlerFunc {
//		return func(ctx router.Context) router.Response {
//			start := time.Now()
//			response := next(ctx)
//			log.Printf("%s %s - %v", ctx.Request().Method, ctx.Request().URL.Path, time.Since(start))
//			return response
//		}
//	}
//
//	// Auth middleware
//	func authMiddleware(next router.HandlerFunc) router.HandlerFunc {
//		return func(ctx router.Context) router.Response {
//			token := ctx.Request().Header.Get("Authorization")
//			if !isValidToken(token) {
//				return response.Error(401, "Unauthorized")
//			}
//			ctx.SetValue("userID", extractUserID(token))
//			return next(ctx)
//		}
//	}
//
//	// Apply middleware
//	r.Use(loggingMiddleware)
//	r.Use(authMiddleware)
//
// # Route Groups
//
// Group routes with common middleware:
//
//	// API v1 routes
//	v1 := r.Group("/api/v1")
//	v1.Use(apiMiddleware)
//	v1.GET("/users", listUsersHandler)
//	v1.POST("/users", createUserHandler)
//
//	// Admin routes
//	admin := r.Group("/admin")
//	admin.Use(adminMiddleware)
//	admin.GET("/dashboard", dashboardHandler)
//	admin.GET("/users", adminUsersHandler)
//
// # Error Handling
//
// Implement custom error handling:
//
//	r.SetErrorHandler(func(ctx router.Context, err error) {
//		log.Printf("Route error: %v", err)
//		switch {
//		case errors.Is(err, ErrNotFound):
//			response.Error(404, "Not found")(ctx.ResponseWriter(), ctx.Request())
//		case errors.Is(err, ErrUnauthorized):
//			response.Error(401, "Unauthorized")(ctx.ResponseWriter(), ctx.Request())
//		default:
//			response.Error(500, "Internal server error")(ctx.ResponseWriter(), ctx.Request())
//		}
//	})
//
// # Sub-routers and Mounting
//
// Mount sub-routers for modular applications:
//
//	// User routes
//	userRouter := router.New()
//	userRouter.GET("/", listUsersHandler)
//	userRouter.POST("/", createUserHandler)
//	userRouter.GET("/{id}", getUserHandler)
//
//	// Product routes
//	productRouter := router.New()
//	productRouter.GET("/", listProductsHandler)
//	productRouter.POST("/", createProductHandler)
//
//	// Mount sub-routers
//	r.Mount("/users", userRouter)
//	r.Mount("/products", productRouter)
//
// # Best Practices
//
//   - Use middleware for cross-cutting concerns
//   - Group related routes together
//   - Implement proper error handling
//   - Use path parameters for dynamic routes
//   - Apply authentication/authorization middleware appropriately
//   - Log requests and responses for debugging
//   - Use context for request-scoped data
//   - Keep handlers focused on single responsibilities
package router
