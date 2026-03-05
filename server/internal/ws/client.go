package ws

import (
	"log"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

const (
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = (pongWait * 9) / 10
	maxMessageSize = 512
)

// Client wraps a single WebSocket connection.
type Client struct {
	hub    *Hub
	roomID uuid.UUID
	conn   *websocket.Conn
	send   chan []byte

	// spectator is true when the client is watching the game but is not a
	// participant (not in room_players). Spectators receive most events but
	// are silently excluded from rematch_vote broadcasts.
	spectator bool
}

func newClient(hub *Hub, roomID uuid.UUID, conn *websocket.Conn, spectator bool) *Client {
	return &Client{
		hub:       hub,
		roomID:    roomID,
		conn:      conn,
		send:      make(chan []byte, 64),
		spectator: spectator,
	}
}

// readPump drains incoming messages (we only need pong frames; clients don't send).
func (c *Client) readPump() {
	wsConnectionsActive.Inc()
	defer func() {
		wsConnectionsActive.Dec()
		c.hub.unsubscribe(c.roomID, c)
		c.conn.Close()
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
func (c *Client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
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
		}
	}
}
