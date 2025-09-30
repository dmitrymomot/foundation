package server

import "time"

const (
	DefaultReadTimeout         = 15 * time.Second
	DefaultWriteTimeout        = 15 * time.Second
	DefaultIdleTimeout         = 60 * time.Second
	DefaultShutdownTimeout     = 30 * time.Second
	DefaultMaxHeaderBytes      = 1 << 20 // 1 MB
	DefaultDomainLookupTimeout = 10 * time.Second
)
