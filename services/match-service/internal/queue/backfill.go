// Ranked bot backfill — injects idle bots into the queue when a human has been
// waiting alone long enough that the match is unlikely to arrive organically.
//
// # Why this lives inside the matchmaker tick
//
// BackfillScan is invoked from the same goroutine that runs FindAndPropose,
// immediately after the main pairing pass. Both read the same Redis state
// (queue:ranked, queue:meta:*) and both need mutual exclusion with other
// match-service instances — the existing queue:lock already handles that,
// so sharing the tick keeps the detector simple and avoids a second lock.
//
// # Alternative: Asynq periodic task
//
// The backfill detector could also run as a standalone Asynq periodic task
// with its own cron and worker pool. That approach has benefits we did not
// need for Phase 3, but if any of these pressures show up we should migrate:
//
//   - Independent cadence: if the backfill scan should run more (or less)
//     frequently than the matchmaker tick.
//   - Automatic retries: Asynq persists tasks and retries on failure. The
//     current ticker has no retry — a panic or transient Redis error drops
//     a single tick silently.
//   - Separate observability: Asynqmon surfaces the task as its own queue
//     with duration / failure metrics, rather than lumping everything under
//     the matchmaker tick.
//   - Worker pool isolation: a slow matchmaker run would delay the next
//     backfill scan today; on Asynq they run in independent pools.
//
// The scan logic below does not care about its trigger — extracting it to
// an Asynq handler is purely a wiring change.
//
// # Bot identification
//
// The detector distinguishes humans from bots via `bot:known` (SET populated
// by bot-runner at startup). A queued ID that is *not* in bot:known is
// treated as human. `bot:available` is the subset of known bots that are
// idle and eligible for activation; a bot removes itself from the set while
// it is playing and re-adds on completion.
//
// Activation is broadcast on the `bot.activate` pub/sub channel. The payload
// is `{"player_id": "<uuid>"}` — bot-runner instances filter by their own ID.
//
// # Race model
//
// The SREM on bot:available doubles as an atomic claim — if two instances
// somehow both selected the same bot, only one SREM returns 1 and the other
// aborts. queue:lock already prevents this in the common case; the SREM is
// defense-in-depth.
package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	ratingv1 "github.com/recess/shared/proto/rating/v1"
)

// ---------------------------------------------------------------------------
// Redis keys
// ---------------------------------------------------------------------------

const (
	// keyBotKnown is the SET of every bot player UUID that has registered
	// with the service. Populated by bot-runner at startup.
	keyBotKnown = "bot:known"

	// keyBotAvailable is the SET of bot UUIDs that are currently idle and
	// eligible to be injected into the queue.
	keyBotAvailable = "bot:available"

	// ChannelBotActivate is the pub/sub channel the detector publishes on.
	// Payload: {"player_id": "<uuid>"}.
	ChannelBotActivate = "bot.activate"
)

// ---------------------------------------------------------------------------
// Config
// ---------------------------------------------------------------------------

// BackfillConfig controls the backfill detector.
type BackfillConfig struct {
	// Enabled toggles the feature. When false, BackfillScan is a no-op.
	Enabled bool

	// Threshold is how long a lone human must wait before a bot is injected.
	Threshold time.Duration

	// MaxActive caps the number of bots that can be simultaneously queued
	// or in a match. Computed as SCARD(bot:known) - SCARD(bot:available).
	MaxActive int
}

// DefaultBackfillConfig returns the recommended defaults: disabled, 15 s
// threshold, 3 concurrent bots.
func DefaultBackfillConfig() BackfillConfig {
	return BackfillConfig{
		Enabled:   false,
		Threshold: 15 * time.Second,
		MaxActive: 3,
	}
}

// SetBackfillConfig replaces the service's backfill configuration.
// Safe to call during initialization; not goroutine-safe thereafter.
func (s *Service) SetBackfillConfig(cfg BackfillConfig) {
	s.backfillCfg = cfg
}

// ---------------------------------------------------------------------------
// Scan
// ---------------------------------------------------------------------------

// activatePayload is the body published on ChannelBotActivate.
type activatePayload struct {
	PlayerID string `json:"player_id"`
}

// BackfillScan looks for a lone human who has been waiting longer than the
// configured threshold and, if one is found, picks the closest-MMR idle bot
// and publishes an activation signal.
//
// Safe to call every matchmaker tick — short-circuits if disabled, if the
// queue is empty, or if no human satisfies the wait criterion.
func (s *Service) BackfillScan(ctx context.Context) {
	cfg := s.backfillCfg
	if !cfg.Enabled {
		return
	}

	queued, err := s.rdb.ZRangeWithScores(ctx, keyQueueSortedSet, 0, -1).Result()
	if err != nil {
		slog.Error("backfill: zrange queue", "error", err)
		return
	}
	if len(queued) == 0 {
		return
	}

	knownBots, err := s.rdb.SMembers(ctx, keyBotKnown).Result()
	if err != nil {
		slog.Error("backfill: smembers known", "error", err)
		return
	}
	botSet := make(map[string]struct{}, len(knownBots))
	for _, id := range knownBots {
		botSet[id] = struct{}{}
	}

	// Classify queued entries.
	var humans []redis.Z
	var botsInQueue int
	for _, z := range queued {
		id, _ := z.Member.(string)
		if _, isBot := botSet[id]; isBot {
			botsInQueue++
			continue
		}
		humans = append(humans, z)
	}
	// Only backfill when exactly one human waits and no bot is already in.
	if len(humans) != 1 || botsInQueue > 0 {
		return
	}
	human := humans[0]
	humanIDStr, _ := human.Member.(string)
	humanID, err := uuid.Parse(humanIDStr)
	if err != nil {
		return
	}

	// Wait-time gate.
	waitedEnough, err := s.humanWaitedLongEnough(ctx, humanID, cfg.Threshold)
	if err != nil || !waitedEnough {
		return
	}

	// Capacity gate.
	availableCount, err := s.rdb.SCard(ctx, keyBotAvailable).Result()
	if err != nil || availableCount == 0 {
		return
	}
	active := int64(len(knownBots)) - availableCount
	if active >= int64(cfg.MaxActive) {
		slog.Debug("backfill: max active reached", "active", active, "max", cfg.MaxActive)
		return
	}

	// Pick the closest-MMR idle bot.
	botID, err := s.pickClosestBot(ctx, human.Score)
	if err != nil {
		slog.Error("backfill: pick bot", "error", err)
		return
	}
	if botID == uuid.Nil {
		return
	}

	// Atomic claim. SREM returning 0 means another scan beat us to it —
	// silently abort rather than risk a double-activation.
	removed, err := s.rdb.SRem(ctx, keyBotAvailable, botID.String()).Result()
	if err != nil || removed == 0 {
		return
	}

	// Publish activation. If the publish fails we re-add the bot so a later
	// tick can retry — otherwise the bot would be stuck off-queue.
	payload, _ := json.Marshal(activatePayload{PlayerID: botID.String()})
	if err := s.rdb.Publish(ctx, ChannelBotActivate, payload).Err(); err != nil {
		slog.Error("backfill: publish activate", "bot", botID, "error", err)
		_ = s.rdb.SAdd(ctx, keyBotAvailable, botID.String()).Err()
		return
	}

	slog.Info("backfill: bot activated",
		"bot", botID,
		"human", humanID,
		"human_mmr", human.Score,
		"waited_secs", cfg.Threshold.Seconds(),
	)
}

// humanWaitedLongEnough returns true if the lone human's joined_at is older
// than the configured threshold. A missing or malformed timestamp is treated
// as "not yet" to avoid injecting a bot against freshly enqueued humans whose
// metadata might not have been written.
func (s *Service) humanWaitedLongEnough(ctx context.Context, humanID uuid.UUID, threshold time.Duration) (bool, error) {
	joinedAtStr, err := s.rdb.HGet(ctx, metaKey(humanID), "joined_at").Result()
	if err == redis.Nil {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("hget joined_at: %w", err)
	}
	joinedAt, err := time.Parse(time.RFC3339, joinedAtStr)
	if err != nil {
		return false, nil
	}
	return time.Since(joinedAt) >= threshold, nil
}

// pickClosestBot scans bot:available and returns the bot whose MMR (fetched
// via rating-service) is closest to targetMMR. Returns uuid.Nil if no bot is
// available or if the rating-service is unreachable for all candidates.
//
// The rating-service call is bounded — if it fails for a specific bot we
// skip that candidate and keep looking. The matchmaker falls back to the
// default MMR when the rating-service is entirely down, which would still
// produce a usable (but suboptimal) pick here.
func (s *Service) pickClosestBot(ctx context.Context, targetMMR float64) (uuid.UUID, error) {
	available, err := s.rdb.SMembers(ctx, keyBotAvailable).Result()
	if err != nil {
		return uuid.Nil, err
	}

	var best uuid.UUID
	bestDelta := math.MaxFloat64

	for _, idStr := range available {
		id, err := uuid.Parse(idStr)
		if err != nil {
			continue
		}
		mmr, err := s.botMMR(ctx, id)
		if err != nil {
			// Keep the bot in the pool — transient rating-service errors
			// shouldn't poison a later retry. We just skip it for this pick.
			slog.Debug("backfill: rating lookup failed", "bot", id, "error", err)
			continue
		}
		delta := math.Abs(mmr - targetMMR)
		if delta < bestDelta {
			bestDelta = delta
			best = id
		}
	}
	return best, nil
}

// botMMR fetches a bot's current MMR via rating-service gRPC.
func (s *Service) botMMR(ctx context.Context, botID uuid.UUID) (float64, error) {
	if s.ratingClient == nil {
		return 0, fmt.Errorf("no rating client")
	}
	// Short timeout — we run on a 5 s tick; one rating call per bot must
	// stay well under that budget.
	callCtx, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
	defer cancel()

	resp, err := s.ratingClient.GetRating(callCtx, &ratingv1.GetRatingRequest{
		PlayerId: botID.String(),
		GameId:   s.rankedGameID,
	})
	if err != nil {
		return 0, err
	}
	return resp.Mmr, nil
}

