package event

import (
	"encoding/json"
	"fmt"
	"reflect"
)

// getEventName extracts the type name from an event value, unwrapping any pointer types.
//
// DESIGN DECISION: Returns only the bare type name without package path (e.g., "UserCreated").
// This is intentional for simplicity in micro-SaaS applications where package namespacing
// is not typically needed. Users must ensure unique event type names across their codebase
// to avoid handler collisions. This trade-off favors simplicity over package isolation.
//
// Example: Both users.UserCreated and billing.UserCreated would resolve to "UserCreated"
// and trigger the same handlers. Use distinct type names if this is not desired.
func getEventName(v any) string {
	t := reflect.TypeOf(v)

	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}

	return t.Name()
}

func unmarshalPayload[T any](payload any) (T, error) {
	var zero T

	if v, ok := payload.(T); ok {
		return v, nil
	}

	if data, ok := payload.([]byte); ok {
		var evt T
		if err := json.Unmarshal(data, &evt); err != nil {
			return zero, fmt.Errorf("failed to unmarshal event: %w", err)
		}
		return evt, nil
	}

	return zero, fmt.Errorf("unexpected payload type: %T", payload)
}
