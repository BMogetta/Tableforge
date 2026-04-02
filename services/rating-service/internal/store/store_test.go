package store_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/recess/rating-service/internal/store"
	"github.com/recess/shared/testutil"
)

type testEnv struct {
	store store.Store
	pool  *pgxpool.Pool
}

func newTestEnv(t *testing.T) testEnv {
	t.Helper()
	dsn := testutil.NewTestDB(t, testutil.MigrationInitial, testutil.MigrationRating)

	s, err := store.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("failed to connect pool: %v", err)
	}
	t.Cleanup(pool.Close)

	return testEnv{store: s, pool: pool}
}

func (e testEnv) seedPlayer(t *testing.T, username string) uuid.UUID {
	t.Helper()
	id := uuid.New()
	_, err := e.pool.Exec(context.Background(),
		`INSERT INTO players (id, username) VALUES ($1, $2)`, id, username)
	if err != nil {
		t.Fatalf("seedPlayer: %v", err)
	}
	return id
}

// seedGameResult creates the full FK chain (room → session → result) and returns the result ID.
func (e testEnv) seedGameResult(t *testing.T, ownerID uuid.UUID, gameID string) uuid.UUID {
	t.Helper()
	ctx := context.Background()

	roomID := uuid.New()
	code := uuid.New().String()[:8]
	_, err := e.pool.Exec(ctx,
		`INSERT INTO rooms (id, code, game_id, owner_id, max_players) VALUES ($1, $2, $3, $4, 2)`,
		roomID, code, gameID, ownerID)
	if err != nil {
		t.Fatalf("seedRoom: %v", err)
	}

	sessionID := uuid.New()
	_, err = e.pool.Exec(ctx,
		`INSERT INTO game_sessions (id, room_id, game_id, state) VALUES ($1, $2, $3, '{}')`,
		sessionID, roomID, gameID)
	if err != nil {
		t.Fatalf("seedSession: %v", err)
	}

	resultID := uuid.New()
	_, err = e.pool.Exec(ctx,
		`INSERT INTO game_results (id, session_id, game_id, winner_id, ended_by) VALUES ($1, $2, $3, $4, 'win')`,
		resultID, sessionID, gameID, ownerID)
	if err != nil {
		t.Fatalf("seedResult: %v", err)
	}

	return resultID
}

func TestGetRatingDefault(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()

	playerID := env.seedPlayer(t, "alice")

	r, err := env.store.GetRating(ctx, playerID, "tictactoe")
	if err != nil {
		t.Fatalf("GetRating: %v", err)
	}
	if r.MMR != 1500 {
		t.Errorf("expected default MMR 1500, got %f", r.MMR)
	}
	if r.GamesPlayed != 0 {
		t.Errorf("expected 0 games, got %d", r.GamesPlayed)
	}
}

func TestGetRatingsDefaultBatch(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()

	p1 := env.seedPlayer(t, "alice")
	p2 := env.seedPlayer(t, "bob")

	ratings, err := env.store.GetRatings(ctx, []uuid.UUID{p1, p2}, "tictactoe")
	if err != nil {
		t.Fatalf("GetRatings: %v", err)
	}
	if len(ratings) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(ratings))
	}
	for _, id := range []uuid.UUID{p1, p2} {
		if ratings[id].MMR != 1500 {
			t.Errorf("player %s: expected MMR 1500, got %f", id, ratings[id].MMR)
		}
	}
}

func TestGetRatingsEmpty(t *testing.T) {
	env := newTestEnv(t)
	ratings, err := env.store.GetRatings(context.Background(), nil, "tictactoe")
	if err != nil {
		t.Fatalf("GetRatings: %v", err)
	}
	if len(ratings) != 0 {
		t.Errorf("expected empty map, got %d", len(ratings))
	}
}

func TestUpsertAndGetRating(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()

	p1 := env.seedPlayer(t, "alice")

	err := env.store.UpsertRatings(ctx, []*store.PlayerRating{
		{PlayerID: p1, GameID: "tictactoe", MMR: 1550, DisplayRating: 1540, GamesPlayed: 5, WinStreak: 3},
	}, nil)
	if err != nil {
		t.Fatalf("UpsertRatings: %v", err)
	}

	r, _ := env.store.GetRating(ctx, p1, "tictactoe")
	if r.MMR != 1550 {
		t.Errorf("expected MMR 1550, got %f", r.MMR)
	}
	if r.GamesPlayed != 5 {
		t.Errorf("expected 5 games, got %d", r.GamesPlayed)
	}
	if r.WinStreak != 3 {
		t.Errorf("expected win streak 3, got %d", r.WinStreak)
	}
}

func TestUpsertRatingsWithHistory(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()

	p1 := env.seedPlayer(t, "alice")
	resultID := env.seedGameResult(t, p1, "tictactoe")

	err := env.store.UpsertRatings(ctx, []*store.PlayerRating{
		{PlayerID: p1, GameID: "tictactoe", MMR: 1520, DisplayRating: 1515, GamesPlayed: 1, WinStreak: 1},
	}, []store.HistoryEntry{
		{PlayerID: p1, GameID: "tictactoe", ResultID: resultID, MMRBefore: 1500, MMRAfter: 1520, Delta: 20},
	})
	if err != nil {
		t.Fatalf("UpsertRatings with history: %v", err)
	}

	// Verify history was inserted.
	var count int
	env.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM ratings.rating_history WHERE player_id = $1 AND game_id = $2`,
		p1, "tictactoe",
	).Scan(&count)
	if count != 1 {
		t.Errorf("expected 1 history entry, got %d", count)
	}
}

func TestUpsertOverwritesExisting(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()

	p1 := env.seedPlayer(t, "alice")

	env.store.UpsertRatings(ctx, []*store.PlayerRating{
		{PlayerID: p1, GameID: "tictactoe", MMR: 1500, DisplayRating: 1500, GamesPlayed: 1},
	}, nil)

	env.store.UpsertRatings(ctx, []*store.PlayerRating{
		{PlayerID: p1, GameID: "tictactoe", MMR: 1600, DisplayRating: 1580, GamesPlayed: 10, WinStreak: 5},
	}, nil)

	r, _ := env.store.GetRating(ctx, p1, "tictactoe")
	if r.MMR != 1600 {
		t.Errorf("expected MMR 1600 after upsert, got %f", r.MMR)
	}
	if r.GamesPlayed != 10 {
		t.Errorf("expected 10 games after upsert, got %d", r.GamesPlayed)
	}
}

func TestLeaderboard(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()

	p1 := env.seedPlayer(t, "alice")
	p2 := env.seedPlayer(t, "bob")
	p3 := env.seedPlayer(t, "carol")

	env.store.UpsertRatings(ctx, []*store.PlayerRating{
		{PlayerID: p1, GameID: "tictactoe", MMR: 1600, DisplayRating: 1600, GamesPlayed: 10},
		{PlayerID: p2, GameID: "tictactoe", MMR: 1500, DisplayRating: 1500, GamesPlayed: 5},
		{PlayerID: p3, GameID: "tictactoe", MMR: 1550, DisplayRating: 1550, GamesPlayed: 2},
	}, nil)

	// minGames=3 should exclude carol (2 games).
	lb, err := env.store.GetLeaderboard(ctx, "tictactoe", 10, 0, 3)
	if err != nil {
		t.Fatalf("GetLeaderboard: %v", err)
	}
	if len(lb) != 2 {
		t.Fatalf("expected 2 entries (carol excluded), got %d", len(lb))
	}
	if lb[0].PlayerID != p1 {
		t.Error("expected alice first (highest display_rating)")
	}

	count, err := env.store.CountLeaderboard(ctx, "tictactoe", 3)
	if err != nil {
		t.Fatalf("CountLeaderboard: %v", err)
	}
	if count != 2 {
		t.Errorf("expected count 2, got %d", count)
	}
}

func TestLeaderboardPagination(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()

	p1 := env.seedPlayer(t, "alice")
	p2 := env.seedPlayer(t, "bob")

	env.store.UpsertRatings(ctx, []*store.PlayerRating{
		{PlayerID: p1, GameID: "tictactoe", MMR: 1600, DisplayRating: 1600, GamesPlayed: 10},
		{PlayerID: p2, GameID: "tictactoe", MMR: 1500, DisplayRating: 1500, GamesPlayed: 10},
	}, nil)

	// Page 1: limit 1.
	page1, _ := env.store.GetLeaderboard(ctx, "tictactoe", 1, 0, 0)
	if len(page1) != 1 || page1[0].PlayerID != p1 {
		t.Error("page 1 should have alice")
	}

	// Page 2: offset 1.
	page2, _ := env.store.GetLeaderboard(ctx, "tictactoe", 1, 1, 0)
	if len(page2) != 1 || page2[0].PlayerID != p2 {
		t.Error("page 2 should have bob")
	}
}
