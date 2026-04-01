package ws

import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	sharedws "github.com/recess/shared/ws"
)

// EventType and Event are re-exported from shared/ws so callers in this module
// don't need to change their import paths during the migration.
type EventType = sharedws.EventType
type Event = sharedws.Event

const (
	EventMoveApplied          = sharedws.EventMoveApplied
	EventGameOver             = sharedws.EventGameOver
	EventPlayerJoined         = sharedws.EventPlayerJoined
	EventPlayerLeft           = sharedws.EventPlayerLeft
	EventOwnerChanged         = sharedws.EventOwnerChanged
	EventRoomClosed           = sharedws.EventRoomClosed
	EventGameStarted          = sharedws.EventGameStarted
	EventRematchVote          = sharedws.EventRematchVote
	EventGameReady            = sharedws.EventGameReady
	EventPlayerReady          = sharedws.EventPlayerReady
	EventRematchReady         = sharedws.EventRematchReady
	EventSettingUpdated       = sharedws.EventSettingUpdated
	EventSpectatorJoined      = sharedws.EventSpectatorJoined
	EventSpectatorLeft        = sharedws.EventSpectatorLeft
	EventPresenceUpdated      = sharedws.EventPresenceUpdate
	EventChatMessage          = sharedws.EventChatMessage
	EventChatMessageHidden    = sharedws.EventChatMessageHidden
	EventDMReceived           = sharedws.EventDMReceived
	EventDMRead               = sharedws.EventDMRead
	EventPauseVoteUpdate      = sharedws.EventPauseVoteUpdate
	EventSessionSuspended     = sharedws.EventSessionSuspended
	EventResumeVoteUpdate     = sharedws.EventResumeVoteUpdate
	EventSessionResumed       = sharedws.EventSessionResumed
	EventQueueJoined          = sharedws.EventQueueJoined
	EventQueueLeft            = sharedws.EventQueueLeft
	EventMatchFound           = sharedws.EventMatchFound
	EventMatchCancelled       = sharedws.EventMatchCancelled
	EventMatchReady           = sharedws.EventMatchReady
	EventNotificationReceived = sharedws.EventNotificationReceived
)

// Hub publishes events to Redis pub/sub channels.
// ws-gateway is the sole subscriber — it fans out to connected WebSocket clients.
// The Hub no longer manages local client connections.
type Hub struct {
	rdb *redis.Client
}

func NewHub() *Hub {
	// NewHub without Redis is a no-op publisher — safe for tests.
	return &Hub{}
}

func NewHubWithRedis(rdb *redis.Client) *Hub {
	return &Hub{rdb: rdb}
}

// Broadcast publishes an event to all clients in a room via Redis.
func (h *Hub) Broadcast(roomID uuid.UUID, event Event) {
	if h == nil || h.rdb == nil {
		return
	}
	data, err := json.Marshal(event)
	if err != nil {
		slog.Error("ws: marshal event", "error", err)
		return
	}
	env, err := json.Marshal(sharedws.FilteredEnvelope{Data: data})
	if err != nil {
		slog.Error("ws: marshal envelope", "error", err)
		return
	}
	if err := h.rdb.Publish(context.Background(), sharedws.RoomChannelKey(roomID), env).Err(); err != nil {
		slog.Error("ws: redis publish room", "error", err)
	}
}

// BroadcastFiltered publishes playerEvent to non-spectators and spectatorEvent
// to spectators in a room.
func (h *Hub) BroadcastFiltered(roomID uuid.UUID, playerEvent Event, spectatorEvent Event) {
	if h == nil || h.rdb == nil {
		return
	}
	if playerEvent.Payload != nil {
		playerData, err := json.Marshal(playerEvent)
		if err != nil {
			slog.Error("ws: BroadcastFiltered marshal player event", "error", err)
			return
		}
		playerEnv, err := json.Marshal(sharedws.FilteredEnvelope{Data: playerData})
		if err != nil {
			slog.Error("ws: BroadcastFiltered marshal player envelope", "error", err)
			return
		}
		if err := h.rdb.Publish(context.Background(), sharedws.RoomChannelKey(roomID), playerEnv).Err(); err != nil {
			slog.Error("ws: redis publish player event", "error", err)
		}
	}
	spectatorData, err := json.Marshal(spectatorEvent)
	if err != nil {
		slog.Error("ws: BroadcastFiltered marshal spectator event", "error", err)
		return
	}
	spectatorEnv, err := json.Marshal(sharedws.FilteredEnvelope{Spectators: true, Data: spectatorData})
	if err != nil {
		slog.Error("ws: BroadcastFiltered marshal spectator envelope", "error", err)
		return
	}
	if err := h.rdb.Publish(context.Background(), sharedws.RoomChannelKey(roomID), spectatorEnv).Err(); err != nil {
		slog.Error("ws: redis publish spectator event", "error", err)
	}
}

// BroadcastToRoom publishes per-player events to each player's channel and
// the spectator event to the room channel.
func (h *Hub) BroadcastToRoom(
	roomID uuid.UUID,
	playerIDs []uuid.UUID,
	eventFn func(playerID uuid.UUID) Event,
	spectatorEvent Event,
) {
	if h == nil || h.rdb == nil {
		return
	}
	for _, playerID := range playerIDs {
		event := eventFn(playerID)
		data, err := json.Marshal(event)
		if err != nil {
			slog.Error("ws: BroadcastToRoom marshal player event", "player_id", playerID, "error", err)
			continue
		}
		if err := h.rdb.Publish(context.Background(), sharedws.PlayerChannelKey(playerID), data).Err(); err != nil {
			slog.Error("ws: BroadcastToRoom redis publish player", "player_id", playerID, "error", err)
		}
	}
	spectatorData, err := json.Marshal(spectatorEvent)
	if err != nil {
		slog.Error("ws: BroadcastToRoom marshal spectator event", "error", err)
		return
	}
	spectatorEnv, err := json.Marshal(sharedws.FilteredEnvelope{Spectators: true, Data: spectatorData})
	if err != nil {
		slog.Error("ws: BroadcastToRoom marshal spectator envelope", "error", err)
		return
	}
	if err := h.rdb.Publish(context.Background(), sharedws.RoomChannelKey(roomID), spectatorEnv).Err(); err != nil {
		slog.Error("ws: BroadcastToRoom redis publish spectator", "error", err)
	}
}
