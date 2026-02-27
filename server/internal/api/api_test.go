package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/tableforge/server/internal/api"
	"github.com/tableforge/server/internal/engine"
	"github.com/tableforge/server/internal/lobby"
	"github.com/tableforge/server/internal/runtime"
	"github.com/tableforge/server/internal/store"
	"github.com/tableforge/server/internal/testutil"
)

// --- Fake store --------------------------------------------------------------

type fakeStore struct{ testutil.FakeStore }

func newFakeStore() *fakeStore {
	return &fakeStore{FakeStore: *testutil.NewFakeStore()}
}

// --- Fake registry + stub game -----------------------------------------------

var errNotFound = errors.New("not found")

type fakeRegistry struct{ games map[string]engine.Game }

func newFakeRegistry(games ...engine.Game) *fakeRegistry {
	r := &fakeRegistry{games: make(map[string]engine.Game)}
	for _, g := range games {
		r.games[g.ID()] = g
	}
	return r
}

func (r *fakeRegistry) Get(id string) (engine.Game, error) {
	g, ok := r.games[id]
	if !ok {
		return nil, errNotFound
	}
	return g, nil
}

type stubGame struct{}

func (g *stubGame) ID() string      { return "chess" }
func (g *stubGame) Name() string    { return "Chess" }
func (g *stubGame) MinPlayers() int { return 2 }
func (g *stubGame) MaxPlayers() int { return 2 }
func (g *stubGame) Init(players []engine.Player) (engine.GameState, error) {
	return engine.GameState{CurrentPlayerID: players[0].ID, Data: map[string]any{}}, nil
}
func (g *stubGame) ValidateMove(_ engine.GameState, _ engine.Move) error { return nil }
func (g *stubGame) ApplyMove(s engine.GameState, _ engine.Move) (engine.GameState, error) {
	return s, nil
}
func (g *stubGame) IsOver(_ engine.GameState) (bool, engine.Result) { return false, engine.Result{} }

// --- Test helpers ------------------------------------------------------------

func newTestRouter(t *testing.T) (http.Handler, *fakeStore) {
	t.Helper()
	s := newFakeStore()
	reg := newFakeRegistry(&stubGame{})
	svc := lobby.New(s, reg)
	rt := runtime.New(s, reg)
	return api.NewRouter(svc, rt, s, nil, nil), s
}

func postJSON(t *testing.T, router http.Handler, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}

func getJSON(t *testing.T, router http.Handler, path string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}

// --- Tests -------------------------------------------------------------------

func TestCreatePlayer(t *testing.T) {
	router, _ := newTestRouter(t)

	w := postJSON(t, router, "/api/v1/players", map[string]string{"username": "alice"})

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var p store.Player
	json.NewDecoder(w.Body).Decode(&p)
	if p.Username != "alice" {
		t.Errorf("expected username alice, got %s", p.Username)
	}
}

func TestCreateRoom(t *testing.T) {
	router, s := newTestRouter(t)
	owner, _ := s.CreatePlayer(context.Background(), "alice")

	w := postJSON(t, router, "/api/v1/rooms", map[string]string{
		"game_id":   "chess",
		"player_id": owner.ID.String(),
	})

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var view lobby.RoomView
	json.NewDecoder(w.Body).Decode(&view)
	if view.Room.GameID != "chess" {
		t.Errorf("expected game_id chess, got %s", view.Room.GameID)
	}
}

func TestCreateRoom_UnknownGame(t *testing.T) {
	router, s := newTestRouter(t)
	owner, _ := s.CreatePlayer(context.Background(), "alice")

	w := postJSON(t, router, "/api/v1/rooms", map[string]string{
		"game_id":   "unknown",
		"player_id": owner.ID.String(),
	})

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestListRooms(t *testing.T) {
	router, s := newTestRouter(t)
	owner, _ := s.CreatePlayer(context.Background(), "alice")

	postJSON(t, router, "/api/v1/rooms", map[string]string{
		"game_id":   "chess",
		"player_id": owner.ID.String(),
	})

	w := getJSON(t, router, "/api/v1/rooms")
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var rooms []lobby.RoomView
	json.NewDecoder(w.Body).Decode(&rooms)
	if len(rooms) != 1 {
		t.Errorf("expected 1 room, got %d", len(rooms))
	}
}

func TestJoinRoom(t *testing.T) {
	router, s := newTestRouter(t)
	owner, _ := s.CreatePlayer(context.Background(), "alice")
	guest, _ := s.CreatePlayer(context.Background(), "bob")

	createResp := postJSON(t, router, "/api/v1/rooms", map[string]string{
		"game_id":   "chess",
		"player_id": owner.ID.String(),
	})
	var view lobby.RoomView
	json.NewDecoder(createResp.Body).Decode(&view)

	w := postJSON(t, router, "/api/v1/rooms/join", map[string]string{
		"code":      view.Room.Code,
		"player_id": guest.ID.String(),
	})

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var joined lobby.RoomView
	json.NewDecoder(w.Body).Decode(&joined)
	if len(joined.Players) != 2 {
		t.Errorf("expected 2 players, got %d", len(joined.Players))
	}
}

func TestStartGame(t *testing.T) {
	router, s := newTestRouter(t)
	owner, _ := s.CreatePlayer(context.Background(), "alice")
	guest, _ := s.CreatePlayer(context.Background(), "bob")

	createResp := postJSON(t, router, "/api/v1/rooms", map[string]string{
		"game_id":   "chess",
		"player_id": owner.ID.String(),
	})
	var view lobby.RoomView
	json.NewDecoder(createResp.Body).Decode(&view)

	postJSON(t, router, "/api/v1/rooms/join", map[string]string{
		"code":      view.Room.Code,
		"player_id": guest.ID.String(),
	})

	w := postJSON(t, router, "/api/v1/rooms/"+view.Room.ID.String()+"/start", map[string]string{
		"player_id": owner.ID.String(),
	})

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}
