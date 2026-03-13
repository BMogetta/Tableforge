package queue_test

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/tableforge/server/internal/platform/queue"
	"github.com/tableforge/server/internal/platform/store"
	"github.com/tableforge/server/internal/testutil"
)

// ---------------------------------------------------------------------------
// Test helpers
// ---------------------------------------------------------------------------

func newTestService(t *testing.T) (*queue.Service, *miniredis.Miniredis, *testutil.FakeStore) {
	t.Helper()
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	t.Cleanup(mr.Close)

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { rdb.Close() })

	fs := testutil.NewFakeStore()
	svc := queue.New(rdb, fs, nil, nil, nil)
	return svc, mr, fs
}

func newPlayerID(t *testing.T) uuid.UUID {
	t.Helper()
	return uuid.New()
}

// seedRating sets a player's MMR in the fake store.
func seedRating(fs *testutil.FakeStore, playerID uuid.UUID, mmr float64) {
	fs.Ratings[playerID] = store.Rating{
		PlayerID:      playerID,
		MMR:           mmr,
		DisplayRating: mmr,
	}
}

// ---------------------------------------------------------------------------
// Enqueue
// ---------------------------------------------------------------------------

func TestEnqueue_Success(t *testing.T) {
	svc, _, _ := newTestService(t)
	ctx := context.Background()
	playerID := newPlayerID(t)

	pos, err := svc.Enqueue(ctx, playerID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pos.Position != 1 {
		t.Errorf("expected position=1, got %d", pos.Position)
	}
}

func TestEnqueue_UsesRatingMMR(t *testing.T) {
	svc, mr, fs := newTestService(t)
	ctx := context.Background()
	playerID := newPlayerID(t)
	seedRating(fs, playerID, 1800.0)

	_, err := svc.Enqueue(ctx, playerID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify the score in the sorted set equals the player's MMR.
	score, err := mr.ZScore("queue:ranked", playerID.String())
	if err != nil {
		t.Fatalf("zscore: %v", err)
	}
	if score != 1800.0 {
		t.Errorf("expected score=1800, got %.1f", score)
	}
}

func TestEnqueue_DefaultMMRWhenNoRating(t *testing.T) {
	svc, mr, _ := newTestService(t)
	ctx := context.Background()
	playerID := newPlayerID(t)

	_, err := svc.Enqueue(ctx, playerID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	score, err := mr.ZScore("queue:ranked", playerID.String())
	if err != nil {
		t.Fatalf("zscore: %v", err)
	}
	if score != 1500.0 {
		t.Errorf("expected default MMR=1500, got %.1f", score)
	}
}

func TestEnqueue_AlreadyQueued(t *testing.T) {
	svc, _, _ := newTestService(t)
	ctx := context.Background()
	playerID := newPlayerID(t)

	if _, err := svc.Enqueue(ctx, playerID); err != nil {
		t.Fatalf("first enqueue: %v", err)
	}

	_, err := svc.Enqueue(ctx, playerID)
	if err == nil {
		t.Fatal("expected ErrAlreadyQueued, got nil")
	}
	if err != queue.ErrAlreadyQueued {
		t.Errorf("expected ErrAlreadyQueued, got %v", err)
	}
}

func TestEnqueue_BannedPlayer(t *testing.T) {
	svc, mr, _ := newTestService(t)
	ctx := context.Background()
	playerID := newPlayerID(t)

	// Manually set a ban key in Redis.
	mr.Set("queue:ban:"+playerID.String(), "1")
	mr.SetTTL("queue:ban:"+playerID.String(), 10*time.Minute)

	_, err := svc.Enqueue(ctx, playerID)
	if err == nil {
		t.Fatal("expected ErrBanned, got nil")
	}
	var banned *queue.ErrBanned
	if !isErrBanned(err, &banned) {
		t.Errorf("expected *ErrBanned, got %T: %v", err, err)
	}
	if banned.RetryAfterSecs <= 0 {
		t.Errorf("expected RetryAfterSecs > 0, got %d", banned.RetryAfterSecs)
	}
}

// isErrBanned checks whether err is *queue.ErrBanned and sets target.
func isErrBanned(err error, target **queue.ErrBanned) bool {
	if b, ok := err.(*queue.ErrBanned); ok {
		*target = b
		return true
	}
	return false
}

// ---------------------------------------------------------------------------
// Dequeue
// ---------------------------------------------------------------------------

func TestDequeue_RemovesPlayer(t *testing.T) {
	svc, mr, _ := newTestService(t)
	ctx := context.Background()
	playerID := newPlayerID(t)

	if _, err := svc.Enqueue(ctx, playerID); err != nil {
		t.Fatalf("enqueue: %v", err)
	}

	removed, err := svc.Dequeue(ctx, playerID)
	if err != nil {
		t.Fatalf("dequeue: %v", err)
	}
	if !removed {
		t.Error("expected removed=true")
	}

	// Verify the player is no longer in the sorted set.
	members, _ := mr.ZMembers("queue:ranked")
	for _, m := range members {
		if m == playerID.String() {
			t.Error("player still present in sorted set after dequeue")
		}
	}
}

func TestDequeue_NotQueued_NoOp(t *testing.T) {
	svc, _, _ := newTestService(t)
	ctx := context.Background()
	playerID := newPlayerID(t)

	removed, err := svc.Dequeue(ctx, playerID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if removed {
		t.Error("expected removed=false for non-queued player")
	}
}

func TestDequeue_ClearsMetadata(t *testing.T) {
	svc, mr, _ := newTestService(t)
	ctx := context.Background()
	playerID := newPlayerID(t)

	if _, err := svc.Enqueue(ctx, playerID); err != nil {
		t.Fatalf("enqueue: %v", err)
	}

	if _, err := svc.Dequeue(ctx, playerID); err != nil {
		t.Fatalf("dequeue: %v", err)
	}

	// Metadata hash should be gone.
	if mr.Exists("queue:meta:" + playerID.String()) {
		t.Error("expected metadata to be deleted after dequeue")
	}
}

// ---------------------------------------------------------------------------
// BanStatus
// ---------------------------------------------------------------------------

func TestBanStatus_NotBanned(t *testing.T) {
	svc, _, _ := newTestService(t)
	ctx := context.Background()
	playerID := newPlayerID(t)

	info, err := svc.BanStatus(ctx, playerID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info.Banned {
		t.Error("expected Banned=false for fresh player")
	}
}

func TestBanStatus_Banned(t *testing.T) {
	svc, mr, _ := newTestService(t)
	ctx := context.Background()
	playerID := newPlayerID(t)

	mr.Set("queue:ban:"+playerID.String(), "1")
	mr.SetTTL("queue:ban:"+playerID.String(), 5*time.Minute)

	info, err := svc.BanStatus(ctx, playerID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !info.Banned {
		t.Error("expected Banned=true")
	}
	if info.RetryAfterSecs <= 0 {
		t.Errorf("expected RetryAfterSecs > 0, got %d", info.RetryAfterSecs)
	}
}

// ---------------------------------------------------------------------------
// Ban duration formula
// ---------------------------------------------------------------------------

func TestBanDurationForOffense(t *testing.T) {
	cases := []struct {
		offense     int
		wantMinutes float64
	}{
		{1, 5},
		{2, 25},
		{3, 125},
		{4, 625},
		{5, 1440}, // capped at BanMaxMinutes
	}

	for _, tc := range cases {
		got := queue.BanDurationForOffense(tc.offense)
		wantDur := time.Duration(tc.wantMinutes) * time.Minute
		if got != wantDur {
			t.Errorf("offense=%d: expected %v, got %v", tc.offense, wantDur, got)
		}
	}
}

// ---------------------------------------------------------------------------
// Accept
// ---------------------------------------------------------------------------

func TestAccept_MatchNotFound(t *testing.T) {
	svc, _, _ := newTestService(t)
	ctx := context.Background()

	err := svc.Accept(ctx, newPlayerID(t), uuid.New())
	if err != queue.ErrMatchNotFound {
		t.Errorf("expected ErrMatchNotFound, got %v", err)
	}
}

func TestAccept_NotParticipant(t *testing.T) {
	svc, mr, _ := newTestService(t)
	ctx := context.Background()

	matchID := uuid.New()
	playerA := newPlayerID(t)
	playerB := newPlayerID(t)
	outsider := newPlayerID(t)

	mr.HSet("queue:pending:"+matchID.String(),
		"player_a", playerA.String(),
		"player_b", playerB.String(),
		"accepted_a", "0",
		"accepted_b", "0",
	)

	err := svc.Accept(ctx, outsider, matchID)
	if err != queue.ErrNotMatchParticipant {
		t.Errorf("expected ErrNotMatchParticipant, got %v", err)
	}
}

func TestAccept_OnePlayer_NoMatchStarted(t *testing.T) {
	svc, mr, _ := newTestService(t)
	ctx := context.Background()

	matchID := uuid.New()
	playerA := newPlayerID(t)
	playerB := newPlayerID(t)

	mr.HSet("queue:pending:"+matchID.String(),
		"player_a", playerA.String(),
		"player_b", playerB.String(),
		"accepted_a", "0",
		"accepted_b", "0",
	)

	// Only playerA accepts — match should not start yet.
	err := svc.Accept(ctx, playerA, matchID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Pending key should still exist.
	if !mr.Exists("queue:pending:" + matchID.String()) {
		t.Error("expected pending key to still exist after one acceptance")
	}
}

// ---------------------------------------------------------------------------
// Decline
// ---------------------------------------------------------------------------

func TestDecline_MatchNotFound(t *testing.T) {
	svc, _, _ := newTestService(t)
	ctx := context.Background()

	err := svc.Decline(ctx, newPlayerID(t), uuid.New())
	if err != queue.ErrMatchNotFound {
		t.Errorf("expected ErrMatchNotFound, got %v", err)
	}
}

func TestDecline_NotParticipant(t *testing.T) {
	svc, mr, _ := newTestService(t)
	ctx := context.Background()

	matchID := uuid.New()
	playerA := newPlayerID(t)
	playerB := newPlayerID(t)
	outsider := newPlayerID(t)

	mr.HSet("queue:pending:"+matchID.String(),
		"player_a", playerA.String(),
		"player_b", playerB.String(),
		"accepted_a", "0",
		"accepted_b", "0",
	)

	err := svc.Decline(ctx, outsider, matchID)
	if err != queue.ErrNotMatchParticipant {
		t.Errorf("expected ErrNotMatchParticipant, got %v", err)
	}
}

func TestDecline_DeletesPendingKey(t *testing.T) {
	svc, mr, _ := newTestService(t)
	ctx := context.Background()

	matchID := uuid.New()
	playerA := newPlayerID(t)
	playerB := newPlayerID(t)

	mr.HSet("queue:pending:"+matchID.String(),
		"player_a", playerA.String(),
		"player_b", playerB.String(),
		"accepted_a", "0",
		"accepted_b", "0",
	)

	if err := svc.Decline(ctx, playerA, matchID); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if mr.Exists("queue:pending:" + matchID.String()) {
		t.Error("expected pending key to be deleted after decline")
	}
}

func TestDecline_RecordsPenalty(t *testing.T) {
	svc, mr, _ := newTestService(t)
	ctx := context.Background()

	playerA := newPlayerID(t)
	playerB := newPlayerID(t)

	// Decline once — no ban yet (threshold is 3).
	for i := 0; i < queue.DeclinePenaltyThreshold-1; i++ {
		matchID := uuid.New()
		mr.HSet("queue:pending:"+matchID.String(),
			"player_a", playerA.String(),
			"player_b", playerB.String(),
			"accepted_a", "0",
			"accepted_b", "0",
		)
		if err := svc.Decline(ctx, playerA, matchID); err != nil {
			t.Fatalf("decline %d: %v", i+1, err)
		}
	}

	// Not banned yet.
	if mr.Exists("queue:ban:" + playerA.String()) {
		t.Error("expected no ban before threshold reached")
	}

	// Third decline — ban should be issued.
	matchID := uuid.New()
	mr.HSet("queue:pending:"+matchID.String(),
		"player_a", playerA.String(),
		"player_b", playerB.String(),
		"accepted_a", "0",
		"accepted_b", "0",
	)
	if err := svc.Decline(ctx, playerA, matchID); err != nil {
		t.Fatalf("third decline: %v", err)
	}

	if !mr.Exists("queue:ban:" + playerA.String()) {
		t.Error("expected ban key after reaching threshold")
	}
}

func TestDecline_BannedPlayerCannotRequeue(t *testing.T) {
	svc, mr, _ := newTestService(t)
	ctx := context.Background()

	playerA := newPlayerID(t)
	playerB := newPlayerID(t)

	// Trigger a ban by hitting the threshold.
	for i := 0; i < queue.DeclinePenaltyThreshold; i++ {
		matchID := uuid.New()
		mr.HSet("queue:pending:"+matchID.String(),
			"player_a", playerA.String(),
			"player_b", playerB.String(),
			"accepted_a", "0",
			"accepted_b", "0",
		)
		svc.Decline(ctx, playerA, matchID)
	}

	_, err := svc.Enqueue(ctx, playerA)
	if err == nil {
		t.Fatal("expected ErrBanned after hitting decline threshold")
	}
	if _, ok := err.(*queue.ErrBanned); !ok {
		t.Errorf("expected *ErrBanned, got %T", err)
	}
}
