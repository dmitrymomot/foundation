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
// Create a provider and check if a feature is enabled:
//
//	import (
//		"context"
//		"github.com/dmitrymomot/foundation/pkg/feature"
//	)
//
//	func main() {
//		// Create a provider with an initial flag
//		provider, err := feature.NewMemoryProvider(
//			&feature.Flag{
//				Name:        "dark-mode",
//				Description: "Enable dark mode UI",
//				Enabled:     true,
//				Tags:        []string{"ui", "theme"},
//			},
//		)
//		if err != nil {
//			panic(err)
//		}
//		defer provider.Close()
//
//		// Check if a feature is enabled
//		ctx := context.Background()
//		enabled, err := provider.IsEnabled(ctx, "dark-mode")
//		if err != nil {
//			// Handle error (e.g., flag not found)
//		}
//		if enabled {
//			// Enable dark mode UI
//		}
//	}
//
// # User-Specific Features
//
// Add user information to context for targeted rollouts:
//
//	// Create flag with group targeting
//	percentage := 25
//	flag := &feature.Flag{
//		Name:        "beta-features",
//		Description: "Enable beta features for specific users",
//		Enabled:     true,
//		Strategy: feature.NewTargetedStrategy(feature.TargetCriteria{
//			Groups:     []string{"beta-users", "internal"},
//			Percentage: &percentage, // 25% rollout
//		}),
//	}
//	provider.CreateFlag(ctx, flag)
//
//	// Add user context for evaluation
//	ctx = feature.WithUserID(ctx, "user-123")
//	ctx = feature.WithUserGroups(ctx, []string{"beta-users"})
//
//	// Check if beta features are enabled for this user
//	enabled, err := provider.IsEnabled(ctx, "beta-features")
//	// Returns: true if user is in "beta-users" group OR in the 25% rollout
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
//	percentage := 20
//	targetedStrategy := feature.NewTargetedStrategy(feature.TargetCriteria{
//		UserIDs:    []string{"user-1", "user-2"},  // Specific users
//		Groups:     []string{"beta", "internal"},  // User groups
//		Percentage: &percentage,                   // 20% of users
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
//	percentage := 10
//	err := provider.CreateFlag(ctx, &feature.Flag{
//		Name:        "new-feature",
//		Description: "A new experimental feature",
//		Enabled:     true,
//		Strategy: feature.NewTargetedStrategy(feature.TargetCriteria{
//			Percentage: &percentage, // 10% rollout
//		}),
//		Tags: []string{"experimental"},
//	})
//
//	// Update an existing flag
//	flag, err := provider.GetFlag(ctx, "new-feature")
//	if err == nil {
//		newPercentage := 50
//		flag.Strategy = feature.NewTargetedStrategy(feature.TargetCriteria{
//			Percentage: &newPercentage, // Increase to 50% rollout
//		})
//		err = provider.UpdateFlag(ctx, flag)
//	}
//
//	// Delete a flag
//	err = provider.DeleteFlag(ctx, "deprecated-feature")
//
//	// List all flags or filter by tags
//	allFlags, err := provider.ListFlags(ctx)
//	uiFlags, err := provider.ListFlags(ctx, "ui", "theme")
//
// # Context Helpers
//
// The package provides type-safe context helpers:
//
//	// Set user context data
//	ctx = feature.WithUserID(ctx, "user-123")
//	ctx = feature.WithUserGroups(ctx, []string{"beta"})
//	ctx = feature.WithEnvironment(ctx, "production")
//
//	// Retrieve user context data
//	userID, hasUserID := feature.GetUserID(ctx)
//	groups, hasGroups := feature.GetUserGroups(ctx)
//	env, hasEnv := feature.GetEnvironment(ctx)
//
// # Error Handling
//
// The package defines several error variables:
//
//	feature.ErrFlagNotFound         // Requested flag doesn't exist
//	feature.ErrInvalidFlag          // Invalid flag parameters
//	feature.ErrProviderNotInitialized // Provider not properly initialized
//	feature.ErrInvalidContext       // Required context values missing
//	feature.ErrInvalidStrategy      // Strategy configuration error
//	feature.ErrOperationFailed      // General operation failure
//
// Always check for these errors in production code:
//
//	enabled, err := provider.IsEnabled(ctx, "feature-name")
//	if err != nil {
//		if err == feature.ErrFlagNotFound {
//			// Flag doesn't exist, use default behavior
//			enabled = false
//		} else {
//			// Handle other errors appropriately
//			log.Error("Feature flag evaluation failed", "error", err)
//			enabled = false // Fail safe
//		}
//	}
//
// # Thread Safety
//
// All provider implementations are thread-safe and can be safely used
// concurrently from multiple goroutines. The MemoryProvider uses read-write
// mutexes to ensure data consistency while minimizing lock contention.
//
// # HTTP Middleware Integration
//
// For web applications, create middleware to populate context automatically:
//
//	func FeatureFlagMiddleware(next http.Handler) http.Handler {
//		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
//			ctx := r.Context()
//
//			// Extract user info from session/JWT/etc
//			if userID := getUserFromRequest(r); userID != "" {
//				ctx = feature.WithUserID(ctx, userID)
//			}
//			if groups := getGroupsFromRequest(r); len(groups) > 0 {
//				ctx = feature.WithUserGroups(ctx, groups)
//			}
//			ctx = feature.WithEnvironment(ctx, "production")
//
//			next.ServeHTTP(w, r.WithContext(ctx))
//		})
//	}
package feature
