package event

import (
	"encoding/json"
	"fmt"
	"reflect"
)

// getEventName extracts the type name from an event value, unwrapping any pointer types.
// Note: Returns only the bare type name without package path (e.g., "UserCreated").
// This means structs with identical names from different packages will have the same event name,
// potentially causing handler collisions.
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
