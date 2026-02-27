package auth_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/tableforge/server/internal/auth"
	"github.com/tableforge/server/internal/store"
	"github.com/tableforge/server/internal/testutil"
)

func newHandlerWithStore(t *testing.T) (*auth.Handler, *testutil.FakeStore) {
	t.Helper()
	s := testutil.NewFakeStore()
	h := auth.NewHandler(s, "client-id", "client-secret", "test-secret", false)
	return h, s
}

func TestHandleMe_Unauthenticated(t *testing.T) {
	h, _ := newHandlerWithStore(t)

	req := httptest.NewRequest(http.MethodGet, "/auth/me", nil)
	w := httptest.NewRecorder()
	h.HandleMe(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestHandleMe_PlayerNotFound(t *testing.T) {
	h, _ := newHandlerWithStore(t)

	// Inject a player ID that doesn't exist in the store.
	ctx := auth.ContextWithPlayer(context.Background(), uuid.New(), "ghost")
	req := httptest.NewRequest(http.MethodGet, "/auth/me", nil).WithContext(ctx)
	w := httptest.NewRecorder()
	h.HandleMe(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestHandleMe_OK(t *testing.T) {
	h, s := newHandlerWithStore(t)

	player, _ := s.CreatePlayer(context.Background(), "alice")
	ctx := auth.ContextWithPlayer(context.Background(), player.ID, player.Username)
	req := httptest.NewRequest(http.MethodGet, "/auth/me", nil).WithContext(ctx)
	w := httptest.NewRecorder()
	h.HandleMe(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var got store.Player
	if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got.ID != player.ID {
		t.Errorf("expected player ID %s, got %s", player.ID, got.ID)
	}
	if got.Username != "alice" {
		t.Errorf("expected username alice, got %s", got.Username)
	}
}

func TestHandleLogout(t *testing.T) {
	h, _ := newHandlerWithStore(t)

	req := httptest.NewRequest(http.MethodPost, "/auth/logout", nil)
	w := httptest.NewRecorder()
	h.HandleLogout(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("expected 204, got %d", w.Code)
	}

	// Cookie should be cleared (MaxAge == -1).
	cookies := w.Result().Cookies()
	var found bool
	for _, c := range cookies {
		if c.Name == "tf_session" {
			found = true
			if c.MaxAge != -1 {
				t.Errorf("expected MaxAge -1, got %d", c.MaxAge)
			}
		}
	}
	if !found {
		t.Error("expected tf_session cookie in response")
	}
}
