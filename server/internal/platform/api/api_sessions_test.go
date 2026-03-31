package api_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
)

// deleteJSON sends a DELETE request and returns the response.
func deleteJSON(t *testing.T, router http.Handler, path string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodDelete, path, nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}

// setupActiveSession creates two players, a room, joins both, starts the game,
// and returns the two players and the started session ID as a string.
func setupActiveSession(t *testing.T, router http.Handler, s *fakeStore) (owner, guest playerHandle, sessionID string) {
	t.Helper()
	ctx := context.Background()

	// 1. Crear jugadores en el store persistente del test
	ownerPlayer, _ := s.CreatePlayer(ctx, "alice")
	guestPlayer, _ := s.CreatePlayer(ctx, "bob")

	// 2. Crear Sala (Alice es la dueña)
	// Pasamos el ID en el contexto (postJSONAs) y en el body si el handler lo requiere
	createResp := postJSONAs(t, router, "/api/v1/rooms", ownerPlayer.ID, "player", map[string]any{
		"game_id":   "chess",
		"player_id": ownerPlayer.ID.String(), // Aseguramos coincidencia Body-Context
	})
	requireStatus(t, createResp, http.StatusCreated, "create room")

	var roomView map[string]any
	json.Unmarshal(createResp.Body.Bytes(), &roomView)

	// Navegación segura del mapa para evitar panics
	room, ok := roomView["room"].(map[string]any)
	if !ok {
		t.Fatalf("response missing 'room' object: %s", createResp.Body.String())
	}

	roomID := room["id"].(string)
	code := room["code"].(string)

	// 3. Unirse a la Sala (Bob es el invitado)
	joinResp := postJSONAs(t, router, "/api/v1/rooms/join", guestPlayer.ID, "player", map[string]any{
		"code":      code,
		"player_id": guestPlayer.ID.String(), // Bob firma su propia entrada
	})
	requireStatus(t, joinResp, http.StatusOK, "join room")

	// 4. Empezar Juego (Solo la dueña Alice puede hacerlo)
	startResp := postJSONAs(t, router, "/api/v1/rooms/"+roomID+"/start", ownerPlayer.ID, "player", map[string]any{
		"player_id": ownerPlayer.ID.String(),
	})
	requireStatus(t, startResp, http.StatusOK, "start game")

	var sessionData map[string]any
	json.Unmarshal(startResp.Body.Bytes(), &sessionData)

	sID, ok := sessionData["id"].(string)
	if !ok {
		t.Fatalf("response missing 'id' for session: %s", startResp.Body.String())
	}

	return playerHandle{ownerPlayer.ID.String()},
		playerHandle{guestPlayer.ID.String()},
		sID
}

// Helper auxiliar para evitar repetir if resp.Code != ...
func requireStatus(t *testing.T, w *httptest.ResponseRecorder, want int, action string) {
	t.Helper()
	if w.Code != want {
		t.Fatalf("failed to %s: expected %d, got %d. Body: %s", action, want, w.Code, w.Body.String())
	}
}

// setupSuspendedSession builds on setupActiveSession and casts pause votes from
// both players to bring the session into suspended state.
func setupSuspendedSession(t *testing.T, router http.Handler, s *fakeStore) (owner, guest playerHandle, sessionID string) {
	t.Helper()
	owner, guest, sessionID = setupActiveSession(t, router, s)
	path := "/api/v1/sessions/" + sessionID

	postJSONAs(t, router, path+"/pause", uuid.MustParse(owner.id), "player", nil)
	postJSONAs(t, router, path+"/pause", uuid.MustParse(guest.id), "player", nil)
	return owner, guest, sessionID
}

// playerHandle wraps a player ID string for readability in table-driven tests.
type playerHandle struct{ id string }

// --- POST /sessions/{sessionID}/pause ----------------------------------------

func TestVotePause_FirstVote(t *testing.T) {
	router, s := newTestRouter(t)
	owner, _, sessionID := setupActiveSession(t, router, s)

	w := postJSONAs(t, router, "/api/v1/sessions/"+sessionID+"/pause", uuid.MustParse(owner.id), "player", nil)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var result map[string]any
	if err := json.NewDecoder(w.Body).Decode(&result); err != nil {
		t.Fatalf("failed to decode body: %v", err)
	}

	if result["all_voted"].(bool) {
		t.Error("expected all_voted=false after first vote")
	}

	votesCount, ok := result["votes"].(float64)
	if !ok {
		t.Errorf("expected 'votes' to be a number, got %T", result["votes"])
	} else if int(votesCount) != 1 {
		t.Errorf("expected 1 vote count, got %v", votesCount)
	}

	if int(result["required"].(float64)) != 2 {
		t.Errorf("expected required=2, got %v", result["required"])
	}
}
func TestVotePause_AllVoted_Suspends(t *testing.T) {
	router, s := newTestRouter(t)
	owner, guest, sessionID := setupActiveSession(t, router, s)
	path := "/api/v1/sessions/" + sessionID + "/pause"

	postJSONAs(t, router, path, uuid.MustParse(owner.id), "player", nil)
	w := postJSONAs(t, router, path, uuid.MustParse(guest.id), "player", nil)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var result map[string]any
	json.NewDecoder(w.Body).Decode(&result)
	if !result["all_voted"].(bool) {
		t.Error("expected all_voted=true after all players voted")
	}

	sessionID_uuid := findSessionID(t, s, sessionID)
	gs := s.Sessions[sessionID_uuid]
	if gs.SuspendedAt == nil {
		t.Error("expected session to be suspended after all votes")
	}
}

func TestVotePause_AlreadyPaused(t *testing.T) {
	router, s := newTestRouter(t)
	owner, _, sessionID := setupSuspendedSession(t, router, s)

	// Usamos postJSONAs para que pase el 401 y llegue al error de lógica 409
	w := postJSONAs(t, router, "/api/v1/sessions/"+sessionID+"/pause", uuid.MustParse(owner.id), "player", nil)

	if w.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d: %s", w.Code, w.Body.String())
	}
}

func TestVotePause_GameOver(t *testing.T) {
	router, s := newTestRouter(t)
	owner, _, sessionID := setupActiveSession(t, router, s)

	id := findSessionID(t, s, sessionID)
	s.FinishSession(context.Background(), id)

	w := postJSONAs(t, router, "/api/v1/sessions/"+sessionID+"/pause", uuid.MustParse(owner.id), "player", nil)

	if w.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d: %s", w.Code, w.Body.String())
	}
}

func TestVotePause_NotParticipant(t *testing.T) {
	router, s := newTestRouter(t)
	_, _, sessionID := setupActiveSession(t, router, s)
	outsider, _ := s.CreatePlayer(context.Background(), "charlie")

	w := postJSONAs(t, router, "/api/v1/sessions/"+sessionID+"/pause", outsider.ID, "player", nil)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", w.Code, w.Body.String())
	}
}

func TestVotePause_InvalidSessionID(t *testing.T) {
	router, _ := newTestRouter(t)

	w := postJSON(t, router, "/api/v1/sessions/not-a-uuid/pause", map[string]string{
		"player_id": "also-not-a-uuid",
	})

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestVotePause_InvalidPlayerID(t *testing.T) {
	router, s := newTestRouter(t)
	_, _, sessionID := setupActiveSession(t, router, s)

	w := postJSON(t, router, "/api/v1/sessions/"+sessionID+"/pause", map[string]string{
		"player_id": "not-a-uuid",
	})

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

// --- POST /sessions/{sessionID}/resume ---------------------------------------

func TestVoteResume_FirstVote(t *testing.T) {
	router, s := newTestRouter(t)
	owner, _, sessionID := setupSuspendedSession(t, router, s)

	w := postJSONAs(t, router, "/api/v1/sessions/"+sessionID+"/resume", uuid.MustParse(owner.id), "player", nil)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestVoteResume_AllVoted_Resumes(t *testing.T) {
	router, s := newTestRouter(t)
	owner, guest, sessionID := setupSuspendedSession(t, router, s)
	path := "/api/v1/sessions/" + sessionID + "/resume"

	postJSONAs(t, router, path, uuid.MustParse(owner.id), "player", nil)
	w := postJSONAs(t, router, path, uuid.MustParse(guest.id), "player", nil)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var result map[string]any
	json.NewDecoder(w.Body).Decode(&result)
	if !result["all_voted"].(bool) {
		t.Error("expected all_voted=true after all players voted to resume")
	}

	// Verify the session is no longer suspended in the store.
	id := findSessionID(t, s, sessionID)
	gs := s.Sessions[id]
	if gs.SuspendedAt != nil {
		t.Error("expected session to be resumed (SuspendedAt=nil)")
	}
	if len(s.ResumeVoteMap[id]) != 0 {
		t.Errorf("expected resume votes cleared, got %d", len(s.ResumeVoteMap[id]))
	}
}

func TestVoteResume_NotSuspended(t *testing.T) {
	router, s := newTestRouter(t)
	owner, _, sessionID := setupActiveSession(t, router, s)

	w := postJSONAs(t, router, "/api/v1/sessions/"+sessionID+"/resume", uuid.MustParse(owner.id), "player", nil)

	if w.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d: %s", w.Code, w.Body.String())
	}
}

func TestVoteResume_NotParticipant(t *testing.T) {
	router, s := newTestRouter(t)
	_, _, sessionID := setupSuspendedSession(t, router, s)
	outsider, _ := s.CreatePlayer(context.Background(), "charlie")

	w := postJSONAs(t, router, "/api/v1/sessions/"+sessionID+"/resume", outsider.ID, "player", nil)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", w.Code, w.Body.String())
	}
}

func TestVoteResume_GameOver(t *testing.T) {
	router, s := newTestRouter(t)
	owner, _, sessionID := setupSuspendedSession(t, router, s)

	id := findSessionID(t, s, sessionID)
	s.FinishSession(context.Background(), id)

	w := postJSONAs(t, router, "/api/v1/sessions/"+sessionID+"/resume", uuid.MustParse(owner.id), "player", nil)

	if w.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d: %s", w.Code, w.Body.String())
	}
}

func TestVoteResume_InvalidSessionID(t *testing.T) {
	router, _ := newTestRouter(t)

	w := postJSON(t, router, "/api/v1/sessions/not-a-uuid/resume", map[string]string{
		"player_id": "also-not-a-uuid",
	})

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

// --- DELETE /sessions/{sessionID} (force close, manager-only) ----------------

func TestForceCloseSession(t *testing.T) {
	router, s := newTestRouter(t)
	_, _, sessionID := setupSuspendedSession(t, router, s)

	// requireRole(manager) runs before the handler — without auth middleware
	// in tests this returns 401, consistent with TestHideRoomMessage.
	w := deleteJSON(t, router, "/api/v1/sessions/"+sessionID)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d: %s", w.Code, w.Body.String())
	}
}

func TestForceCloseSession_InvalidSessionID(t *testing.T) {
	router, _ := newTestRouter(t)

	w := deleteJSON(t, router, "/api/v1/sessions/not-a-uuid")

	// requireRole fires first — returns 401 before path parsing.
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

// --- Helpers -----------------------------------------------------------------

// findSessionID resolves a session ID string back to a uuid.UUID by scanning
// the fake store. Used to inspect store state after handler calls.
func findSessionID(t *testing.T, s *fakeStore, sessionIDStr string) uuid.UUID {
	t.Helper()
	for uid := range s.Sessions {
		if uid.String() == sessionIDStr {
			return uid
		}
	}
	t.Fatalf("session %s not found in fake store", sessionIDStr)
	return uuid.Nil
}
