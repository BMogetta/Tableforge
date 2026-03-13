package runtime

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/tableforge/server/internal/domain/rating"
	"github.com/tableforge/server/internal/platform/store"
	"github.com/tableforge/server/internal/testutil"
)

// newRatingService returns a runtime Service wired with a real rating engine
// and a fake store. No timer or event store — not needed for rating tests.
func newRatingService(t *testing.T) (*Service, *testutil.FakeStore) {
	t.Helper()
	fs := testutil.NewFakeStore()
	svc := &Service{
		store:        fs,
		ratingEngine: rating.NewDefaultEngine(),
	}
	return svc, fs
}

// seedSession inserts a session and its room players directly into the fake
// store so applyRatings has something to act on.
func seedSession(t *testing.T, fs *testutil.FakeStore, mode store.SessionMode, players []uuid.UUID) (store.GameSession, []store.RoomPlayer) {
	t.Helper()
	ctx := context.Background()

	room, _ := fs.CreateRoom(ctx, store.CreateRoomParams{
		Code:       "TEST",
		GameID:     "chess",
		OwnerID:    players[0],
		MaxPlayers: len(players),
	})

	for i, pid := range players {
		fs.AddPlayerToRoom(ctx, room.ID, pid, i+1)
	}

	gs, _ := fs.CreateGameSession(ctx, room.ID, "chess", []byte(`{}`), nil, store.SessionModeCasual)
	gs.Mode = mode
	fs.Sessions[gs.ID] = gs

	roomPlayers, _ := fs.ListRoomPlayers(ctx, room.ID)
	return gs, roomPlayers
}

// --- buildRatingResult -------------------------------------------------------

func TestBuildRatingResult_WinnerGetsRank1(t *testing.T) {
	aliceID := uuid.New()
	bobID := uuid.New()

	ratingPlayers := []*rating.Player{
		{ID: aliceID.String(), MMR: 1500},
		{ID: bobID.String(), MMR: 1500},
	}
	roomPlayers := []store.RoomPlayer{
		{PlayerID: aliceID, Seat: 1},
		{PlayerID: bobID, Seat: 2},
	}

	result, err := buildRatingResult(ratingPlayers, roomPlayers, &aliceID, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Placements[0].Rank != 1 {
		t.Errorf("expected alice rank=1, got %d", result.Placements[0].Rank)
	}
	if result.Placements[1].Rank != 2 {
		t.Errorf("expected bob rank=2, got %d", result.Placements[1].Rank)
	}
}

func TestBuildRatingResult_DrawBothRank1(t *testing.T) {
	aliceID := uuid.New()
	bobID := uuid.New()

	ratingPlayers := []*rating.Player{
		{ID: aliceID.String(), MMR: 1500},
		{ID: bobID.String(), MMR: 1500},
	}
	roomPlayers := []store.RoomPlayer{
		{PlayerID: aliceID, Seat: 1},
		{PlayerID: bobID, Seat: 2},
	}

	result, err := buildRatingResult(ratingPlayers, roomPlayers, nil, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for i, p := range result.Placements {
		if p.Rank != 1 {
			t.Errorf("placement[%d]: expected rank=1 for draw, got %d", i, p.Rank)
		}
	}
}

func TestBuildRatingResult_LengthMismatch(t *testing.T) {
	ratingPlayers := []*rating.Player{{ID: "a"}}
	roomPlayers := []store.RoomPlayer{{PlayerID: uuid.New()}, {PlayerID: uuid.New()}}

	_, err := buildRatingResult(ratingPlayers, roomPlayers, nil, false)
	if err == nil {
		t.Fatal("expected error on length mismatch, got nil")
	}
}

// --- applyRatings: no-op paths -----------------------------------------------

func TestApplyRatings_NilEngine_NoOp(t *testing.T) {
	fs := testutil.NewFakeStore()
	svc := &Service{store: fs, ratingEngine: nil}
	ctx := context.Background()

	aliceID := uuid.New()
	bobID := uuid.New()
	gs, roomPlayers := seedSession(t, fs, store.SessionModeRanked, []uuid.UUID{aliceID, bobID})

	// Should not panic and should not write any ratings.
	svc.applyRatings(ctx, gs, roomPlayers, &aliceID, false)

	if len(fs.Ratings) != 0 {
		t.Errorf("expected no ratings written, got %d", len(fs.Ratings))
	}
}

// --- applyRatings: 1v1 win ---------------------------------------------------

func TestApplyRatings_Win_UpdatesRatings(t *testing.T) {
	svc, fs := newRatingService(t)
	ctx := context.Background()

	aliceID := uuid.New()
	bobID := uuid.New()
	gs, roomPlayers := seedSession(t, fs, store.SessionModeRanked, []uuid.UUID{aliceID, bobID})

	svc.applyRatings(ctx, gs, roomPlayers, &aliceID, false)

	aliceRating, ok := fs.Ratings[aliceID]
	if !ok {
		t.Fatal("expected rating for alice, got none")
	}
	bobRating, ok := fs.Ratings[bobID]
	if !ok {
		t.Fatal("expected rating for bob, got none")
	}

	// Both start at 1500 equal MMR.
	// surpriseFactor = |1.0 - 0.5| = 0.5, upsetMul = clamp(1+0.5, 1, 2) = 1.5
	// K = KMax(48) * streakMul(1.0) * upsetMul(1.5) = 72
	// delta = 0.5, MMR change = 72 * 0.5 = 36
	// Winner: 1500 + 36 = 1536. Loser: 1500 - 36 = 1464.
	const wantWinner = 1536.0
	const wantLoser = 1464.0
	const epsilon = 0.001

	if diff := aliceRating.MMR - wantWinner; diff < -epsilon || diff > epsilon {
		t.Errorf("alice MMR: expected %.3f, got %.3f", wantWinner, aliceRating.MMR)
	}
	if diff := bobRating.MMR - wantLoser; diff < -epsilon || diff > epsilon {
		t.Errorf("bob MMR: expected %.3f, got %.3f", wantLoser, bobRating.MMR)
	}
}

func TestApplyRatings_Win_StreaksUpdated(t *testing.T) {
	svc, fs := newRatingService(t)
	ctx := context.Background()

	aliceID := uuid.New()
	bobID := uuid.New()
	gs, roomPlayers := seedSession(t, fs, store.SessionModeRanked, []uuid.UUID{aliceID, bobID})

	svc.applyRatings(ctx, gs, roomPlayers, &aliceID, false)

	aliceRating := fs.Ratings[aliceID]
	bobRating := fs.Ratings[bobID]

	if aliceRating.WinStreak != 1 {
		t.Errorf("expected alice win_streak=1, got %d", aliceRating.WinStreak)
	}
	if aliceRating.LossStreak != 0 {
		t.Errorf("expected alice loss_streak=0, got %d", aliceRating.LossStreak)
	}
	if bobRating.LossStreak != 1 {
		t.Errorf("expected bob loss_streak=1, got %d", bobRating.LossStreak)
	}
	if bobRating.WinStreak != 0 {
		t.Errorf("expected bob win_streak=0, got %d", bobRating.WinStreak)
	}
}

func TestApplyRatings_Win_GamesPlayedIncremented(t *testing.T) {
	svc, fs := newRatingService(t)
	ctx := context.Background()

	aliceID := uuid.New()
	bobID := uuid.New()
	gs, roomPlayers := seedSession(t, fs, store.SessionModeRanked, []uuid.UUID{aliceID, bobID})

	svc.applyRatings(ctx, gs, roomPlayers, &aliceID, false)

	if fs.Ratings[aliceID].GamesPlayed != 1 {
		t.Errorf("expected alice games_played=1, got %d", fs.Ratings[aliceID].GamesPlayed)
	}
	if fs.Ratings[bobID].GamesPlayed != 1 {
		t.Errorf("expected bob games_played=1, got %d", fs.Ratings[bobID].GamesPlayed)
	}
}

// --- applyRatings: 1v1 draw --------------------------------------------------

func TestApplyRatings_Draw_NoMMRChange(t *testing.T) {
	svc, fs := newRatingService(t)
	ctx := context.Background()

	aliceID := uuid.New()
	bobID := uuid.New()
	gs, roomPlayers := seedSession(t, fs, store.SessionModeRanked, []uuid.UUID{aliceID, bobID})

	svc.applyRatings(ctx, gs, roomPlayers, nil, true)

	const epsilon = 0.001
	aliceMMR := fs.Ratings[aliceID].MMR
	bobMMR := fs.Ratings[bobID].MMR

	// Equal MMR draw: expected score = 0.5, actual score = 0.5, delta = 0.
	if diff := aliceMMR - 1500.0; diff < -epsilon || diff > epsilon {
		t.Errorf("alice MMR: expected 1500.000 on draw, got %.3f", aliceMMR)
	}
	if diff := bobMMR - 1500.0; diff < -epsilon || diff > epsilon {
		t.Errorf("bob MMR: expected 1500.000 on draw, got %.3f", bobMMR)
	}
}

func TestApplyRatings_Draw_GamesPlayedIncremented(t *testing.T) {
	svc, fs := newRatingService(t)
	ctx := context.Background()

	aliceID := uuid.New()
	bobID := uuid.New()
	gs, roomPlayers := seedSession(t, fs, store.SessionModeRanked, []uuid.UUID{aliceID, bobID})

	svc.applyRatings(ctx, gs, roomPlayers, nil, true)

	if fs.Ratings[aliceID].GamesPlayed != 1 {
		t.Errorf("expected alice games_played=1, got %d", fs.Ratings[aliceID].GamesPlayed)
	}
	if fs.Ratings[bobID].GamesPlayed != 1 {
		t.Errorf("expected bob games_played=1, got %d", fs.Ratings[bobID].GamesPlayed)
	}
}

// --- applyRatings: existing rating rows --------------------------------------

func TestApplyRatings_ExistingRating_UsedAsBaseline(t *testing.T) {
	svc, fs := newRatingService(t)
	ctx := context.Background()

	aliceID := uuid.New()
	bobID := uuid.New()

	// Pre-seed alice with a higher MMR.
	fs.Ratings[aliceID] = store.Rating{
		PlayerID:      aliceID,
		MMR:           1800.0,
		DisplayRating: 1800.0,
		GamesPlayed:   50,
	}

	gs, roomPlayers := seedSession(t, fs, store.SessionModeRanked, []uuid.UUID{aliceID, bobID})

	// Bob wins as the underdog.
	svc.applyRatings(ctx, gs, roomPlayers, &bobID, false)

	aliceMMR := fs.Ratings[aliceID].MMR
	bobMMR := fs.Ratings[bobID].MMR

	// Alice had higher MMR so she loses more; bob gains more.
	if aliceMMR >= 1800.0 {
		t.Errorf("expected alice MMR to drop below 1800, got %.3f", aliceMMR)
	}
	if bobMMR <= 1500.0 {
		t.Errorf("expected bob MMR to rise above 1500, got %.3f", bobMMR)
	}
}

// --- applyRatings: casual mode is a no-op ------------------------------------

func TestApplyRatings_CasualMode_NotCalledByRuntime(t *testing.T) {
	// applyRatings itself doesn't check mode — the caller (ApplyMove/Surrender)
	// gates on session.Mode == SessionModeRanked before calling applyRatings.
	// This test verifies applyRatings still writes ratings when called directly
	// (it is mode-agnostic), confirming the gate belongs in the caller.
	svc, fs := newRatingService(t)
	ctx := context.Background()

	aliceID := uuid.New()
	bobID := uuid.New()
	gs, roomPlayers := seedSession(t, fs, store.SessionModeCasual, []uuid.UUID{aliceID, bobID})

	// Called directly — bypasses the mode gate in ApplyMove/Surrender.
	svc.applyRatings(ctx, gs, roomPlayers, &aliceID, false)

	if len(fs.Ratings) != 2 {
		t.Errorf("expected 2 ratings when called directly, got %d", len(fs.Ratings))
	}
}
