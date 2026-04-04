package api

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/recess/services/user-service/internal/store"
	"github.com/recess/shared/events"
)

const (
	channelPlayerBanned        = "player.banned"
	channelPlayerUnbanned      = "player.unbanned"
	channelFriendshipRequested = "friendship.requested"
	channelFriendshipAccepted  = "friendship.accepted"
	channelBroadcastSent       = "admin.broadcast.sent"
)

// Publisher publishes user-service domain events to Redis Pub/Sub.
type Publisher struct {
	rdb *redis.Client
}

func NewPublisher(rdb *redis.Client) *Publisher {
	return &Publisher{rdb: rdb}
}

func (p *Publisher) PublishPlayerBanned(ctx context.Context, ban store.Ban) {
	var expiresAt *string
	if ban.ExpiresAt != nil {
		s := ban.ExpiresAt.Format(time.RFC3339)
		expiresAt = &s
	}
	var reason string
	if ban.Reason != nil {
		reason = *ban.Reason
	}
	evt := events.PlayerBanned{
		Meta:      newMeta(),
		PlayerID:  ban.PlayerID.String(),
		BanID:     ban.ID.String(),
		BannedBy:  ban.BannedBy.String(),
		Reason:    reason,
		ExpiresAt: expiresAt,
	}
	p.publish(ctx, channelPlayerBanned, evt)
}

func (p *Publisher) PublishPlayerUnbanned(ctx context.Context, banID, playerID, liftedBy uuid.UUID) {
	evt := events.PlayerUnbanned{
		Meta:     newMeta(),
		PlayerID: playerID.String(),
		BanID:    banID.String(),
		LiftedBy: liftedBy.String(),
	}
	p.publish(ctx, channelPlayerUnbanned, evt)
}

func (p *Publisher) PublishFriendshipRequested(ctx context.Context, requesterID, requesterUsername, addresseeID string) {
	evt := events.FriendshipRequested{
		Meta:             newMeta(),
		RequesterID:      requesterID,
		RequesterUsername: requesterUsername,
		AddresseeID:      addresseeID,
	}
	p.publish(ctx, channelFriendshipRequested, evt)
}

func (p *Publisher) PublishFriendshipAccepted(ctx context.Context, f store.Friendship) {
	evt := events.FriendshipAccepted{
		Meta:        newMeta(),
		RequesterID: f.RequesterID.String(),
		AddresseeID: f.AddresseeID.String(),
	}
	p.publish(ctx, channelFriendshipAccepted, evt)
}

func (p *Publisher) PublishBroadcast(ctx context.Context, message, broadcastType, sentBy string) {
	evt := events.AdminBroadcastSent{
		Meta:          newMeta(),
		Message:       message,
		BroadcastType: broadcastType,
		SentBy:        sentBy,
	}
	p.publish(ctx, channelBroadcastSent, evt)
}

func (p *Publisher) publish(ctx context.Context, channel string, evt any) {
	b, err := json.Marshal(evt)
	if err != nil {
		slog.Error("failed to marshal event", "channel", channel, "error", err)
		return
	}
	if err := p.rdb.Publish(ctx, channel, b).Err(); err != nil {
		slog.Error("failed to publish event", "channel", channel, "error", err)
	}
}

func newMeta() events.Meta {
	return events.Meta{
		EventID:    uuid.New().String(),
		OccurredAt: time.Now().UTC(),
		Version:    1,
	}
}
