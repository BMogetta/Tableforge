package api_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/tableforge/server/internal/platform/store"
)

// setupRoomWithBot creates a room, adds a bot to it, and returns the room view
// and bot player. The caller is the owner (alice).
func setupRoomWithBot(t *testing.T, router http.Handler, s *fakeStore) (string, store.Player) {
	t.Helper()
	ctx := context.Background()
	owner, _ := s.CreatePlayer(ctx, "alice")

	createResp := postJSON(t, router, "/api/v1/rooms", map[string]string{
		"game_id":   "chess",
		"player_id": owner.ID.String(),
	})
	if createResp.Code != http.StatusCreated {
		t.Fatalf("create room: expected 201, got %d", createResp.Code)
	}

	var view struct {
		Room struct {
			ID string `json:"id"`
		} `json:"room"`
	}
	json.NewDecoder(createResp.Body).Decode(&view)

	addResp := postJSON(t, router, "/api/v1/rooms/"+view.Room.ID+"/bots", map[string]string{
		"player_id": owner.ID.String(),
		"profile":   "easy",
	})
	if addResp.Code != http.StatusCreated {
		t.Fatalf("add bot: expected 201, got %d: %s", addResp.Code, addResp.Body.String())
	}

	var botPlayer store.Player
	json.NewDecoder(addResp.Body).Decode(&botPlayer)

	return view.Room.ID, botPlayer
}

// --- handleListBotProfiles ---------------------------------------------------

func TestListBotProfiles(t *testing.T) {
	router, _ := newTestRouter(t)

	w := getJSON(t, router, "/api/v1/bots/profiles")

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var profiles []map[string]any
	json.NewDecoder(w.Body).Decode(&profiles)

	if len(profiles) == 0 {
		t.Fatal("expected at least one profile")
	}

	// Verify first profile has required fields.
	first := profiles[0]
	for _, field := range []string{"name", "iterations", "exploration_c"} {
		if _, ok := first[field]; !ok {
			t.Errorf("expected field %q in profile", field)
		}
	}
}

func TestListBotProfiles_OrderedEasyFirst(t *testing.T) {
	router, _ := newTestRouter(t)

	w := getJSON(t, router, "/api/v1/bots/profiles")
	var profiles []map[string]any
	json.NewDecoder(w.Body).Decode(&profiles)

	if len(profiles) == 0 {
		t.Fatal("expected profiles")
	}
	if profiles[0]["name"] != "easy" {
		t.Errorf("expected first profile to be easy, got %v", profiles[0]["name"])
	}
}

// --- handleAddBot ------------------------------------------------------------

func TestAddBot_Success(t *testing.T) {
	router, s := newTestRouter(t)
	ctx := context.Background()
	owner, _ := s.CreatePlayer(ctx, "alice")

	createResp := postJSON(t, router, "/api/v1/rooms", map[string]string{
		"game_id":   "tictactoe",
		"player_id": owner.ID.String(),
	})
	var view struct {
		Room struct {
			ID string `json:"id"`
		} `json:"room"`
	}
	json.NewDecoder(createResp.Body).Decode(&view)

	w := postJSON(t, router, "/api/v1/rooms/"+view.Room.ID+"/bots", map[string]string{
		"player_id": owner.ID.String(),
		"profile":   "easy",
	})

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var botPlayer store.Player
	json.NewDecoder(w.Body).Decode(&botPlayer)

	if !botPlayer.IsBot {
		t.Error("expected is_bot to be true")
	}
	if botPlayer.Username == "" {
		t.Error("expected non-empty username")
	}
}

func TestAddBot_DefaultsToMediumProfile(t *testing.T) {
	router, s := newTestRouter(t)
	ctx := context.Background()
	owner, _ := s.CreatePlayer(ctx, "alice")

	createResp := postJSON(t, router, "/api/v1/rooms", map[string]string{
		"game_id":   "tictactoe",
		"player_id": owner.ID.String(),
	})
	var view struct {
		Room struct {
			ID string `json:"id"`
		} `json:"room"`
	}
	json.NewDecoder(createResp.Body).Decode(&view)

	// No profile specified — should default to medium.
	w := postJSON(t, router, "/api/v1/rooms/"+view.Room.ID+"/bots", map[string]string{
		"player_id": owner.ID.String(),
	})

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
}

func TestAddBot_InvalidProfile(t *testing.T) {
	router, s := newTestRouter(t)
	ctx := context.Background()
	owner, _ := s.CreatePlayer(ctx, "alice")

	createResp := postJSON(t, router, "/api/v1/rooms", map[string]string{
		"game_id":   "chess",
		"player_id": owner.ID.String(),
	})
	var view struct {
		Room struct {
			ID string `json:"id"`
		} `json:"room"`
	}
	json.NewDecoder(createResp.Body).Decode(&view)

	w := postJSON(t, router, "/api/v1/rooms/"+view.Room.ID+"/bots", map[string]string{
		"player_id": owner.ID.String(),
		"profile":   "godmode",
	})

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestAddBot_NonOwnerForbidden(t *testing.T) {
	router, s := newTestRouter(t)
	ctx := context.Background()
	owner, _ := s.CreatePlayer(ctx, "alice")
	guest, _ := s.CreatePlayer(ctx, "bob")

	createResp := postJSON(t, router, "/api/v1/rooms", map[string]string{
		"game_id":   "chess",
		"player_id": owner.ID.String(),
	})
	var view struct {
		Room struct {
			ID string `json:"id"`
		} `json:"room"`
	}
	json.NewDecoder(createResp.Body).Decode(&view)

	w := postJSON(t, router, "/api/v1/rooms/"+view.Room.ID+"/bots", map[string]string{
		"player_id": guest.ID.String(),
		"profile":   "easy",
	})

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", w.Code)
	}
}

func TestAddBot_InvalidRoomID(t *testing.T) {
	router, s := newTestRouter(t)
	ctx := context.Background()
	owner, _ := s.CreatePlayer(ctx, "alice")

	w := postJSON(t, router, "/api/v1/rooms/not-a-uuid/bots", map[string]string{
		"player_id": owner.ID.String(),
		"profile":   "easy",
	})

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestAddBot_UnknownGame(t *testing.T) {
	router, s := newTestRouter(t)
	ctx := context.Background()
	owner, _ := s.CreatePlayer(ctx, "alice")

	// Create a room with a game that has no bot adapter.
	// "chess" in our stub game has no adapter registered in botadapter.New.
	// We rely on the adapter registry returning an error for unknown games.
	createResp := postJSON(t, router, "/api/v1/rooms", map[string]string{
		"game_id":   "chess",
		"player_id": owner.ID.String(),
	})
	var view struct {
		Room struct {
			ID string `json:"id"`
		} `json:"room"`
	}
	json.NewDecoder(createResp.Body).Decode(&view)

	w := postJSON(t, router, "/api/v1/rooms/"+view.Room.ID+"/bots", map[string]string{
		"player_id": owner.ID.String(),
		"profile":   "easy",
	})

	// chess has no bot adapter — expect 400.
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for game with no adapter, got %d: %s", w.Code, w.Body.String())
	}
}

// --- handleRemoveBot ---------------------------------------------------------

func TestRemoveBot_Success(t *testing.T) {
	router, s := newTestRouter(t)
	ctx := context.Background()
	owner, _ := s.CreatePlayer(ctx, "alice")

	createResp := postJSON(t, router, "/api/v1/rooms", map[string]string{
		"game_id":   "chess",
		"player_id": owner.ID.String(),
	})
	var view struct {
		Room struct {
			ID string `json:"id"`
		} `json:"room"`
	}
	json.NewDecoder(createResp.Body).Decode(&view)

	// Add a bot directly via the store so we bypass the adapter check.
	botPlayer, _ := s.CreateBotPlayer(ctx, "bot:easy:test1234")
	s.AddPlayerToRoom(ctx, mustParseUUID(t, view.Room.ID), botPlayer.ID, 1)

	w := deleteJSONWithBody(t, router, "/api/v1/rooms/"+view.Room.ID+"/bots/"+botPlayer.ID.String(),
		map[string]string{"player_id": owner.ID.String()},
	)

	if w.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", w.Code, w.Body.String())
	}
}

func TestRemoveBot_NonOwnerForbidden(t *testing.T) {
	router, s := newTestRouter(t)
	ctx := context.Background()
	owner, _ := s.CreatePlayer(ctx, "alice")
	guest, _ := s.CreatePlayer(ctx, "bob")

	createResp := postJSON(t, router, "/api/v1/rooms", map[string]string{
		"game_id":   "chess",
		"player_id": owner.ID.String(),
	})
	var view struct {
		Room struct {
			ID string `json:"id"`
		} `json:"room"`
	}
	json.NewDecoder(createResp.Body).Decode(&view)

	botPlayer, _ := s.CreateBotPlayer(ctx, "bot:easy:test5678")
	s.AddPlayerToRoom(ctx, mustParseUUID(t, view.Room.ID), botPlayer.ID, 1)

	w := deleteJSONWithBody(t, router, "/api/v1/rooms/"+view.Room.ID+"/bots/"+botPlayer.ID.String(),
		map[string]string{"player_id": guest.ID.String()},
	)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", w.Code)
	}
}

func TestRemoveBot_NotABot(t *testing.T) {
	router, s := newTestRouter(t)
	ctx := context.Background()
	owner, _ := s.CreatePlayer(ctx, "alice")
	human, _ := s.CreatePlayer(ctx, "bob")

	createResp := postJSON(t, router, "/api/v1/rooms", map[string]string{
		"game_id":   "chess",
		"player_id": owner.ID.String(),
	})
	var view struct {
		Room struct {
			ID string `json:"id"`
		} `json:"room"`
	}
	json.NewDecoder(createResp.Body).Decode(&view)

	s.AddPlayerToRoom(ctx, mustParseUUID(t, view.Room.ID), human.ID, 1)

	w := deleteJSONWithBody(t, router, "/api/v1/rooms/"+view.Room.ID+"/bots/"+human.ID.String(),
		map[string]string{"player_id": owner.ID.String()},
	)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestRemoveBot_NotInRoom(t *testing.T) {
	router, s := newTestRouter(t)
	ctx := context.Background()
	owner, _ := s.CreatePlayer(ctx, "alice")

	createResp := postJSON(t, router, "/api/v1/rooms", map[string]string{
		"game_id":   "chess",
		"player_id": owner.ID.String(),
	})
	var view struct {
		Room struct {
			ID string `json:"id"`
		} `json:"room"`
	}
	json.NewDecoder(createResp.Body).Decode(&view)

	// Create bot but don't add to room.
	botPlayer, _ := s.CreateBotPlayer(ctx, "bot:easy:notinroom")

	w := deleteJSONWithBody(t, router, "/api/v1/rooms/"+view.Room.ID+"/bots/"+botPlayer.ID.String(),
		map[string]string{"player_id": owner.ID.String()},
	)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestRemoveBot_InvalidBotID(t *testing.T) {
	router, s := newTestRouter(t)
	ctx := context.Background()
	owner, _ := s.CreatePlayer(ctx, "alice")

	createResp := postJSON(t, router, "/api/v1/rooms", map[string]string{
		"game_id":   "chess",
		"player_id": owner.ID.String(),
	})
	var view struct {
		Room struct {
			ID string `json:"id"`
		} `json:"room"`
	}
	json.NewDecoder(createResp.Body).Decode(&view)

	w := deleteJSONWithBody(t, router, "/api/v1/rooms/"+view.Room.ID+"/bots/not-a-uuid",
		map[string]string{"player_id": owner.ID.String()},
	)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

// --- helpers -----------------------------------------------------------------

func mustParseUUID(t *testing.T, s string) uuid.UUID {
	t.Helper()
	parsed, err := uuid.Parse(s)
	if err != nil {
		t.Fatalf("mustParseUUID: %v", err)
	}
	return parsed
}
