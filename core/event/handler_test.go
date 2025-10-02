package event_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/dmitrymomot/foundation/core/event"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test payload types for handler tests
type UserCreated struct {
	UserID string
	Email  string
}

type OrderPlaced struct {
	OrderID   string
	UserID    string
	Total     float64
	Amount    float64 // For compatibility with other test files
	ProductID string  // For compatibility with integration tests
	Items     []string
	Timestamp time.Time
}

type NestedEvent struct {
	ID       string
	Metadata map[string]any
	Related  *RelatedData
}

type RelatedData struct {
	Type  string
	Value int
}

type SimpleEvent struct {
	Message string
}

// Types used in other test files

type NestedPayload struct {
	ID   string
	Data struct {
		Name  string
		Items []string
	}
}

type UnmarshalableType struct {
	Channel chan int // channels cannot be marshaled to JSON
}

type PaymentProcessed struct {
	PaymentID string
	Amount    float64
	Status    string
}

type EmailSent struct {
	To      string
	Subject string
}

type NotificationSent struct {
	UserID  string
	Message string
}

// TestNewEvent_WithStructPayload verifies event creation with struct payload
func TestNewEvent_WithStructPayload(t *testing.T) {
	t.Parallel()

	payload := UserCreated{
		UserID: "user-123",
		Email:  "user@example.com",
	}

	evt := event.NewEvent(payload)

	require.NotEmpty(t, evt.ID, "Event ID should not be empty")
	assert.Equal(t, "UserCreated", evt.Name, "Event name should match type name")
	assert.Equal(t, payload, evt.Payload, "Payload should match input")
	assert.WithinDuration(t, time.Now(), evt.CreatedAt, time.Second, "CreatedAt should be recent")

	// Verify ID is valid UUID
	_, err := uuid.Parse(evt.ID)
	assert.NoError(t, err, "Event ID should be valid UUID")
}

// TestNewEvent_WithPointerPayload verifies event creation with pointer payload
func TestNewEvent_WithPointerPayload(t *testing.T) {
	t.Parallel()

	payload := &OrderPlaced{
		OrderID:   "order-456",
		UserID:    "user-123",
		Total:     99.99,
		Items:     []string{"item1", "item2"},
		Timestamp: time.Now(),
	}

	evt := event.NewEvent(payload)

	require.NotEmpty(t, evt.ID)
	// Pointer types unwrap to base type name
	assert.Equal(t, "OrderPlaced", evt.Name, "Event name should unwrap pointer type")
	assert.Equal(t, payload, evt.Payload)
	assert.WithinDuration(t, time.Now(), evt.CreatedAt, time.Second)
}

// TestNewEvent_WithNestedPayload verifies event creation with nested structures
func TestNewEvent_WithNestedPayload(t *testing.T) {
	t.Parallel()

	payload := NestedEvent{
		ID: "nested-789",
		Metadata: map[string]any{
			"source":  "api",
			"version": 2,
		},
		Related: &RelatedData{
			Type:  "reference",
			Value: 42,
		},
	}

	evt := event.NewEvent(payload)

	require.NotEmpty(t, evt.ID)
	assert.Equal(t, "NestedEvent", evt.Name)
	assert.Equal(t, payload, evt.Payload)

	// Verify nested data is preserved
	typed, ok := evt.Payload.(NestedEvent)
	require.True(t, ok, "Should be able to type assert payload")
	assert.Equal(t, "nested-789", typed.ID)
	require.NotNil(t, typed.Related)
	assert.Equal(t, "reference", typed.Related.Type)
	assert.Equal(t, 42, typed.Related.Value)
}

// TestNewEvent_UniqueIDs verifies each event gets unique ID
func TestNewEvent_UniqueIDs(t *testing.T) {
	t.Parallel()

	events := make([]event.Event, 100)
	ids := make(map[string]bool)

	for i := 0; i < 100; i++ {
		events[i] = event.NewEvent(SimpleEvent{Message: "test"})
		ids[events[i].ID] = true
	}

	assert.Len(t, ids, 100, "All event IDs should be unique")
}

// TestNewEvent_AllFieldsPopulated verifies all event fields are set
func TestNewEvent_AllFieldsPopulated(t *testing.T) {
	t.Parallel()

	evt := event.NewEvent(UserCreated{UserID: "123", Email: "test@test.com"})

	assert.NotEmpty(t, evt.ID, "ID should be populated")
	assert.NotEmpty(t, evt.Name, "Name should be populated")
	assert.NotNil(t, evt.Payload, "Payload should be populated")
	assert.False(t, evt.CreatedAt.IsZero(), "CreatedAt should be populated")
}

// TestNewHandlerFunc_AutomaticEventName verifies handler with auto-derived name
func TestNewHandlerFunc_AutomaticEventName(t *testing.T) {
	t.Parallel()

	var capturedPayload UserCreated
	handler := event.NewHandlerFunc(func(ctx context.Context, evt UserCreated) error {
		capturedPayload = evt
		return nil
	})

	assert.Equal(t, "UserCreated", handler.EventName(), "Event name should be auto-derived from type parameter")

	// Test handler execution
	payload := UserCreated{UserID: "user-123", Email: "user@example.com"}
	err := handler.Handle(context.Background(), payload)

	require.NoError(t, err)
	assert.Equal(t, payload, capturedPayload, "Handler should receive correct payload")
}

// TestNewHandlerFunc_WithPointerType verifies handler with pointer type parameter
func TestNewHandlerFunc_WithPointerType(t *testing.T) {
	t.Parallel()

	var capturedPayload *OrderPlaced
	handler := event.NewHandlerFunc(func(ctx context.Context, evt *OrderPlaced) error {
		capturedPayload = evt
		return nil
	})

	assert.Equal(t, "OrderPlaced", handler.EventName(), "Event name should unwrap pointer type")

	// Test with pointer payload
	payload := &OrderPlaced{
		OrderID: "order-123",
		UserID:  "user-456",
		Total:   199.99,
	}
	err := handler.Handle(context.Background(), payload)

	require.NoError(t, err)
	assert.Equal(t, payload, capturedPayload)
}

// TestNewHandler_ExplicitEventName verifies handler with explicit name
func TestNewHandler_ExplicitEventName(t *testing.T) {
	t.Parallel()

	customName := "user.registration.completed"
	var capturedPayload UserCreated

	handler := event.NewHandler(customName, func(ctx context.Context, evt UserCreated) error {
		capturedPayload = evt
		return nil
	})

	assert.Equal(t, customName, handler.EventName(), "Handler should use explicit event name")

	// Test handler execution
	payload := UserCreated{UserID: "user-789", Email: "custom@example.com"}
	err := handler.Handle(context.Background(), payload)

	require.NoError(t, err)
	assert.Equal(t, payload, capturedPayload)
}

// TestHandler_WithContext verifies handler receives context correctly
func TestHandler_WithContext(t *testing.T) {
	t.Parallel()

	type contextKey string
	const testKey contextKey = "test"

	var receivedValue string
	handler := event.NewHandlerFunc(func(ctx context.Context, evt SimpleEvent) error {
		if val := ctx.Value(testKey); val != nil {
			receivedValue = val.(string)
		}
		return nil
	})

	ctx := context.WithValue(context.Background(), testKey, "test-value")
	err := handler.Handle(ctx, SimpleEvent{Message: "test"})

	require.NoError(t, err)
	assert.Equal(t, "test-value", receivedValue, "Handler should receive context values")
}

// TestHandler_WithJSONPayload verifies handler can process JSON-marshaled payload
func TestHandler_WithJSONPayload(t *testing.T) {
	t.Parallel()

	var capturedPayload OrderPlaced
	handler := event.NewHandlerFunc(func(ctx context.Context, evt OrderPlaced) error {
		capturedPayload = evt
		return nil
	})

	// Simulate payload as JSON bytes (as it would come from event bus)
	original := OrderPlaced{
		OrderID: "order-999",
		UserID:  "user-111",
		Total:   299.99,
		Items:   []string{"laptop", "mouse"},
	}
	jsonPayload, err := json.Marshal(original)
	require.NoError(t, err)

	err = handler.Handle(context.Background(), jsonPayload)

	require.NoError(t, err)
	assert.Equal(t, original.OrderID, capturedPayload.OrderID)
	assert.Equal(t, original.UserID, capturedPayload.UserID)
	assert.Equal(t, original.Total, capturedPayload.Total)
	assert.Equal(t, original.Items, capturedPayload.Items)
}

// TestHandler_TypeMismatch verifies error on wrong payload type
func TestHandler_TypeMismatch(t *testing.T) {
	t.Parallel()

	handler := event.NewHandlerFunc(func(ctx context.Context, evt UserCreated) error {
		return nil
	})

	// Try to pass wrong type
	wrongPayload := OrderPlaced{OrderID: "order-123", UserID: "user-456"}
	err := handler.Handle(context.Background(), wrongPayload)

	require.Error(t, err, "Handler should return error for type mismatch")
	assert.Contains(t, err.Error(), "unexpected payload type", "Error should indicate type mismatch")
}

// TestHandler_InvalidJSONPayload verifies error on malformed JSON
func TestHandler_InvalidJSONPayload(t *testing.T) {
	t.Parallel()

	handler := event.NewHandlerFunc(func(ctx context.Context, evt UserCreated) error {
		return nil
	})

	// Invalid JSON
	invalidJSON := []byte(`{"UserID": "123", "Email": invalid}`)
	err := handler.Handle(context.Background(), invalidJSON)

	require.Error(t, err, "Handler should return error for invalid JSON")
	assert.Contains(t, err.Error(), "failed to unmarshal event", "Error should indicate unmarshal failure")
}

// TestHandler_WithNestedPayload verifies handler processes nested structures
func TestHandler_WithNestedPayload(t *testing.T) {
	t.Parallel()

	var capturedPayload NestedEvent
	handler := event.NewHandlerFunc(func(ctx context.Context, evt NestedEvent) error {
		capturedPayload = evt
		return nil
	})

	payload := NestedEvent{
		ID: "nested-001",
		Metadata: map[string]any{
			"priority": "high",
			"retries":  3,
		},
		Related: &RelatedData{
			Type:  "dependency",
			Value: 100,
		},
	}

	// Test with direct payload
	err := handler.Handle(context.Background(), payload)
	require.NoError(t, err)
	assert.Equal(t, payload.ID, capturedPayload.ID)
	require.NotNil(t, capturedPayload.Related)
	assert.Equal(t, 100, capturedPayload.Related.Value)

	// Test with JSON payload
	jsonPayload, err := json.Marshal(payload)
	require.NoError(t, err)

	err = handler.Handle(context.Background(), jsonPayload)
	require.NoError(t, err)
	assert.Equal(t, payload.ID, capturedPayload.ID)
}

// TestHandler_PointerVsValue verifies handlers work with both pointer and value types
func TestHandler_PointerVsValue(t *testing.T) {
	t.Parallel()

	t.Run("value receiver handler with value payload", func(t *testing.T) {
		t.Parallel()

		var captured UserCreated
		handler := event.NewHandlerFunc(func(ctx context.Context, evt UserCreated) error {
			captured = evt
			return nil
		})

		payload := UserCreated{UserID: "123", Email: "test@test.com"}
		err := handler.Handle(context.Background(), payload)

		require.NoError(t, err)
		assert.Equal(t, payload, captured)
	})

	t.Run("pointer receiver handler with pointer payload", func(t *testing.T) {
		t.Parallel()

		var captured *UserCreated
		handler := event.NewHandlerFunc(func(ctx context.Context, evt *UserCreated) error {
			captured = evt
			return nil
		})

		payload := &UserCreated{UserID: "456", Email: "pointer@test.com"}
		err := handler.Handle(context.Background(), payload)

		require.NoError(t, err)
		assert.Equal(t, payload, captured)
	})

	t.Run("pointer handler with JSON payload", func(t *testing.T) {
		t.Parallel()

		var captured *UserCreated
		handler := event.NewHandlerFunc(func(ctx context.Context, evt *UserCreated) error {
			captured = evt
			return nil
		})

		original := UserCreated{UserID: "789", Email: "json@test.com"}
		jsonPayload, err := json.Marshal(original)
		require.NoError(t, err)

		err = handler.Handle(context.Background(), jsonPayload)

		require.NoError(t, err)
		require.NotNil(t, captured)
		assert.Equal(t, original.UserID, captured.UserID)
		assert.Equal(t, original.Email, captured.Email)
	})
}

// TestHandler_ConcurrentExecution verifies handlers are safe for concurrent use
func TestHandler_ConcurrentExecution(t *testing.T) {
	t.Parallel()

	processed := make(chan UserCreated, 100)
	handler := event.NewHandlerFunc(func(ctx context.Context, evt UserCreated) error {
		processed <- evt
		return nil
	})

	// Execute handler concurrently
	const numGoroutines = 50
	done := make(chan bool, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			payload := UserCreated{
				UserID: string(rune(id)),
				Email:  "concurrent@test.com",
			}
			err := handler.Handle(context.Background(), payload)
			require.NoError(t, err)
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < numGoroutines; i++ {
		<-done
	}
	close(processed)

	// Verify all events were processed
	count := 0
	for range processed {
		count++
	}
	assert.Equal(t, numGoroutines, count, "All events should be processed")
}

// TestHandler_MultipleHandlersSameEventType verifies multiple handlers can be created for same type
func TestHandler_MultipleHandlersSameEventType(t *testing.T) {
	t.Parallel()

	var captured1, captured2 UserCreated

	handler1 := event.NewHandlerFunc(func(ctx context.Context, evt UserCreated) error {
		captured1 = evt
		return nil
	})

	handler2 := event.NewHandlerFunc(func(ctx context.Context, evt UserCreated) error {
		captured2 = evt
		return nil
	})

	// Both handlers should have same event name
	assert.Equal(t, handler1.EventName(), handler2.EventName())

	payload := UserCreated{UserID: "multi-123", Email: "multi@test.com"}

	// Both handlers should process the same payload independently
	err1 := handler1.Handle(context.Background(), payload)
	err2 := handler2.Handle(context.Background(), payload)

	require.NoError(t, err1)
	require.NoError(t, err2)
	assert.Equal(t, payload, captured1)
	assert.Equal(t, payload, captured2)
}

// TestEventName_Derivation verifies event name derivation from various types
func TestEventName_Derivation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		createEvent  func() event.Event
		expectedName string
	}{
		{
			name: "simple struct",
			createEvent: func() event.Event {
				return event.NewEvent(SimpleEvent{Message: "test"})
			},
			expectedName: "SimpleEvent",
		},
		{
			name: "complex struct",
			createEvent: func() event.Event {
				return event.NewEvent(OrderPlaced{OrderID: "123", UserID: "456"})
			},
			expectedName: "OrderPlaced",
		},
		{
			name: "nested struct",
			createEvent: func() event.Event {
				return event.NewEvent(NestedEvent{ID: "789"})
			},
			expectedName: "NestedEvent",
		},
		{
			name: "pointer to struct",
			createEvent: func() event.Event {
				return event.NewEvent(&UserCreated{UserID: "ptr", Email: "ptr@test.com"})
			},
			expectedName: "UserCreated",
		},
		{
			name: "pointer to complex struct",
			createEvent: func() event.Event {
				return event.NewEvent(&OrderPlaced{OrderID: "ptr-order"})
			},
			expectedName: "OrderPlaced",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			evt := tt.createEvent()
			assert.Equal(t, tt.expectedName, evt.Name, "Event name should match expected")
		})
	}
}
