package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/tableforge/server/internal/platform/store"
)

// deleteJSONWithBody sends a DELETE request with a JSON body and returns the response.
// Used for unmute, which requires player_id in the body.
func deleteJSONWithBody(t *testing.T, router http.Handler, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodDelete, path, bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}

// --- POST /players/{playerID}/mute -------------------------------------------

func TestMutePlayer(t *testing.T) {
	router, s := newTestRouter(t)
	ctx := context.Background()
	alice, _ := s.CreatePlayer(ctx, "alice")
	bob, _ := s.CreatePlayer(ctx, "bob")

	w := postJSON(t, router, "/api/v1/players/"+bob.ID.String()+"/mute", map[string]string{
		"player_id": alice.ID.String(),
		"muted_id":  bob.ID.String(),
	})

	if w.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", w.Code, w.Body.String())
	}
}

func TestMutePlayer_Idempotent(t *testing.T) {
	router, s := newTestRouter(t)
	ctx := context.Background()
	alice, _ := s.CreatePlayer(ctx, "alice")
	bob, _ := s.CreatePlayer(ctx, "bob")

	body := map[string]string{
		"player_id": alice.ID.String(),
		"muted_id":  bob.ID.String(),
	}
	postJSON(t, router, "/api/v1/players/"+bob.ID.String()+"/mute", body)
	w := postJSON(t, router, "/api/v1/players/"+bob.ID.String()+"/mute", body)

	if w.Code != http.StatusNoContent {
		t.Fatalf("expected 204 on duplicate mute, got %d: %s", w.Code, w.Body.String())
	}
}

func TestMutePlayer_SelfMute(t *testing.T) {
	router, s := newTestRouter(t)
	alice, _ := s.CreatePlayer(context.Background(), "alice")

	w := postJSON(t, router, "/api/v1/players/"+alice.ID.String()+"/mute", map[string]string{
		"player_id": alice.ID.String(),
		"muted_id":  alice.ID.String(),
	})

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestMutePlayer_MutedIDMismatch(t *testing.T) {
	router, s := newTestRouter(t)
	ctx := context.Background()
	alice, _ := s.CreatePlayer(ctx, "alice")
	bob, _ := s.CreatePlayer(ctx, "bob")
	charlie, _ := s.CreatePlayer(ctx, "charlie")

	// Path says bob, body says charlie.
	w := postJSON(t, router, "/api/v1/players/"+bob.ID.String()+"/mute", map[string]string{
		"player_id": alice.ID.String(),
		"muted_id":  charlie.ID.String(),
	})

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestMutePlayer_InvalidPathID(t *testing.T) {
	router, _ := newTestRouter(t)

	w := postJSON(t, router, "/api/v1/players/not-a-uuid/mute", map[string]string{
		"player_id": "also-not-a-uuid",
		"muted_id":  "also-not-a-uuid",
	})

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestMutePlayer_InvalidPlayerID(t *testing.T) {
	router, s := newTestRouter(t)
	bob, _ := s.CreatePlayer(context.Background(), "bob")

	w := postJSON(t, router, "/api/v1/players/"+bob.ID.String()+"/mute", map[string]string{
		"player_id": "not-a-uuid",
		"muted_id":  bob.ID.String(),
	})

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

// --- DELETE /players/{playerID}/mute/{mutedID} --------------------------------

func TestUnmutePlayer(t *testing.T) {
	router, s := newTestRouter(t)
	ctx := context.Background()
	alice, _ := s.CreatePlayer(ctx, "alice")
	bob, _ := s.CreatePlayer(ctx, "bob")

	// Mute first.
	postJSON(t, router, "/api/v1/players/"+bob.ID.String()+"/mute", map[string]string{
		"player_id": alice.ID.String(),
		"muted_id":  bob.ID.String(),
	})

	w := deleteJSONWithBody(t, router,
		"/api/v1/players/"+alice.ID.String()+"/mute/"+bob.ID.String(),
		map[string]string{"player_id": alice.ID.String()},
	)

	if w.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", w.Code, w.Body.String())
	}
}

func TestUnmutePlayer_NoOp(t *testing.T) {
	router, s := newTestRouter(t)
	ctx := context.Background()
	alice, _ := s.CreatePlayer(ctx, "alice")
	bob, _ := s.CreatePlayer(ctx, "bob")

	// Unmute without a prior mute — should still be 204.
	w := deleteJSONWithBody(t, router,
		"/api/v1/players/"+alice.ID.String()+"/mute/"+bob.ID.String(),
		map[string]string{"player_id": alice.ID.String()},
	)

	if w.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", w.Code, w.Body.String())
	}
}

func TestUnmutePlayer_Forbidden(t *testing.T) {
	router, s := newTestRouter(t)
	ctx := context.Background()
	alice, _ := s.CreatePlayer(ctx, "alice")
	bob, _ := s.CreatePlayer(ctx, "bob")
	charlie, _ := s.CreatePlayer(ctx, "charlie")

	// Charlie tries to remove Alice's mute on Bob.
	w := deleteJSONWithBody(t, router,
		"/api/v1/players/"+alice.ID.String()+"/mute/"+bob.ID.String(),
		map[string]string{"player_id": charlie.ID.String()},
	)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", w.Code, w.Body.String())
	}
}

func TestUnmutePlayer_InvalidPathID(t *testing.T) {
	router, _ := newTestRouter(t)

	w := deleteJSONWithBody(t, router,
		"/api/v1/players/not-a-uuid/mute/also-not-a-uuid",
		map[string]string{"player_id": "x"},
	)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestUnmutePlayer_InvalidMutedID(t *testing.T) {
	router, s := newTestRouter(t)
	alice, _ := s.CreatePlayer(context.Background(), "alice")

	w := deleteJSONWithBody(t, router,
		"/api/v1/players/"+alice.ID.String()+"/mute/not-a-uuid",
		map[string]string{"player_id": alice.ID.String()},
	)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

// --- GET /players/{playerID}/mutes -------------------------------------------

func TestGetMutedPlayers_Empty(t *testing.T) {
	router, s := newTestRouter(t)
	alice, _ := s.CreatePlayer(context.Background(), "alice")

	w := getJSONWithBody(t, router,
		"/api/v1/players/"+alice.ID.String()+"/mutes",
		map[string]string{"player_id": alice.ID.String()},
	)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var mutes []store.PlayerMute
	json.NewDecoder(w.Body).Decode(&mutes)
	if len(mutes) != 0 {
		t.Errorf("expected 0 mutes, got %d", len(mutes))
	}
}

func TestGetMutedPlayers_AfterMute(t *testing.T) {
	router, s := newTestRouter(t)
	ctx := context.Background()
	alice, _ := s.CreatePlayer(ctx, "alice")
	bob, _ := s.CreatePlayer(ctx, "bob")
	charlie, _ := s.CreatePlayer(ctx, "charlie")

	postJSON(t, router, "/api/v1/players/"+bob.ID.String()+"/mute", map[string]string{
		"player_id": alice.ID.String(),
		"muted_id":  bob.ID.String(),
	})
	postJSON(t, router, "/api/v1/players/"+charlie.ID.String()+"/mute", map[string]string{
		"player_id": alice.ID.String(),
		"muted_id":  charlie.ID.String(),
	})

	w := getJSONWithBody(t, router,
		"/api/v1/players/"+alice.ID.String()+"/mutes",
		map[string]string{"player_id": alice.ID.String()},
	)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var mutes []store.PlayerMute
	json.NewDecoder(w.Body).Decode(&mutes)
	if len(mutes) != 2 {
		t.Errorf("expected 2 mutes, got %d", len(mutes))
	}
}

func TestGetMutedPlayers_Forbidden(t *testing.T) {
	router, s := newTestRouter(t)
	ctx := context.Background()
	alice, _ := s.CreatePlayer(ctx, "alice")
	bob, _ := s.CreatePlayer(ctx, "bob")

	// Bob tries to read Alice's mute list without manager role.
	w := getJSONWithBody(t, router,
		"/api/v1/players/"+alice.ID.String()+"/mutes",
		map[string]string{"player_id": bob.ID.String()},
	)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", w.Code, w.Body.String())
	}
}

func TestGetMutedPlayers_InvalidPathID(t *testing.T) {
	router, _ := newTestRouter(t)

	w := getJSONWithBody(t, router,
		"/api/v1/players/not-a-uuid/mutes",
		map[string]string{"player_id": "x"},
	)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestGetMutedPlayers_InvalidPlayerID(t *testing.T) {
	router, s := newTestRouter(t)
	alice, _ := s.CreatePlayer(context.Background(), "alice")

	w := getJSONWithBody(t, router,
		"/api/v1/players/"+alice.ID.String()+"/mutes",
		map[string]string{"player_id": "not-a-uuid"},
	)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

// --- Helpers -----------------------------------------------------------------

// getJSONWithBody sends a GET request with a JSON body and returns the response.
// Used for endpoints that follow the established player_id-in-body pattern.
func getJSONWithBody(t *testing.T, router http.Handler, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodGet, path, bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}
