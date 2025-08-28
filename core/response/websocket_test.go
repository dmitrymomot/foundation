package response_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dmitrymomot/foundation/core/response"
)

func TestWebSocket_BasicUpgrade(t *testing.T) {
	t.Parallel()

	t.Run("successful_upgrade", func(t *testing.T) {
		t.Parallel()

		var (
			connEstablished bool
			mu              sync.Mutex
		)
		handler := response.WebSocket(
			func(ctx context.Context, conn *websocket.Conn) error {
				mu.Lock()
				connEstablished = true
				mu.Unlock()
				return nil
			},
			response.WithWSAllowAnyOrigin(),
		)

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			err := handler(w, r)
			assert.NoError(t, err)
		}))
		defer server.Close()

		wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
		conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
		require.NoError(t, err)
		defer conn.Close()

		time.Sleep(10 * time.Millisecond)
		mu.Lock()
		assert.True(t, connEstablished)
		mu.Unlock()
	})

	t.Run("upgrade_with_custom_buffer_sizes", func(t *testing.T) {
		t.Parallel()

		handler := response.WebSocket(
			func(ctx context.Context, conn *websocket.Conn) error {
				return nil
			},
			response.WithWSReadBuffer(2048),
			response.WithWSWriteBuffer(2048),
			response.WithWSAllowAnyOrigin(),
		)

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			err := handler(w, r)
			assert.NoError(t, err)
		}))
		defer server.Close()

		wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
		conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
		require.NoError(t, err)
		defer conn.Close()
	})

	t.Run("upgrade_with_subprotocols", func(t *testing.T) {
		t.Parallel()

		handler := response.WebSocket(
			func(ctx context.Context, conn *websocket.Conn) error {
				return nil
			},
			response.WithWSSubprotocols("chat", "superchat"),
			response.WithWSAllowAnyOrigin(),
		)

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			err := handler(w, r)
			assert.NoError(t, err)
		}))
		defer server.Close()

		wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
		dialer := websocket.Dialer{
			Subprotocols: []string{"chat"},
		}
		conn, resp, err := dialer.Dial(wsURL, nil)
		require.NoError(t, err)
		defer conn.Close()

		assert.Equal(t, "chat", resp.Header.Get("Sec-Websocket-Protocol"))
	})
}

func TestWebSocket_MessageHandling(t *testing.T) {
	t.Parallel()

	t.Run("echo_messages", func(t *testing.T) {
		t.Parallel()

		handler := response.EchoWebSocket(response.WithWSAllowAnyOrigin())

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			err := handler(w, r)
			assert.NoError(t, err)
		}))
		defer server.Close()

		wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
		conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
		require.NoError(t, err)
		defer conn.Close()

		testMessage := "Hello, WebSocket!"
		err = conn.WriteMessage(websocket.TextMessage, []byte(testMessage))
		require.NoError(t, err)

		msgType, data, err := conn.ReadMessage()
		require.NoError(t, err)
		assert.Equal(t, websocket.TextMessage, msgType)
		assert.Equal(t, testMessage, string(data))
	})

	t.Run("binary_messages", func(t *testing.T) {
		t.Parallel()

		handler := response.EchoWebSocket(response.WithWSAllowAnyOrigin())

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			err := handler(w, r)
			assert.NoError(t, err)
		}))
		defer server.Close()

		wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
		conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
		require.NoError(t, err)
		defer conn.Close()

		testData := []byte{0x01, 0x02, 0x03, 0x04}
		err = conn.WriteMessage(websocket.BinaryMessage, testData)
		require.NoError(t, err)

		msgType, data, err := conn.ReadMessage()
		require.NoError(t, err)
		assert.Equal(t, websocket.BinaryMessage, msgType)
		assert.Equal(t, testData, data)
	})
}

func TestWebSocket_Channels(t *testing.T) {
	t.Parallel()

	t.Run("bidirectional_communication", func(t *testing.T) {
		t.Parallel()

		incoming := make(chan response.WebSocketMessage, 10)
		outgoing := make(chan response.WebSocketMessage, 10)

		handler := response.WebSocketWithChannels(incoming, outgoing, response.WithWSAllowAnyOrigin())

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			err := handler(w, r)
			assert.NoError(t, err)
		}))
		defer server.Close()

		wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
		conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
		require.NoError(t, err)
		defer conn.Close()

		testMessage := "Test from client"
		err = conn.WriteMessage(websocket.TextMessage, []byte(testMessage))
		require.NoError(t, err)

		select {
		case msg := <-incoming:
			assert.Equal(t, websocket.TextMessage, msg.Type)
			assert.Equal(t, testMessage, string(msg.Data))
		case <-time.After(time.Second):
			t.Fatal("timeout waiting for incoming message")
		}

		responseMessage := "Response from server"
		outgoing <- response.WebSocketMessage{
			Type: websocket.TextMessage,
			Data: []byte(responseMessage),
		}

		msgType, data, err := conn.ReadMessage()
		require.NoError(t, err)
		assert.Equal(t, websocket.TextMessage, msgType)
		assert.Equal(t, responseMessage, string(data))
	})

	t.Run("channel_close_on_disconnect", func(t *testing.T) {
		t.Parallel()

		incoming := make(chan response.WebSocketMessage, 10)
		outgoing := make(chan response.WebSocketMessage, 10)

		handler := response.WebSocketWithChannels(incoming, outgoing, response.WithWSAllowAnyOrigin())

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			err := handler(w, r)
			assert.NoError(t, err)
		}))
		defer server.Close()

		wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
		conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
		require.NoError(t, err)

		err = conn.Close()
		require.NoError(t, err)

		time.Sleep(50 * time.Millisecond)

		select {
		case _, ok := <-incoming:
			assert.False(t, ok, "incoming channel should be closed")
		case <-time.After(100 * time.Millisecond):
		}
	})
}

func TestWebSocket_Callbacks(t *testing.T) {
	t.Parallel()

	t.Run("connection_lifecycle", func(t *testing.T) {
		t.Parallel()

		var (
			connected    bool
			disconnected bool
			mu           sync.Mutex
		)

		handler := response.WebSocket(
			func(ctx context.Context, conn *websocket.Conn) error {
				time.Sleep(10 * time.Millisecond)
				return nil
			},
			response.WithWSOnConnect(func(ctx context.Context, conn *websocket.Conn) error {
				mu.Lock()
				connected = true
				mu.Unlock()
				return nil
			}),
			response.WithWSOnDisconnect(func(ctx context.Context, conn *websocket.Conn) {
				mu.Lock()
				disconnected = true
				mu.Unlock()
			}),
			response.WithWSAllowAnyOrigin(),
		)

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			err := handler(w, r)
			assert.NoError(t, err)
		}))
		defer server.Close()

		wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
		conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
		require.NoError(t, err)

		time.Sleep(20 * time.Millisecond)
		mu.Lock()
		assert.True(t, connected)
		mu.Unlock()

		err = conn.Close()
		require.NoError(t, err)

		time.Sleep(50 * time.Millisecond)
		mu.Lock()
		assert.True(t, disconnected)
		mu.Unlock()
	})

	t.Run("error_handler", func(t *testing.T) {
		t.Parallel()

		var (
			errorCaught bool
			mu          sync.Mutex
		)

		handler := response.WebSocket(
			func(ctx context.Context, conn *websocket.Conn) error {
				for {
					select {
					case <-ctx.Done():
						return ctx.Err()
					default:
						_, _, err := conn.ReadMessage()
						if err != nil {
							return err
						}
					}
				}
			},
			response.WithWSErrorHandler(func(ctx context.Context, err error) {
				mu.Lock()
				errorCaught = true
				mu.Unlock()
			}),
			response.WithWSAllowAnyOrigin(),
		)

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			err := handler(w, r)
			assert.NoError(t, err)
		}))
		defer server.Close()

		wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
		conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
		require.NoError(t, err)

		err = conn.Close()
		require.NoError(t, err)

		time.Sleep(50 * time.Millisecond)
		mu.Lock()
		assert.True(t, errorCaught)
		mu.Unlock()
	})
}

func TestWebSocket_OriginCheck(t *testing.T) {
	t.Parallel()

	t.Run("custom_origin_check", func(t *testing.T) {
		t.Parallel()

		allowedOrigin := "http://allowed.example.com"
		handler := response.WebSocket(
			func(ctx context.Context, conn *websocket.Conn) error {
				return nil
			},
			response.WithWSOriginCheck(func(r *http.Request) bool {
				origin := r.Header.Get("Origin")
				return origin == allowedOrigin
			}),
		)

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			err := handler(w, r)
			assert.NoError(t, err)
		}))
		defer server.Close()

		wsURL := "ws" + strings.TrimPrefix(server.URL, "http")

		dialer := websocket.DefaultDialer
		headers := http.Header{
			"Origin": []string{"http://forbidden.example.com"},
		}
		_, resp, err := dialer.Dial(wsURL, headers)
		assert.Error(t, err)
		if resp != nil {
			assert.Equal(t, http.StatusForbidden, resp.StatusCode)
		}

		headers["Origin"] = []string{allowedOrigin}
		conn, _, err := dialer.Dial(wsURL, headers)
		require.NoError(t, err)
		defer conn.Close()
	})
}

func TestWebSocket_HandshakeTimeout(t *testing.T) {
	t.Parallel()

	t.Run("with_handshake_timeout", func(t *testing.T) {
		t.Parallel()

		handler := response.WebSocket(
			func(ctx context.Context, conn *websocket.Conn) error {
				return nil
			},
			response.WithWSHandshakeTimeout(5*time.Second),
			response.WithWSAllowAnyOrigin(),
		)

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			err := handler(w, r)
			assert.NoError(t, err)
		}))
		defer server.Close()

		wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
		conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
		require.NoError(t, err)
		defer conn.Close()
	})
}

func TestWebSocket_ResponseHeaders(t *testing.T) {
	t.Parallel()

	t.Run("custom_response_headers", func(t *testing.T) {
		t.Parallel()

		customHeaders := http.Header{
			"X-Custom-Header": []string{"test-value"},
		}

		handler := response.WebSocket(
			func(ctx context.Context, conn *websocket.Conn) error {
				return nil
			},
			response.WithWSUpgradeHeaders(customHeaders),
			response.WithWSAllowAnyOrigin(),
		)

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			err := handler(w, r)
			assert.NoError(t, err)
		}))
		defer server.Close()

		wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
		conn, resp, err := websocket.DefaultDialer.Dial(wsURL, nil)
		require.NoError(t, err)
		defer conn.Close()

		assert.Equal(t, "test-value", resp.Header.Get("X-Custom-Header"))
	})
}
