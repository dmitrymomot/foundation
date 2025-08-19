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
	SetValue(key, val any)
}
