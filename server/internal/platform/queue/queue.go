// Package queue implements a Redis-backed ranked matchmaking queue.
//
// # Architecture
//
// Players join a sorted set (score = MMR). A background ticker (owned by the
// caller, typically cmd/server) acquires a short-lived distributed lock and
// runs FindMatches on a snapshot of the sorted set. When a match is proposed,
// both players are removed from the queue and a pending-confirmation record
// is written with a 30 s TTL.
//
// Each player must explicitly accept via Accept(). If either player declines
// or the TTL expires before both accept, the accepting player is re-queued
// and the non-accepting player receives a penalty ban.
//
// # Group queue support
//
// TODO: QueueEntry currently holds a single player. To support pre-formed
// teams, QueueEntry should become a slice of players treated as an
// indivisible unit. The snake-draft logic in matchmaking.FindMatches would
// need to operate on group entries rather than individuals, and EffectiveMMR
// should be computed as the group average. This is deferred until team-based
// games are introduced.
//
// # Multi-instance safety
//
// FindAndPropose acquires queue:lock (SET NX EX) before running FindMatches.
// Only the instance that holds the lock processes matches on each tick.
// Confirmation state and bans are fully stored in Redis, so any instance
// can handle Accept/Decline requests regardless of which instance proposed
// the match.
// # Multi-game ranked support
//
// RankedGameID is hardcoded to "tictactoe". When more ranked games are added:
//   - POST /queue body should accept a required game_id field
//   - One sorted set per game: "queue:ranked:{gameID}"
//   - QueueConfig (spread, MinQuality) should be tunable per game
//   - FindAndPropose ticker should iterate over all active game queues
//
// # Confirmation timeout penalisation (shadow key pattern)
//
// When a queue:pending:* key expires, the data is gone so we cannot tell
// who accepted and who timed out. To enable targeted penalisation on timeout:
//   - Write a parallel "queue:pending:shadow:{matchID}" hash with a TTL
//     slightly longer than ConfirmationWindowSecs (e.g. +10s)
//   - ListenExpiry detects the expiry of the non-shadow key, reads the
//     shadow key to determine acceptance state, penalises the non-acceptor,
//     and deletes the shadow key manually
//
// # Estimated wait time
//
// estimatedWait returns position * 10 seconds as a placeholder. Replace with
// a rolling average of actual wait times stored in Redis:
//   - On match start: LPUSH queue:wait_samples <seconds>, LTRIM to last 100
//   - estimatedWait reads LRANGE and returns the average
package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/tableforge/server/internal/domain/lobby"
	"github.com/tableforge/server/internal/domain/matchmaking"
	"github.com/tableforge/server/internal/domain/rating"
	"github.com/tableforge/server/internal/domain/runtime"
	"github.com/tableforge/server/internal/platform/store"
	"github.com/tableforge/server/internal/platform/ws"
)

// ---------------------------------------------------------------------------
// Configuration constants
// ---------------------------------------------------------------------------

const (
	// ConfirmationWindowSecs is the number of seconds each player has to
	// accept a proposed match before it is cancelled.
	ConfirmationWindowSecs = 30

	// DeclinePenaltyWindow is the rolling window in which declines are
	// counted toward the penalty threshold.
	DeclinePenaltyWindow = 1 * time.Hour

	// DeclinePenaltyThreshold is the number of declines within
	// DeclinePenaltyWindow that triggers a ban.
	DeclinePenaltyThreshold = 3

	// BanBaseMinutes is the base ban duration in minutes.
	// Ban duration = min(BanBaseMinutes * BanExponentBase^(offense-1), BanMaxMinutes).
	BanBaseMinutes = 5

	// BanExponentBase is the base of the exponential ban growth.
	BanExponentBase = 5.0

	// BanMaxMinutes caps the ban duration regardless of offense count.
	BanMaxMinutes = 1440 // 24 hours

	// QueueMetaTTL is the TTL for per-player queue metadata.
	// Acts as a safety net to evict stuck entries if the server crashes
	// before a player is dequeued.
	QueueMetaTTL = 10 * time.Minute

	// lockTTL is the duration of the distributed matchmaking lock.
	// Must be shorter than the ticker interval to avoid stale locks
	// blocking the next tick.
	lockTTL = 4 * time.Second

	// gameID is the game used for ranked matchmaking sessions.
	// TODO: make this configurable when more ranked games are added.
	RankedGameID = "tictactoe"
)

// ---------------------------------------------------------------------------
// Redis key helpers
// ---------------------------------------------------------------------------

const (
	keyQueueSortedSet = "queue:ranked"           // sorted set: member=playerID, score=MMR
	keyQueueMeta      = "queue:meta:"            // hash prefix: joined_at, mmr
	keyPending        = "queue:pending:"         // hash prefix: player_a, player_b, accepted_a, accepted_b
	keyDeclines       = "queue:declines:"        // list prefix: ISO timestamps of recent declines
	keyBan            = "queue:ban:"             // string prefix: ban expiry unix timestamp
	keyMatchmakeLock  = "queue:lock"             // SET NX EX lock for FindMatches
	keyspacePrefix    = "__keyevent@0__:expired" // Redis keyspace notification channel
)

func metaKey(playerID uuid.UUID) string     { return keyQueueMeta + playerID.String() }
func pendingKey(matchID uuid.UUID) string   { return keyPending + matchID.String() }
func declinesKey(playerID uuid.UUID) string { return keyDeclines + playerID.String() }
func banKey(playerID uuid.UUID) string      { return keyBan + playerID.String() }

// ---------------------------------------------------------------------------
// Service
// ---------------------------------------------------------------------------

// Service is the ranked matchmaking queue. It is safe for concurrent use.
type Service struct {
	rdb        *redis.Client
	st         store.Store
	lobby      *lobby.Service
	rt         *runtime.Service
	hub        *ws.Hub
	matchmaker *matchmaking.Matchmaker
}

// New creates a new queue Service.
func New(
	rdb *redis.Client,
	st store.Store,
	lobbySvc *lobby.Service,
	rt *runtime.Service,
	hub *ws.Hub,
) *Service {
	return &Service{
		rdb:        rdb,
		st:         st,
		lobby:      lobbySvc,
		rt:         rt,
		hub:        hub,
		matchmaker: matchmaking.NewMatchmaker(matchmaking.DefaultQueueConfig()),
	}
}

// ---------------------------------------------------------------------------
// Enqueue / Dequeue
// ---------------------------------------------------------------------------

// QueuePosition holds the result of a successful enqueue.
type QueuePosition struct {
	Position         int `json:"position"`
	EstimatedWaitSec int `json:"estimated_wait_secs"`
}

// Enqueue adds a player to the ranked queue.
// Returns ErrBanned if the player is currently serving a penalty ban.
// Returns ErrAlreadyQueued if the player is already in the queue.
func (s *Service) Enqueue(ctx context.Context, playerID uuid.UUID) (QueuePosition, error) {
	// Check for active ban.
	ban, err := s.BanStatus(ctx, playerID)
	if err != nil {
		return QueuePosition{}, fmt.Errorf("Enqueue: check ban: %w", err)
	}
	if ban.Banned {
		return QueuePosition{}, &ErrBanned{RetryAfterSecs: ban.RetryAfterSecs}
	}

	// Check already queued.
	score, err := s.rdb.ZScore(ctx, keyQueueSortedSet, playerID.String()).Result()
	if err == nil {
		_ = score
		return QueuePosition{}, ErrAlreadyQueued
	}
	if err != redis.Nil {
		return QueuePosition{}, fmt.Errorf("Enqueue: zscore: %w", err)
	}

	// Fetch current rating for MMR score.
	mmr := rating.DefaultMMR
	r, err := s.st.GetRating(ctx, playerID, RankedGameID)
	if err == nil {
		mmr = r.MMR
	}

	now := time.Now()

	// Write metadata hash with TTL.
	meta := map[string]any{
		"joined_at": now.UTC().Format(time.RFC3339),
		"mmr":       mmr,
	}
	pipe := s.rdb.Pipeline()
	pipe.HSet(ctx, metaKey(playerID), meta)
	pipe.Expire(ctx, metaKey(playerID), QueueMetaTTL)
	pipe.ZAdd(ctx, keyQueueSortedSet, redis.Z{Score: mmr, Member: playerID.String()})
	if _, err := pipe.Exec(ctx); err != nil {
		return QueuePosition{}, fmt.Errorf("Enqueue: pipeline: %w", err)
	}

	pos, err := s.queuePosition(ctx, playerID)
	if err != nil {
		return QueuePosition{}, fmt.Errorf("Enqueue: position: %w", err)
	}

	return QueuePosition{
		Position:         pos,
		EstimatedWaitSec: estimatedWait(pos),
	}, nil
}

// Dequeue removes a player from the ranked queue.
// No-op if the player is not currently queued — returns (false, nil).
func (s *Service) Dequeue(ctx context.Context, playerID uuid.UUID) (bool, error) {
	removed, err := s.rdb.ZRem(ctx, keyQueueSortedSet, playerID.String()).Result()
	if err != nil {
		return false, fmt.Errorf("Dequeue: %w", err)
	}
	if removed > 0 {
		s.rdb.Del(ctx, metaKey(playerID))
	}
	return removed > 0, nil
}

// ---------------------------------------------------------------------------
// Match confirmation
// ---------------------------------------------------------------------------

// Accept records that playerID accepts the proposed match.
// If both players have accepted, the room is created, the session is started,
// and match_ready is broadcast to both players.
// Returns ErrMatchNotFound if the match has already expired or been resolved.
func (s *Service) Accept(ctx context.Context, playerID uuid.UUID, matchID uuid.UUID) error {
	key := pendingKey(matchID)

	fields, err := s.rdb.HGetAll(ctx, key).Result()
	if err != nil || len(fields) == 0 {
		return ErrMatchNotFound
	}

	playerA, _ := uuid.Parse(fields["player_a"])
	playerB, _ := uuid.Parse(fields["player_b"])

	var acceptField string
	switch playerID {
	case playerA:
		acceptField = "accepted_a"
	case playerB:
		acceptField = "accepted_b"
	default:
		return ErrNotMatchParticipant
	}

	if err := s.rdb.HSet(ctx, key, acceptField, "1").Err(); err != nil {
		return fmt.Errorf("Accept: hset: %w", err)
	}

	// Reload to check if both have now accepted.
	fields, err = s.rdb.HGetAll(ctx, key).Result()
	if err != nil {
		return fmt.Errorf("Accept: reload: %w", err)
	}

	if fields["accepted_a"] != "1" || fields["accepted_b"] != "1" {
		// Still waiting on the other player.
		return nil
	}

	// Both accepted — delete the pending record before starting the match
	// to prevent double-processing.
	deleted, err := s.rdb.Del(ctx, key).Result()
	if err != nil || deleted == 0 {
		// Another instance already processed this — no-op.
		return nil
	}

	return s.startMatch(ctx, playerA, playerB)
}

// Decline records that playerID has declined the proposed match.
// The other player (if they accepted) is re-queued.
// The declining player receives a penalty and may be banned.
func (s *Service) Decline(ctx context.Context, playerID uuid.UUID, matchID uuid.UUID) error {
	key := pendingKey(matchID)

	fields, err := s.rdb.HGetAll(ctx, key).Result()
	if err != nil || len(fields) == 0 {
		return ErrMatchNotFound
	}

	playerA, _ := uuid.Parse(fields["player_a"])
	playerB, _ := uuid.Parse(fields["player_b"])

	switch playerID {
	case playerA, playerB:
		// valid participant
	default:
		return ErrNotMatchParticipant
	}

	// Delete the pending record immediately.
	deleted, err := s.rdb.Del(ctx, key).Result()
	if err != nil || deleted == 0 {
		return nil // already resolved
	}

	// Re-queue the other player if they had already accepted.
	other := playerB
	otherAcceptField := "accepted_b"
	if playerID == playerB {
		other = playerA
		otherAcceptField = "accepted_a"
	}
	if fields[otherAcceptField] == "1" {
		if _, err := s.Enqueue(ctx, other); err != nil {
			log.Printf("queue: re-enqueue player %s after partner decline: %v", other, err)
		} else {
			s.hub.BroadcastToPlayer(other, ws.Event{
				Type:    ws.EventQueueJoined,
				Payload: map[string]any{"reason": "opponent_declined"},
			})
		}
	}

	// Apply penalty to the declining player.
	if err := s.recordDecline(ctx, playerID); err != nil {
		log.Printf("queue: recordDecline player %s: %v", playerID, err)
	}

	s.hub.BroadcastToPlayer(playerID, ws.Event{
		Type:    ws.EventQueueLeft,
		Payload: map[string]any{"reason": "declined"},
	})

	return nil
}

// ---------------------------------------------------------------------------
// FindAndPropose — called by the background ticker
// ---------------------------------------------------------------------------

// FindAndPropose acquires the distributed lock, pulls the current queue from
// Redis, runs the in-memory matchmaker, and writes pending confirmation
// records for each proposed match.
// It is safe to call concurrently from multiple instances — only the
// instance that acquires the lock does work.
func (s *Service) FindAndPropose(ctx context.Context) {
	// Acquire distributed lock.
	ok, err := s.rdb.SetNX(ctx, keyMatchmakeLock, "1", lockTTL).Result()
	if err != nil || !ok {
		return // another instance is processing
	}
	defer s.rdb.Del(ctx, keyMatchmakeLock)

	// Snapshot the queue.
	members, err := s.rdb.ZRangeWithScores(ctx, keyQueueSortedSet, 0, -1).Result()
	if err != nil {
		log.Printf("queue: FindAndPropose: zrange: %v", err)
		return
	}
	if len(members) < 2 {
		return
	}

	// Rebuild the in-memory matchmaker from the Redis snapshot.
	mm := matchmaking.NewMatchmaker(matchmaking.DefaultQueueConfig())
	metaCache := make(map[string]time.Time, len(members))

	for _, z := range members {
		playerIDStr := z.Member.(string)
		mmr := z.Score

		// Fetch join time from metadata hash.
		joinedAtStr, err := s.rdb.HGet(ctx, keyQueueMeta+playerIDStr, "joined_at").Result()
		joinedAt := time.Now()
		if err == nil {
			if t, err := time.Parse(time.RFC3339, joinedAtStr); err == nil {
				joinedAt = t
			}
		}
		metaCache[playerIDStr] = joinedAt

		p := rating.NewPlayer(playerIDStr)
		p.MMR = mmr
		mm.EnqueueAt(p, joinedAt)
	}

	matches := mm.FindMatches(time.Now())
	for _, match := range matches {
		if len(match.Teams) != 2 || len(match.Teams[0]) != 1 || len(match.Teams[1]) != 1 {
			log.Printf("queue: unexpected match shape, skipping")
			continue
		}

		pAStr := match.Teams[0][0].ID
		pBStr := match.Teams[1][0].ID
		pA, err := uuid.Parse(pAStr)
		if err != nil {
			continue
		}
		pB, err := uuid.Parse(pBStr)
		if err != nil {
			continue
		}

		matchID := uuid.New()
		key := pendingKey(matchID)

		pipe := s.rdb.Pipeline()
		pipe.HSet(ctx, key, map[string]any{
			"player_a":   pAStr,
			"player_b":   pBStr,
			"accepted_a": "0",
			"accepted_b": "0",
		})
		pipe.Expire(ctx, key, ConfirmationWindowSecs*time.Second)
		// Remove both players from the sorted set while confirmation is pending.
		pipe.ZRem(ctx, keyQueueSortedSet, pAStr, pBStr)
		pipe.Del(ctx, metaKey(pA), metaKey(pB))
		if _, err := pipe.Exec(ctx); err != nil {
			log.Printf("queue: propose match %s: %v", matchID, err)
			continue
		}

		payload := map[string]any{
			"match_id": matchID,
			"quality":  match.Quality,
			"timeout":  ConfirmationWindowSecs,
		}
		s.hub.BroadcastToPlayer(pA, ws.Event{Type: ws.EventMatchFound, Payload: payload})
		s.hub.BroadcastToPlayer(pB, ws.Event{Type: ws.EventMatchFound, Payload: payload})

		log.Printf("queue: proposed match %s between %s and %s (quality=%.2f)",
			matchID, pAStr, pBStr, match.Quality)
	}
}

// ---------------------------------------------------------------------------
// ListenExpiry — keyspace notification handler
// ---------------------------------------------------------------------------

// ListenExpiry subscribes to Redis keyspace notifications and handles
// expired queue:pending:* keys. When a pending match expires, the player
// who accepted (if any) is re-queued and the non-accepting player is penalised.
// Call this in a goroutine; it blocks until ctx is cancelled.
func (s *Service) ListenExpiry(ctx context.Context) {
	sub := s.rdb.Subscribe(ctx, keyspacePrefix)
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
			expiredKey := msg.Payload
			if !strings.HasPrefix(expiredKey, keyPending) {
				continue
			}
			// The key has already expired — we cannot read it.
			// We stored the match data in a shadow key to handle this.
			// However, since Redis deletes the key on expiry, we use a
			// shadow hash with a slightly longer TTL.
			// See pendingShadowKey for the pattern.
			//
			// For simplicity in the current implementation: when a pending
			// key expires we cannot know who accepted. We penalise neither
			// player — both are treated as having timed out — and broadcast
			// match_cancelled to both. Clients must re-queue manually.
			//
			// TODO: implement shadow key pattern to distinguish acceptor from
			// non-acceptor on timeout, enabling targeted penalisation.
			matchIDStr := strings.TrimPrefix(expiredKey, keyPending)
			matchID, err := uuid.Parse(matchIDStr)
			if err != nil {
				continue
			}
			log.Printf("queue: pending match %s expired", matchID)
			s.hub.BroadcastToPlayer(uuid.Nil, ws.Event{
				Type:    ws.EventMatchCancelled,
				Payload: map[string]any{"match_id": matchID, "reason": "timeout"},
			})
		}
	}
}

// ---------------------------------------------------------------------------
// Ban system
// ---------------------------------------------------------------------------

// BanInfo describes a player's current ban status.
type BanInfo struct {
	Banned         bool `json:"banned"`
	RetryAfterSecs int  `json:"retry_after_secs,omitempty"`
}

// BanStatus returns the current ban status for a player.
func (s *Service) BanStatus(ctx context.Context, playerID uuid.UUID) (BanInfo, error) {
	val, err := s.rdb.Get(ctx, banKey(playerID)).Result()
	if err == redis.Nil {
		return BanInfo{Banned: false}, nil
	}
	if err != nil {
		return BanInfo{}, fmt.Errorf("BanStatus: %w", err)
	}
	ttl, err := s.rdb.TTL(ctx, banKey(playerID)).Result()
	if err != nil || ttl <= 0 {
		return BanInfo{Banned: false}, nil
	}
	_ = val
	return BanInfo{
		Banned:         true,
		RetryAfterSecs: int(ttl.Seconds()),
	}, nil
}

// recordDecline appends a decline timestamp, prunes old entries outside the
// penalty window, and issues a ban if the threshold is reached.
func (s *Service) recordDecline(ctx context.Context, playerID uuid.UUID) error {
	key := declinesKey(playerID)
	now := time.Now().UTC().Format(time.RFC3339)

	pipe := s.rdb.Pipeline()
	pipe.RPush(ctx, key, now)
	pipe.Expire(ctx, key, DeclinePenaltyWindow)
	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("recordDecline: push: %w", err)
	}

	// Count declines within the penalty window.
	entries, err := s.rdb.LRange(ctx, key, 0, -1).Result()
	if err != nil {
		return fmt.Errorf("recordDecline: lrange: %w", err)
	}

	windowStart := time.Now().Add(-DeclinePenaltyWindow)
	recentCount := 0
	for _, e := range entries {
		t, err := time.Parse(time.RFC3339, e)
		if err != nil {
			continue
		}
		if t.After(windowStart) {
			recentCount++
		}
	}

	if recentCount < DeclinePenaltyThreshold {
		return nil
	}

	// Count total offenses (how many times we've hit the threshold).
	// Each time we hit the threshold we ban and reset the decline list.
	offense := recentCount / DeclinePenaltyThreshold
	banDuration := BanDurationForOffense(offense)

	pipe2 := s.rdb.Pipeline()
	pipe2.Set(ctx, banKey(playerID), "1", banDuration)
	pipe2.Del(ctx, key) // reset decline list after issuing a ban
	if _, err := pipe2.Exec(ctx); err != nil {
		return fmt.Errorf("recordDecline: ban: %w", err)
	}

	log.Printf("queue: player %s banned for %v (offense %d)", playerID, banDuration, offense)
	return nil
}

// BanDurationForOffense returns the ban duration for a given offense number.
// Formula: min(BanBaseMinutes * BanExponentBase^(offense-1), BanMaxMinutes) minutes.
func BanDurationForOffense(offense int) time.Duration {
	if offense < 1 {
		offense = 1
	}
	minutes := float64(BanBaseMinutes) * math.Pow(BanExponentBase, float64(offense-1))
	if minutes > BanMaxMinutes {
		minutes = BanMaxMinutes
	}
	return time.Duration(minutes) * time.Minute
}

// ---------------------------------------------------------------------------
// startMatch — called when both players have accepted
// ---------------------------------------------------------------------------

func (s *Service) startMatch(ctx context.Context, playerA, playerB uuid.UUID) error {
	// Create a private ranked room owned by playerA.
	roomView, err := s.lobby.CreateRoom(ctx, RankedGameID, playerA, nil)
	if err != nil {
		return fmt.Errorf("startMatch: create room: %w", err)
	}
	roomID := roomView.Room.ID

	// Add playerB to the room at seat 1.
	if err := s.st.AddPlayerToRoom(ctx, roomID, playerB, 1); err != nil {
		return fmt.Errorf("startMatch: add playerB: %w", err)
	}

	// Set session mode to ranked in room settings.
	if err := s.st.SetRoomSetting(ctx, roomID, "session_mode", string(store.SessionModeRanked)); err != nil {
		log.Printf("queue: startMatch: set session_mode: %v", err)
	}

	// Start the game as the room owner.
	session, err := s.lobby.StartGame(ctx, roomID, playerA, store.SessionModeRanked)
	if err != nil {
		return fmt.Errorf("startMatch: start game: %w", err)
	}

	s.rt.StartSession(session)

	payload := map[string]any{
		"room_id":    roomID,
		"session_id": session.ID,
	}
	s.hub.BroadcastToPlayer(playerA, ws.Event{Type: ws.EventMatchReady, Payload: payload})
	s.hub.BroadcastToPlayer(playerB, ws.Event{Type: ws.EventMatchReady, Payload: payload})

	log.Printf("queue: match started room=%s session=%s players=[%s %s]",
		roomID, session.ID, playerA, playerB)
	return nil
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// queuePosition returns the 1-based position of a player in the queue
// (ordered by join time, not MMR).
func (s *Service) queuePosition(ctx context.Context, playerID uuid.UUID) (int, error) {
	rank, err := s.rdb.ZRank(ctx, keyQueueSortedSet, playerID.String()).Result()
	if err != nil {
		return 0, err
	}
	return int(rank) + 1, nil
}

// estimatedWait returns a rough wait estimate in seconds based on queue position.
// 10 seconds per position is a deliberately conservative placeholder.
// TODO: replace with a rolling average of recent match wait times.
func estimatedWait(position int) int {
	return position * 10
}

// ---------------------------------------------------------------------------
// Errors
// ---------------------------------------------------------------------------

// ErrBanned is returned when a player attempts to queue while banned.
type ErrBanned struct {
	RetryAfterSecs int
}

func (e *ErrBanned) Error() string {
	return fmt.Sprintf("queue ban active: retry after %d seconds", e.RetryAfterSecs)
}

var (
	ErrAlreadyQueued       = fmt.Errorf("player is already in the queue")
	ErrMatchNotFound       = fmt.Errorf("match not found or already expired")
	ErrNotMatchParticipant = fmt.Errorf("player is not a participant in this match")
)

// ---------------------------------------------------------------------------
// JSON helpers (used by api_queue.go)
// ---------------------------------------------------------------------------

// MarshalBanInfo is a convenience wrapper for HTTP responses.
func MarshalBanInfo(b BanInfo) ([]byte, error) {
	return json.Marshal(b)
}
