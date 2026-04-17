// Package consumer subscribes to Redis events for achievement tracking.
package consumer

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/recess/services/user-service/internal/achievements"
	"github.com/recess/services/user-service/internal/store"
	"github.com/recess/shared/events"
)

const channelSessionFinished = "game.session.finished"

// AchievementPublisher publishes achievement unlock events.
type AchievementPublisher interface {
	PublishAchievementUnlocked(ctx context.Context, playerID, achievementKey string, tier int, tierName string)
}

// Consumer subscribes to game events and evaluates achievements.
type Consumer struct {
	rdb   *redis.Client
	store store.Store
	pub   AchievementPublisher
	log   *slog.Logger
}

func New(rdb *redis.Client, st store.Store, pub AchievementPublisher, log *slog.Logger) *Consumer {
	return &Consumer{rdb: rdb, store: st, pub: pub, log: log}
}

// Run blocks until ctx is cancelled.
func (c *Consumer) Run(ctx context.Context) error {
	sub := c.rdb.Subscribe(ctx, channelSessionFinished)
	defer sub.Close()

	ch := sub.Channel()
	c.log.Info("achievement consumer subscribed", "channel", channelSessionFinished)

	for {
		select {
		case <-ctx.Done():
			return nil
		case msg, ok := <-ch:
			if !ok {
				return errors.New("redis subscription channel closed unexpectedly")
			}
			if err := c.handleSessionFinished(ctx, msg.Payload); err != nil {
				c.log.Error("failed to handle game.session.finished",
					"error", err,
				)
			}
		}
	}
}

func (c *Consumer) handleSessionFinished(ctx context.Context, payload string) error {
	var evt events.GameSessionFinished
	if err := json.Unmarshal([]byte(payload), &evt); err != nil {
		return fmt.Errorf("unmarshal GameSessionFinished: %w", err)
	}

	for _, sp := range evt.Players {
		if err := c.evaluatePlayer(ctx, evt, sp); err != nil {
			c.log.Error("achievement evaluation failed",
				"player_id", sp.PlayerID,
				"session_id", evt.SessionID,
				"error", err,
			)
		}
	}
	return nil
}

func (c *Consumer) evaluatePlayer(ctx context.Context, evt events.GameSessionFinished, sp events.SessionPlayer) error {
	playerID, err := uuid.Parse(sp.PlayerID)
	if err != nil {
		return fmt.Errorf("invalid player_id: %w", err)
	}

	// Load current achievement state from DB.
	existing, err := c.store.ListAchievements(ctx, playerID)
	if err != nil {
		return fmt.Errorf("list achievements: %w", err)
	}

	currentState := make(map[string]achievements.Progress, len(existing))
	for _, a := range existing {
		currentState[a.AchievementKey] = achievements.Progress{
			Tier:     a.Tier,
			Progress: a.Progress,
		}
	}

	// Evaluate for unlocks. Each Definition.ComputeProgress closure derives
	// its own new-progress value from the event + outcome + cur, so no
	// per-achievement parameters need to leak into this orchestration.
	unlocks := achievements.Evaluate(evt, sp.Outcome, currentState)

	// Also update progress for all applicable definitions (even without tier-up).
	defs := achievements.ForGame(evt.GameID)
	for _, def := range defs {
		cur := currentState[def.Key]
		newProgress, changed := achievements.ComputeNewProgress(def, cur, evt, sp.Outcome)
		if !changed {
			continue
		}

		// Check if this def had an unlock.
		newTier := cur.Tier
		for _, u := range unlocks {
			if u.Key == def.Key {
				newTier = u.NewTier
				break
			}
		}

		if _, err := c.store.UpsertAchievement(ctx, playerID, def.Key, newTier, newProgress); err != nil {
			c.log.Error("upsert achievement failed",
				"player_id", sp.PlayerID,
				"key", def.Key,
				"error", err,
			)
		}
	}

	// Publish unlock events.
	for _, u := range unlocks {
		c.pub.PublishAchievementUnlocked(ctx, sp.PlayerID, u.Key, u.NewTier, u.TierName)
		c.log.Info("achievement unlocked",
			"player_id", sp.PlayerID,
			"key", u.Key,
			"tier", u.NewTier,
			"tier_name", u.TierName,
		)
	}

	return nil
}
