package response

import (
	"context"
	"net/http"
	"time"

	"github.com/dmitrymomot/gokit/core/handler"
	"github.com/gorilla/websocket"
)

type wsConfig struct {
	upgrader       *websocket.Upgrader
	responseHeader http.Header
	messageHandler func(context.Context, *websocket.Conn) error
	onConnect      func(context.Context, *websocket.Conn) error
	onDisconnect   func(context.Context, *websocket.Conn)
	onError        func(context.Context, error)
}

type WebSocketOption func(*wsConfig)

func WithWSReadBuffer(size int) WebSocketOption {
	return func(c *wsConfig) {
		c.upgrader.ReadBufferSize = size
	}
}

func WithWSWriteBuffer(size int) WebSocketOption {
	return func(c *wsConfig) {
		c.upgrader.WriteBufferSize = size
	}
}

func WithWSHandshakeTimeout(timeout time.Duration) WebSocketOption {
	return func(c *wsConfig) {
		c.upgrader.HandshakeTimeout = timeout
	}
}

func WithWSOriginCheck(fn func(r *http.Request) bool) WebSocketOption {
	return func(c *wsConfig) {
		c.upgrader.CheckOrigin = fn
	}
}

func WithWSAllowAnyOrigin() WebSocketOption {
	return func(c *wsConfig) {
		c.upgrader.CheckOrigin = func(r *http.Request) bool {
			return true
		}
	}
}

func WithWSSubprotocols(protocols ...string) WebSocketOption {
	return func(c *wsConfig) {
		c.upgrader.Subprotocols = protocols
	}
}

func WithWSUpgradeHeaders(header http.Header) WebSocketOption {
	return func(c *wsConfig) {
		c.responseHeader = header
	}
}

func WithWSOnConnect(fn func(context.Context, *websocket.Conn) error) WebSocketOption {
	return func(c *wsConfig) {
		c.onConnect = fn
	}
}

func WithWSOnDisconnect(fn func(context.Context, *websocket.Conn)) WebSocketOption {
	return func(c *wsConfig) {
		c.onDisconnect = fn
	}
}

func WithWSErrorHandler(fn func(context.Context, error)) WebSocketOption {
	return func(c *wsConfig) {
		c.onError = fn
	}
}

func WebSocket(messageHandler func(context.Context, *websocket.Conn) error, opts ...WebSocketOption) handler.Response {
	cfg := &wsConfig{
		upgrader: &websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
		},
		messageHandler: messageHandler,
	}

	for _, opt := range opts {
		opt(cfg)
	}

	return func(w http.ResponseWriter, r *http.Request) error {
		conn, err := cfg.upgrader.Upgrade(w, r, cfg.responseHeader)
		if err != nil {
			if cfg.onError != nil {
				cfg.onError(r.Context(), err)
			}
			return nil
		}
		defer func() {
			_ = conn.Close()
			if cfg.onDisconnect != nil {
				cfg.onDisconnect(r.Context(), conn)
			}
		}()

		if cfg.onConnect != nil {
			if err := cfg.onConnect(r.Context(), conn); err != nil {
				if cfg.onError != nil {
					cfg.onError(r.Context(), err)
				}
				return nil
			}
		}

		if err := cfg.messageHandler(r.Context(), conn); err != nil {
			if cfg.onError != nil {
				cfg.onError(r.Context(), err)
			}
			return nil
		}

		return nil
	}
}

type WebSocketMessage struct {
	Type int
	Data []byte
}

func WebSocketWithChannels(incoming chan<- WebSocketMessage, outgoing <-chan WebSocketMessage, opts ...WebSocketOption) handler.Response {
	return WebSocket(func(ctx context.Context, conn *websocket.Conn) error {
		go func() {
			defer close(incoming)
			for {
				select {
				case <-ctx.Done():
					return
				default:
					msgType, data, err := conn.ReadMessage()
					if err != nil {
						if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
							return
						}
						return
					}
					select {
					case incoming <- WebSocketMessage{Type: msgType, Data: data}:
					case <-ctx.Done():
						return
					}
				}
			}
		}()

		for {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case msg, ok := <-outgoing:
				if !ok {
					_ = conn.WriteMessage(websocket.CloseMessage, []byte{})
					return nil
				}
				if err := conn.WriteMessage(msg.Type, msg.Data); err != nil {
					return err
				}
			}
		}
	}, opts...)
}

func EchoWebSocket(opts ...WebSocketOption) handler.Response {
	return WebSocket(func(ctx context.Context, conn *websocket.Conn) error {
		for {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
				msgType, data, err := conn.ReadMessage()
				if err != nil {
					if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
						return err
					}
					return nil
				}
				if err := conn.WriteMessage(msgType, data); err != nil {
					return err
				}
			}
		}
	}, opts...)
}
