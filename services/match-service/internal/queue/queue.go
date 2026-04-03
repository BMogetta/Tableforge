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
// # Multi-instance safety
//
// FindAndPropose acquires queue:lock (SET NX EX) before running FindMatches.
// Only the instance that holds the lock processes matches on each tick.
// Confirmation state and bans are fully stored in Redis, so any instance
// can handle Accept/Decline requests regardless of which instance proposed
// the match.
package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	"github.com/redis/go-redis/v9"
	"github.com/recess/shared/domain/matchmaking"
	"github.com/recess/shared/domain/rating"
	gamev1 "github.com/recess/shared/proto/game/v1"
	lobbyv1 "github.com/recess/shared/proto/lobby/v1"
	ratingv1 "github.com/recess/shared/proto/rating/v1"
	sharedws "github.com/recess/shared/ws"
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

	// DefaultRankedGameID is the default game used for ranked matchmaking.
	DefaultRankedGameID = "tictactoe"
)

// ---------------------------------------------------------------------------
// Redis key helpers
// ---------------------------------------------------------------------------

const (
	keyQueueSortedSet = "queue:ranked"           // sorted set: member=playerID, score=MMR
	keyQueueMeta      = "queue:meta:"            // hash prefix: joined_at, mmr
	keyPending        = "queue:pending:"         // hash prefix: player_a, player_b, accepted_a, accepted_b, mmr_a, mmr_b
	keyShadow         = "queue:shadow:"          // hash prefix: player_a, player_b, accepted_a, accepted_b — survives pending expiry
	keyDeclines       = "queue:declines:"        // list prefix: ISO timestamps of recent declines
	keyBan            = "queue:ban:"             // string prefix: ban expiry unix timestamp
	keyMatchmakeLock  = "queue:lock"             // SET NX EX lock for FindMatches

	// TypeMatchExpiry is the Asynq task type for pending match expiration.
	TypeMatchExpiry = "queue:match_expiry"
	asynqQueue      = "match"

	// shadowTTL is the TTL for shadow keys. Must be longer than
	// ConfirmationWindowSecs so the shadow outlives the pending record.
	shadowTTL = 60 * time.Second
)

func metaKey(playerID uuid.UUID) string      { return keyQueueMeta + playerID.String() }
func pendingKey(matchID uuid.UUID) string    { return keyPending + matchID.String() }
func shadowKey(matchID uuid.UUID) string     { return keyShadow + matchID.String() }
func expiryTaskID(matchID uuid.UUID) string  { return "queue:expiry:" + matchID.String() }
func declinesKey(playerID uuid.UUID) string  { return keyDeclines + playerID.String() }
func banKey(playerID uuid.UUID) string       { return keyBan + playerID.String() }

// ---------------------------------------------------------------------------
// Service
// ---------------------------------------------------------------------------

// Service is the ranked matchmaking queue. It is safe for concurrent use.
type Service struct {
	rdb            *redis.Client
	ratingClient   ratingv1.RatingServiceClient
	lobbyClient    lobbyv1.LobbyServiceClient
	gameClient     gamev1.GameServiceClient
	matchmaker     *matchmaking.Matchmaker
	rankedGameID   string
	asynqClient    *asynq.Client
	asynqInspector *asynq.Inspector
}

// New creates a new queue Service. gameID is the game used for ranked
// matchmaking (e.g. "tictactoe", "loveletter"). Pass DefaultRankedGameID
// if you don't need to override it.
func New(
	rdb *redis.Client,
	ratingClient ratingv1.RatingServiceClient,
	lobbyClient lobbyv1.LobbyServiceClient,
	gameClient gamev1.GameServiceClient,
	gameID string,
	asynqClient *asynq.Client,
	asynqInspector *asynq.Inspector,
) *Service {
	if gameID == "" {
		gameID = DefaultRankedGameID
	}
	return &Service{
		rdb:            rdb,
		ratingClient:   ratingClient,
		lobbyClient:    lobbyClient,
		gameClient:     gameClient,
		matchmaker:     matchmaking.NewMatchmaker(matchmaking.DefaultQueueConfig()),
		rankedGameID:   gameID,
		asynqClient:    asynqClient,
		asynqInspector: asynqInspector,
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
	_, err = s.rdb.ZScore(ctx, keyQueueSortedSet, playerID.String()).Result()
	if err == nil {
		return QueuePosition{}, ErrAlreadyQueued
	}
	if err != redis.Nil {
		return QueuePosition{}, fmt.Errorf("Enqueue: zscore: %w", err)
	}

	// Fetch current rating for MMR score via rating-service gRPC.
	mmr := rating.DefaultMMR
	resp, err := s.ratingClient.GetRating(ctx, &ratingv1.GetRatingRequest{
		PlayerId: playerID.String(),
		GameId:   s.rankedGameID,
	})
	if err == nil {
		mmr = resp.Mmr
	}

	now := time.Now()

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
// and match_ready is published to both players via Redis.
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

	pipe := s.rdb.Pipeline()
	pipe.HSet(ctx, key, acceptField, "1")
	pipe.HSet(ctx, shadowKey(matchID), acceptField, "1")
	if _, err := pipe.Exec(ctx); err != nil {
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

	// Extract MMRs before deleting the pending record.
	mmrA, _ := strconv.ParseFloat(fields["mmr_a"], 64)
	mmrB, _ := strconv.ParseFloat(fields["mmr_b"], 64)

	// Both accepted — delete pending + shadow and cancel expiry task
	// before starting the match to prevent double-processing.
	deleted, err := s.rdb.Del(ctx, key, shadowKey(matchID)).Result()
	if err != nil || deleted == 0 {
		// Another instance already processed this — no-op.
		return nil
	}
	s.cancelExpiry(matchID)

	return s.startMatch(ctx, playerA, playerB, mmrA, mmrB)
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

	// Delete the pending + shadow records and cancel expiry task.
	deleted, err := s.rdb.Del(ctx, key, shadowKey(matchID)).Result()
	if err != nil || deleted == 0 {
		return nil // already resolved
	}
	s.cancelExpiry(matchID)

	// Re-queue the other player if they had already accepted.
	other := playerB
	otherAcceptField := "accepted_b"
	if playerID == playerB {
		other = playerA
		otherAcceptField = "accepted_a"
	}
	if fields[otherAcceptField] == "1" {
		if _, err := s.Enqueue(ctx, other); err != nil {
			slog.Error("queue: re-enqueue player after partner decline", "player_id", other, "error", err)
		} else {
			s.publishToPlayer(ctx, other, sharedws.Event{
				Type:    sharedws.EventQueueJoined,
				Payload: map[string]any{"reason": "opponent_declined"},
			})
		}
	}

	// Apply penalty to the declining player.
	if err := s.recordDecline(ctx, playerID); err != nil {
		slog.Error("queue: recordDecline", "player_id", playerID, "error", err)
	}

	s.publishToPlayer(ctx, playerID, sharedws.Event{
		Type:    sharedws.EventQueueLeft,
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
		slog.Error("queue: FindAndPropose: zrange", "error", err)
		return
	}
	if len(members) < 2 {
		return
	}

	// Rebuild the in-memory matchmaker from the Redis snapshot.
	mm := matchmaking.NewMatchmaker(matchmaking.DefaultQueueConfig())

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

		p := rating.NewPlayer(playerIDStr)
		p.MMR = mmr
		mm.EnqueueAt(p, joinedAt)
	}

	matches := mm.FindMatches(time.Now())
	for _, match := range matches {
		if len(match.Teams) != 2 || len(match.Teams[0]) != 1 || len(match.Teams[1]) != 1 {
			slog.Warn("queue: unexpected match shape, skipping")
			continue
		}

		pAStr := match.Teams[0][0].ID
		pBStr := match.Teams[1][0].ID
		mmrA := match.Teams[0][0].MMR
		mmrB := match.Teams[1][0].MMR

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

		sKey := shadowKey(matchID)
		pipe := s.rdb.Pipeline()
		pipe.HSet(ctx, key, map[string]any{
			"player_a":   pAStr,
			"player_b":   pBStr,
			"accepted_a": "0",
			"accepted_b": "0",
			"mmr_a":      strconv.FormatFloat(mmrA, 'f', -1, 64),
			"mmr_b":      strconv.FormatFloat(mmrB, 'f', -1, 64),
		})
		pipe.Expire(ctx, key, ConfirmationWindowSecs*time.Second)
		// Shadow key outlives pending — used by ListenExpiry to identify
		// who accepted vs who timed out.
		pipe.HSet(ctx, sKey, map[string]any{
			"player_a":   pAStr,
			"player_b":   pBStr,
			"accepted_a": "0",
			"accepted_b": "0",
		})
		pipe.Expire(ctx, sKey, shadowTTL)
		// Remove both players from the sorted set while confirmation is pending.
		pipe.ZRem(ctx, keyQueueSortedSet, pAStr, pBStr)
		pipe.Del(ctx, metaKey(pA), metaKey(pB))
		if _, err := pipe.Exec(ctx); err != nil {
			slog.Error("queue: propose match", "match_id", matchID, "error", err)
			continue
		}

		payload := map[string]any{
			"match_id": matchID,
			"quality":  match.Quality,
			"timeout":  ConfirmationWindowSecs,
		}
		s.scheduleExpiry(matchID)

		s.publishToPlayer(ctx, pA, sharedws.Event{Type: sharedws.EventMatchFound, Payload: payload})
		s.publishToPlayer(ctx, pB, sharedws.Event{Type: sharedws.EventMatchFound, Payload: payload})

		slog.Info("queue: proposed match",
			"match_id", matchID,
			"player_a", pAStr,
			"player_b", pBStr,
			"quality", match.Quality,
		)
	}
}

// scheduleExpiry enqueues an Asynq task that fires after the confirmation
// window expires. Replaces Redis keyspace notifications.
func (s *Service) scheduleExpiry(matchID uuid.UUID) {
	payload, _ := json.Marshal(map[string]string{"match_id": matchID.String()})
	task := asynq.NewTask(TypeMatchExpiry, payload)

	// Delete any existing task first (idempotent reschedule).
	if s.asynqInspector != nil {
		_ = s.asynqInspector.DeleteTask(asynqQueue, expiryTaskID(matchID))
	}

	if s.asynqClient != nil {
		if _, err := s.asynqClient.Enqueue(task,
			asynq.TaskID(expiryTaskID(matchID)),
			asynq.ProcessIn(ConfirmationWindowSecs*time.Second),
			asynq.MaxRetry(3),
			asynq.Queue(asynqQueue),
		); err != nil {
			slog.Error("queue: schedule expiry failed", "match_id", matchID, "error", err)
		}
	}
}

// cancelExpiry removes the pending expiry task for a resolved match.
func (s *Service) cancelExpiry(matchID uuid.UUID) {
	if s.asynqInspector != nil {
		if err := s.asynqInspector.DeleteTask(asynqQueue, expiryTaskID(matchID)); err != nil && err != asynq.ErrTaskNotFound {
			slog.Error("queue: cancel expiry failed", "match_id", matchID, "error", err)
		}
	}
}

// HandleMatchExpiry is the Asynq handler for match expiry tasks.
func (s *Service) HandleMatchExpiry(_ context.Context, task *asynq.Task) error {
	var p struct {
		MatchID string `json:"match_id"`
	}
	if err := json.Unmarshal(task.Payload(), &p); err != nil {
		return fmt.Errorf("unmarshal match expiry payload: %w", err)
	}
	matchID, err := uuid.Parse(p.MatchID)
	if err != nil {
		return fmt.Errorf("invalid match_id: %w", err)
	}
	s.handleMatchExpiry(context.Background(), matchID)
	return nil
}

// handleMatchExpiry is called when a queue:pending:* key expires.
// Reads the shadow key to determine who accepted and applies targeted penalties.
func (s *Service) handleMatchExpiry(ctx context.Context, matchID uuid.UUID) {
	sKey := shadowKey(matchID)
	fields, err := s.rdb.HGetAll(ctx, sKey).Result()
	if err != nil || len(fields) == 0 {
		slog.Warn("queue: pending match expired (no shadow data)", "match_id", matchID)
		return
	}
	// Clean up shadow key now that we've read it.
	s.rdb.Del(ctx, sKey)

	playerA, _ := uuid.Parse(fields["player_a"])
	playerB, _ := uuid.Parse(fields["player_b"])
	acceptedA := fields["accepted_a"] == "1"
	acceptedB := fields["accepted_b"] == "1"

	switch {
	case acceptedA && !acceptedB:
		// A accepted, B timed out → re-queue A, penalize B
		s.requeueAfterTimeout(ctx, playerA, "opponent_timeout")
		s.penalizeTimeout(ctx, playerB)
		slog.Info("queue: match expired", "match_id", matchID, "acceptor", playerA, "timed_out", playerB)

	case !acceptedA && acceptedB:
		// B accepted, A timed out → re-queue B, penalize A
		s.requeueAfterTimeout(ctx, playerB, "opponent_timeout")
		s.penalizeTimeout(ctx, playerA)
		slog.Info("queue: match expired", "match_id", matchID, "acceptor", playerB, "timed_out", playerA)

	default:
		// Neither accepted or both somehow — no penalty, notify both
		s.publishToPlayer(ctx, playerA, sharedws.Event{
			Type:    sharedws.EventMatchCancelled,
			Payload: map[string]any{"reason": "timeout"},
		})
		s.publishToPlayer(ctx, playerB, sharedws.Event{
			Type:    sharedws.EventMatchCancelled,
			Payload: map[string]any{"reason": "timeout"},
		})
		slog.Info("queue: match expired (no acceptor)", "match_id", matchID)
	}
}

// requeueAfterTimeout re-enqueues a player who accepted but whose opponent timed out.
func (s *Service) requeueAfterTimeout(ctx context.Context, playerID uuid.UUID, reason string) {
	if _, err := s.Enqueue(ctx, playerID); err != nil {
		slog.Error("queue: re-enqueue after timeout", "player_id", playerID, "error", err)
		return
	}
	s.publishToPlayer(ctx, playerID, sharedws.Event{
		Type:    sharedws.EventQueueJoined,
		Payload: map[string]any{"reason": reason},
	})
}

// penalizeTimeout records a decline-equivalent penalty for a player who
// let a match confirmation expire without responding.
func (s *Service) penalizeTimeout(ctx context.Context, playerID uuid.UUID) {
	if err := s.recordDecline(ctx, playerID); err != nil {
		slog.Error("queue: penalize timeout", "player_id", playerID, "error", err)
	}
	s.publishToPlayer(ctx, playerID, sharedws.Event{
		Type:    sharedws.EventMatchCancelled,
		Payload: map[string]any{"reason": "timeout_penalty"},
	})
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

	offense := recentCount / DeclinePenaltyThreshold
	banDuration := BanDurationForOffense(offense)

	pipe2 := s.rdb.Pipeline()
	pipe2.Set(ctx, banKey(playerID), "1", banDuration)
	pipe2.Del(ctx, key) // reset decline list after issuing a ban
	if _, err := pipe2.Exec(ctx); err != nil {
		return fmt.Errorf("recordDecline: ban: %w", err)
	}

	slog.Warn("queue: player banned", "player_id", playerID, "duration", banDuration, "offense", offense)
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

func (s *Service) startMatch(ctx context.Context, playerA, playerB uuid.UUID, mmrA, mmrB float64) error {
	// Create a private ranked room via game-server's lobby.v1 gRPC.
	roomResp, err := s.lobbyClient.CreateRankedRoom(ctx, &lobbyv1.CreateRankedRoomRequest{
		PlayerAId: playerA.String(),
		PlayerBId: playerB.String(),
		GameId:    s.rankedGameID,
		MmrA:      mmrA,
		MmrB:      mmrB,
	})
	if err != nil {
		return fmt.Errorf("startMatch: create ranked room: %w", err)
	}

	// Start the session via game-server's game.v1 gRPC.
	sessionResp, err := s.gameClient.StartSession(ctx, &gamev1.StartSessionRequest{
		RoomId: roomResp.RoomId,
		GameId: s.rankedGameID,
		Mode:   "ranked",
	})
	if err != nil {
		return fmt.Errorf("startMatch: start session: %w", err)
	}

	payload := map[string]any{
		"room_id":    roomResp.RoomId,
		"session_id": sessionResp.SessionId,
	}
	s.publishToPlayer(ctx, playerA, sharedws.Event{Type: sharedws.EventMatchReady, Payload: payload})
	s.publishToPlayer(ctx, playerB, sharedws.Event{Type: sharedws.EventMatchReady, Payload: payload})

	slog.Info("queue: match started",
		"room_id", roomResp.RoomId,
		"session_id", sessionResp.SessionId,
		"player_a", playerA,
		"player_b", playerB,
	)
	return nil
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// queuePosition returns the 1-based position of a player in the queue.
func (s *Service) queuePosition(ctx context.Context, playerID uuid.UUID) (int, error) {
	rank, err := s.rdb.ZRank(ctx, keyQueueSortedSet, playerID.String()).Result()
	if err != nil {
		return 0, err
	}
	return int(rank) + 1, nil
}

// estimatedWait returns a rough wait estimate in seconds based on queue position.
// 10 seconds per position is a deliberately conservative placeholder.
func estimatedWait(position int) int {
	return position * 10
}

// publishToPlayer sends a WS event to a player via Redis pub/sub.
// The ws-gateway is the sole subscriber and fans out to connected clients.
func (s *Service) publishToPlayer(ctx context.Context, playerID uuid.UUID, event sharedws.Event) {
	data, err := json.Marshal(event)
	if err != nil {
		slog.Error("queue: marshal event", "player_id", playerID, "error", err)
		return
	}
	if err := s.rdb.Publish(ctx, sharedws.PlayerChannelKey(playerID), data).Err(); err != nil {
		slog.Error("queue: redis publish", "player_id", playerID, "error", err)
	}
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
