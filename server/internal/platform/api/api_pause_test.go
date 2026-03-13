package api_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/google/uuid"
)

// setupActiveSession creates two players, a room, joins both, starts the game,
// and returns the two players and the started session ID as a string.
func setupActiveSession(t *testing.T, router http.Handler, s *fakeStore) (owner, guest playerHandle, sessionID string) {
	t.Helper()
	ctx := context.Background()

	ownerPlayer, _ := s.CreatePlayer(ctx, "alice")
	guestPlayer, _ := s.CreatePlayer(ctx, "bob")

	createResp := postJSON(t, router, "/api/v1/rooms", map[string]string{
		"game_id":   "chess",
		"player_id": ownerPlayer.ID.String(),
	})
	var roomView map[string]any
	json.NewDecoder(createResp.Body).Decode(&roomView)
	room := roomView["room"].(map[string]any)
	roomID := room["id"].(string)
	code := room["code"].(string)

	postJSON(t, router, "/api/v1/rooms/join", map[string]string{
		"code":      code,
		"player_id": guestPlayer.ID.String(),
	})

	startResp := postJSON(t, router, "/api/v1/rooms/"+roomID+"/start", map[string]string{
		"player_id": ownerPlayer.ID.String(),
	})
	var session map[string]any
	json.NewDecoder(startResp.Body).Decode(&session)

	return playerHandle{ownerPlayer.ID.String()},
		playerHandle{guestPlayer.ID.String()},
		session["id"].(string)
}

// setupSuspendedSession builds on setupActiveSession and casts pause votes from
// both players to bring the session into suspended state.
func setupSuspendedSession(t *testing.T, router http.Handler, s *fakeStore) (owner, guest playerHandle, sessionID string) {
	t.Helper()
	owner, guest, sessionID = setupActiveSession(t, router, s)
	path := "/api/v1/sessions/" + sessionID

	postJSON(t, router, path+"/pause", map[string]string{"player_id": owner.id})
	postJSON(t, router, path+"/pause", map[string]string{"player_id": guest.id})
	return owner, guest, sessionID
}

// playerHandle wraps a player ID string for readability in table-driven tests.
type playerHandle struct{ id string }

// --- POST /sessions/{sessionID}/pause ----------------------------------------

func TestVotePause_FirstVote(t *testing.T) {
	router, s := newTestRouter(t)
	owner, _, sessionID := setupActiveSession(t, router, s)

	w := postJSON(t, router, "/api/v1/sessions/"+sessionID+"/pause", map[string]string{
		"player_id": owner.id,
	})

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var result map[string]any
	json.NewDecoder(w.Body).Decode(&result)

	if result["all_voted"].(bool) {
		t.Error("expected all_voted=false after first vote")
	}
	votes := result["votes"].([]any)
	if len(votes) != 1 {
		t.Errorf("expected 1 vote, got %d", len(votes))
	}
	if int(result["required"].(float64)) != 2 {
		t.Errorf("expected required=2, got %v", result["required"])
	}
}

func TestVotePause_AllVoted_Suspends(t *testing.T) {
	router, s := newTestRouter(t)
	owner, guest, sessionID := setupActiveSession(t, router, s)
	path := "/api/v1/sessions/" + sessionID + "/pause"

	postJSON(t, router, path, map[string]string{"player_id": owner.id})
	w := postJSON(t, router, path, map[string]string{"player_id": guest.id})

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var result map[string]any
	json.NewDecoder(w.Body).Decode(&result)
	if !result["all_voted"].(bool) {
		t.Error("expected all_voted=true after all players voted")
	}

	// Verify the session is now suspended in the store.
	sessionID_uuid := findSessionID(t, s, sessionID)
	gs := s.Sessions[sessionID_uuid]
	if gs.SuspendedAt == nil {
		t.Error("expected session to be suspended after all votes")
	}
	if len(gs.PauseVotes) != 0 {
		t.Errorf("expected pause_votes cleared, got %d", len(gs.PauseVotes))
	}
}

func TestVotePause_AlreadyPaused(t *testing.T) {
	router, s := newTestRouter(t)
	owner, _, sessionID := setupSuspendedSession(t, router, s)

	w := postJSON(t, router, "/api/v1/sessions/"+sessionID+"/pause", map[string]string{
		"player_id": owner.id,
	})

	if w.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d: %s", w.Code, w.Body.String())
	}
}

func TestVotePause_GameOver(t *testing.T) {
	router, s := newTestRouter(t)
	owner, _, sessionID := setupActiveSession(t, router, s)

	// Finish the session directly via the store.
	id := findSessionID(t, s, sessionID)
	s.FinishSession(context.Background(), id)

	w := postJSON(t, router, "/api/v1/sessions/"+sessionID+"/pause", map[string]string{
		"player_id": owner.id,
	})

	if w.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d: %s", w.Code, w.Body.String())
	}
}

func TestVotePause_NotParticipant(t *testing.T) {
	router, s := newTestRouter(t)
	_, _, sessionID := setupActiveSession(t, router, s)
	outsider, _ := s.CreatePlayer(context.Background(), "charlie")

	w := postJSON(t, router, "/api/v1/sessions/"+sessionID+"/pause", map[string]string{
		"player_id": outsider.ID.String(),
	})

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

	w := postJSON(t, router, "/api/v1/sessions/"+sessionID+"/resume", map[string]string{
		"player_id": owner.id,
	})

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var result map[string]any
	json.NewDecoder(w.Body).Decode(&result)
	if result["all_voted"].(bool) {
		t.Error("expected all_voted=false after first resume vote")
	}
	votes := result["votes"].([]any)
	if len(votes) != 1 {
		t.Errorf("expected 1 vote, got %d", len(votes))
	}
}

func TestVoteResume_AllVoted_Resumes(t *testing.T) {
	router, s := newTestRouter(t)
	owner, guest, sessionID := setupSuspendedSession(t, router, s)
	path := "/api/v1/sessions/" + sessionID + "/resume"

	postJSON(t, router, path, map[string]string{"player_id": owner.id})
	w := postJSON(t, router, path, map[string]string{"player_id": guest.id})

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
	if len(gs.ResumeVotes) != 0 {
		t.Errorf("expected resume_votes cleared, got %d", len(gs.ResumeVotes))
	}
}

func TestVoteResume_NotSuspended(t *testing.T) {
	router, s := newTestRouter(t)
	owner, _, sessionID := setupActiveSession(t, router, s)

	w := postJSON(t, router, "/api/v1/sessions/"+sessionID+"/resume", map[string]string{
		"player_id": owner.id,
	})

	if w.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d: %s", w.Code, w.Body.String())
	}
}

func TestVoteResume_NotParticipant(t *testing.T) {
	router, s := newTestRouter(t)
	_, _, sessionID := setupSuspendedSession(t, router, s)
	outsider, _ := s.CreatePlayer(context.Background(), "charlie")

	w := postJSON(t, router, "/api/v1/sessions/"+sessionID+"/resume", map[string]string{
		"player_id": outsider.ID.String(),
	})

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", w.Code, w.Body.String())
	}
}

func TestVoteResume_GameOver(t *testing.T) {
	router, s := newTestRouter(t)
	owner, _, sessionID := setupSuspendedSession(t, router, s)

	id := findSessionID(t, s, sessionID)
	s.FinishSession(context.Background(), id)

	w := postJSON(t, router, "/api/v1/sessions/"+sessionID+"/resume", map[string]string{
		"player_id": owner.id,
	})

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
