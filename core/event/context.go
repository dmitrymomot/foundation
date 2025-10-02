package event

import (
	"context"
	"time"
)

type eventIDCtx struct{}

// WithEventID attaches an event ID to the context.
func WithEventID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, eventIDCtx{}, id)
}

// EventID extracts the event ID from the context.
// Returns empty string if not present.
func EventID(ctx context.Context) string {
	if id, ok := ctx.Value(eventIDCtx{}).(string); ok {
		return id
	}
	return ""
}

type eventNameCtx struct{}

// WithEventName attaches an event name to the context.
func WithEventName(ctx context.Context, name string) context.Context {
	return context.WithValue(ctx, eventNameCtx{}, name)
}

// EventName extracts the event name from the context.
// Returns empty string if not present.
func EventName(ctx context.Context) string {
	if name, ok := ctx.Value(eventNameCtx{}).(string); ok {
		return name
	}
	return ""
}

type eventTimeCtx struct{}

// WithEventTime attaches the event creation time to the context.
func WithEventTime(ctx context.Context, t time.Time) context.Context {
	return context.WithValue(ctx, eventTimeCtx{}, t)
}

// EventTime extracts the event creation time from the context.
// Returns zero time if not present.
func EventTime(ctx context.Context) time.Time {
	if t, ok := ctx.Value(eventTimeCtx{}).(time.Time); ok {
		return t
	}
	return time.Time{}
}

// WithEventMeta attaches all event metadata (ID, Name, CreateAt) to the context.
func WithEventMeta(ctx context.Context, event Event) context.Context {
	ctx = WithEventID(ctx, event.ID)
	ctx = WithEventName(ctx, event.Name)
	ctx = WithEventTime(ctx, event.CreateAt)
	return ctx
}

type startProcessingAt struct{}

// WithStartProcessingTime attaches the processing start time to the context.
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
