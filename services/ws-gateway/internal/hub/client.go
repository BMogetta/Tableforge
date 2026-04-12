package hub

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/recess/services/ws-gateway/internal/presence"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

const (
	writeWait       = 10 * time.Second
	pongWait        = 60 * time.Second
	pingPeriod      = (pongWait * 9) / 10
	heartbeatPeriod = 15 * time.Second
	maxMessageSize  = 512
)

// Client wraps a single WebSocket connection.
type Client struct {
	hub      *Hub
	RoomID   uuid.UUID // uuid.Nil for player-channel-only connections
	PlayerID uuid.UUID
	conn     *websocket.Conn
	send     chan []byte
	presence *presence.Store // nil for spectators and player-channel clients

	// Spectator is true when the client is watching but not a participant.
	Spectator bool

	// closeMu guards `closed` and serializes close/send so multiple goroutines
	// (DisconnectPlayer, fanout*, BroadcastAll, gracefulShutdown) can never
	// race to either double-close the send channel or send on a closed one.
	closeMu sync.Mutex
	closed  bool
}

// closeSend closes the send channel idempotently. Safe to call from any
// goroutine — subsequent calls are no-ops.
func (c *Client) closeSend() {
	c.closeMu.Lock()
	defer c.closeMu.Unlock()
	if c.closed {
		return
	}
	c.closed = true
	close(c.send)
}

// trySend non-blockingly delivers data to the client. Returns true on success.
// If the client is already closed, it is a no-op. If the send buffer is full,
// the channel is closed (slow consumer) and false is returned.
func (c *Client) trySend(data []byte) bool {
	c.closeMu.Lock()
	defer c.closeMu.Unlock()
	if c.closed {
		return false
	}
	select {
	case c.send <- data:
		return true
	default:
		c.closed = true
		close(c.send)
		return false
	}
}

// trySendOrDrop is like trySend but never closes the channel on full buffer —
// the message is simply dropped. Use for best-effort, fire-and-forget sends
// (e.g. presence snapshots) where a slow consumer shouldn't tear down the WS.
func (c *Client) trySendOrDrop(data []byte) {
	c.closeMu.Lock()
	defer c.closeMu.Unlock()
	if c.closed {
		return
	}
	select {
	case c.send <- data:
	default:
	}
}

func NewClient(hub *Hub, roomID, playerID uuid.UUID, conn *websocket.Conn, spectator bool, ps *presence.Store) *Client {
	return &Client{
		hub:       hub,
		RoomID:    roomID,
		PlayerID:  playerID,
		conn:      conn,
		send:      make(chan []byte, 64),
		Spectator: spectator,
		presence:  ps,
	}
}

// ReadPump drains incoming messages (clients only send pong frames).
// On exit it cleans up room + player subscriptions and marks presence offline.
// A span is opened for the lifetime of the connection and closed on disconnect.
func (c *Client) ReadPump() {
	_, span := otel.Tracer("ws-gateway").Start(context.Background(), "ws.connection",
		trace.WithSpanKind(trace.SpanKindServer),
		trace.WithAttributes(
			attribute.String("player_id", c.PlayerID.String()),
			attribute.String("room_id", c.RoomID.String()),
			attribute.Bool("spectator", c.Spectator),
		),
	)
	defer span.End()

	wsConnectionsActive.Inc()
	defer func() {
		wsConnectionsActive.Dec()
		if c.RoomID != uuid.Nil {
			c.hub.UnsubscribeRoom(c.RoomID, c)
		}
		if !c.Spectator {
			c.hub.UnsubscribePlayer(c.PlayerID, c)
		}
		c.conn.Close()
		if !c.Spectator && c.presence != nil {
			c.presence.Del(context.Background(), c.PlayerID)
		}
	}()

	c.conn.SetReadLimit(maxMessageSize)
	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		_, _, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				slog.Info("ws: read error", "error", err)
			}
			break
		}
	}
}

// WritePump fans out messages from the send channel to the WebSocket.
// For participant clients it also runs a presence heartbeat every 15s.
func (c *Client) WritePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	var heartbeat *time.Ticker
	if !c.Spectator && c.presence != nil {
		heartbeat = time.NewTicker(heartbeatPeriod)
		defer heartbeat.Stop()
	}

	for {
		var heartbeatC <-chan time.Time
		if heartbeat != nil {
			heartbeatC = heartbeat.C
		}

		select {
		case msg, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := c.conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				slog.Info("ws: write error", "error", err)
				return
			}

		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}

		case <-heartbeatC:
			go func() {
				if err := c.presence.Set(context.Background(), c.PlayerID); err != nil {
					slog.Error("ws: presence heartbeat", "error", err)
				}
			}()
		}
	}
}
