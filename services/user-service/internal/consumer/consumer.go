// Package consumer reads game events for achievement tracking.
//
// game.session.finished is consumed via Redis Streams (consumer group
// "user-service") so multiple replicas can run without re-evaluating
// the same session N times.
package consumer

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"

	"github.com/recess/services/user-service/internal/store"
	"github.com/recess/shared/achievements"
	"github.com/recess/shared/events"
	"github.com/recess/shared/featureflags"
	"github.com/recess/shared/streams"
)

const (
	streamGameSessionFinished = "game.session.finished"
	consumerGroup             = "user-service"
)

// FlagAchievementsEnabled gates achievement evaluation. When OFF, the consumer
// still drains the stream (so backlog doesn't grow) but skips any progress
// updates. Existing rows in the DB stay visible.
const FlagAchievementsEnabled = "achievements-enabled"

// AchievementPublisher publishes achievement unlock events.
type AchievementPublisher interface {
	PublishAchievementUnlocked(ctx context.Context, playerID, achievementKey string, tier int, tierName string)
}

// Consumer reads game.session.finished and evaluates achievements.
type Consumer struct {
	worker *streams.Worker
	store  store.Store
	pub    AchievementPublisher
	log    *slog.Logger
	flags  featureflags.Checker
}

// New constructs a Consumer. flags may be nil — evaluation treats an absent
// client as "flag unknown" and uses the default-on behavior. consumerName
// is the per-instance identity (typically the pod hostname) so XAUTOCLAIM
// can tell replicas apart.
func New(rdb *redis.Client, st store.Store, pub AchievementPublisher, log *slog.Logger, flags featureflags.Checker, consumerName string) *Consumer {
	c := &Consumer{store: st, pub: pub, log: log, flags: flags}
	c.worker = streams.NewWorker(
		rdb,
		streamGameSessionFinished,
		consumerGroup,
		consumerName,
		c.handle,
		streams.WithLogger(log),
	)
	return c
}

// Run blocks until ctx is cancelled.
func (c *Consumer) Run(ctx context.Context) error {
	c.log.Info("achievement consumer starting", "stream", streamGameSessionFinished, "group", consumerGroup)
	return c.worker.Run(ctx)
}

func (c *Consumer) handle(ctx context.Context, payload []byte) error {
	var evt events.GameSessionFinished
	if err := json.Unmarshal(payload, &evt); err != nil {
		return fmt.Errorf("unmarshal GameSessionFinished: %w", err)
	}

	// Flag OFF → drop the event on the floor (already decoded, so parse
	// errors still surface). Players will not get credit for this session;
	// previously recorded achievements stay intact.
	if c.flags != nil && !c.flags.IsEnabled(FlagAchievementsEnabled, true) {
		c.log.Debug("achievements-enabled is OFF; skipping", "session_id", evt.SessionID)
		return nil
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
