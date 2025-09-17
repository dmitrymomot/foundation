package server

import "time"

const (
	// DefaultReadTimeout is the default timeout for reading the request.
	DefaultReadTimeout = 15 * time.Second

	// DefaultWriteTimeout is the default timeout for writing the response.
	DefaultWriteTimeout = 15 * time.Second

	// DefaultIdleTimeout is the default timeout for idle connections.
	DefaultIdleTimeout = 60 * time.Second

	// DefaultShutdownTimeout is the default timeout for graceful shutdown.
	DefaultShutdownTimeout = 30 * time.Second

	// DefaultMaxHeaderBytes is the default maximum size of request headers.
	DefaultMaxHeaderBytes = 1 << 20 // 1 MB
)
