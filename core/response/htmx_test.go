package response_test

import (
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/dmitrymomot/gokit/core/response"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWithHTMX(t *testing.T) {
	t.Parallel()

	t.Run("nil_response", func(t *testing.T) {
		t.Parallel()

		result := response.WithHTMX(nil, response.Trigger(map[string]any{"event": "test"}))
		assert.Nil(t, result)
	})

	t.Run("no_options", func(t *testing.T) {
		t.Parallel()

		baseResponse := response.String("test content")
		wrappedResponse := response.WithHTMX(baseResponse)

		req := httptest.NewRequest("GET", "/test", nil)
		w := httptest.NewRecorder()

		err := wrappedResponse(w, req)

		assert.NoError(t, err)
		assert.Equal(t, "test content", w.Body.String())
		assert.Equal(t, "text/plain; charset=utf-8", w.Header().Get("Content-Type"))
		// No HTMX headers should be set
		assert.Empty(t, w.Header().Get(response.HeaderHXTrigger))
	})

	t.Run("with_trigger", func(t *testing.T) {
		t.Parallel()

		baseResponse := response.HTML("<div>Updated</div>")
		wrappedResponse := response.WithHTMX(
			baseResponse,
			response.Trigger(map[string]any{
				"showMessage": "Item saved successfully",
				"updateCount": 42,
			}),
		)

		req := httptest.NewRequest("GET", "/test", nil)
		w := httptest.NewRecorder()

		err := wrappedResponse(w, req)

		assert.NoError(t, err)
		assert.Equal(t, "<div>Updated</div>", w.Body.String())

		// Check trigger header
		triggerHeader := w.Header().Get(response.HeaderHXTrigger)
		assert.NotEmpty(t, triggerHeader)

		var triggerData map[string]any
		err = json.Unmarshal([]byte(triggerHeader), &triggerData)
		require.NoError(t, err)
		assert.Equal(t, "Item saved successfully", triggerData["showMessage"])
		assert.Equal(t, float64(42), triggerData["updateCount"])
	})

	t.Run("with_trigger_event", func(t *testing.T) {
		t.Parallel()

		baseResponse := response.NoContent()
		wrappedResponse := response.WithHTMX(
			baseResponse,
			response.TriggerEvent("userUpdated", map[string]any{
				"id":   123,
				"name": "John Doe",
			}),
			response.TriggerEvent("refreshList", true),
		)

		req := httptest.NewRequest("POST", "/test", nil)
		w := httptest.NewRecorder()

		err := wrappedResponse(w, req)

		assert.NoError(t, err)

		// Check merged trigger events
		triggerHeader := w.Header().Get(response.HeaderHXTrigger)
		assert.NotEmpty(t, triggerHeader)

		var triggerData map[string]any
		err = json.Unmarshal([]byte(triggerHeader), &triggerData)
		require.NoError(t, err)

		userData, ok := triggerData["userUpdated"].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, float64(123), userData["id"])
		assert.Equal(t, "John Doe", userData["name"])
		assert.Equal(t, true, triggerData["refreshList"])
	})

	t.Run("with_location_string", func(t *testing.T) {
		t.Parallel()

		baseResponse := response.NoContent()
		wrappedResponse := response.WithHTMX(
			baseResponse,
			response.Location("/dashboard"),
		)

		req := httptest.NewRequest("POST", "/test", nil)
		w := httptest.NewRecorder()

		err := wrappedResponse(w, req)

		assert.NoError(t, err)
		assert.Equal(t, "/dashboard", w.Header().Get(response.HeaderHXLocation))
	})

	t.Run("with_location_object", func(t *testing.T) {
		t.Parallel()

		locationObj := map[string]any{
			"path":   "/users/123",
			"target": "#main-content",
			"swap":   "innerHTML",
			"values": map[string]any{
				"userId": 123,
			},
		}

		baseResponse := response.NoContent()
		wrappedResponse := response.WithHTMX(
			baseResponse,
			response.Location(locationObj),
		)

		req := httptest.NewRequest("POST", "/test", nil)
		w := httptest.NewRecorder()

		err := wrappedResponse(w, req)

		assert.NoError(t, err)

		locationHeader := w.Header().Get(response.HeaderHXLocation)
		assert.NotEmpty(t, locationHeader)

		var locationData map[string]any
		err = json.Unmarshal([]byte(locationHeader), &locationData)
		require.NoError(t, err)
		assert.Equal(t, "/users/123", locationData["path"])
		assert.Equal(t, "#main-content", locationData["target"])
	})

	t.Run("with_push_url", func(t *testing.T) {
		t.Parallel()

		baseResponse := response.HTML("<div>Content</div>")
		wrappedResponse := response.WithHTMX(
			baseResponse,
			response.PushURL("/new-url"),
		)

		req := httptest.NewRequest("GET", "/test", nil)
		w := httptest.NewRecorder()

		err := wrappedResponse(w, req)

		assert.NoError(t, err)
		assert.Equal(t, "/new-url", w.Header().Get(response.HeaderHXPushURL))
	})

	t.Run("with_replace_url", func(t *testing.T) {
		t.Parallel()

		baseResponse := response.HTML("<div>Content</div>")
		wrappedResponse := response.WithHTMX(
			baseResponse,
			response.ReplaceURL("/replaced-url"),
		)

		req := httptest.NewRequest("GET", "/test", nil)
		w := httptest.NewRecorder()

		err := wrappedResponse(w, req)

		assert.NoError(t, err)
		assert.Equal(t, "/replaced-url", w.Header().Get(response.HeaderHXReplaceURL))
	})

	t.Run("with_htmx_redirect", func(t *testing.T) {
		t.Parallel()

		baseResponse := response.NoContent()
		wrappedResponse := response.WithHTMX(
			baseResponse,
			response.HTMXRedirect("/redirect-target"),
		)

		req := httptest.NewRequest("POST", "/test", nil)
		w := httptest.NewRecorder()

		err := wrappedResponse(w, req)

		assert.NoError(t, err)
		assert.Equal(t, "/redirect-target", w.Header().Get(response.HeaderHXRedirect))
	})

	t.Run("with_refresh", func(t *testing.T) {
		t.Parallel()

		baseResponse := response.NoContent()
		wrappedResponse := response.WithHTMX(
			baseResponse,
			response.Refresh(),
		)

		req := httptest.NewRequest("POST", "/test", nil)
		w := httptest.NewRecorder()

		err := wrappedResponse(w, req)

		assert.NoError(t, err)
		assert.Equal(t, "true", w.Header().Get(response.HeaderHXRefresh))
	})

	t.Run("with_reswap", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name      string
			method    string
			modifiers []string
			expected  string
		}{
			{
				name:     "simple_reswap",
				method:   "outerHTML",
				expected: "outerHTML",
			},
			{
				name:      "reswap_with_timing",
				method:    "innerHTML",
				modifiers: []string{"swap:500ms"},
				expected:  "innerHTML swap:500ms",
			},
			{
				name:      "reswap_with_multiple_modifiers",
				method:    "beforeend",
				modifiers: []string{"swap:200ms", "settle:1s", "scroll:top"},
				expected:  "beforeend swap:200ms settle:1s scroll:top",
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				baseResponse := response.HTML("<span>New content</span>")
				wrappedResponse := response.WithHTMX(
					baseResponse,
					response.Reswap(tt.method, tt.modifiers...),
				)

				req := httptest.NewRequest("GET", "/test", nil)
				w := httptest.NewRecorder()

				err := wrappedResponse(w, req)

				assert.NoError(t, err)
				assert.Equal(t, tt.expected, w.Header().Get(response.HeaderHXReswap))
			})
		}
	})

	t.Run("with_retarget", func(t *testing.T) {
		t.Parallel()

		baseResponse := response.HTML("<div>Content</div>")
		wrappedResponse := response.WithHTMX(
			baseResponse,
			response.Retarget("#different-element"),
		)

		req := httptest.NewRequest("GET", "/test", nil)
		w := httptest.NewRecorder()

		err := wrappedResponse(w, req)

		assert.NoError(t, err)
		assert.Equal(t, "#different-element", w.Header().Get(response.HeaderHXRetarget))
	})

	t.Run("with_reselect", func(t *testing.T) {
		t.Parallel()

		baseResponse := response.HTML("<div><span>Part to select</span></div>")
		wrappedResponse := response.WithHTMX(
			baseResponse,
			response.Reselect("span"),
		)

		req := httptest.NewRequest("GET", "/test", nil)
		w := httptest.NewRecorder()

		err := wrappedResponse(w, req)

		assert.NoError(t, err)
		assert.Equal(t, "span", w.Header().Get(response.HeaderHXReselect))
	})

	t.Run("trigger_after_swap", func(t *testing.T) {
		t.Parallel()

		baseResponse := response.HTML("<div>Content</div>")
		wrappedResponse := response.WithHTMX(
			baseResponse,
			response.TriggerAfterSwap(map[string]any{
				"fadeIn": true,
				"focus":  "#input-field",
			}),
		)

		req := httptest.NewRequest("GET", "/test", nil)
		w := httptest.NewRecorder()

		err := wrappedResponse(w, req)

		assert.NoError(t, err)

		triggerHeader := w.Header().Get(response.HeaderHXTriggerAfterSwap)
		assert.NotEmpty(t, triggerHeader)

		var triggerData map[string]any
		err = json.Unmarshal([]byte(triggerHeader), &triggerData)
		require.NoError(t, err)
		assert.Equal(t, true, triggerData["fadeIn"])
		assert.Equal(t, "#input-field", triggerData["focus"])
	})

	t.Run("trigger_after_settle", func(t *testing.T) {
		t.Parallel()

		baseResponse := response.HTML("<div>Content</div>")
		wrappedResponse := response.WithHTMX(
			baseResponse,
			response.TriggerAfterSettle(map[string]any{
				"animationComplete": true,
				"logEvent":          "settled",
			}),
		)

		req := httptest.NewRequest("GET", "/test", nil)
		w := httptest.NewRecorder()

		err := wrappedResponse(w, req)

		assert.NoError(t, err)

		triggerHeader := w.Header().Get(response.HeaderHXTriggerAfterSettle)
		assert.NotEmpty(t, triggerHeader)

		var triggerData map[string]any
		err = json.Unmarshal([]byte(triggerHeader), &triggerData)
		require.NoError(t, err)
		assert.Equal(t, true, triggerData["animationComplete"])
		assert.Equal(t, "settled", triggerData["logEvent"])
	})

	t.Run("multiple_options_combined", func(t *testing.T) {
		t.Parallel()

		baseResponse := response.HTML("<div>Updated content</div>")
		wrappedResponse := response.WithHTMX(
			baseResponse,
			response.Trigger(map[string]any{"saved": true}),
			response.PushURL("/items/123"),
			response.Reswap("outerHTML", "swap:200ms"),
			response.Retarget("#item-123"),
			response.TriggerAfterSettle(map[string]any{"highlight": "#item-123"}),
		)

		req := httptest.NewRequest("POST", "/test", nil)
		w := httptest.NewRecorder()

		err := wrappedResponse(w, req)

		assert.NoError(t, err)
		assert.Equal(t, "<div>Updated content</div>", w.Body.String())

		// Check all headers are set
		assert.NotEmpty(t, w.Header().Get(response.HeaderHXTrigger))
		assert.Equal(t, "/items/123", w.Header().Get(response.HeaderHXPushURL))
		assert.Equal(t, "outerHTML swap:200ms", w.Header().Get(response.HeaderHXReswap))
		assert.Equal(t, "#item-123", w.Header().Get(response.HeaderHXRetarget))
		assert.NotEmpty(t, w.Header().Get(response.HeaderHXTriggerAfterSettle))
	})

	t.Run("with_other_decorators", func(t *testing.T) {
		t.Parallel()

		baseResponse := response.JSON(map[string]any{"status": "ok"})
		wrappedResponse := response.WithHeaders(
			response.WithHTMX(
				baseResponse,
				response.Trigger(map[string]any{"dataLoaded": true}),
			),
			map[string]string{
				"X-Custom-Header": "custom-value",
			},
		)

		req := httptest.NewRequest("GET", "/test", nil)
		w := httptest.NewRecorder()

		err := wrappedResponse(w, req)

		assert.NoError(t, err)
		assert.Equal(t, "custom-value", w.Header().Get("X-Custom-Header"))
		assert.NotEmpty(t, w.Header().Get(response.HeaderHXTrigger))
		assert.Equal(t, "application/json; charset=utf-8", w.Header().Get("Content-Type"))
	})
}

func TestIsHTMXRequest(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		headers  map[string]string
		expected bool
	}{
		{
			name:     "htmx_request",
			headers:  map[string]string{response.HeaderHXRequest: "true"},
			expected: true,
		},
		{
			name:     "non_htmx_request",
			headers:  map[string]string{},
			expected: false,
		},
		{
			name:     "htmx_request_false_value",
			headers:  map[string]string{response.HeaderHXRequest: "false"},
			expected: false,
		},
		{
			name:     "htmx_request_invalid_value",
			headers:  map[string]string{response.HeaderHXRequest: "yes"},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequest("GET", "/test", nil)
			for k, v := range tt.headers {
				req.Header.Set(k, v)
			}

			result := response.IsHTMXRequest(req)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestIsHTMXBoosted(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		headers  map[string]string
		expected bool
	}{
		{
			name:     "boosted_request",
			headers:  map[string]string{response.HeaderHXBoosted: "true"},
			expected: true,
		},
		{
			name:     "non_boosted_request",
			headers:  map[string]string{},
			expected: false,
		},
		{
			name:     "boosted_false",
			headers:  map[string]string{response.HeaderHXBoosted: "false"},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequest("GET", "/test", nil)
			for k, v := range tt.headers {
				req.Header.Set(k, v)
			}

			result := response.IsHTMXBoosted(req)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetHTMXHeaders(t *testing.T) {
	t.Parallel()

	t.Run("full_headers", func(t *testing.T) {
		t.Parallel()

		req := httptest.NewRequest("POST", "/test", nil)
		req.Header.Set(response.HeaderHXRequest, "true")
		req.Header.Set(response.HeaderHXBoosted, "true")
		req.Header.Set(response.HeaderHXCurrentURL, "https://example.com/page")
		req.Header.Set(response.HeaderHXHistoryRestoreRequest, "true")
		req.Header.Set(response.HeaderHXPrompt, "user input")
		req.Header.Set(response.HeaderHXTarget, "main-content")
		req.Header.Set(response.HeaderHXTriggerName, "submit-button")
		req.Header.Set(response.HeaderHXTriggerHeader, "form-1")

		headers := response.GetHTMXHeaders(req)

		assert.True(t, headers.Request)
		assert.True(t, headers.Boosted)
		assert.Equal(t, "https://example.com/page", headers.CurrentURL)
		assert.True(t, headers.HistoryRestore)
		assert.Equal(t, "user input", headers.Prompt)
		assert.Equal(t, "main-content", headers.Target)
		assert.Equal(t, "submit-button", headers.TriggerName)
		assert.Equal(t, "form-1", headers.Trigger)
	})

	t.Run("partial_headers", func(t *testing.T) {
		t.Parallel()

		req := httptest.NewRequest("GET", "/test", nil)
		req.Header.Set(response.HeaderHXRequest, "true")
		req.Header.Set(response.HeaderHXTarget, "sidebar")

		headers := response.GetHTMXHeaders(req)

		assert.True(t, headers.Request)
		assert.False(t, headers.Boosted)
		assert.Empty(t, headers.CurrentURL)
		assert.False(t, headers.HistoryRestore)
		assert.Empty(t, headers.Prompt)
		assert.Equal(t, "sidebar", headers.Target)
		assert.Empty(t, headers.TriggerName)
		assert.Empty(t, headers.Trigger)
	})

	t.Run("no_headers", func(t *testing.T) {
		t.Parallel()

		req := httptest.NewRequest("GET", "/test", nil)

		headers := response.GetHTMXHeaders(req)

		assert.False(t, headers.Request)
		assert.False(t, headers.Boosted)
		assert.Empty(t, headers.CurrentURL)
		assert.False(t, headers.HistoryRestore)
		assert.Empty(t, headers.Prompt)
		assert.Empty(t, headers.Target)
		assert.Empty(t, headers.TriggerName)
		assert.Empty(t, headers.Trigger)
	})
}
