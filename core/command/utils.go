package command

import (
	"encoding/json"
	"fmt"
	"reflect"
)

// getCommandName extracts the type name from a command value, unwrapping any pointer types.
//
// DESIGN DECISION: Returns only the bare type name without package path (e.g., "CreateUser").
// This is intentional for simplicity in micro-SaaS applications where package namespacing
// is not typically needed. Users must ensure unique command type names across their codebase
// to avoid handler collisions. This trade-off favors simplicity over package isolation.
//
// Example: Both users.CreateUser and billing.CreateUser would resolve to "CreateUser"
// and trigger the same handler. Use distinct type names if this is not desired.
func getCommandName(v any) string {
	t := reflect.TypeOf(v)

	for t != nil && t.Kind() == reflect.Pointer {
		t = t.Elem()
	}

	return t.Name()
}

// unmarshalPayload converts a payload of unknown type to the concrete command type T.
// Handles three scenarios: direct type match, raw JSON bytes, and map[string]any from JSON unmarshaling.
// The map[string]any case occurs when Command.Payload is deserialized from JSON without type information.
func unmarshalPayload[T any](payload any) (T, error) {
	var zero T

	if v, ok := payload.(T); ok {
		return v, nil
	}

	if data, ok := payload.([]byte); ok {
		var cmd T
		if err := json.Unmarshal(data, &cmd); err != nil {
			return zero, fmt.Errorf("failed to unmarshal command: %w", err)
		}
		return cmd, nil
	}

	if m, ok := payload.(map[string]any); ok {
		data, err := json.Marshal(m)
		if err != nil {
			return zero, fmt.Errorf("failed to marshal map payload: %w", err)
		}
		var cmd T
		if err := json.Unmarshal(data, &cmd); err != nil {
			return zero, fmt.Errorf("failed to unmarshal map payload: %w", err)
		}
		return cmd, nil
	}

	return zero, fmt.Errorf("unsupported payload type %T: expected %T, []byte, or map[string]any", payload, zero)
}
