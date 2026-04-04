package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/recess/game-server/internal/domain/lobby"
	sharedmw "github.com/recess/shared/middleware"
)

// putJSONAs sends a PUT request with a JSON body and injected auth context.
func putJSONAs(t *testing.T, router http.Handler, path string, playerID uuid.UUID, role string, body any) *httptest.ResponseRecorder {
	t.Helper()
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPut, path, bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(sharedmw.ContextWithPlayer(req.Context(), playerID, "testuser", role))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}

// --- GET /api/v1/games -------------------------------------------------------

func TestListGames(t *testing.T) {
	router, _ := newTestRouter(t)

	w := getJSON(t, router, "/api/v1/games")

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var games []map[string]any
	json.NewDecoder(w.Body).Decode(&games)
	if len(games) == 0 {
		t.Fatal("expected at least one game")
	}
	for _, g := range games {
		if g["id"] == nil || g["name"] == nil {
			t.Error("expected game to have id and name fields")
		}
		if g["min_players"] == nil || g["max_players"] == nil {
			t.Error("expected game to have min_players and max_players fields")
		}
	}
}

// --- GET /api/v1/rooms/{roomID} ----------------------------------------------

func TestGetRoom_Success(t *testing.T) {
	router, s := newTestRouter(t)
	owner, _ := s.CreatePlayer(context.Background(), "alice")

	createResp := postJSONAs(t, router, "/api/v1/rooms", owner.ID, "player", map[string]string{
		"game_id": "chess",
	})
	var view lobby.RoomView
	json.NewDecoder(createResp.Body).Decode(&view)

	w := getJSON(t, router, "/api/v1/rooms/"+view.Room.ID.String(), owner.ID)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var got lobby.RoomView
	json.NewDecoder(w.Body).Decode(&got)
	if got.Room.ID != view.Room.ID {
		t.Errorf("expected room ID %s, got %s", view.Room.ID, got.Room.ID)
	}
}

func TestGetRoom_InvalidID(t *testing.T) {
	router, _ := newTestRouter(t)

	w := getJSON(t, router, "/api/v1/rooms/not-a-uuid")
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestGetRoom_NotFound(t *testing.T) {
	router, _ := newTestRouter(t)

	w := getJSON(t, router, "/api/v1/rooms/"+uuid.New().String())
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

// --- POST /api/v1/rooms/join — error cases -----------------------------------

func TestJoinRoom_NotFound(t *testing.T) {
	router, s := newTestRouter(t)
	player, _ := s.CreatePlayer(context.Background(), "alice")

	w := postJSONAs(t, router, "/api/v1/rooms/join", player.ID, "player", map[string]string{
		"code": "ZZZZZ1",
	})

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestJoinRoom_RoomFull(t *testing.T) {
	router, s := newTestRouter(t)
	owner, _ := s.CreatePlayer(context.Background(), "alice")
	guest, _ := s.CreatePlayer(context.Background(), "bob")
	extra, _ := s.CreatePlayer(context.Background(), "carol")

	createResp := postJSONAs(t, router, "/api/v1/rooms", owner.ID, "player", map[string]string{
		"game_id": "chess",
	})
	var view lobby.RoomView
	json.NewDecoder(createResp.Body).Decode(&view)

	postJSONAs(t, router, "/api/v1/rooms/join", guest.ID, "player", map[string]string{
		"code": view.Room.Code,
	})

	w := postJSONAs(t, router, "/api/v1/rooms/join", extra.ID, "player", map[string]string{
		"code": view.Room.Code,
	})

	if w.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d: %s", w.Code, w.Body.String())
	}
}

func TestJoinRoom_AlreadyInRoom(t *testing.T) {
	router, s := newTestRouter(t)
	owner, _ := s.CreatePlayer(context.Background(), "alice")

	createResp := postJSONAs(t, router, "/api/v1/rooms", owner.ID, "player", map[string]string{
		"game_id": "chess",
	})
	var view lobby.RoomView
	json.NewDecoder(createResp.Body).Decode(&view)

	w := postJSONAs(t, router, "/api/v1/rooms/join", owner.ID, "player", map[string]string{
		"code": view.Room.Code,
	})

	if w.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d: %s", w.Code, w.Body.String())
	}
}

// --- POST /api/v1/rooms/{roomID}/leave ---------------------------------------

func TestLeaveRoom_Success(t *testing.T) {
	router, s := newTestRouter(t)
	owner, _ := s.CreatePlayer(context.Background(), "alice")
	guest, _ := s.CreatePlayer(context.Background(), "bob")

	createResp := postJSONAs(t, router, "/api/v1/rooms", owner.ID, "player", map[string]string{
		"game_id": "chess",
	})
	var view lobby.RoomView
	json.NewDecoder(createResp.Body).Decode(&view)

	postJSONAs(t, router, "/api/v1/rooms/join", guest.ID, "player", map[string]string{
		"code": view.Room.Code,
	})

	w := postJSONAs(t, router, "/api/v1/rooms/"+view.Room.ID.String()+"/leave", guest.ID, "player", map[string]string{})

	if w.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", w.Code, w.Body.String())
	}
}

func TestLeaveRoom_OwnerTransfersOwnership(t *testing.T) {
	router, s := newTestRouter(t)
	owner, _ := s.CreatePlayer(context.Background(), "alice")
	guest, _ := s.CreatePlayer(context.Background(), "bob")

	createResp := postJSONAs(t, router, "/api/v1/rooms", owner.ID, "player", map[string]string{
		"game_id": "chess",
	})
	var view lobby.RoomView
	json.NewDecoder(createResp.Body).Decode(&view)

	postJSONAs(t, router, "/api/v1/rooms/join", guest.ID, "player", map[string]string{
		"code": view.Room.Code,
	})

	w := postJSONAs(t, router, "/api/v1/rooms/"+view.Room.ID.String()+"/leave", owner.ID, "player", map[string]string{})

	if w.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", w.Code, w.Body.String())
	}

	// Verify ownership transferred.
	room := s.Rooms[view.Room.ID]
	if room.OwnerID != guest.ID {
		t.Errorf("expected new owner to be bob (%s), got %s", guest.ID, room.OwnerID)
	}
}

func TestLeaveRoom_LastPlayerClosesRoom(t *testing.T) {
	router, s := newTestRouter(t)
	owner, _ := s.CreatePlayer(context.Background(), "alice")

	createResp := postJSONAs(t, router, "/api/v1/rooms", owner.ID, "player", map[string]string{
		"game_id": "chess",
	})
	var view lobby.RoomView
	json.NewDecoder(createResp.Body).Decode(&view)

	w := postJSONAs(t, router, "/api/v1/rooms/"+view.Room.ID.String()+"/leave", owner.ID, "player", map[string]string{})

	if w.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", w.Code, w.Body.String())
	}

	// Verify room was deleted.
	if _, ok := s.Rooms[view.Room.ID]; ok {
		t.Error("expected room to be deleted after last player leaves")
	}
}

func TestLeaveRoom_InvalidRoomID(t *testing.T) {
	router, s := newTestRouter(t)
	player, _ := s.CreatePlayer(context.Background(), "alice")

	w := postJSONAs(t, router, "/api/v1/rooms/not-a-uuid/leave", player.ID, "player", map[string]string{})

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestLeaveRoom_NotFound(t *testing.T) {
	router, s := newTestRouter(t)
	player, _ := s.CreatePlayer(context.Background(), "alice")

	w := postJSONAs(t, router, "/api/v1/rooms/"+uuid.New().String()+"/leave", player.ID, "player", map[string]string{})

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

// --- POST /api/v1/rooms/{roomID}/start — error cases -------------------------

func TestStartGame_InvalidRoomID(t *testing.T) {
	router, s := newTestRouter(t)
	player, _ := s.CreatePlayer(context.Background(), "alice")

	w := postJSONAs(t, router, "/api/v1/rooms/not-a-uuid/start", player.ID, "player", map[string]string{})

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestStartGame_NotOwner(t *testing.T) {
	router, s := newTestRouter(t)
	owner, _ := s.CreatePlayer(context.Background(), "alice")
	guest, _ := s.CreatePlayer(context.Background(), "bob")

	createResp := postJSONAs(t, router, "/api/v1/rooms", owner.ID, "player", map[string]string{
		"game_id": "chess",
	})
	var view lobby.RoomView
	json.NewDecoder(createResp.Body).Decode(&view)

	postJSONAs(t, router, "/api/v1/rooms/join", guest.ID, "player", map[string]string{
		"code": view.Room.Code,
	})

	w := postJSONAs(t, router, "/api/v1/rooms/"+view.Room.ID.String()+"/start", guest.ID, "player", map[string]string{})

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", w.Code, w.Body.String())
	}
}

func TestStartGame_NotEnoughPlayers(t *testing.T) {
	router, s := newTestRouter(t)
	owner, _ := s.CreatePlayer(context.Background(), "alice")

	createResp := postJSONAs(t, router, "/api/v1/rooms", owner.ID, "player", map[string]string{
		"game_id": "chess",
	})
	var view lobby.RoomView
	json.NewDecoder(createResp.Body).Decode(&view)

	w := postJSONAs(t, router, "/api/v1/rooms/"+view.Room.ID.String()+"/start", owner.ID, "player", map[string]string{})

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

// --- PUT /api/v1/rooms/{roomID}/settings/{key} ------------------------------

func TestUpdateRoomSetting_Success(t *testing.T) {
	router, s := newTestRouter(t)
	owner, _ := s.CreatePlayer(context.Background(), "alice")

	createResp := postJSONAs(t, router, "/api/v1/rooms", owner.ID, "player", map[string]string{
		"game_id": "chess",
	})
	var view lobby.RoomView
	json.NewDecoder(createResp.Body).Decode(&view)

	w := putJSONAs(t, router, "/api/v1/rooms/"+view.Room.ID.String()+"/settings/room_visibility", owner.ID, "player", map[string]string{
		"value": "private",
	})

	if w.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", w.Code, w.Body.String())
	}

	// Verify setting was updated.
	settings := s.RoomSettings[view.Room.ID]
	if settings["room_visibility"] != "private" {
		t.Errorf("expected room_visibility=private, got %s", settings["room_visibility"])
	}
}

func TestUpdateRoomSetting_NotOwner(t *testing.T) {
	router, s := newTestRouter(t)
	owner, _ := s.CreatePlayer(context.Background(), "alice")
	guest, _ := s.CreatePlayer(context.Background(), "bob")

	createResp := postJSONAs(t, router, "/api/v1/rooms", owner.ID, "player", map[string]string{
		"game_id": "chess",
	})
	var view lobby.RoomView
	json.NewDecoder(createResp.Body).Decode(&view)

	postJSONAs(t, router, "/api/v1/rooms/join", guest.ID, "player", map[string]string{
		"code": view.Room.Code,
	})

	w := putJSONAs(t, router, "/api/v1/rooms/"+view.Room.ID.String()+"/settings/room_visibility", guest.ID, "player", map[string]string{
		"value": "private",
	})

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", w.Code, w.Body.String())
	}
}

func TestUpdateRoomSetting_UnknownKey(t *testing.T) {
	router, s := newTestRouter(t)
	owner, _ := s.CreatePlayer(context.Background(), "alice")

	createResp := postJSONAs(t, router, "/api/v1/rooms", owner.ID, "player", map[string]string{
		"game_id": "chess",
	})
	var view lobby.RoomView
	json.NewDecoder(createResp.Body).Decode(&view)

	w := putJSONAs(t, router, "/api/v1/rooms/"+view.Room.ID.String()+"/settings/nonexistent_setting", owner.ID, "player", map[string]string{
		"value": "something",
	})

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestUpdateRoomSetting_InvalidValue(t *testing.T) {
	router, s := newTestRouter(t)
	owner, _ := s.CreatePlayer(context.Background(), "alice")

	createResp := postJSONAs(t, router, "/api/v1/rooms", owner.ID, "player", map[string]string{
		"game_id": "chess",
	})
	var view lobby.RoomView
	json.NewDecoder(createResp.Body).Decode(&view)

	w := putJSONAs(t, router, "/api/v1/rooms/"+view.Room.ID.String()+"/settings/room_visibility", owner.ID, "player", map[string]string{
		"value": "invalid_value",
	})

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestUpdateRoomSetting_InvalidRoomID(t *testing.T) {
	router, s := newTestRouter(t)
	player, _ := s.CreatePlayer(context.Background(), "alice")

	w := putJSONAs(t, router, "/api/v1/rooms/not-a-uuid/settings/room_visibility", player.ID, "player", map[string]string{
		"value": "private",
	})

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

// --- GET /api/v1/players/{playerID}/matches ----------------------------------

func TestListPlayerMatches_Success(t *testing.T) {
	router, s := newTestRouter(t)
	player, _ := s.CreatePlayer(context.Background(), "alice")

	w := getJSON(t, router, "/api/v1/players/"+player.ID.String()+"/matches", player.ID)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]any
	json.NewDecoder(w.Body).Decode(&resp)
	if _, ok := resp["total"]; !ok {
		t.Error("expected 'total' field in response")
	}
	if _, ok := resp["matches"]; !ok {
		t.Error("expected 'matches' field in response")
	}
}

func TestListPlayerMatches_InvalidID(t *testing.T) {
	router, _ := newTestRouter(t)

	w := getJSON(t, router, "/api/v1/players/not-a-uuid/matches")
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestListPlayerMatches_WithPagination(t *testing.T) {
	router, s := newTestRouter(t)
	player, _ := s.CreatePlayer(context.Background(), "alice")

	w := getJSON(t, router, "/api/v1/players/"+player.ID.String()+"/matches?limit=5&offset=10", player.ID)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

// --- GET /api/v1/sessions/{sessionID} ----------------------------------------

func TestGetSession_Success(t *testing.T) {
	router, s := newTestRouter(t)
	owner, guest, sessionID := setupActiveSession(t, router, s)
	_ = owner
	_ = guest

	w := getJSON(t, router, "/api/v1/sessions/"+sessionID)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]any
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["session"] == nil {
		t.Error("expected 'session' field in response")
	}
	if resp["state"] == nil {
		t.Error("expected 'state' field in response")
	}
}

func TestGetSession_InvalidID(t *testing.T) {
	router, _ := newTestRouter(t)

	w := getJSON(t, router, "/api/v1/sessions/not-a-uuid")
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestGetSession_NotFound(t *testing.T) {
	router, _ := newTestRouter(t)

	w := getJSON(t, router, "/api/v1/sessions/"+uuid.New().String())
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

// --- POST /api/v1/sessions/{sessionID}/ready ---------------------------------

func TestPlayerReady_Success(t *testing.T) {
	router, s := newTestRouter(t)
	owner, _, sessionID := setupActiveSession(t, router, s)

	w := postJSONAs(t, router, "/api/v1/sessions/"+sessionID+"/ready", uuid.MustParse(owner.id), "player", nil)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestPlayerReady_InvalidSessionID(t *testing.T) {
	router, _ := newTestRouter(t)
	player := uuid.New()

	w := postJSONAs(t, router, "/api/v1/sessions/not-a-uuid/ready", player, "player", nil)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestPlayerReady_SessionNotFound(t *testing.T) {
	router, _ := newTestRouter(t)
	player := uuid.New()

	w := postJSONAs(t, router, "/api/v1/sessions/"+uuid.New().String()+"/ready", player, "player", nil)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

// --- POST /api/v1/sessions/{sessionID}/surrender -----------------------------

func TestSurrender_Success(t *testing.T) {
	router, s := newTestRouter(t)
	owner, _, sessionID := setupActiveSession(t, router, s)

	w := postJSONAs(t, router, "/api/v1/sessions/"+sessionID+"/surrender", uuid.MustParse(owner.id), "player", map[string]string{})

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]any
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["is_over"] != true {
		t.Error("expected is_over=true after surrender")
	}
}

func TestSurrender_InvalidSessionID(t *testing.T) {
	router, _ := newTestRouter(t)
	player := uuid.New()

	w := postJSONAs(t, router, "/api/v1/sessions/not-a-uuid/surrender", player, "player", map[string]string{})

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestSurrender_SessionNotFound(t *testing.T) {
	router, _ := newTestRouter(t)
	player := uuid.New()

	w := postJSONAs(t, router, "/api/v1/sessions/"+uuid.New().String()+"/surrender", player, "player", map[string]string{})

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

// --- POST /api/v1/sessions/{sessionID}/rematch -------------------------------

func TestRematch_InvalidSessionID(t *testing.T) {
	router, _ := newTestRouter(t)
	player := uuid.New()

	w := postJSONAs(t, router, "/api/v1/sessions/not-a-uuid/rematch", player, "player", map[string]string{})

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestRematch_SessionNotFound(t *testing.T) {
	router, _ := newTestRouter(t)
	player := uuid.New()

	w := postJSONAs(t, router, "/api/v1/sessions/"+uuid.New().String()+"/rematch", player, "player", map[string]string{})

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

// --- POST /api/v1/sessions/{sessionID}/move ----------------------------------

func TestMove_InvalidSessionID(t *testing.T) {
	router, _ := newTestRouter(t)
	player := uuid.New()

	w := postJSONAs(t, router, "/api/v1/sessions/not-a-uuid/move", player, "player", map[string]any{
		"payload": map[string]any{"cell": 0},
	})

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestMove_SessionNotFound(t *testing.T) {
	router, _ := newTestRouter(t)
	player := uuid.New()

	w := postJSONAs(t, router, "/api/v1/sessions/"+uuid.New().String()+"/move", player, "player", map[string]any{
		"payload": map[string]any{"cell": 0},
	})

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

// --- DELETE /api/v1/sessions/{sessionID} (force close) -----------------------

func TestForceCloseSession_AsManager(t *testing.T) {
	router, s := newTestRouter(t)
	_, _, sessionID := setupSuspendedSession(t, router, s)

	manager := uuid.New()
	w := deleteJSONAs(t, router, "/api/v1/sessions/"+sessionID, manager, "manager")

	if w.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", w.Code, w.Body.String())
	}
}

func TestForceCloseSession_AsOwner(t *testing.T) {
	router, s := newTestRouter(t)
	_, _, sessionID := setupSuspendedSession(t, router, s)

	owner := uuid.New()
	w := deleteJSONAs(t, router, "/api/v1/sessions/"+sessionID, owner, "owner")

	if w.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", w.Code, w.Body.String())
	}
}

func TestForceCloseSession_AsPlayerForbidden(t *testing.T) {
	router, s := newTestRouter(t)
	_, _, sessionID := setupSuspendedSession(t, router, s)

	player := uuid.New()
	w := deleteJSONAs(t, router, "/api/v1/sessions/"+sessionID, player, "player")

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", w.Code, w.Body.String())
	}
}

func TestForceCloseSession_SessionNotFound(t *testing.T) {
	router, _ := newTestRouter(t)

	manager := uuid.New()
	w := deleteJSONAs(t, router, "/api/v1/sessions/"+uuid.New().String(), manager, "manager")

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

// --- GET /healthz ------------------------------------------------------------

func TestHealthz(t *testing.T) {
	router, _ := newTestRouter(t)

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp map[string]string
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["status"] != "ok" {
		t.Errorf("expected status ok, got %s", resp["status"])
	}
}
