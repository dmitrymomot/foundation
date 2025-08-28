package feature

import "context"

// Unexported context key types for type-safe context values.
// These follow the gokit middleware pattern of using empty structs
// as context keys to prevent collisions and ensure type safety.

type userIDCtxKey struct{}
type userGroupsCtxKey struct{}
type environmentCtxKey struct{}
type flagNameCtxKey struct{}

// WithUserID returns a new context with the user ID set.
// The user ID is used by targeted strategies to enable features for specific users.
func WithUserID(ctx context.Context, userID string) context.Context {
	return context.WithValue(ctx, userIDCtxKey{}, userID)
}

// GetUserID retrieves the user ID from the context.
// Returns the user ID and a boolean indicating whether it was found.
func GetUserID(ctx context.Context) (string, bool) {
	userID, ok := ctx.Value(userIDCtxKey{}).(string)
	return userID, ok
}

// WithUserGroups returns a new context with the user groups set.
// User groups are used by targeted strategies to enable features for specific groups.
func WithUserGroups(ctx context.Context, groups []string) context.Context {
	return context.WithValue(ctx, userGroupsCtxKey{}, groups)
}

// GetUserGroups retrieves the user groups from the context.
// Returns the groups and a boolean indicating whether they were found.
func GetUserGroups(ctx context.Context) ([]string, bool) {
	groups, ok := ctx.Value(userGroupsCtxKey{}).([]string)
	return groups, ok
}

// WithEnvironment returns a new context with the environment set.
// The environment is used by environment strategies to enable features in specific environments.
func WithEnvironment(ctx context.Context, env string) context.Context {
	return context.WithValue(ctx, environmentCtxKey{}, env)
}

// GetEnvironment retrieves the environment from the context.
// Returns the environment and a boolean indicating whether it was found.
func GetEnvironment(ctx context.Context) (string, bool) {
	env, ok := ctx.Value(environmentCtxKey{}).(string)
	return env, ok
}

// WithFlagName returns a new context with the flag name set.
// This is used internally to ensure percentage-based rollouts are independent per flag.
func WithFlagName(ctx context.Context, flagName string) context.Context {
	return context.WithValue(ctx, flagNameCtxKey{}, flagName)
}

// GetFlagName retrieves the flag name from the context.
// Returns the flag name and a boolean indicating whether it was found.
// This is primarily used internally for hash calculations in percentage rollouts.
func GetFlagName(ctx context.Context) (string, bool) {
	flagName, ok := ctx.Value(flagNameCtxKey{}).(string)
	return flagName, ok
}
