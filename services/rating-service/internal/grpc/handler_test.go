package grpchandler

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/recess/rating-service/internal/store"
	ratingv1 "github.com/recess/shared/proto/rating/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// --- mock store --------------------------------------------------------------

type mockStore struct {
	ratings     map[uuid.UUID]*store.PlayerRating
	leaderboard []*store.LeaderboardRow
	lbCount     int
}

func newMockStore() *mockStore {
	return &mockStore{ratings: make(map[uuid.UUID]*store.PlayerRating)}
}

func (m *mockStore) GetRating(_ context.Context, playerID uuid.UUID, gameID string) (*store.PlayerRating, error) {
	if pr, ok := m.ratings[playerID]; ok {
		return pr, nil
	}
	return &store.PlayerRating{PlayerID: playerID, GameID: gameID, MMR: 1500, DisplayRating: 1500}, nil
}

func (m *mockStore) GetRatings(_ context.Context, playerIDs []uuid.UUID, gameID string) (map[uuid.UUID]*store.PlayerRating, error) {
	result := make(map[uuid.UUID]*store.PlayerRating, len(playerIDs))
	for _, id := range playerIDs {
		if pr, ok := m.ratings[id]; ok {
			result[id] = pr
		} else {
			result[id] = &store.PlayerRating{PlayerID: id, GameID: gameID, MMR: 1500, DisplayRating: 1500}
		}
	}
	return result, nil
}

func (m *mockStore) GetLeaderboard(_ context.Context, _ string, limit, offset, _ int, _ bool) ([]*store.LeaderboardRow, error) {
	end := offset + limit
	if end > len(m.leaderboard) {
		end = len(m.leaderboard)
	}
	if offset >= len(m.leaderboard) {
		return nil, nil
	}
	return m.leaderboard[offset:end], nil
}

func (m *mockStore) CountLeaderboard(_ context.Context, _ string, _ int, _ bool) (int, error) {
	return m.lbCount, nil
}

func (m *mockStore) UpsertRatings(_ context.Context, _ []*store.PlayerRating, _ []store.HistoryEntry) error {
	return nil
}

// --- GetRating ---------------------------------------------------------------

func TestGetRating_Default(t *testing.T) {
	h := New(newMockStore())

	resp, err := h.GetRating(context.Background(), &ratingv1.GetRatingRequest{
		PlayerId: uuid.NewString(),
		GameId:   "tictactoe",
	})
	if err != nil {
		t.Fatalf("GetRating: %v", err)
	}
	if resp.Mmr != 1500 {
		t.Errorf("expected default MMR 1500, got %f", resp.Mmr)
	}
}

func TestGetRating_Existing(t *testing.T) {
	ms := newMockStore()
	pid := uuid.New()
	ms.ratings[pid] = &store.PlayerRating{
		PlayerID: pid, GameID: "tictactoe", MMR: 1650, DisplayRating: 1640, GamesPlayed: 10, WinStreak: 3,
	}
	h := New(ms)

	resp, err := h.GetRating(context.Background(), &ratingv1.GetRatingRequest{
		PlayerId: pid.String(),
		GameId:   "tictactoe",
	})
	if err != nil {
		t.Fatalf("GetRating: %v", err)
	}
	if resp.Mmr != 1650 {
		t.Errorf("expected MMR 1650, got %f", resp.Mmr)
	}
	if resp.WinStreak != 3 {
		t.Errorf("expected win streak 3, got %d", resp.WinStreak)
	}
}

func TestGetRating_MissingPlayerID(t *testing.T) {
	h := New(newMockStore())
	_, err := h.GetRating(context.Background(), &ratingv1.GetRatingRequest{GameId: "tictactoe"})
	if status.Code(err) != codes.InvalidArgument {
		t.Errorf("expected InvalidArgument, got %s", status.Code(err))
	}
}

func TestGetRating_MissingGameID(t *testing.T) {
	h := New(newMockStore())
	_, err := h.GetRating(context.Background(), &ratingv1.GetRatingRequest{PlayerId: uuid.NewString()})
	if status.Code(err) != codes.InvalidArgument {
		t.Errorf("expected InvalidArgument, got %s", status.Code(err))
	}
}

func TestGetRating_InvalidPlayerID(t *testing.T) {
	h := New(newMockStore())
	_, err := h.GetRating(context.Background(), &ratingv1.GetRatingRequest{PlayerId: "bad", GameId: "tictactoe"})
	if status.Code(err) != codes.InvalidArgument {
		t.Errorf("expected InvalidArgument, got %s", status.Code(err))
	}
}

// --- GetRatings --------------------------------------------------------------

func TestGetRatings_Batch(t *testing.T) {
	ms := newMockStore()
	p1, p2 := uuid.New(), uuid.New()
	ms.ratings[p1] = &store.PlayerRating{PlayerID: p1, GameID: "tictactoe", MMR: 1600, DisplayRating: 1600}
	h := New(ms)

	resp, err := h.GetRatings(context.Background(), &ratingv1.GetRatingsRequest{
		PlayerIds: []string{p1.String(), p2.String()},
		GameId:    "tictactoe",
	})
	if err != nil {
		t.Fatalf("GetRatings: %v", err)
	}
	if len(resp.Ratings) != 2 {
		t.Fatalf("expected 2, got %d", len(resp.Ratings))
	}
	// Order preserved: p1 first.
	if resp.Ratings[0].Mmr != 1600 {
		t.Errorf("expected p1 MMR 1600, got %f", resp.Ratings[0].Mmr)
	}
	if resp.Ratings[1].Mmr != 1500 {
		t.Errorf("expected p2 default MMR 1500, got %f", resp.Ratings[1].Mmr)
	}
}

func TestGetRatings_EmptyPlayerIDs(t *testing.T) {
	h := New(newMockStore())
	_, err := h.GetRatings(context.Background(), &ratingv1.GetRatingsRequest{
		PlayerIds: []string{},
		GameId:    "tictactoe",
	})
	if status.Code(err) != codes.InvalidArgument {
		t.Errorf("expected InvalidArgument, got %s", status.Code(err))
	}
}

func TestGetRatings_MissingGameID(t *testing.T) {
	h := New(newMockStore())
	_, err := h.GetRatings(context.Background(), &ratingv1.GetRatingsRequest{
		PlayerIds: []string{uuid.NewString()},
	})
	if status.Code(err) != codes.InvalidArgument {
		t.Errorf("expected InvalidArgument, got %s", status.Code(err))
	}
}

func TestGetRatings_InvalidPlayerID(t *testing.T) {
	h := New(newMockStore())
	_, err := h.GetRatings(context.Background(), &ratingv1.GetRatingsRequest{
		PlayerIds: []string{uuid.NewString(), "bad"},
		GameId:    "tictactoe",
	})
	if status.Code(err) != codes.InvalidArgument {
		t.Errorf("expected InvalidArgument, got %s", status.Code(err))
	}
}

// --- GetLeaderboard ----------------------------------------------------------

func TestGetLeaderboard(t *testing.T) {
	ms := newMockStore()
	p1, p2 := uuid.New(), uuid.New()
	ms.leaderboard = []*store.LeaderboardRow{
		{PlayerRating: store.PlayerRating{PlayerID: p1, GameID: "tictactoe", DisplayRating: 1700, GamesPlayed: 20}, Username: "p1"},
		{PlayerRating: store.PlayerRating{PlayerID: p2, GameID: "tictactoe", DisplayRating: 1600, GamesPlayed: 15}, Username: "p2"},
	}
	ms.lbCount = 2
	h := New(ms)

	resp, err := h.GetLeaderboard(context.Background(), &ratingv1.GetLeaderboardRequest{
		GameId: "tictactoe",
		Limit:  10,
	})
	if err != nil {
		t.Fatalf("GetLeaderboard: %v", err)
	}
	if len(resp.Entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(resp.Entries))
	}
	if resp.Entries[0].Rank != 1 {
		t.Errorf("expected rank 1, got %d", resp.Entries[0].Rank)
	}
	if resp.Entries[0].DisplayRating != 1700 {
		t.Errorf("expected 1700, got %f", resp.Entries[0].DisplayRating)
	}
	if resp.Total != 2 {
		t.Errorf("expected total 2, got %d", resp.Total)
	}
}

func TestGetLeaderboard_DefaultLimit(t *testing.T) {
	h := New(newMockStore())

	// Limit 0 should use default (100), not error.
	resp, err := h.GetLeaderboard(context.Background(), &ratingv1.GetLeaderboardRequest{
		GameId: "tictactoe",
	})
	if err != nil {
		t.Fatalf("GetLeaderboard: %v", err)
	}
	if resp == nil {
		t.Fatal("expected non-nil response")
	}
}

func TestGetLeaderboard_ExceedsMaxLimit(t *testing.T) {
	h := New(newMockStore())
	_, err := h.GetLeaderboard(context.Background(), &ratingv1.GetLeaderboardRequest{
		GameId: "tictactoe",
		Limit:  501,
	})
	if status.Code(err) != codes.InvalidArgument {
		t.Errorf("expected InvalidArgument, got %s", status.Code(err))
	}
}

func TestGetLeaderboard_MissingGameID(t *testing.T) {
	h := New(newMockStore())
	_, err := h.GetLeaderboard(context.Background(), &ratingv1.GetLeaderboardRequest{})
	if status.Code(err) != codes.InvalidArgument {
		t.Errorf("expected InvalidArgument, got %s", status.Code(err))
	}
}

func TestGetLeaderboard_Pagination(t *testing.T) {
	ms := newMockStore()
	p1, p2 := uuid.New(), uuid.New()
	ms.leaderboard = []*store.LeaderboardRow{
		{PlayerRating: store.PlayerRating{PlayerID: p1, GameID: "tictactoe", DisplayRating: 1700, GamesPlayed: 20}, Username: "p1"},
		{PlayerRating: store.PlayerRating{PlayerID: p2, GameID: "tictactoe", DisplayRating: 1600, GamesPlayed: 15}, Username: "p2"},
	}
	ms.lbCount = 2
	h := New(ms)

	resp, err := h.GetLeaderboard(context.Background(), &ratingv1.GetLeaderboardRequest{
		GameId: "tictactoe",
		Limit:  1,
		Offset: 1,
	})
	if err != nil {
		t.Fatalf("GetLeaderboard: %v", err)
	}
	if len(resp.Entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(resp.Entries))
	}
	// Rank should be offset-aware.
	if resp.Entries[0].Rank != 2 {
		t.Errorf("expected rank 2 (offset=1), got %d", resp.Entries[0].Rank)
	}
}
