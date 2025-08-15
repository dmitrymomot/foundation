package gokit_test

import (
	"context"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/dmitrymomot/gokit"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestContext_Deadline(t *testing.T) {
	t.Parallel()

	t.Run("with_deadline", func(t *testing.T) {
		t.Parallel()

		deadline := time.Now().Add(5 * time.Second)
		ctx, cancel := context.WithDeadline(context.Background(), deadline)
		defer cancel()

		req := httptest.NewRequest("GET", "/", nil).WithContext(ctx)
		w := httptest.NewRecorder()

		c := gokit.NewContext(w, req)

		gotDeadline, ok := c.Deadline()
		assert.True(t, ok)
		assert.WithinDuration(t, deadline, gotDeadline, time.Millisecond)
	})

	t.Run("without_deadline", func(t *testing.T) {
		t.Parallel()

		req := httptest.NewRequest("GET", "/", nil)
		w := httptest.NewRecorder()

		c := gokit.NewContext(w, req)

		_, ok := c.Deadline()
		assert.False(t, ok)
	})
}

func TestContext_Done(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	req := httptest.NewRequest("GET", "/", nil).WithContext(ctx)
	w := httptest.NewRecorder()

	c := gokit.NewContext(w, req)

	// Done channel should not be closed initially
	select {
	case <-c.Done():
		t.Fatal("Done channel should not be closed initially")
	default:
		// Expected
	}

	// Cancel the context
	cancel()

	// Done channel should be closed after cancellation
	select {
	case <-c.Done():
		// Expected
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Done channel should be closed after cancellation")
	}
}

func TestContext_Err(t *testing.T) {
	t.Parallel()

	t.Run("context_canceled", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithCancel(context.Background())
		req := httptest.NewRequest("GET", "/", nil).WithContext(ctx)
		w := httptest.NewRecorder()

		c := gokit.NewContext(w, req)

		// No error initially
		assert.NoError(t, c.Err())

		// Cancel the context
		cancel()

		// Should return context.Canceled error
		assert.Equal(t, context.Canceled, c.Err())
	})

	t.Run("context_deadline_exceeded", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
		defer cancel()

		req := httptest.NewRequest("GET", "/", nil).WithContext(ctx)
		w := httptest.NewRecorder()

		c := gokit.NewContext(w, req)

		// Wait for timeout
		time.Sleep(10 * time.Millisecond)

		// Should return context.DeadlineExceeded error
		assert.Equal(t, context.DeadlineExceeded, c.Err())
	})
}

func TestContext_Value(t *testing.T) {
	t.Parallel()

	type contextKey string
	const testKey contextKey = "test-key"
	const testValue = "test-value"

	ctx := context.WithValue(context.Background(), testKey, testValue)
	req := httptest.NewRequest("GET", "/", nil).WithContext(ctx)
	w := httptest.NewRecorder()

	c := gokit.NewContext(w, req)

	// Should retrieve the value from the request context
	value := c.Value(testKey)
	assert.Equal(t, testValue, value)

	// Should return nil for non-existent key
	assert.Nil(t, c.Value("non-existent"))
}

func TestContext_Request(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest("POST", "/test?foo=bar", nil)
	req.Header.Set("X-Test", "test-value")
	w := httptest.NewRecorder()

	c := gokit.NewContext(w, req)

	// Should return the same request
	gotReq := c.Request()
	require.NotNil(t, gotReq)
	assert.Equal(t, req, gotReq)
	assert.Equal(t, "POST", gotReq.Method)
	assert.Equal(t, "/test?foo=bar", gotReq.RequestURI)
	assert.Equal(t, "test-value", gotReq.Header.Get("X-Test"))
}
