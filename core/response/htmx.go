package response

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/dmitrymomot/gokit/core/handler"
)

// HTMX Response Headers - sent by server to control HTMX behavior
const (
	HeaderHXLocation           = "HX-Location"
	HeaderHXPushURL            = "HX-Push-Url"
	HeaderHXRedirect           = "HX-Redirect"
	HeaderHXRefresh            = "HX-Refresh"
	HeaderHXReplaceURL         = "HX-Replace-Url"
	HeaderHXReswap             = "HX-Reswap"
	HeaderHXRetarget           = "HX-Retarget"
	HeaderHXReselect           = "HX-Reselect"
	HeaderHXTrigger            = "HX-Trigger"
	HeaderHXTriggerAfterSwap   = "HX-Trigger-After-Swap"
	HeaderHXTriggerAfterSettle = "HX-Trigger-After-Settle"
)

// HTMX Request Headers - sent by HTMX client to server
const (
	HeaderHXRequest               = "HX-Request"
	HeaderHXBoosted               = "HX-Boosted"
	HeaderHXCurrentURL            = "HX-Current-URL"
	HeaderHXHistoryRestoreRequest = "HX-History-Restore-Request"
	HeaderHXPrompt                = "HX-Prompt"
	HeaderHXTarget                = "HX-Target"
	HeaderHXTriggerName           = "HX-Trigger-Name"
	HeaderHXTriggerHeader         = "HX-Trigger"
)

// HTMXOption configures HTMX-specific response headers.
type HTMXOption func(*htmxConfig)

// htmxConfig accumulates HTMX header configurations.
type htmxConfig struct {
	trigger            map[string]any
	triggerAfterSwap   map[string]any
	triggerAfterSettle map[string]any
	pushURL            string
	replaceURL         string
	redirect           string
	refresh            bool
	reswap             string
	retarget           string
	reselect           string
	location           any // Can be string or location object
}

// WithHTMX wraps any response with HTMX-specific headers.
// It applies the provided options to set various HTMX response headers
// that control client-side behavior.
func WithHTMX(response handler.Response, opts ...HTMXOption) handler.Response {
	if response == nil {
		return nil
	}

	// If no options provided, return the original response
	if len(opts) == 0 {
		return response
	}

	return func(w http.ResponseWriter, r *http.Request) error {
		// Build config from options
		cfg := &htmxConfig{}
		for _, opt := range opts {
			opt(cfg)
		}

		// Apply HTMX headers based on config

		// Location header (for client-side redirects)
		if cfg.location != nil {
			switch v := cfg.location.(type) {
			case string:
				w.Header().Set(HeaderHXLocation, v)
			case map[string]any:
				if data, err := json.Marshal(v); err == nil {
					w.Header().Set(HeaderHXLocation, string(data))
				}
			}
		}

		// URL manipulation headers
		if cfg.pushURL != "" {
			w.Header().Set(HeaderHXPushURL, cfg.pushURL)
		}
		if cfg.replaceURL != "" {
			w.Header().Set(HeaderHXReplaceURL, cfg.replaceURL)
		}
		if cfg.redirect != "" {
			w.Header().Set(HeaderHXRedirect, cfg.redirect)
		}

		// Refresh header
		if cfg.refresh {
			w.Header().Set(HeaderHXRefresh, "true")
		}

		// Swap behavior headers
		if cfg.reswap != "" {
			w.Header().Set(HeaderHXReswap, cfg.reswap)
		}
		if cfg.retarget != "" {
			w.Header().Set(HeaderHXRetarget, cfg.retarget)
		}
		if cfg.reselect != "" {
			w.Header().Set(HeaderHXReselect, cfg.reselect)
		}

		// Trigger headers (events)
		if len(cfg.trigger) > 0 {
			if data, err := json.Marshal(cfg.trigger); err == nil {
				w.Header().Set(HeaderHXTrigger, string(data))
			}
		}
		if len(cfg.triggerAfterSwap) > 0 {
			if data, err := json.Marshal(cfg.triggerAfterSwap); err == nil {
				w.Header().Set(HeaderHXTriggerAfterSwap, string(data))
			}
		}
		if len(cfg.triggerAfterSettle) > 0 {
			if data, err := json.Marshal(cfg.triggerAfterSettle); err == nil {
				w.Header().Set(HeaderHXTriggerAfterSettle, string(data))
			}
		}

		// Execute the wrapped response
		return response(w, r)
	}
}

// Trigger sets the HX-Trigger header with multiple events.
// Events are serialized as JSON.
func Trigger(events map[string]any) HTMXOption {
	return func(cfg *htmxConfig) {
		cfg.trigger = events
	}
}

// TriggerEvent sets a single event in the HX-Trigger header.
// If called multiple times, events are merged.
func TriggerEvent(name string, detail any) HTMXOption {
	return func(cfg *htmxConfig) {
		if cfg.trigger == nil {
			cfg.trigger = make(map[string]any)
		}
		cfg.trigger[name] = detail
	}
}

// TriggerAfterSwap sets the HX-Trigger-After-Swap header with multiple events.
// These events are triggered after the swap phase.
func TriggerAfterSwap(events map[string]any) HTMXOption {
	return func(cfg *htmxConfig) {
		cfg.triggerAfterSwap = events
	}
}

// TriggerAfterSettle sets the HX-Trigger-After-Settle header with multiple events.
// These events are triggered after the settle phase.
func TriggerAfterSettle(events map[string]any) HTMXOption {
	return func(cfg *htmxConfig) {
		cfg.triggerAfterSettle = events
	}
}

// PushURL sets the HX-Push-Url header to update browser URL without reload.
// Use "false" to prevent URL update.
func PushURL(url string) HTMXOption {
	return func(cfg *htmxConfig) {
		cfg.pushURL = url
	}
}

// ReplaceURL sets the HX-Replace-Url header to replace browser URL without reload.
// Use "false" to prevent URL replacement.
func ReplaceURL(url string) HTMXOption {
	return func(cfg *htmxConfig) {
		cfg.replaceURL = url
	}
}

// HTMXRedirect sets the HX-Redirect header for client-side redirect.
// This causes a full page redirect on the client.
func HTMXRedirect(url string) HTMXOption {
	return func(cfg *htmxConfig) {
		cfg.redirect = url
	}
}

// Refresh sets the HX-Refresh header to trigger a full page refresh.
func Refresh() HTMXOption {
	return func(cfg *htmxConfig) {
		cfg.refresh = true
	}
}

// Reswap sets the HX-Reswap header to modify swap behavior.
// Examples: "innerHTML", "outerHTML", "beforebegin", "afterend"
// Can include modifiers: "innerHTML swap:500ms" or "innerHTML settle:1s"
func Reswap(method string, modifiers ...string) HTMXOption {
	return func(cfg *htmxConfig) {
		if len(modifiers) > 0 {
			cfg.reswap = method + " " + strings.Join(modifiers, " ")
		} else {
			cfg.reswap = method
		}
	}
}

// Retarget sets the HX-Retarget header to change the target element.
// The selector should be a CSS selector.
func Retarget(selector string) HTMXOption {
	return func(cfg *htmxConfig) {
		cfg.retarget = selector
	}
}

// Reselect sets the HX-Reselect header to select a subset of the response.
// The selector should be a CSS selector.
func Reselect(selector string) HTMXOption {
	return func(cfg *htmxConfig) {
		cfg.reselect = selector
	}
}

// Location sets the HX-Location header for client-side navigation.
// Can be a URL string or a location object with path, target, etc.
func Location(urlOrObject any) HTMXOption {
	return func(cfg *htmxConfig) {
		cfg.location = urlOrObject
	}
}

// IsHTMXRequest checks if the request is from an HTMX client.
func IsHTMXRequest(r *http.Request) bool {
	return r.Header.Get(HeaderHXRequest) == "true"
}

// IsHTMXBoosted checks if the request is from HTMX boost.
func IsHTMXBoosted(r *http.Request) bool {
	return r.Header.Get(HeaderHXBoosted) == "true"
}

// HTMXRequestHeaders contains parsed HTMX request headers.
type HTMXRequestHeaders struct {
	Request        bool   // HX-Request header presence
	Boosted        bool   // HX-Boosted header
	CurrentURL     string // HX-Current-URL header
	HistoryRestore bool   // HX-History-Restore-Request header
	Prompt         string // HX-Prompt header (user input)
	Target         string // HX-Target header (target element ID)
	TriggerName    string // HX-Trigger-Name header (triggering element name)
	Trigger        string // HX-Trigger header (triggering element ID)
}

// GetHTMXHeaders extracts and parses all HTMX headers from the request.
func GetHTMXHeaders(r *http.Request) HTMXRequestHeaders {
	return HTMXRequestHeaders{
		Request:        r.Header.Get(HeaderHXRequest) == "true",
		Boosted:        r.Header.Get(HeaderHXBoosted) == "true",
		CurrentURL:     r.Header.Get(HeaderHXCurrentURL),
		HistoryRestore: r.Header.Get(HeaderHXHistoryRestoreRequest) == "true",
		Prompt:         r.Header.Get(HeaderHXPrompt),
		Target:         r.Header.Get(HeaderHXTarget),
		TriggerName:    r.Header.Get(HeaderHXTriggerName),
		Trigger:        r.Header.Get(HeaderHXTriggerHeader),
	}
}
