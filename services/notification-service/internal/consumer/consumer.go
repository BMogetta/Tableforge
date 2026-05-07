// Package consumer reads events that trigger notification creation.
//
// Two transports run concurrently:
//
//   - Pub/Sub (friendship.requested, friendship.accepted, achievement.unlocked).
//     Will migrate to Asynq in P3.6 Phase 3b — the consuming side is
//     point-to-point so a task queue is the right fit.
//
//   - Streams (player.banned). Migrated in P3.6 Phase 2 because three
//     services consume this event (auth, match, notification); Streams
//     consumer groups give natural fan-out without re-publishing.
package consumer

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"

	"github.com/recess/notification-service/internal/store"
	"github.com/recess/shared/events"
	"github.com/recess/shared/streams"
	sharedws "github.com/recess/shared/ws"
)

const (
	channelFriendshipRequested = "friendship.requested"
	channelFriendshipAccepted  = "friendship.accepted"
	channelAchievementUnlocked = "achievement.unlocked"

	streamPlayerBanned = "player.banned"
	consumerGroup      = "notification-service"

	dedupeKeyPrefix = "dedup:notification-service:"
	dedupeTTL       = 24 * time.Hour
)

// NotificationCreator persists notifications. Implemented by *store.Store.
type NotificationCreator interface {
	Create(ctx context.Context, p store.CreateParams) (store.Notification, error)
}

// Publisher is the interface used to deliver real-time WS events.
type Publisher interface {
	PublishToPlayer(ctx context.Context, playerID uuid.UUID, event sharedws.Event)
}

// Consumer drives notification creation off both Pub/Sub and Streams.
type Consumer struct {
	rdb       *redis.Client
	store     NotificationCreator
	pub       Publisher
	log       *slog.Logger
	banWorker *streams.Worker
}

func New(rdb *redis.Client, st NotificationCreator, pub Publisher, log *slog.Logger, consumerName string) *Consumer {
	c := &Consumer{rdb: rdb, store: st, pub: pub, log: log}
	if rdb != nil {
		c.banWorker = streams.NewWorker(
			rdb,
			streamPlayerBanned,
			consumerGroup,
			consumerName,
			c.handlePlayerBanned,
			streams.WithLogger(log),
		)
	}
	return c
}

// Run blocks until ctx is cancelled. Launches the legacy Pub/Sub loop and
// the player.banned Streams worker concurrently.
func (c *Consumer) Run(ctx context.Context) error {
	if c.banWorker == nil {
		return c.runPubSub(ctx)
	}

	var wg sync.WaitGroup
	errCh := make(chan error, 2)

	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := c.runPubSub(ctx); err != nil {
			errCh <- fmt.Errorf("pubsub: %w", err)
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := c.banWorker.Run(ctx); err != nil {
			errCh <- fmt.Errorf("ban worker: %w", err)
		}
	}()

	wg.Wait()
	close(errCh)
	for err := range errCh {
		if err != nil {
			return err
		}
	}
	return nil
}

func (c *Consumer) runPubSub(ctx context.Context) error {
	sub := c.rdb.Subscribe(ctx, channelFriendshipRequested, channelFriendshipAccepted, channelAchievementUnlocked)
	defer sub.Close()

	ch := sub.Channel()
	c.log.Info("subscribed to Redis channels",
		"channels", []string{channelFriendshipRequested, channelFriendshipAccepted, channelAchievementUnlocked},
	)

	for {
		select {
		case <-ctx.Done():
			return nil
		case msg, ok := <-ch:
			if !ok {
				return errors.New("redis subscription channel closed unexpectedly")
			}
			if err := c.handlePubSub(ctx, msg); err != nil {
				c.log.Error("failed to handle event",
					"channel", msg.Channel,
					"error", err,
				)
			}
		}
	}
}

func (c *Consumer) handlePubSub(ctx context.Context, msg *redis.Message) error {
	switch msg.Channel {
	case channelFriendshipRequested:
		return c.handleFriendshipRequested(ctx, msg.Payload)
	case channelFriendshipAccepted:
		return c.handleFriendshipAccepted(ctx, msg.Payload)
	case channelAchievementUnlocked:
		return c.handleAchievementUnlocked(ctx, msg.Payload)
	default:
		return fmt.Errorf("unknown channel: %s", msg.Channel)
	}
}

func (c *Consumer) handleFriendshipRequested(ctx context.Context, payload string) error {
	var evt events.FriendshipRequested
	if err := json.Unmarshal([]byte(payload), &evt); err != nil {
		return fmt.Errorf("unmarshal FriendshipRequested: %w", err)
	}

	addresseeID, err := uuid.Parse(evt.AddresseeID)
	if err != nil {
		return fmt.Errorf("invalid addressee_id: %w", err)
	}

	if seen, err := c.seenEvent(ctx, evt.EventID); err != nil {
		c.log.Warn("dedupe check failed, processing event", "event_id", evt.EventID, "error", err)
	} else if seen {
		c.log.Info("skipping duplicate friendship.requested", "event_id", evt.EventID)
		return nil
	}

	p := store.PayloadFriendRequest{
		FromPlayerID: evt.RequesterID,
		FromUsername: evt.RequesterUsername,
	}
	payloadJSON, err := json.Marshal(p)
	if err != nil {
		return fmt.Errorf("marshal PayloadFriendRequest: %w", err)
	}

	expires := time.Now().Add(7 * 24 * time.Hour)
	n, err := c.store.Create(ctx, store.CreateParams{
		PlayerID:        addresseeID,
		Type:            store.NotificationTypeFriendRequest,
		Payload:         payloadJSON,
		ActionExpiresAt: &expires,
		SourceEventID:   parseSourceEventID(evt.EventID),
	})
	if err != nil {
		if errors.Is(err, store.ErrDuplicateNotification) {
			c.log.Info("friend_request notification already exists, skipping", "event_id", evt.EventID)
			return nil
		}
		return fmt.Errorf("create friend_request notification: %w", err)
	}

	c.pub.PublishToPlayer(ctx, addresseeID, sharedws.Event{
		Type:    sharedws.EventNotificationReceived,
		Payload: n,
	})

	c.log.Debug("sent friend_request notification",
		"event_id", evt.EventID,
		"requester_id", evt.RequesterID,
		"addressee_id", addresseeID,
	)
	return nil
}

func (c *Consumer) handleFriendshipAccepted(ctx context.Context, payload string) error {
	var evt events.FriendshipAccepted
	if err := json.Unmarshal([]byte(payload), &evt); err != nil {
		return fmt.Errorf("unmarshal FriendshipAccepted: %w", err)
	}

	requesterID, err := uuid.Parse(evt.RequesterID)
	if err != nil {
		return fmt.Errorf("invalid requester_id: %w", err)
	}
	addresseeID, err := uuid.Parse(evt.AddresseeID)
	if err != nil {
		return fmt.Errorf("invalid addressee_id: %w", err)
	}

	if seen, err := c.seenEvent(ctx, evt.EventID); err != nil {
		c.log.Warn("dedupe check failed, processing event", "event_id", evt.EventID, "error", err)
	} else if seen {
		c.log.Info("skipping duplicate friendship.accepted", "event_id", evt.EventID)
		return nil
	}

	p := store.PayloadFriendRequestAccepted{
		FromPlayerID: addresseeID.String(),
		FromUsername: evt.AddresseeUsername,
	}
	payloadJSON, err := json.Marshal(p)
	if err != nil {
		return fmt.Errorf("marshal PayloadFriendRequestAccepted: %w", err)
	}

	n, err := c.store.Create(ctx, store.CreateParams{
		PlayerID:      requesterID,
		Type:          store.NotificationTypeFriendRequestAccepted,
		Payload:       payloadJSON,
		SourceEventID: parseSourceEventID(evt.EventID),
	})
	if err != nil {
		if errors.Is(err, store.ErrDuplicateNotification) {
			c.log.Info("friend_request_accepted notification already exists, skipping", "event_id", evt.EventID)
			return nil
		}
		return fmt.Errorf("create friend_request_accepted notification: %w", err)
	}

	c.pub.PublishToPlayer(ctx, requesterID, sharedws.Event{
		Type:    sharedws.EventNotificationReceived,
		Payload: n,
	})

	c.log.Debug("sent friend_request_accepted notification",
		"event_id", evt.EventID,
		"requester_id", requesterID,
		"addressee_id", addresseeID,
	)
	return nil
}

func (c *Consumer) handlePlayerBanned(ctx context.Context, payload []byte) error {
	var evt events.PlayerBanned
	if err := json.Unmarshal(payload, &evt); err != nil {
		return fmt.Errorf("unmarshal PlayerBanned: %w", err)
	}

	playerID, err := uuid.Parse(evt.PlayerID)
	if err != nil {
		return fmt.Errorf("invalid player_id: %w", err)
	}

	if seen, err := c.seenEvent(ctx, evt.EventID); err != nil {
		c.log.Warn("dedupe check failed, processing event", "event_id", evt.EventID, "error", err)
	} else if seen {
		c.log.Info("skipping duplicate player.banned", "event_id", evt.EventID)
		return nil
	}

	var expiresAt time.Time
	var durationSecs int
	if evt.ExpiresAt != nil {
		expiresAt, err = time.Parse(time.RFC3339, *evt.ExpiresAt)
		if err != nil {
			return fmt.Errorf("parse expires_at: %w", err)
		}
		durationSecs = int(time.Until(expiresAt).Seconds())
	}

	p := store.PayloadBanIssued{
		Reason:       store.BanReason(evt.Reason),
		DurationSecs: durationSecs,
		ExpiresAt:    expiresAt,
	}
	payloadJSON, err := json.Marshal(p)
	if err != nil {
		return fmt.Errorf("marshal PayloadBanIssued: %w", err)
	}

	n, err := c.store.Create(ctx, store.CreateParams{
		PlayerID:      playerID,
		Type:          store.NotificationTypeBanIssued,
		Payload:       payloadJSON,
		SourceEventID: parseSourceEventID(evt.EventID),
	})
	if err != nil {
		if errors.Is(err, store.ErrDuplicateNotification) {
			c.log.Info("ban_issued notification already exists, skipping", "event_id", evt.EventID)
			return nil
		}
		return fmt.Errorf("create ban_issued notification: %w", err)
	}

	c.pub.PublishToPlayer(ctx, playerID, sharedws.Event{
		Type:    sharedws.EventNotificationReceived,
		Payload: n,
	})

	c.log.Debug("sent ban_issued notification",
		"event_id", evt.EventID,
		"player_id", playerID,
		"reason", evt.Reason,
	)
	return nil
}

func (c *Consumer) handleAchievementUnlocked(ctx context.Context, payload string) error {
	var evt events.AchievementUnlocked
	if err := json.Unmarshal([]byte(payload), &evt); err != nil {
		return fmt.Errorf("unmarshal AchievementUnlocked: %w", err)
	}

	playerID, err := uuid.Parse(evt.PlayerID)
	if err != nil {
		return fmt.Errorf("invalid player_id: %w", err)
	}

	if seen, err := c.seenEvent(ctx, evt.EventID); err != nil {
		c.log.Warn("dedupe check failed, processing event", "event_id", evt.EventID, "error", err)
	} else if seen {
		c.log.Info("skipping duplicate achievement.unlocked", "event_id", evt.EventID)
		return nil
	}

	p := store.PayloadAchievementUnlocked{
		AchievementKey: evt.AchievementKey,
		Tier:           evt.Tier,
		TierName:       evt.TierName,
	}
	payloadJSON, err := json.Marshal(p)
	if err != nil {
		return fmt.Errorf("marshal PayloadAchievementUnlocked: %w", err)
	}

	n, err := c.store.Create(ctx, store.CreateParams{
		PlayerID:      playerID,
		Type:          store.NotificationTypeAchievementUnlocked,
		Payload:       payloadJSON,
		SourceEventID: parseSourceEventID(evt.EventID),
	})
	if err != nil {
		if errors.Is(err, store.ErrDuplicateNotification) {
			c.log.Info("achievement_unlocked notification already exists, skipping", "event_id", evt.EventID)
			return nil
		}
		return fmt.Errorf("create achievement_unlocked notification: %w", err)
	}

	c.pub.PublishToPlayer(ctx, playerID, sharedws.Event{
		Type:    sharedws.EventNotificationReceived,
		Payload: n,
	})

	c.log.Debug("sent achievement_unlocked notification",
		"event_id", evt.EventID,
		"player_id", playerID,
		"achievement_key", evt.AchievementKey,
		"tier", evt.Tier,
	)
	return nil
}

// seenEvent returns true if the event was already processed within the 24h
// dedupe window; otherwise records it. Redis errors fail open so a flaky
// Redis never swallows events — duplicates are still prevented by the
// (player_id, source_event_id) unique index in the notifications table.
func (c *Consumer) seenEvent(ctx context.Context, eventID string) (bool, error) {
	if eventID == "" || c.rdb == nil {
		return false, nil
	}
	res, err := c.rdb.SetArgs(ctx, dedupeKeyPrefix+eventID, "1", redis.SetArgs{
		Mode: "NX",
		TTL:  dedupeTTL,
	}).Result()
	if err != nil {
		// Redis returns Nil when the key already exists (NX failed).
		if errors.Is(err, redis.Nil) {
			return true, nil
		}
		return false, err
	}
	// Set returned OK — the key was freshly inserted.
	_ = res
	return false, nil
}

// parseSourceEventID turns a string event_id into a pointer for CreateParams.
// Returns nil on empty or invalid input so the NULL constraint on
// source_event_id applies.
func parseSourceEventID(eventID string) *uuid.UUID {
	if eventID == "" {
		return nil
	}
	id, err := uuid.Parse(eventID)
	if err != nil {
		return nil
	}
	return &id
}
