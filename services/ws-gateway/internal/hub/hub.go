// Package hub manages WebSocket client connections and Redis pub/sub fan-out.
//
// Architecture:
//
//	browser ──WS──► ws-gateway hub ──Redis sub──► fan-out to local clients
//	any service    ──Redis pub──► ws-gateway hub ──► fan-out to local clients
//
// All services (game-server, chat-service, etc.) publish events to Redis.
// The hub is the sole consumer — no service broadcasts directly to clients.
//
// Redis channel conventions (must match all publishers):
//
//	ws:room:{room_id}     — room-scoped events (game, chat, presence)
//	ws:player:{player_id} — player-scoped events (DMs, notifications, queue)
//
// Envelope format for room channels:
//
//	{ "s": bool, "d": <raw event JSON> }
//	s=false → deliver to all clients (players + spectators)
//	s=true  → deliver to spectators only
//
// Player channel messages are raw event JSON (no envelope wrapper).
package hub

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/redis/go-redis/v9"
	sharedws "github.com/recess/shared/ws"
)

// EventType and Event are imported from shared/ws — the single source of truth
// for client-facing WebSocket event contracts.
type EventType = sharedws.EventType
type Event = sharedws.Event

var wsConnectionsActive = promauto.NewGauge(prometheus.GaugeOpts{
	Name: "recess_ws_connections_active",
	Help: "Number of active WebSocket connections.",
})

// Hub manages per-room and per-player client sets.
// All broadcast operations go through Redis pub/sub — no direct in-memory
// broadcast is supported. This ensures correctness across multiple instances.
type Hub struct {
	mu          sync.RWMutex
	rooms       map[uuid.UUID]map[*Client]struct{}
	roomCancels map[uuid.UUID]context.CancelFunc

	pmu           sync.RWMutex
	players       map[uuid.UUID]map[*Client]struct{}
	playerCancels map[uuid.UUID]context.CancelFunc

	rdb *redis.Client
}

func New(rdb *redis.Client) *Hub {
	return &Hub{
		rooms:         make(map[uuid.UUID]map[*Client]struct{}),
		roomCancels:   make(map[uuid.UUID]context.CancelFunc),
		players:       make(map[uuid.UUID]map[*Client]struct{}),
		playerCancels: make(map[uuid.UUID]context.CancelFunc),
		rdb:           rdb,
	}
}


// SubscribeRoom adds a client to the room index and starts a Redis listener
// if this is the first client in the room. The listener is bound to a
// per-channel context so it terminates deterministically when the room
// empties — preventing zombie goroutines from accumulating across
// connect/disconnect cycles.
func (h *Hub) SubscribeRoom(roomID uuid.UUID, c *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.rooms[roomID] == nil {
		h.rooms[roomID] = make(map[*Client]struct{})
	}
	h.rooms[roomID][c] = struct{}{}
	if _, running := h.roomCancels[roomID]; !running && h.rdb != nil {
		ctx, cancel := context.WithCancel(context.Background())
		h.roomCancels[roomID] = cancel
		go h.listenRoom(ctx, roomID)
	}
}

// UnsubscribeRoom removes a client from the room index and cancels the
// listener goroutine when the room is empty.
func (h *Hub) UnsubscribeRoom(roomID uuid.UUID, c *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.rooms[roomID], c)
	if len(h.rooms[roomID]) == 0 {
		delete(h.rooms, roomID)
		if cancel, ok := h.roomCancels[roomID]; ok {
			cancel()
			delete(h.roomCancels, roomID)
		}
	}
}

// SubscribePlayer adds a client to the player index and starts a Redis listener
// if this is the first connection for that player. See SubscribeRoom for the
// lifecycle invariant that prevents zombie listeners.
func (h *Hub) SubscribePlayer(playerID uuid.UUID, c *Client) {
	h.pmu.Lock()
	defer h.pmu.Unlock()
	if h.players[playerID] == nil {
		h.players[playerID] = make(map[*Client]struct{})
	}
	h.players[playerID][c] = struct{}{}
	if _, running := h.playerCancels[playerID]; !running && h.rdb != nil {
		ctx, cancel := context.WithCancel(context.Background())
		h.playerCancels[playerID] = cancel
		go h.listenPlayer(ctx, playerID)
	}
}

// UnsubscribePlayer removes a client from the player index and cancels the
// listener goroutine when no connections remain for the player.
func (h *Hub) UnsubscribePlayer(playerID uuid.UUID, c *Client) {
	h.pmu.Lock()
	defer h.pmu.Unlock()
	delete(h.players[playerID], c)
	if len(h.players[playerID]) == 0 {
		delete(h.players, playerID)
		if cancel, ok := h.playerCancels[playerID]; ok {
			cancel()
			delete(h.playerCancels, playerID)
		}
	}
}

// DisconnectPlayer closes all WebSocket connections for a player by closing
// their send channels. ReadPump detects the close and cleans up.
// Used by the session-revoked consumer to force-disconnect banned players.
func (h *Hub) DisconnectPlayer(playerID uuid.UUID) {
	h.pmu.RLock()
	clients := make([]*Client, 0, len(h.players[playerID]))
	for c := range h.players[playerID] {
		clients = append(clients, c)
	}
	h.pmu.RUnlock()

	for _, c := range clients {
		// nil signals WritePump to close; if the buffer is full, close directly.
		if !c.trySend(nil) {
			c.closeSend()
		}
	}
}

// SpectatorCount returns the number of spectators in a room.
func (h *Hub) SpectatorCount(roomID uuid.UUID) int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	count := 0
	for c := range h.rooms[roomID] {
		if c.Spectator {
			count++
		}
	}
	return count
}

// Publish publishes an event to a room channel via Redis.
// All connected clients (players + spectators) receive it unless
// the event type is in spectatorBlocklist.
func (h *Hub) Publish(ctx context.Context, roomID uuid.UUID, event Event) {
	data, err := json.Marshal(event)
	if err != nil {
		slog.Error("hub: marshal event", "error", err)
		return
	}
	env, err := json.Marshal(sharedws.FilteredEnvelope{Data: data})
	if err != nil {
		slog.Error("hub: marshal envelope", "error", err)
		return
	}
	if err := h.rdb.Publish(ctx, sharedws.RoomChannelKey(roomID), env).Err(); err != nil {
		slog.Error("hub: redis publish room", "room_id", roomID, "error", err)
	}
}

// PublishToPlayer publishes an event to a player's personal channel via Redis.
func (h *Hub) PublishToPlayer(ctx context.Context, playerID uuid.UUID, event Event) {
	data, err := json.Marshal(event)
	if err != nil {
		slog.Error("hub: marshal player event", "error", err)
		return
	}
	if err := h.rdb.Publish(ctx, sharedws.PlayerChannelKey(playerID), data).Err(); err != nil {
		slog.Error("hub: redis publish player", "player_id", playerID, "error", err)
	}
}

// listenRoom subscribes to the Redis room channel and fans out to local
// clients. Exits when ctx is cancelled (i.e. the room emptied) so duplicate
// listeners can't accumulate across reconnect cycles.
func (h *Hub) listenRoom(ctx context.Context, roomID uuid.UUID) {
	sub := h.rdb.Subscribe(ctx, sharedws.RoomChannelKey(roomID))
	defer sub.Close()
	ch := sub.Channel()

	for {
		select {
		case <-ctx.Done():
			return
		case msg, ok := <-ch:
			if !ok {
				return
			}
			var env sharedws.FilteredEnvelope
			if err := json.Unmarshal([]byte(msg.Payload), &env); err != nil {
				slog.Error("hub: unmarshal room envelope", "error", err)
				continue
			}

			var header struct {
				Type EventType `json:"type"`
			}
			_ = json.Unmarshal(env.Data, &header)

			if env.Spectators {
				h.fanoutSpectatorsOnly(roomID, header.Type, env.Data)
			} else {
				h.fanout(roomID, header.Type, env.Data)
			}
		}
	}
}

// listenPlayer subscribes to the Redis player channel and fans out to local
// clients. Exits when ctx is cancelled.
func (h *Hub) listenPlayer(ctx context.Context, playerID uuid.UUID) {
	sub := h.rdb.Subscribe(ctx, sharedws.PlayerChannelKey(playerID))
	defer sub.Close()
	ch := sub.Channel()

	for {
		select {
		case <-ctx.Done():
			return
		case msg, ok := <-ch:
			if !ok {
				return
			}
			h.fanoutPlayer(playerID, []byte(msg.Payload))
		}
	}
}

func (h *Hub) fanout(roomID uuid.UUID, eventType EventType, data []byte) {
	h.mu.RLock()
	clients := h.rooms[roomID]
	h.mu.RUnlock()

	blocked := sharedws.SpectatorBlocklist[eventType]
	for c := range clients {
		if blocked && c.Spectator {
			continue
		}
		if c.trySend(data) {
			wsMessagesDelivered.WithLabelValues(string(eventType)).Inc()
		} else {
			slog.Warn("hub: dropping slow client", "room_id", roomID)
		}
	}
}

func (h *Hub) fanoutSpectatorsOnly(roomID uuid.UUID, eventType EventType, data []byte) {
	h.mu.RLock()
	clients := h.rooms[roomID]
	h.mu.RUnlock()

	if sharedws.SpectatorBlocklist[eventType] {
		return
	}
	for c := range clients {
		if !c.Spectator {
			continue
		}
		if c.trySend(data) {
			wsMessagesDelivered.WithLabelValues(string(eventType)).Inc()
		} else {
			slog.Warn("hub: dropping slow spectator", "room_id", roomID)
		}
	}
}

func (h *Hub) fanoutPlayer(playerID uuid.UUID, data []byte) {
	h.pmu.RLock()
	clients := h.players[playerID]
	h.pmu.RUnlock()

	for c := range clients {
		if c.trySend(data) {
			wsMessagesDelivered.WithLabelValues("player_event").Inc()
		} else {
			slog.Warn("hub: dropping slow player client", "player_id", playerID)
		}
	}
}

// BroadcastAll delivers data to every connected client (all rooms + all player channels).
// Used for admin broadcast messages that must reach all players.
func (h *Hub) BroadcastAll(data []byte) {
	sent := make(map[*Client]struct{})

	h.mu.RLock()
	for _, clients := range h.rooms {
		for c := range clients {
			if _, ok := sent[c]; ok {
				continue
			}
			sent[c] = struct{}{}
			if c.trySend(data) {
				wsMessagesDelivered.WithLabelValues(string(sharedws.EventBroadcast)).Inc()
			} else {
				slog.Warn("hub: dropping slow client on broadcast")
			}
		}
	}
	h.mu.RUnlock()

	h.pmu.RLock()
	for _, clients := range h.players {
		for c := range clients {
			if _, ok := sent[c]; ok {
				continue
			}
			sent[c] = struct{}{}
			if c.trySend(data) {
				wsMessagesDelivered.WithLabelValues(string(sharedws.EventBroadcast)).Inc()
			} else {
				slog.Warn("hub: dropping slow client on broadcast")
			}
		}
	}
	h.pmu.RUnlock()
}

// SendDirect delivers data directly to a client's send channel.
// Used for presence snapshots on connect — bypasses Redis.
// Silently drops on closed/full channels (best-effort).
func (h *Hub) SendDirect(c *Client, data []byte) {
	c.trySendOrDrop(data)
}

// gracefulShutdown closes all client connections cleanly.
// Called on service shutdown.
func (h *Hub) gracefulShutdown(timeout time.Duration) {
	deadline := time.Now().Add(timeout)
	_ = deadline
	h.mu.Lock()
	for _, clients := range h.rooms {
		for c := range clients {
			c.closeSend()
		}
	}
	h.mu.Unlock()

	h.pmu.Lock()
	for _, clients := range h.players {
		for c := range clients {
			c.closeSend()
		}
	}
	h.pmu.Unlock()
}

var wsMessagesDelivered = promauto.NewCounterVec(prometheus.CounterOpts{
	Name: "recess_ws_messages_delivered_total",
	Help: "Total WebSocket messages delivered to clients.",
}, []string{"event_type"})
