package server

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testHandler creates a simple test handler
func testHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		io.WriteString(w, "OK")
	})
}

// getFreePort returns a free port for testing
func getFreePort(t *testing.T) int {
	t.Helper()
	listener, err := net.Listen("tcp", ":0")
	require.NoError(t, err)
	port := listener.Addr().(*net.TCPAddr).Port
	listener.Close()
	return port
}

// TestServerDoubleRun tests that calling Run twice returns an error
func TestServerDoubleRun(t *testing.T) {
	t.Parallel()

	port := getFreePort(t)
	server := New(fmt.Sprintf(":%d", port))

	// Start first server
	ctx1, cancel1 := context.WithCancel(context.Background())
	var err1 error
	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		err1 = server.Run(ctx1, testHandler())
	}()

	// Give server time to start
	time.Sleep(50 * time.Millisecond)

	// Try to start second server - should fail
	ctx2 := context.Background()
	err2 := server.Run(ctx2, testHandler())
	require.Error(t, err2)
	assert.Contains(t, err2.Error(), "server is already running")

	// Cleanup
	cancel1()
	wg.Wait()
	assert.NoError(t, err1)
}

// TestServerPortConflict tests behavior when port is already in use
func TestServerPortConflict(t *testing.T) {
	t.Parallel()

	port := getFreePort(t)
	addr := fmt.Sprintf(":%d", port)

	// Start first server to occupy the port
	server1 := New(addr)
	ctx1, cancel1 := context.WithCancel(context.Background())
	var err1 error
	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		err1 = server1.Run(ctx1, testHandler())
	}()

	// Give first server time to bind port
	time.Sleep(50 * time.Millisecond)

	// Try to start second server on same port
	server2 := New(addr)
	ctx2, cancel2 := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel2()

	err2 := server2.Run(ctx2, testHandler())
	require.Error(t, err2)
	assert.Contains(t, err2.Error(), "address already in use")

	// Cleanup
	cancel1()
	wg.Wait()
	assert.NoError(t, err1)
}

// TestServerConcurrentRunShutdown tests race conditions between Run and shutdown
func TestServerConcurrentRunShutdown(t *testing.T) {
	t.Parallel()

	port := getFreePort(t)
	server := New(fmt.Sprintf(":%d", port))

	var runErr error
	var wg sync.WaitGroup

	// Start server
	ctx, cancel := context.WithCancel(context.Background())
	wg.Add(1)
	go func() {
		defer wg.Done()
		runErr = server.Run(ctx, testHandler())
	}()

	// Give server minimal time to start
	time.Sleep(10 * time.Millisecond)

	// Trigger shutdown immediately
	cancel()

	// Wait for completion
	wg.Wait()

	// Should complete without error (graceful shutdown)
	assert.NoError(t, runErr)
}

// TestServerGracefulShutdownTimeout tests what happens when shutdown times out
func TestServerGracefulShutdownTimeout(t *testing.T) {
	t.Parallel()

	port := getFreePort(t)
	server := New(fmt.Sprintf(":%d", port), WithShutdownTimeout(10*time.Millisecond))

	// Handler that blocks during shutdown
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Block for longer than shutdown timeout
		time.Sleep(50 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	})

	ctx, cancel := context.WithCancel(context.Background())
	var runErr error
	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		runErr = server.Run(ctx, handler)
	}()

	// Give server time to start
	time.Sleep(20 * time.Millisecond)

	// Make a request that will block
	go func() {
		_, _ = http.Get(fmt.Sprintf("http://localhost:%d", port))
	}()

	// Give request time to start
	time.Sleep(10 * time.Millisecond)

	// Trigger shutdown - should timeout
	cancel()

	wg.Wait()
	// Server should return the shutdown timeout error
	assert.Error(t, runErr)
}

// TestServerInvalidAddress tests behavior with invalid addresses
func TestServerInvalidAddress(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		addr string
	}{
		{"invalid port", ":999999"},
		{"invalid format", "::invalid::"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := New(tt.addr)
			ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
			defer cancel()

			err := server.Run(ctx, testHandler())
			require.Error(t, err)
		})
	}
}

// TestServerTLSRaceCondition tests setting TLS config while server might be running
func TestServerTLSRaceCondition(t *testing.T) {
	t.Parallel()

	port := getFreePort(t)
	server := New(fmt.Sprintf(":%d", port))

	// Create TLS config
	tlsConfig := &tls.Config{
		MinVersion: tls.VersionTLS12,
	}

	// Apply TLS config concurrently with potential server start
	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		WithTLS(tlsConfig)(server)
	}()

	go func() {
		defer wg.Done()
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
		defer cancel()
		_ = server.Run(ctx, testHandler())
	}()

	wg.Wait()
	// Test passes if no race condition detected
}

// TestServerContextCancellationDuringStartup tests context cancellation during server startup
func TestServerContextCancellationDuringStartup(t *testing.T) {
	t.Parallel()

	port := getFreePort(t)
	server := New(fmt.Sprintf(":%d", port))

	// Cancel context immediately
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := server.Run(ctx, testHandler())
	// Should handle cancellation gracefully
	assert.NoError(t, err)
}

// TestServerShutdownWithoutRun tests calling gracefulShutdown without Run
func TestServerShutdownWithoutRun(t *testing.T) {
	t.Parallel()

	server := New(":0")
	ctx := context.Background()

	// Should not panic or error
	err := server.gracefulShutdown(ctx)
	assert.NoError(t, err)
}

// TestServerRunIntegration tests actual HTTP server functionality
func TestServerRunIntegration(t *testing.T) {
	t.Parallel()

	port := getFreePort(t)
	server := New(fmt.Sprintf(":%d", port))

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		io.WriteString(w, "integration test")
	})

	ctx, cancel := context.WithCancel(context.Background())
	var runErr error
	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		runErr = server.Run(ctx, handler)
	}()

	// Give server time to start
	time.Sleep(50 * time.Millisecond)

	// Make HTTP request
	resp, err := http.Get(fmt.Sprintf("http://localhost:%d", port))
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	assert.Equal(t, "integration test", string(body))

	// Shutdown
	cancel()
	wg.Wait()
	assert.NoError(t, runErr)
}

// TestRunConvenienceFunction tests the convenience Run function
func TestRunConvenienceFunction(t *testing.T) {
	t.Parallel()

	port := getFreePort(t)
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	err := Run(ctx, fmt.Sprintf(":%d", port), testHandler())
	assert.NoError(t, err)
}
