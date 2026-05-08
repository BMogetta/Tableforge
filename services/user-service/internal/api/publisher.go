package api

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	"github.com/redis/go-redis/v9"

	"github.com/recess/services/user-service/internal/store"
	"github.com/recess/shared/events"
	"github.com/recess/shared/streams"
)

const (
	channelPlayerUnbanned = "player.unbanned"
	channelBroadcastSent  = "admin.broadcast.sent"

	// streamPlayerBanned is the Streams stream name. Migrated from Pub/Sub
	// in P3.6 Phase 2 so auth/match/notification can scale horizontally.
	streamPlayerBanned = "player.banned"

	// QueueNotification is the Asynq queue carrying notification-service
	// tasks (friendship.*, achievement.unlocked). Migrated from Pub/Sub
	// in P3.6 Phase 3b.
	QueueNotification = "notification"

	TaskFriendshipRequested = "notification:friendship-requested"
	TaskFriendshipAccepted  = "notification:friendship-accepted"
	TaskAchievementUnlocked = "notification:achievement-unlocked"
)

// Publisher publishes user-service domain events. Routing per event:
//
//   - player.banned        → Streams (consumer groups: auth, match, notification)
//   - friendship.requested → Asynq queue "notification" (notification-service)
//   - friendship.accepted  → Asynq queue "notification" (notification-service)
//   - achievement.unlocked → Asynq queue "notification" (notification-service)
//   - admin.broadcast.sent → Pub/Sub (ws-gateway; moves to Asynq in 3c)
//   - player.unbanned      → Pub/Sub (no consumer wired today)
type Publisher struct {
	rdb         *redis.Client
	producer    *streams.Producer
	asynqClient *asynq.Client
}

func NewPublisher(rdb *redis.Client, asynqClient *asynq.Client) *Publisher {
	var prod *streams.Producer
	if rdb != nil {
		prod = streams.NewProducer(rdb)
	}
	return &Publisher{rdb: rdb, producer: prod, asynqClient: asynqClient}
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
	if p.producer == nil {
		return
	}
	if _, err := p.producer.Publish(ctx, streamPlayerBanned, evt); err != nil {
		slog.Error("failed to xadd player.banned", "error", err)
	}
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
		Meta:              newMeta(),
		RequesterID:       requesterID,
		RequesterUsername: requesterUsername,
		AddresseeID:       addresseeID,
	}
	p.enqueue(ctx, TaskFriendshipRequested, evt)
}

func (p *Publisher) PublishFriendshipAccepted(ctx context.Context, f store.Friendship) {
	evt := events.FriendshipAccepted{
		Meta:        newMeta(),
		RequesterID: f.RequesterID.String(),
		AddresseeID: f.AddresseeID.String(),
	}
	p.enqueue(ctx, TaskFriendshipAccepted, evt)
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

func (p *Publisher) PublishAchievementUnlocked(ctx context.Context, playerID, achievementKey string, tier int, tierName string) {
	evt := events.AchievementUnlocked{
		Meta:           newMeta(),
		PlayerID:       playerID,
		AchievementKey: achievementKey,
		Tier:           tier,
		TierName:       tierName,
	}
	p.enqueue(ctx, TaskAchievementUnlocked, evt)
}

func (p *Publisher) publish(ctx context.Context, channel string, evt any) {
	b, err := json.Marshal(evt)
	if err != nil {
		slog.Error("failed to marshal event", "channel", channel, "error", err)
		return
	}
	if p.rdb == nil {
		return
	}
	if err := p.rdb.Publish(ctx, channel, b).Err(); err != nil {
		slog.Error("failed to publish event", "channel", channel, "error", err)
	}
}

func (p *Publisher) enqueue(_ context.Context, taskType string, evt any) {
	b, err := json.Marshal(evt)
	if err != nil {
		slog.Error("failed to marshal event", "task_type", taskType, "error", err)
		return
	}
	if p.asynqClient == nil {
		return
	}
	task := asynq.NewTask(taskType, b)
	if _, err := p.asynqClient.Enqueue(task,
		asynq.Queue(QueueNotification),
		asynq.MaxRetry(5),
		asynq.Retention(24*time.Hour),
	); err != nil {
		slog.Error("failed to enqueue task", "task_type", taskType, "error", err)
	}
}

func newMeta() events.Meta {
	return events.Meta{
		EventID:    uuid.New().String(),
		OccurredAt: time.Now().UTC(),
		Version:    1,
	}
}
