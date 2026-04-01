package queue

import (
	"context"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	ratingv1 "github.com/tableforge/shared/proto/rating/v1"
	"google.golang.org/grpc"
)

// stubRatingClient returns default ratings for any request.
type stubRatingClient struct {
	ratingv1.RatingServiceClient
}

func (s *stubRatingClient) GetRating(_ context.Context, _ *ratingv1.GetRatingRequest, _ ...grpc.CallOption) (*ratingv1.GetRatingResponse, error) {
	return &ratingv1.GetRatingResponse{Mmr: 1500}, nil
}

func newTestService(t *testing.T) (*Service, *miniredis.Miniredis) {
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
		ratingClient: &stubRatingClient{},
		matchmaker:   nil, // not needed for expiry tests
	}
	return svc, mr
}

func TestHandleMatchExpiry_OneAccepted(t *testing.T) {
	svc, _ := newTestService(t)
	ctx := context.Background()

	matchID := uuid.New()
	playerA := uuid.New()
	playerB := uuid.New()

	// Set up shadow key: A accepted, B did not
	svc.rdb.HSet(ctx, shadowKey(matchID), map[string]any{
		"player_a":   playerA.String(),
		"player_b":   playerB.String(),
		"accepted_a": "1",
		"accepted_b": "0",
	})

	svc.handleMatchExpiry(ctx, matchID)

	// Shadow key should be cleaned up
	exists, _ := svc.rdb.Exists(ctx, shadowKey(matchID)).Result()
	if exists != 0 {
		t.Error("shadow key should be deleted after processing")
	}

	// Player A (acceptor) should be re-queued
	score, err := svc.rdb.ZScore(ctx, keyQueueSortedSet, playerA.String()).Result()
	if err != nil {
		t.Errorf("player A should be re-queued, got error: %v", err)
	}
	if score != 1500 {
		t.Errorf("expected MMR 1500, got %f", score)
	}

	// Player B (timed out) should have a decline recorded
	declines, _ := svc.rdb.LLen(ctx, declinesKey(playerB)).Result()
	if declines != 1 {
		t.Errorf("expected 1 decline for player B, got %d", declines)
	}

	// Player A should NOT have declines
	declinesA, _ := svc.rdb.LLen(ctx, declinesKey(playerA)).Result()
	if declinesA != 0 {
		t.Errorf("player A should not have declines, got %d", declinesA)
	}
}

func TestHandleMatchExpiry_OtherAccepted(t *testing.T) {
	svc, _ := newTestService(t)
	ctx := context.Background()

	matchID := uuid.New()
	playerA := uuid.New()
	playerB := uuid.New()

	// B accepted, A did not
	svc.rdb.HSet(ctx, shadowKey(matchID), map[string]any{
		"player_a":   playerA.String(),
		"player_b":   playerB.String(),
		"accepted_a": "0",
		"accepted_b": "1",
	})

	svc.handleMatchExpiry(ctx, matchID)

	// Player B should be re-queued
	_, err := svc.rdb.ZScore(ctx, keyQueueSortedSet, playerB.String()).Result()
	if err != nil {
		t.Errorf("player B should be re-queued: %v", err)
	}

	// Player A should have a decline
	declines, _ := svc.rdb.LLen(ctx, declinesKey(playerA)).Result()
	if declines != 1 {
		t.Errorf("expected 1 decline for player A, got %d", declines)
	}

	// Player B should NOT have declines
	declinesB, _ := svc.rdb.LLen(ctx, declinesKey(playerB)).Result()
	if declinesB != 0 {
		t.Errorf("player B should not have declines, got %d", declinesB)
	}
}

func TestHandleMatchExpiry_NeitherAccepted(t *testing.T) {
	svc, _ := newTestService(t)
	ctx := context.Background()

	matchID := uuid.New()
	playerA := uuid.New()
	playerB := uuid.New()

	// Neither accepted
	svc.rdb.HSet(ctx, shadowKey(matchID), map[string]any{
		"player_a":   playerA.String(),
		"player_b":   playerB.String(),
		"accepted_a": "0",
		"accepted_b": "0",
	})

	svc.handleMatchExpiry(ctx, matchID)

	// Neither should be queued
	_, errA := svc.rdb.ZScore(ctx, keyQueueSortedSet, playerA.String()).Result()
	_, errB := svc.rdb.ZScore(ctx, keyQueueSortedSet, playerB.String()).Result()
	if errA == nil {
		t.Error("player A should not be re-queued when neither accepted")
	}
	if errB == nil {
		t.Error("player B should not be re-queued when neither accepted")
	}

	// Neither should have declines
	declinesA, _ := svc.rdb.LLen(ctx, declinesKey(playerA)).Result()
	declinesB, _ := svc.rdb.LLen(ctx, declinesKey(playerB)).Result()
	if declinesA != 0 || declinesB != 0 {
		t.Errorf("no declines expected when neither accepted, got A=%d B=%d", declinesA, declinesB)
	}
}

func TestHandleMatchExpiry_NoShadowKey(t *testing.T) {
	svc, _ := newTestService(t)
	ctx := context.Background()

	// Call with a matchID that has no shadow key — should not panic
	svc.handleMatchExpiry(ctx, uuid.New())
}

func TestHandleMatchExpiry_RepeatedTimeouts_TriggerBan(t *testing.T) {
	svc, _ := newTestService(t)
	ctx := context.Background()

	timedOutPlayer := uuid.New()

	// Simulate DeclinePenaltyThreshold (3) timeouts to trigger a ban
	for i := range DeclinePenaltyThreshold {
		matchID := uuid.New()
		acceptor := uuid.New()

		svc.rdb.HSet(ctx, shadowKey(matchID), map[string]any{
			"player_a":   acceptor.String(),
			"player_b":   timedOutPlayer.String(),
			"accepted_a": "1",
			"accepted_b": "0",
		})

		svc.handleMatchExpiry(ctx, matchID)
		_ = i
	}

	// Player should now be banned
	ban, err := svc.BanStatus(ctx, timedOutPlayer)
	if err != nil {
		t.Fatalf("BanStatus: %v", err)
	}
	if !ban.Banned {
		t.Error("expected player to be banned after repeated timeouts")
	}
}
