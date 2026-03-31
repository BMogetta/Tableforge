package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

// eventType mirrors the ws.EventType constants in the monolith.
// The hub subscribes to these channels and broadcasts to WS clients.
type eventType string

const (
	eventChatMessage       eventType = "chat_message"
	eventChatMessageHidden eventType = "chat_message_hidden"
	eventDMReceived        eventType = "dm_received"
	eventDMRead            eventType = "dm_read"
)

// event is the envelope published to Redis and consumed by the WS hub.
type event struct {
	Type    eventType `json:"type"`
	Payload any       `json:"payload"`
}

// Publisher publishes chat events to Redis pub/sub channels.
// The monolith hub subscribes to ws:room:{id} and ws:player:{id}
// and fans out to connected WebSocket clients.
type Publisher struct {
	rdb *redis.Client
}

func NewPublisher(rdb *redis.Client) *Publisher {
	return &Publisher{rdb: rdb}
}

// PublishRoomEvent publishes an event to ws:room:{roomID}.
func (p *Publisher) PublishRoomEvent(ctx context.Context, roomID uuid.UUID, typ eventType, payload any) {
	p.publish(ctx, fmt.Sprintf("ws:room:%s", roomID), typ, payload)
}

// PublishPlayerEvent publishes an event to ws:player:{playerID}.
func (p *Publisher) PublishPlayerEvent(ctx context.Context, playerID uuid.UUID, typ eventType, payload any) {
	p.publish(ctx, fmt.Sprintf("ws:player:%s", playerID), typ, payload)
}

func (p *Publisher) publish(ctx context.Context, channel string, typ eventType, payload any) {
	if p.rdb == nil {
		return
	}
	ev := event{Type: typ, Payload: payload}
	data, err := json.Marshal(ev)
	if err != nil {
		slog.Error("chat: failed to marshal event", "type", typ, "error", err)
		return
	}

	// Wrap in filteredEnvelope — required by the hub's listenRoom consumer.
	// S=false means deliver to all clients (players + spectators).
	env := filteredEnvelope{S: false, Data: data}
	wrapped, err := json.Marshal(env)
	if err != nil {
		slog.Error("chat: failed to marshal envelope", "type", typ, "error", err)
		return
	}

	if err := p.rdb.Publish(ctx, channel, wrapped).Err(); err != nil {
		slog.Error("chat: failed to publish event", "channel", channel, "error", err)
	}
}

// filteredEnvelope mirrors the hub's internal envelope.
type filteredEnvelope struct {
	S    bool            `json:"s,omitempty"`
	Data json.RawMessage `json:"d"`
}
