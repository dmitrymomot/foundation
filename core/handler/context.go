package handler

import (
	"context"
	"net/http"
)

// Context defines the contract for request contexts in the framework.
// Use Context for the default implementation.
type Context interface {
	context.Context
	Request() *http.Request
	ResponseWriter() http.ResponseWriter
	Param(key string) string
	// Set(key, val any)
	// Get(key any) any
	// SetCookie(cookie *http.Cookie) error
	// GetCookie(name string) (*http.Cookie, error)
}
