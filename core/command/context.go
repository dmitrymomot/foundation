package command

import (
	"context"
	"time"
)

type commandIDCtx struct{}

// WithCommandID attaches a command ID to the context for tracing and correlation.
func WithCommandID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, commandIDCtx{}, id)
}

// CommandID extracts the command ID from the context.
// Returns empty string if not present.
func CommandID(ctx context.Context) string {
	if id, ok := ctx.Value(commandIDCtx{}).(string); ok {
		return id
	}
	return ""
}

type commandNameCtx struct{}

// WithCommandName attaches a command name to the context for logging and metrics.
func WithCommandName(ctx context.Context, name string) context.Context {
	return context.WithValue(ctx, commandNameCtx{}, name)
}

// CommandName extracts the command name from the context.
// Returns empty string if not present.
func CommandName(ctx context.Context) string {
	if name, ok := ctx.Value(commandNameCtx{}).(string); ok {
		return name
	}
	return ""
}

type commandTimeCtx struct{}

// WithCommandTime attaches the command creation time to the context for latency tracking.
func WithCommandTime(ctx context.Context, t time.Time) context.Context {
	return context.WithValue(ctx, commandTimeCtx{}, t)
}

// CommandTime extracts the command creation time from the context.
// Returns zero time if not present.
func CommandTime(ctx context.Context) time.Time {
	if t, ok := ctx.Value(commandTimeCtx{}).(time.Time); ok {
		return t
	}
	return time.Time{}
}

// WithCommandMeta attaches all command metadata (ID, Name, CreatedAt) to the context.
func WithCommandMeta(ctx context.Context, command Command) context.Context {
	ctx = WithCommandID(ctx, command.ID)
	ctx = WithCommandName(ctx, command.Name)
	ctx = WithCommandTime(ctx, command.CreatedAt)
	return ctx
}

type startProcessingAt struct{}

// WithStartProcessingTime attaches the processing start time to the context for handler duration metrics.
func WithStartProcessingTime(ctx context.Context, t time.Time) context.Context {
	return context.WithValue(ctx, startProcessingAt{}, t)
}

// StartProcessingTime extracts the processing start time from the context.
// Returns zero time if not present.
func StartProcessingTime(ctx context.Context) time.Time {
	if t, ok := ctx.Value(startProcessingAt{}).(time.Time); ok {
		return t
	}
	return time.Time{}
}
