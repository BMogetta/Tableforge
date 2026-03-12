package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/tableforge/server/internal/store"
	"github.com/tableforge/server/internal/testutil"
)

func newTestHandlerWithStore(s *testutil.FakeStore) *Handler {
	return NewHandler(s, "", "", string(testSecret), false)
}

// sentinel is a simple next handler that records whether it was called.
func sentinel(called *bool) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*called = true
		w.WriteHeader(http.StatusOK)
	})
}

func TestMiddleware_NoCookie(t *testing.T) {
	h := newTestHandlerWithStore(testutil.NewFakeStore())
	called := false
	handler := h.Middleware(sentinel(&called))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
	if called {
		t.Error("next handler should not have been called")
	}
}

func TestMiddleware_InvalidToken(t *testing.T) {
	h := newTestHandlerWithStore(testutil.NewFakeStore())
	called := false
	handler := h.Middleware(sentinel(&called))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: cookieName, Value: "garbage"})
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
	if called {
		t.Error("next handler should not have been called")
	}
}

func TestMiddleware_ValidToken(t *testing.T) {
	playerID := uuid.New()
	username := "alice"

	// Seed the player so GetPlayer succeeds during middleware execution.
	s := testutil.NewFakeStore()
	s.Players[playerID] = store.Player{
		ID:        playerID,
		Username:  username,
		CreatedAt: time.Now(),
	}

	h := newTestHandlerWithStore(s)
	token, _ := signToken(testSecret, playerID, username)

	var (
		called        bool
		gotPlayerID   uuid.UUID
		gotUsername   string
		playerIDFound bool
		usernameFound bool
	)

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		gotPlayerID, playerIDFound = PlayerIDFromContext(r.Context())
		gotUsername, usernameFound = UsernameFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: cookieName, Value: token})
	w := httptest.NewRecorder()
	h.Middleware(next).ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	if !called {
		t.Fatal("next handler was not called")
	}
	if !playerIDFound || gotPlayerID != playerID {
		t.Errorf("expected player ID %s in context, got %s", playerID, gotPlayerID)
	}
	if !usernameFound || gotUsername != username {
		t.Errorf("expected username %s in context, got %s", username, gotUsername)
	}
}
