// Package feature provides a flexible and extensible feature flagging system
// for Go applications. It supports various rollout strategies including
// percentage-based rollouts, targeted users/groups, environment-based flags,
// and composite conditions. The package is designed to be thread-safe and
// suitable for concurrent usage across your application.
//
// # Features
//
//   - Pluggable storage backends with a generic Provider interface
//   - Ready-to-use in-memory implementation for quick setup and testing
//   - Multiple rollout strategies (always on/off, targeted, percentage-based, environment-based)
//   - Support for composite conditions with logical AND/OR operations
//   - Context-based evaluation for user-specific and environment-specific flags
//   - Tag-based flag organization for easier management
//   - Thread-safe implementations for concurrent usage
//
// # Basic Usage
//
// Create a provider with initial flags:
//
//	provider, err := feature.NewMemoryProvider(
//		&feature.Flag{
//			Name:        "dark-mode",
//			Description: "Enable dark mode UI",
//			Enabled:     true,
//			Tags:        []string{"ui", "theme"},
//		},
//		&feature.Flag{
//			Name:        "beta-features",
//			Description: "Enable beta features",
//			Enabled:     true,
//			Strategy:    feature.NewTargetedStrategy(feature.TargetCriteria{
//				Groups: []string{"beta-users", "internal"},
//			}),
//		},
//	)
//	if err != nil {
//		log.Fatal(err)
//	}
//	defer provider.Close()
//
//	// Check if a feature is enabled
//	ctx := context.Background()
//	if enabled, err := provider.IsEnabled(ctx, "dark-mode"); enabled {
//		// Enable dark mode UI
//	}
//
// # User-Specific Features
//
// Add user information to context for targeted rollouts:
//
//	ctx := context.Background()
//	ctx = feature.WithUserID(ctx, "user-123")
//	ctx = feature.WithUserGroups(ctx, []string{"beta-users"})
//
//	// Check if beta features are enabled for this user
//	betaEnabled, err := provider.IsEnabled(ctx, "beta-features")
//	// Returns: true if user is in "beta-users" group
//
// # Rollout Strategies
//
// The package provides several built-in strategies:
//
//	// Always on/off strategies
//	alwaysOn := feature.NewAlwaysOnStrategy()
//	alwaysOff := feature.NewAlwaysOffStrategy()
//
//	// Environment-based strategy
//	envStrategy := feature.NewEnvironmentStrategy("dev", "staging")
//
//	// Targeted strategy with multiple criteria
//	targetedStrategy := feature.NewTargetedStrategy(feature.TargetCriteria{
//		UserIDs:    []string{"user-1", "user-2"},  // Specific users
//		Groups:     []string{"beta", "internal"},  // User groups
//		Percentage: ptr(20),                       // 20% of users
//		AllowList:  []string{"vip-1"},             // Always enabled
//		DenyList:   []string{"banned-user"},       // Never enabled
//	})
//
//	// Composite strategies
//	andStrategy := feature.NewAndStrategy(
//		envStrategy,      // Must be in dev/staging
//		targetedStrategy, // AND must match target criteria
//	)
//
//	orStrategy := feature.NewOrStrategy(
//		envStrategy,      // Either in dev/staging
//		targetedStrategy, // OR matches target criteria
//	)
//
// # Managing Flags
//
// Flags can be created, updated, and deleted dynamically:
//
//	// Create a new flag
//	err := provider.CreateFlag(ctx, &feature.Flag{
//		Name:        "new-feature",
//		Description: "A new experimental feature",
//		Enabled:     true,
//		Strategy:    feature.NewTargetedStrategy(feature.TargetCriteria{
//			Percentage: ptr(10), // 10% rollout
//		}),
//		Tags: []string{"experimental"},
//	})
//
//	// Update an existing flag
//	flag, _ := provider.GetFlag(ctx, "new-feature")
//	flag.Strategy = feature.NewTargetedStrategy(feature.TargetCriteria{
//		Percentage: ptr(50), // Increase to 50% rollout
//	})
//	err = provider.UpdateFlag(ctx, flag)
//
//	// Delete a flag
//	err = provider.DeleteFlag(ctx, "deprecated-feature")
//
//	// List all flags or filter by tags
//	allFlags, _ := provider.ListFlags(ctx)
//	uiFlags, _ := provider.ListFlags(ctx, "ui", "theme")
//
// # Context Usage
//
// The package provides type-safe context helpers for storing evaluation data:
//
//	ctx = feature.WithUserID(ctx, "user-123")           // Set user ID
//	userID, ok := feature.GetUserID(ctx)                // Get user ID
//
//	ctx = feature.WithUserGroups(ctx, []string{"beta"}) // Set groups
//	groups, ok := feature.GetUserGroups(ctx)            // Get groups
//
//	ctx = feature.WithEnvironment(ctx, "production")    // Set environment
//	env, ok := feature.GetEnvironment(ctx)              // Get environment
//
// # Error Handling
//
// The package defines several error variables for common scenarios:
//
//	feature.ErrFlagNotFound         // Requested flag doesn't exist
//	feature.ErrInvalidFlag          // Invalid flag parameters
//	feature.ErrProviderNotInitialized // Provider not properly initialized
//	feature.ErrInvalidContext       // Required context values missing
//	feature.ErrInvalidStrategy      // Strategy configuration error
//	feature.ErrOperationFailed      // General operation failure
//
// # Best Practices
//
// 1. Configuration Management:
//   - Keep flag definitions in a central location for easier maintenance
//   - Document the purpose of each flag in its description field
//   - Use meaningful names and consistent naming conventions
//
// 2. Rollout Strategy:
//   - Start with small percentage rollouts for risky features
//   - Use environment-based strategies for proper staging
//   - Leverage allow-lists for internal testing before wider rollout
//
// 3. Context Usage:
//   - Add required data to context early in your request lifecycle
//   - Standardize how user IDs and groups are populated in your application
//   - Consider creating middleware to automatically enrich context with user data
//
// 4. Error Handling:
//   - Always check errors from feature flag operations
//   - Implement graceful fallbacks when flags cannot be evaluated
//   - Log flag evaluation failures for debugging
//
// # Thread Safety
//
// All provider implementations in this package are thread-safe and can be
// safely used concurrently from multiple goroutines. The MemoryProvider uses
// read-write mutexes to ensure data consistency while minimizing lock contention
// for read operations.
//
// # Integration with HTTP Middleware
//
// For web applications, consider creating middleware to automatically populate
// context with user information:
//
//	func FeatureFlagMiddleware(provider feature.Provider) func(http.Handler) http.Handler {
//		return func(next http.Handler) http.Handler {
//			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
//				ctx := r.Context()
//
//				// Extract user info from session/JWT/etc
//				userID := getUserID(r)
//				groups := getUserGroups(r)
//
//				ctx = feature.WithUserID(ctx, userID)
//				ctx = feature.WithUserGroups(ctx, groups)
//				ctx = feature.WithEnvironment(ctx, os.Getenv("APP_ENV"))
//
//				next.ServeHTTP(w, r.WithContext(ctx))
//			})
//		}
//	}
package feature
