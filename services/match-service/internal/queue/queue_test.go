package queue

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/recess/shared/domain/matchmaking"
	ratingv1 "github.com/recess/shared/proto/rating/v1"
	"google.golang.org/grpc"
)

// errRatingClient always returns an error on GetRating.
type errRatingClient struct {
	ratingv1.RatingServiceClient
}

func (e *errRatingClient) GetRating(_ context.Context, _ *ratingv1.GetRatingRequest, _ ...grpc.CallOption) (*ratingv1.GetRatingResponse, error) {
	return nil, errors.New("rating unavailable")
}

// customRatingClient returns a configurable MMR.
type customRatingClient struct {
	ratingv1.RatingServiceClient
	mmr float64
}

func (c *customRatingClient) GetRating(_ context.Context, _ *ratingv1.GetRatingRequest, _ ...grpc.CallOption) (*ratingv1.GetRatingResponse, error) {
	return &ratingv1.GetRatingResponse{Mmr: c.mmr}, nil
}

// newQueueTestService creates a Service backed by miniredis with a stub rating client.
func newQueueTestService(t *testing.T, ratingClient ratingv1.RatingServiceClient) (*Service, *miniredis.Miniredis) {
	t.Helper()
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	t.Cleanup(mr.Close)

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { rdb.Close() })

	svc := &Service{
		rdb:          rdb,
		ratingClient: ratingClient,
		rankedGameID: DefaultRankedGameID,
	}
	return svc, mr
}

// ---------------------------------------------------------------------------
// Enqueue / Dequeue
// ---------------------------------------------------------------------------

func TestEnqueue_Success(t *testing.T) {
	svc, _ := newQueueTestService(t, &stubRatingClient{})
	ctx := context.Background()

	playerID := uuid.New()
	pos, err := svc.Enqueue(ctx, playerID)
	if err != nil {
		t.Fatalf("Enqueue: %v", err)
	}
	if pos.Position != 1 {
		t.Errorf("expected position 1, got %d", pos.Position)
	}
	if pos.EstimatedWaitSec != 10 {
		t.Errorf("expected wait 10, got %d", pos.EstimatedWaitSec)
	}
}

func TestEnqueue_AlreadyQueued(t *testing.T) {
	svc, _ := newQueueTestService(t, &stubRatingClient{})
	ctx := context.Background()

	playerID := uuid.New()
	if _, err := svc.Enqueue(ctx, playerID); err != nil {
		t.Fatalf("first Enqueue: %v", err)
	}

	_, err := svc.Enqueue(ctx, playerID)
	if !errors.Is(err, ErrAlreadyQueued) {
		t.Fatalf("expected ErrAlreadyQueued, got %v", err)
	}
}

func TestEnqueue_Banned(t *testing.T) {
	svc, mr := newQueueTestService(t, &stubRatingClient{})
	ctx := context.Background()

	playerID := uuid.New()
	// Set a ban key in Redis
	mr.Set(banKey(playerID), "1")
	mr.SetTTL(banKey(playerID), 120*time.Second)

	_, err := svc.Enqueue(ctx, playerID)
	var banned *ErrBanned
	if !errors.As(err, &banned) {
		t.Fatalf("expected ErrBanned, got %v", err)
	}
	if banned.RetryAfterSecs <= 0 {
		t.Errorf("expected positive RetryAfterSecs, got %d", banned.RetryAfterSecs)
	}
}

func TestEnqueue_RatingServiceError_UsesDefaultMMR(t *testing.T) {
	svc, _ := newQueueTestService(t, &errRatingClient{})
	ctx := context.Background()

	playerID := uuid.New()
	pos, err := svc.Enqueue(ctx, playerID)
	if err != nil {
		t.Fatalf("Enqueue: %v", err)
	}
	// Should still succeed with default MMR (1500)
	if pos.Position != 1 {
		t.Errorf("expected position 1, got %d", pos.Position)
	}

	// Verify the player was added with default MMR
	score, err := svc.rdb.ZScore(ctx, keyQueueSortedSet, playerID.String()).Result()
	if err != nil {
		t.Fatalf("ZScore: %v", err)
	}
	if score != 1500.0 {
		t.Errorf("expected MMR 1500, got %f", score)
	}
}

func TestEnqueue_CustomMMR(t *testing.T) {
	svc, _ := newQueueTestService(t, &customRatingClient{mmr: 2000})
	ctx := context.Background()

	playerID := uuid.New()
	if _, err := svc.Enqueue(ctx, playerID); err != nil {
		t.Fatalf("Enqueue: %v", err)
	}

	score, err := svc.rdb.ZScore(ctx, keyQueueSortedSet, playerID.String()).Result()
	if err != nil {
		t.Fatalf("ZScore: %v", err)
	}
	if score != 2000.0 {
		t.Errorf("expected MMR 2000, got %f", score)
	}
}

func TestEnqueue_MultiplePlayersPositions(t *testing.T) {
	// Use different MMRs so positions are deterministic (sorted by score).
	svc, _ := newQueueTestService(t, &customRatingClient{mmr: 1000})
	ctx := context.Background()

	// Enqueue 3 players — all get the same MMR from the stub, but positions
	// are based on sorted set rank. With same score, order is lexicographic
	// by member. We just verify all positions are valid (>= 1) and the queue
	// has 3 members total.
	p1 := uuid.New()
	p2 := uuid.New()
	p3 := uuid.New()

	pos1, _ := svc.Enqueue(ctx, p1)
	pos2, _ := svc.Enqueue(ctx, p2)
	pos3, _ := svc.Enqueue(ctx, p3)

	if pos1.Position < 1 || pos2.Position < 1 || pos3.Position < 1 {
		t.Errorf("positions should be >= 1: got %d, %d, %d", pos1.Position, pos2.Position, pos3.Position)
	}

	// Verify the queue size is 3.
	count, err := svc.rdb.ZCard(ctx, keyQueueSortedSet).Result()
	if err != nil {
		t.Fatalf("ZCard: %v", err)
	}
	if count != 3 {
		t.Errorf("expected 3 players in queue, got %d", count)
	}
}

func TestDequeue_Success(t *testing.T) {
	svc, _ := newQueueTestService(t, &stubRatingClient{})
	ctx := context.Background()

	playerID := uuid.New()
	if _, err := svc.Enqueue(ctx, playerID); err != nil {
		t.Fatalf("Enqueue: %v", err)
	}

	removed, err := svc.Dequeue(ctx, playerID)
	if err != nil {
		t.Fatalf("Dequeue: %v", err)
	}
	if !removed {
		t.Error("expected removed=true")
	}

	// Verify player is no longer in queue
	_, err = svc.rdb.ZScore(ctx, keyQueueSortedSet, playerID.String()).Result()
	if !errors.Is(err, redis.Nil) {
		t.Error("player should not be in queue after dequeue")
	}

	// Meta key should be cleaned up
	exists, _ := svc.rdb.Exists(ctx, metaKey(playerID)).Result()
	if exists != 0 {
		t.Error("meta key should be deleted after dequeue")
	}
}

func TestDequeue_NotQueued(t *testing.T) {
	svc, _ := newQueueTestService(t, &stubRatingClient{})
	ctx := context.Background()

	removed, err := svc.Dequeue(ctx, uuid.New())
	if err != nil {
		t.Fatalf("Dequeue: %v", err)
	}
	if removed {
		t.Error("expected removed=false for player not in queue")
	}
}

// ---------------------------------------------------------------------------
// BanStatus
// ---------------------------------------------------------------------------

func TestBanStatus_NotBanned(t *testing.T) {
	svc, _ := newQueueTestService(t, &stubRatingClient{})
	ctx := context.Background()

	ban, err := svc.BanStatus(ctx, uuid.New())
	if err != nil {
		t.Fatalf("BanStatus: %v", err)
	}
	if ban.Banned {
		t.Error("expected not banned")
	}
	if ban.RetryAfterSecs != 0 {
		t.Errorf("expected 0 RetryAfterSecs, got %d", ban.RetryAfterSecs)
	}
}

func TestBanStatus_Banned(t *testing.T) {
	svc, mr := newQueueTestService(t, &stubRatingClient{})
	ctx := context.Background()

	playerID := uuid.New()
	mr.Set(banKey(playerID), "1")
	mr.SetTTL(banKey(playerID), 60*time.Second)

	ban, err := svc.BanStatus(ctx, playerID)
	if err != nil {
		t.Fatalf("BanStatus: %v", err)
	}
	if !ban.Banned {
		t.Error("expected banned")
	}
	if ban.RetryAfterSecs <= 0 {
		t.Errorf("expected positive RetryAfterSecs, got %d", ban.RetryAfterSecs)
	}
}

func TestBanStatus_ExpiredBan(t *testing.T) {
	svc, mr := newQueueTestService(t, &stubRatingClient{})
	ctx := context.Background()

	playerID := uuid.New()
	mr.Set(banKey(playerID), "1")
	mr.SetTTL(banKey(playerID), 1*time.Second)
	// Fast-forward miniredis so the key expires
	mr.FastForward(2 * time.Second)

	ban, err := svc.BanStatus(ctx, playerID)
	if err != nil {
		t.Fatalf("BanStatus: %v", err)
	}
	if ban.Banned {
		t.Error("expected not banned after TTL expires")
	}
}

// ---------------------------------------------------------------------------
// Accept / Decline
// ---------------------------------------------------------------------------

func TestAccept_MatchNotFound(t *testing.T) {
	svc, _ := newQueueTestService(t, &stubRatingClient{})
	ctx := context.Background()

	err := svc.Accept(ctx, uuid.New(), uuid.New())
	if !errors.Is(err, ErrMatchNotFound) {
		t.Fatalf("expected ErrMatchNotFound, got %v", err)
	}
}

func TestAccept_NotParticipant(t *testing.T) {
	svc, _ := newQueueTestService(t, &stubRatingClient{})
	ctx := context.Background()

	matchID := uuid.New()
	playerA := uuid.New()
	playerB := uuid.New()
	outsider := uuid.New()

	// Set up pending match
	svc.rdb.HSet(ctx, pendingKey(matchID), map[string]any{
		"player_a":   playerA.String(),
		"player_b":   playerB.String(),
		"accepted_a": "0",
		"accepted_b": "0",
		"mmr_a":      "1500",
		"mmr_b":      "1500",
	})

	err := svc.Accept(ctx, outsider, matchID)
	if !errors.Is(err, ErrNotMatchParticipant) {
		t.Fatalf("expected ErrNotMatchParticipant, got %v", err)
	}
}

func TestAccept_SinglePlayer(t *testing.T) {
	svc, _ := newQueueTestService(t, &stubRatingClient{})
	ctx := context.Background()

	matchID := uuid.New()
	playerA := uuid.New()
	playerB := uuid.New()

	svc.rdb.HSet(ctx, pendingKey(matchID), map[string]any{
		"player_a":   playerA.String(),
		"player_b":   playerB.String(),
		"accepted_a": "0",
		"accepted_b": "0",
		"mmr_a":      "1500",
		"mmr_b":      "1500",
	})
	// Also set shadow key
	svc.rdb.HSet(ctx, shadowKey(matchID), map[string]any{
		"player_a":   playerA.String(),
		"player_b":   playerB.String(),
		"accepted_a": "0",
		"accepted_b": "0",
	})

	// Player A accepts
	err := svc.Accept(ctx, playerA, matchID)
	if err != nil {
		t.Fatalf("Accept: %v", err)
	}

	// Verify accepted_a is set in pending key
	val, _ := svc.rdb.HGet(ctx, pendingKey(matchID), "accepted_a").Result()
	if val != "1" {
		t.Errorf("expected accepted_a=1, got %q", val)
	}

	// Pending key should still exist (waiting for player B)
	exists, _ := svc.rdb.Exists(ctx, pendingKey(matchID)).Result()
	if exists == 0 {
		t.Error("pending key should still exist after one accept")
	}
}

func TestDecline_MatchNotFound(t *testing.T) {
	svc, _ := newQueueTestService(t, &stubRatingClient{})
	ctx := context.Background()

	err := svc.Decline(ctx, uuid.New(), uuid.New())
	if !errors.Is(err, ErrMatchNotFound) {
		t.Fatalf("expected ErrMatchNotFound, got %v", err)
	}
}

func TestDecline_NotParticipant(t *testing.T) {
	svc, _ := newQueueTestService(t, &stubRatingClient{})
	ctx := context.Background()

	matchID := uuid.New()
	playerA := uuid.New()
	playerB := uuid.New()
	outsider := uuid.New()

	svc.rdb.HSet(ctx, pendingKey(matchID), map[string]any{
		"player_a":   playerA.String(),
		"player_b":   playerB.String(),
		"accepted_a": "0",
		"accepted_b": "0",
	})

	err := svc.Decline(ctx, outsider, matchID)
	if !errors.Is(err, ErrNotMatchParticipant) {
		t.Fatalf("expected ErrNotMatchParticipant, got %v", err)
	}
}

func TestDecline_RemovesPendingAndShadow(t *testing.T) {
	svc, _ := newQueueTestService(t, &stubRatingClient{})
	ctx := context.Background()

	matchID := uuid.New()
	playerA := uuid.New()
	playerB := uuid.New()

	svc.rdb.HSet(ctx, pendingKey(matchID), map[string]any{
		"player_a":   playerA.String(),
		"player_b":   playerB.String(),
		"accepted_a": "0",
		"accepted_b": "0",
	})
	svc.rdb.HSet(ctx, shadowKey(matchID), map[string]any{
		"player_a":   playerA.String(),
		"player_b":   playerB.String(),
		"accepted_a": "0",
		"accepted_b": "0",
	})

	err := svc.Decline(ctx, playerA, matchID)
	if err != nil {
		t.Fatalf("Decline: %v", err)
	}

	// Both pending and shadow should be deleted
	existsPending, _ := svc.rdb.Exists(ctx, pendingKey(matchID)).Result()
	existsShadow, _ := svc.rdb.Exists(ctx, shadowKey(matchID)).Result()
	if existsPending != 0 {
		t.Error("pending key should be deleted after decline")
	}
	if existsShadow != 0 {
		t.Error("shadow key should be deleted after decline")
	}
}

func TestDecline_RequeuesAcceptedOpponent(t *testing.T) {
	svc, _ := newQueueTestService(t, &stubRatingClient{})
	ctx := context.Background()

	matchID := uuid.New()
	playerA := uuid.New()
	playerB := uuid.New()

	// Player B had already accepted, Player A declines
	svc.rdb.HSet(ctx, pendingKey(matchID), map[string]any{
		"player_a":   playerA.String(),
		"player_b":   playerB.String(),
		"accepted_a": "0",
		"accepted_b": "1",
	})
	svc.rdb.HSet(ctx, shadowKey(matchID), map[string]any{
		"player_a":   playerA.String(),
		"player_b":   playerB.String(),
		"accepted_a": "0",
		"accepted_b": "1",
	})

	err := svc.Decline(ctx, playerA, matchID)
	if err != nil {
		t.Fatalf("Decline: %v", err)
	}

	// Player B should be re-queued
	_, err = svc.rdb.ZScore(ctx, keyQueueSortedSet, playerB.String()).Result()
	if err != nil {
		t.Errorf("player B should be re-queued after opponent decline: %v", err)
	}

	// Player A (decliner) should have a decline recorded
	declines, _ := svc.rdb.LLen(ctx, declinesKey(playerA)).Result()
	if declines != 1 {
		t.Errorf("expected 1 decline for player A, got %d", declines)
	}
}

func TestDecline_PlayerB_Declines(t *testing.T) {
	svc, _ := newQueueTestService(t, &stubRatingClient{})
	ctx := context.Background()

	matchID := uuid.New()
	playerA := uuid.New()
	playerB := uuid.New()

	// Player A had already accepted, Player B declines
	svc.rdb.HSet(ctx, pendingKey(matchID), map[string]any{
		"player_a":   playerA.String(),
		"player_b":   playerB.String(),
		"accepted_a": "1",
		"accepted_b": "0",
	})
	svc.rdb.HSet(ctx, shadowKey(matchID), map[string]any{
		"player_a":   playerA.String(),
		"player_b":   playerB.String(),
		"accepted_a": "1",
		"accepted_b": "0",
	})

	err := svc.Decline(ctx, playerB, matchID)
	if err != nil {
		t.Fatalf("Decline: %v", err)
	}

	// Player A should be re-queued
	_, err = svc.rdb.ZScore(ctx, keyQueueSortedSet, playerA.String()).Result()
	if err != nil {
		t.Errorf("player A should be re-queued after opponent decline: %v", err)
	}

	// Player B (decliner) should have a decline recorded
	declines, _ := svc.rdb.LLen(ctx, declinesKey(playerB)).Result()
	if declines != 1 {
		t.Errorf("expected 1 decline for player B, got %d", declines)
	}
}

func TestDecline_NeitherAccepted_NoRequeue(t *testing.T) {
	svc, _ := newQueueTestService(t, &stubRatingClient{})
	ctx := context.Background()

	matchID := uuid.New()
	playerA := uuid.New()
	playerB := uuid.New()

	svc.rdb.HSet(ctx, pendingKey(matchID), map[string]any{
		"player_a":   playerA.String(),
		"player_b":   playerB.String(),
		"accepted_a": "0",
		"accepted_b": "0",
	})
	svc.rdb.HSet(ctx, shadowKey(matchID), map[string]any{
		"player_a":   playerA.String(),
		"player_b":   playerB.String(),
		"accepted_a": "0",
		"accepted_b": "0",
	})

	err := svc.Decline(ctx, playerA, matchID)
	if err != nil {
		t.Fatalf("Decline: %v", err)
	}

	// Player B should NOT be re-queued (they didn't accept)
	_, err = svc.rdb.ZScore(ctx, keyQueueSortedSet, playerB.String()).Result()
	if !errors.Is(err, redis.Nil) {
		t.Error("player B should not be re-queued when they didn't accept")
	}
}

// ---------------------------------------------------------------------------
// recordDecline / Ban escalation
// ---------------------------------------------------------------------------

func TestRecordDecline_UnderThreshold_NoBan(t *testing.T) {
	svc, _ := newQueueTestService(t, &stubRatingClient{})
	ctx := context.Background()

	playerID := uuid.New()

	// Record 2 declines (threshold is 3)
	for range 2 {
		if err := svc.recordDecline(ctx, playerID); err != nil {
			t.Fatalf("recordDecline: %v", err)
		}
	}

	ban, err := svc.BanStatus(ctx, playerID)
	if err != nil {
		t.Fatalf("BanStatus: %v", err)
	}
	if ban.Banned {
		t.Error("should not be banned with only 2 declines")
	}
}

func TestRecordDecline_AtThreshold_IssuesBan(t *testing.T) {
	svc, _ := newQueueTestService(t, &stubRatingClient{})
	ctx := context.Background()

	playerID := uuid.New()

	for range DeclinePenaltyThreshold {
		if err := svc.recordDecline(ctx, playerID); err != nil {
			t.Fatalf("recordDecline: %v", err)
		}
	}

	ban, err := svc.BanStatus(ctx, playerID)
	if err != nil {
		t.Fatalf("BanStatus: %v", err)
	}
	if !ban.Banned {
		t.Error("should be banned after reaching threshold")
	}

	// Decline list should be cleared after ban
	count, _ := svc.rdb.LLen(ctx, declinesKey(playerID)).Result()
	if count != 0 {
		t.Errorf("decline list should be cleared after ban, got %d", count)
	}
}

// ---------------------------------------------------------------------------
// FindAndPropose
// ---------------------------------------------------------------------------

func TestFindAndPropose_EmptyQueue(t *testing.T) {
	svc, _ := newQueueTestService(t, &stubRatingClient{})
	ctx := context.Background()

	// Should not panic on empty queue
	svc.FindAndPropose(ctx)
}

func TestFindAndPropose_SinglePlayer(t *testing.T) {
	svc, _ := newQueueTestService(t, &stubRatingClient{})
	ctx := context.Background()

	playerID := uuid.New()
	if _, err := svc.Enqueue(ctx, playerID); err != nil {
		t.Fatalf("Enqueue: %v", err)
	}

	// Should not panic or match with a single player
	svc.FindAndPropose(ctx)

	// Player should still be in queue (no match possible)
	_, err := svc.rdb.ZScore(ctx, keyQueueSortedSet, playerID.String()).Result()
	if err != nil {
		t.Errorf("player should still be in queue: %v", err)
	}
}

func TestFindAndPropose_LockContention(t *testing.T) {
	svc, _ := newQueueTestService(t, &stubRatingClient{})
	ctx := context.Background()

	// Simulate another instance holding the lock
	svc.rdb.SetNX(ctx, keyMatchmakeLock, "1", lockTTL)

	p1 := uuid.New()
	p2 := uuid.New()
	svc.Enqueue(ctx, p1)
	svc.Enqueue(ctx, p2)

	// Should return immediately without processing
	svc.FindAndPropose(ctx)

	// Both players should still be in queue
	_, err1 := svc.rdb.ZScore(ctx, keyQueueSortedSet, p1.String()).Result()
	_, err2 := svc.rdb.ZScore(ctx, keyQueueSortedSet, p2.String()).Result()
	if err1 != nil || err2 != nil {
		t.Error("players should still be in queue when lock is held by another instance")
	}
}

func TestFindAndPropose_TwoPlayersMatch(t *testing.T) {
	svc, _ := newQueueTestService(t, &customRatingClient{mmr: 1500})
	ctx := context.Background()

	// Need to set up the matchmaker since it's nil in newQueueTestService
	svc.matchmaker = matchmaking.NewMatchmaker(matchmaking.DefaultQueueConfig())

	p1 := uuid.New()
	p2 := uuid.New()

	// Enqueue both players
	if _, err := svc.Enqueue(ctx, p1); err != nil {
		t.Fatalf("Enqueue p1: %v", err)
	}
	if _, err := svc.Enqueue(ctx, p2); err != nil {
		t.Fatalf("Enqueue p2: %v", err)
	}

	// Both should be in queue
	count, _ := svc.rdb.ZCard(ctx, keyQueueSortedSet).Result()
	if count != 2 {
		t.Fatalf("expected 2 in queue, got %d", count)
	}

	svc.FindAndPropose(ctx)

	// After matching, both should be removed from the queue
	count, _ = svc.rdb.ZCard(ctx, keyQueueSortedSet).Result()
	if count != 0 {
		t.Errorf("expected 0 in queue after match, got %d", count)
	}

	// There should be a pending match record
	keys, _ := svc.rdb.Keys(ctx, "queue:pending:*").Result()
	if len(keys) != 1 {
		t.Errorf("expected 1 pending match, got %d", len(keys))
	}

	// There should be a shadow key too
	shadowKeys, _ := svc.rdb.Keys(ctx, "queue:shadow:*").Result()
	if len(shadowKeys) != 1 {
		t.Errorf("expected 1 shadow key, got %d", len(shadowKeys))
	}
}

// ---------------------------------------------------------------------------
// New constructor
// ---------------------------------------------------------------------------

func TestNew_DefaultGameID(t *testing.T) {
	svc := New(nil, nil, nil, nil, "", nil, nil)
	if svc.rankedGameID != DefaultRankedGameID {
		t.Errorf("expected default game ID %q, got %q", DefaultRankedGameID, svc.rankedGameID)
	}
}

func TestNew_CustomGameID(t *testing.T) {
	svc := New(nil, nil, nil, nil, "rootaccess", nil, nil)
	if svc.rankedGameID != "rootaccess" {
		t.Errorf("expected game ID %q, got %q", "rootaccess", svc.rankedGameID)
	}
}

// ---------------------------------------------------------------------------
// Existing tests
// ---------------------------------------------------------------------------

func TestBanDurationForOffense(t *testing.T) {
	tests := []struct {
		offense  int
		expected time.Duration
	}{
		{1, 5 * time.Minute},
		{2, 25 * time.Minute},
		{3, 125 * time.Minute},
		{4, 625 * time.Minute},
		{5, 1440 * time.Minute}, // capped at 24h
		{6, 1440 * time.Minute}, // still capped
		{10, 1440 * time.Minute},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			got := BanDurationForOffense(tt.offense)
			if got != tt.expected {
				t.Errorf("BanDurationForOffense(%d) = %v, want %v", tt.offense, got, tt.expected)
			}
		})
	}
}

func TestBanDurationForOffense_Zero(t *testing.T) {
	// offense 0 (and negative) should be treated as offense 1 → 5 min
	for _, offense := range []int{0, -1, -100} {
		got := BanDurationForOffense(offense)
		want := 5 * time.Minute
		if got != want {
			t.Errorf("BanDurationForOffense(%d) = %v, want %v", offense, got, want)
		}
	}
}

func TestEstimatedWait(t *testing.T) {
	tests := []struct {
		position int
		expected int
	}{
		{1, 10},
		{5, 50},
		{10, 100},
		{0, 0},
	}

	for _, tt := range tests {
		got := estimatedWait(tt.position)
		if got != tt.expected {
			t.Errorf("estimatedWait(%d) = %d, want %d", tt.position, got, tt.expected)
		}
	}
}

func TestKeyHelpers(t *testing.T) {
	id := uuid.MustParse("00000000-0000-0000-0000-000000000001")

	if got := metaKey(id); got != "queue:meta:00000000-0000-0000-0000-000000000001" {
		t.Errorf("metaKey = %q", got)
	}
	if got := pendingKey(id); got != "queue:pending:00000000-0000-0000-0000-000000000001" {
		t.Errorf("pendingKey = %q", got)
	}
	if got := declinesKey(id); got != "queue:declines:00000000-0000-0000-0000-000000000001" {
		t.Errorf("declinesKey = %q", got)
	}
	if got := banKey(id); got != "queue:ban:00000000-0000-0000-0000-000000000001" {
		t.Errorf("banKey = %q", got)
	}
	if got := shadowKey(id); got != "queue:shadow:00000000-0000-0000-0000-000000000001" {
		t.Errorf("shadowKey = %q", got)
	}
	if got := expiryTaskID(id); got != "queue:expiry:00000000-0000-0000-0000-000000000001" {
		t.Errorf("expiryTaskID = %q", got)
	}
}

func TestErrBanned_Error(t *testing.T) {
	err := &ErrBanned{RetryAfterSecs: 300}
	want := "queue ban active: retry after 300 seconds"
	if got := err.Error(); got != want {
		t.Errorf("ErrBanned.Error() = %q, want %q", got, want)
	}
}
