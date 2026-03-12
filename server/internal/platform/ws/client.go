package ws

import (
	"context"
	"log"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/tableforge/server/internal/platform/presence"
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
	roomID   uuid.UUID
	playerID uuid.UUID
	conn     *websocket.Conn
	send     chan []byte
	presence *presence.Store // nil for spectators

	// spectator is true when the client is watching but is not a participant.
	spectator bool
}

func newClient(hub *Hub, roomID, playerID uuid.UUID, conn *websocket.Conn, spectator bool, ps *presence.Store) *Client {
	return &Client{
		hub:       hub,
		roomID:    roomID,
		playerID:  playerID,
		conn:      conn,
		send:      make(chan []byte, 64),
		spectator: spectator,
		presence:  ps,
	}
}

// readPump drains incoming messages (we only need pong frames; clients don't send).
func (c *Client) readPump() {
	wsConnectionsActive.Inc()
	defer func() {
		wsConnectionsActive.Dec()
		c.hub.unsubscribe(c.roomID, c)
		c.conn.Close()
		// Mark player offline on disconnect.
		if !c.spectator && c.presence != nil {
			c.presence.Del(context.Background(), c.playerID)
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
				log.Printf("ws: read error: %v", err)
			}
			break
		}
	}
}

// writePump fans out messages from the send channel to the WebSocket.
// For participant clients it also runs a presence heartbeat every 15s.
func (c *Client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	var heartbeat *time.Ticker
	if !c.spectator && c.presence != nil {
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
				log.Printf("ws: write error: %v", err)
				return
			}

		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}

		case <-heartbeatC:
			go func() {
				if err := c.presence.Set(context.Background(), c.playerID); err != nil {
					log.Printf("ws: presence heartbeat: %v", err)
				}
			}()
		}
	}
}
