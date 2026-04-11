package service

import (
	"context"
	"log/slog"
	"testing"

	"github.com/google/uuid"
	"github.com/recess/rating-service/internal/store"
	"github.com/recess/shared/domain/rating"
	"github.com/recess/shared/events"
)

// --- mock store --------------------------------------------------------------

type mockStore struct {
	ratings map[string]*store.PlayerRating // key: "playerID:gameID"
	upserts []*store.PlayerRating
	history []store.HistoryEntry
}

func newMockStore() *mockStore {
	return &mockStore{ratings: make(map[string]*store.PlayerRating)}
}

func (m *mockStore) key(id uuid.UUID, gameID string) string {
	return id.String() + ":" + gameID
}

func (m *mockStore) GetRating(_ context.Context, playerID uuid.UUID, gameID string) (*store.PlayerRating, error) {
	if r, ok := m.ratings[m.key(playerID, gameID)]; ok {
		return r, nil
	}
	return &store.PlayerRating{
		PlayerID:      playerID,
		GameID:        gameID,
		MMR:           rating.DefaultMMR,
		DisplayRating: rating.DefaultMMR,
	}, nil
}

func (m *mockStore) GetRatings(_ context.Context, playerIDs []uuid.UUID, gameID string) (map[uuid.UUID]*store.PlayerRating, error) {
	result := make(map[uuid.UUID]*store.PlayerRating, len(playerIDs))
	for _, id := range playerIDs {
		if r, ok := m.ratings[m.key(id, gameID)]; ok {
			result[id] = r
		} else {
			result[id] = &store.PlayerRating{
				PlayerID:      id,
				GameID:        gameID,
				MMR:           rating.DefaultMMR,
				DisplayRating: rating.DefaultMMR,
			}
		}
	}
	return result, nil
}

func (m *mockStore) GetLeaderboard(_ context.Context, gameID string, limit, offset, minGames int) ([]*store.PlayerRating, error) {
	return nil, nil
}

func (m *mockStore) CountLeaderboard(_ context.Context, gameID string, minGames int) (int, error) {
	return 0, nil
}

func (m *mockStore) UpsertRatings(_ context.Context, updates []*store.PlayerRating, history []store.HistoryEntry) error {
	m.upserts = updates
	m.history = history
	for _, u := range updates {
		m.ratings[m.key(u.PlayerID, u.GameID)] = u
	}
	return nil
}

// --- tests -------------------------------------------------------------------

func TestProcessGameFinished_Ranked1v1(t *testing.T) {
	st := newMockStore()
	engine := rating.NewDefaultEngine()
	svc := New(st, engine, slog.Default())

	winner := uuid.New()
	loser := uuid.New()

	evt := events.GameSessionFinished{
		SessionID: uuid.NewString(),
		ResultID:  uuid.NewString(),
		GameID:    "tictactoe",
		Mode:      "ranked",
		Players: []events.SessionPlayer{
			{PlayerID: winner.String(), Outcome: "win"},
			{PlayerID: loser.String(), Outcome: "loss"},
		},
	}

	if err := svc.ProcessGameFinished(context.Background(), evt); err != nil {
		t.Fatalf("ProcessGameFinished: %v", err)
	}

	if len(st.upserts) != 2 {
		t.Fatalf("expected 2 upserts, got %d", len(st.upserts))
	}
	if len(st.history) != 2 {
		t.Fatalf("expected 2 history entries, got %d", len(st.history))
	}

	var winnerRating, loserRating *store.PlayerRating
	for _, u := range st.upserts {
		if u.PlayerID == winner {
			winnerRating = u
		} else {
			loserRating = u
		}
	}

	if winnerRating.MMR <= rating.DefaultMMR {
		t.Errorf("winner MMR should increase from default, got %f", winnerRating.MMR)
	}
	if loserRating.MMR >= rating.DefaultMMR {
		t.Errorf("loser MMR should decrease from default, got %f", loserRating.MMR)
	}
	if winnerRating.GamesPlayed != 1 {
		t.Errorf("expected 1 game played, got %d", winnerRating.GamesPlayed)
	}
}

func TestProcessGameFinished_SkipsNonRanked(t *testing.T) {
	st := newMockStore()
	engine := rating.NewDefaultEngine()
	svc := New(st, engine, slog.Default())

	evt := events.GameSessionFinished{
		SessionID: uuid.NewString(),
		GameID:    "tictactoe",
		Mode:      "casual",
		Players: []events.SessionPlayer{
			{PlayerID: uuid.NewString(), Outcome: "win"},
			{PlayerID: uuid.NewString(), Outcome: "loss"},
		},
	}

	if err := svc.ProcessGameFinished(context.Background(), evt); err != nil {
		t.Fatalf("ProcessGameFinished: %v", err)
	}

	if len(st.upserts) != 0 {
		t.Errorf("expected no upserts for casual mode, got %d", len(st.upserts))
	}
}

func TestProcessGameFinished_Draw(t *testing.T) {
	st := newMockStore()
	engine := rating.NewDefaultEngine()
	svc := New(st, engine, slog.Default())

	p1 := uuid.New()
	p2 := uuid.New()

	evt := events.GameSessionFinished{
		SessionID: uuid.NewString(),
		ResultID:  uuid.NewString(),
		GameID:    "tictactoe",
		Mode:      "ranked",
		Players: []events.SessionPlayer{
			{PlayerID: p1.String(), Outcome: "draw"},
			{PlayerID: p2.String(), Outcome: "draw"},
		},
	}

	if err := svc.ProcessGameFinished(context.Background(), evt); err != nil {
		t.Fatalf("ProcessGameFinished: %v", err)
	}

	// Both players start at same MMR, draw → delta ≈ 0
	for _, u := range st.upserts {
		delta := u.MMR - rating.DefaultMMR
		if delta > 1 || delta < -1 {
			t.Errorf("draw between equal players should produce ~0 delta, got %f", delta)
		}
	}
}

func TestProcessGameFinished_TooFewPlayers(t *testing.T) {
	st := newMockStore()
	engine := rating.NewDefaultEngine()
	svc := New(st, engine, slog.Default())

	evt := events.GameSessionFinished{
		SessionID: uuid.NewString(),
		GameID:    "tictactoe",
		Mode:      "ranked",
		Players: []events.SessionPlayer{
			{PlayerID: uuid.NewString(), Outcome: "win"},
		},
	}

	if err := svc.ProcessGameFinished(context.Background(), evt); err != nil {
		t.Fatalf("ProcessGameFinished should not error for <2 players: %v", err)
	}
	if len(st.upserts) != 0 {
		t.Errorf("expected no upserts for single player, got %d", len(st.upserts))
	}
}

func TestProcessGameFinished_InvalidPlayerID(t *testing.T) {
	st := newMockStore()
	engine := rating.NewDefaultEngine()
	svc := New(st, engine, slog.Default())

	evt := events.GameSessionFinished{
		SessionID: uuid.NewString(),
		ResultID:  uuid.NewString(),
		GameID:    "tictactoe",
		Mode:      "ranked",
		Players: []events.SessionPlayer{
			{PlayerID: "not-a-uuid", Outcome: "win"},
			{PlayerID: uuid.NewString(), Outcome: "loss"},
		},
	}

	if err := svc.ProcessGameFinished(context.Background(), evt); err == nil {
		t.Fatal("expected error for invalid player_id")
	}
}
