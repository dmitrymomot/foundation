package httpserver

import "errors"

var (
	ErrStart    = errors.New("failed to start HTTP server")
	ErrShutdown = errors.New("failed to shutdown HTTP server gracefully")
)
