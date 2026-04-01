package handler

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/recess/rating-service/internal/store"
	"github.com/recess/shared/domain/rating"
)

// --- mock store --------------------------------------------------------------

type mockStore struct {
	ratings     map[string]*store.PlayerRating // key: "playerID:gameID"
	leaderboard []*store.PlayerRating
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

func (m *mockStore) GetLeaderboard(_ context.Context, _ string, limit, offset, _ int) ([]*store.PlayerRating, error) {
	if offset >= len(m.leaderboard) {
		return nil, nil
	}
	end := offset + limit
	if end > len(m.leaderboard) {
		end = len(m.leaderboard)
	}
	return m.leaderboard[offset:end], nil
}

func (m *mockStore) CountLeaderboard(_ context.Context, _ string, _ int) (int, error) {
	return len(m.leaderboard), nil
}

func (m *mockStore) UpsertRatings(_ context.Context, _ []*store.PlayerRating, _ []store.HistoryEntry) error {
	return nil
}

// --- helpers -----------------------------------------------------------------

func newTestRouter(st store.Store) http.Handler {
	r := chi.NewRouter()
	New(st, slog.Default()).RegisterRoutes(r)
	return r
}

func decodeJSON[T any](t *testing.T, rec *httptest.ResponseRecorder) T {
	t.Helper()
	var v T
	if err := json.NewDecoder(rec.Body).Decode(&v); err != nil {
		t.Fatalf("decode response: %v (body: %s)", err, rec.Body.String())
	}
	return v
}

// --- tests -------------------------------------------------------------------

func TestHandlePlayerRating(t *testing.T) {
	playerID := uuid.New()
	st := newMockStore()
	st.ratings[st.key(playerID, "tictactoe")] = &store.PlayerRating{
		PlayerID:      playerID,
		GameID:        "tictactoe",
		MMR:           1600,
		DisplayRating: 1580,
		GamesPlayed:   10,
		WinStreak:     3,
	}

	router := newTestRouter(st)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/players/"+playerID.String()+"/ratings/tictactoe", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	body := decodeJSON[map[string]any](t, rec)

	if body["display_rating"].(float64) != 1580 {
		t.Errorf("expected display_rating 1580, got %v", body["display_rating"])
	}
	if body["games_played"].(float64) != 10 {
		t.Errorf("expected games_played 10, got %v", body["games_played"])
	}
	// MMR must NOT appear in public response
	if _, ok := body["mmr"]; ok {
		t.Error("MMR should not be exposed in public endpoint")
	}
}

func TestHandlePlayerRating_Default(t *testing.T) {
	st := newMockStore()
	router := newTestRouter(st)

	playerID := uuid.New()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/players/"+playerID.String()+"/ratings/tictactoe", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	body := decodeJSON[map[string]any](t, rec)
	if body["display_rating"].(float64) != rating.DefaultMMR {
		t.Errorf("expected default display_rating %f, got %v", rating.DefaultMMR, body["display_rating"])
	}
}

func TestHandlePlayerRating_InvalidID(t *testing.T) {
	router := newTestRouter(newMockStore())
	req := httptest.NewRequest(http.MethodGet, "/api/v1/players/not-a-uuid/ratings/tictactoe", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

func TestHandleLeaderboard(t *testing.T) {
	st := newMockStore()
	for i := range 3 {
		st.leaderboard = append(st.leaderboard, &store.PlayerRating{
			PlayerID:      uuid.New(),
			GameID:        "tictactoe",
			DisplayRating: float64(1600 - i*50),
			GamesPlayed:   10 + i,
		})
	}

	router := newTestRouter(st)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/ratings/tictactoe/leaderboard?limit=2", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	body := decodeJSON[map[string]any](t, rec)
	entries := body["entries"].([]any)
	if len(entries) != 2 {
		t.Errorf("expected 2 entries, got %d", len(entries))
	}

	first := entries[0].(map[string]any)
	if first["rank"].(float64) != 1 {
		t.Errorf("first entry rank should be 1, got %v", first["rank"])
	}
}

func TestHandleLeaderboard_InvalidLimit(t *testing.T) {
	router := newTestRouter(newMockStore())
	req := httptest.NewRequest(http.MethodGet, "/api/v1/ratings/tictactoe/leaderboard?limit=0", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for limit=0, got %d", rec.Code)
	}
}

func TestHandleLeaderboard_LimitOver500(t *testing.T) {
	router := newTestRouter(newMockStore())
	req := httptest.NewRequest(http.MethodGet, "/api/v1/ratings/tictactoe/leaderboard?limit=501", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for limit=501, got %d", rec.Code)
	}
}
