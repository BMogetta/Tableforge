package api_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/tableforge/server/internal/domain/lobby"
	"github.com/tableforge/server/internal/platform/store"
)

// deleteJSON sends a DELETE request and returns the response.
func deleteJSON(t *testing.T, router http.Handler, path string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodDelete, path, nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}

// setupRoomWithSession creates two players, a room, joins both, and starts the game.
// Returns the two players and the room view.
func setupRoomWithPlayers(t *testing.T, router http.Handler, s *fakeStore) (store.Player, store.Player, lobby.RoomView) {
	t.Helper()
	ctx := context.Background()

	owner, _ := s.CreatePlayer(ctx, "alice")
	guest, _ := s.CreatePlayer(ctx, "bob")

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

	return owner, guest, view
}

// --- Tests -------------------------------------------------------------------

func TestSendRoomMessage(t *testing.T) {
	router, s := newTestRouter(t)
	owner, _, view := setupRoomWithPlayers(t, router, s)

	w := postJSON(t, router, "/api/v1/rooms/"+view.Room.ID.String()+"/messages", map[string]string{
		"player_id": owner.ID.String(),
		"content":   "hello world",
	})

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var msg store.RoomMessage
	json.NewDecoder(w.Body).Decode(&msg)
	if msg.Content != "hello world" {
		t.Errorf("expected content 'hello world', got %q", msg.Content)
	}
	if msg.PlayerID != owner.ID {
		t.Errorf("expected player_id %s, got %s", owner.ID, msg.PlayerID)
	}
	if msg.RoomID != view.Room.ID {
		t.Errorf("expected room_id %s, got %s", view.Room.ID, msg.RoomID)
	}
}

func TestSendRoomMessage_EmptyContent(t *testing.T) {
	router, s := newTestRouter(t)
	owner, _, view := setupRoomWithPlayers(t, router, s)

	w := postJSON(t, router, "/api/v1/rooms/"+view.Room.ID.String()+"/messages", map[string]string{
		"player_id": owner.ID.String(),
		"content":   "",
	})

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestSendRoomMessage_NonParticipant(t *testing.T) {
	router, s := newTestRouter(t)
	_, _, view := setupRoomWithPlayers(t, router, s)

	// A third player who never joined the room.
	outsider, _ := s.CreatePlayer(context.Background(), "charlie")

	w := postJSON(t, router, "/api/v1/rooms/"+view.Room.ID.String()+"/messages", map[string]string{
		"player_id": outsider.ID.String(),
		"content":   "can I chat?",
	})

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSendRoomMessage_InvalidRoomID(t *testing.T) {
	router, _ := newTestRouter(t)

	w := postJSON(t, router, "/api/v1/rooms/not-a-uuid/messages", map[string]string{
		"content": "hello",
	})

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestGetRoomMessages_Empty(t *testing.T) {
	router, s := newTestRouter(t)
	_, _, view := setupRoomWithPlayers(t, router, s)

	w := getJSON(t, router, "/api/v1/rooms/"+view.Room.ID.String()+"/messages")

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var messages []store.RoomMessage
	json.NewDecoder(w.Body).Decode(&messages)
	if len(messages) != 0 {
		t.Errorf("expected 0 messages, got %d", len(messages))
	}
}

func TestGetRoomMessages_AfterSend(t *testing.T) {
	router, s := newTestRouter(t)
	owner, guest, view := setupRoomWithPlayers(t, router, s)
	roomPath := "/api/v1/rooms/" + view.Room.ID.String()

	postJSON(t, router, roomPath+"/messages", map[string]string{
		"player_id": owner.ID.String(),
		"content":   "first",
	})
	postJSON(t, router, roomPath+"/messages", map[string]string{
		"player_id": guest.ID.String(),
		"content":   "second",
	})

	w := getJSON(t, router, roomPath+"/messages")

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var messages []store.RoomMessage
	json.NewDecoder(w.Body).Decode(&messages)
	if len(messages) != 2 {
		t.Errorf("expected 2 messages, got %d", len(messages))
	}
}

func TestReportRoomMessage(t *testing.T) {
	router, s := newTestRouter(t)
	owner, _, view := setupRoomWithPlayers(t, router, s)
	roomPath := "/api/v1/rooms/" + view.Room.ID.String()

	sendResp := postJSON(t, router, roomPath+"/messages", map[string]string{
		"player_id": owner.ID.String(),
		"content":   "bad message",
	})
	var msg store.RoomMessage
	json.NewDecoder(sendResp.Body).Decode(&msg)

	w := postJSON(t, router, roomPath+"/messages/"+msg.ID.String()+"/report", nil)

	if w.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", w.Code, w.Body.String())
	}
}

func TestReportRoomMessage_InvalidID(t *testing.T) {
	router, s := newTestRouter(t)
	_, _, view := setupRoomWithPlayers(t, router, s)

	w := postJSON(t, router, "/api/v1/rooms/"+view.Room.ID.String()+"/messages/not-a-uuid/report", nil)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestHideRoomMessage(t *testing.T) {
	router, s := newTestRouter(t)
	owner, _, view := setupRoomWithPlayers(t, router, s)
	roomPath := "/api/v1/rooms/" + view.Room.ID.String()

	sendResp := postJSON(t, router, roomPath+"/messages", map[string]string{
		"player_id": owner.ID.String(),
		"content":   "offensive",
	})
	var msg store.RoomMessage
	json.NewDecoder(sendResp.Body).Decode(&msg)

	w := deleteJSON(t, router, roomPath+"/messages/"+msg.ID.String())

	// requireRole middleware runs before the handler.
	// Without auth middleware in tests, returns 401.
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHideRoomMessage_InvalidMessageID(t *testing.T) {
	router, s := newTestRouter(t)
	_, _, view := setupRoomWithPlayers(t, router, s)

	w := deleteJSON(t, router, "/api/v1/rooms/"+view.Room.ID.String()+"/messages/not-a-uuid")

	// requireRole middleware runs before the handler.
	// Without auth middleware in tests, returns 401.
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}
