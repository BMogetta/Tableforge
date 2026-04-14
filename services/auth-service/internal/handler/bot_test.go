package handler

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/recess/shared/middleware"
)

func postBotLogin(t *testing.T, h *Handler, secret, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/auth/bot-login", bytes.NewReader([]byte(body)))
	if secret != "" {
		req.Header.Set("X-Bot-Secret", secret)
	}
	w := httptest.NewRecorder()
	h.HandleBotLogin(w, req)
	return w
}

func TestHandleBotLogin_SecretUnset_Rejects(t *testing.T) {
	// BOT_SERVICE_SECRET unset → endpoint is effectively disabled.
	st := newMockStore()
	playerID := uuid.New()
	st.players[playerID] = Player{ID: playerID, Username: "bot_easy_1", Role: "player", IsBot: true}

	h := newHandler(st)
	w := postBotLogin(t, h, "anything", `{"player_id":"`+playerID.String()+`"}`)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401 when BOT_SERVICE_SECRET is unset", w.Code)
	}
}

func TestHandleBotLogin_SecretMismatch_Rejects(t *testing.T) {
	t.Setenv("BOT_SERVICE_SECRET", "correct-horse")

	st := newMockStore()
	playerID := uuid.New()
	st.players[playerID] = Player{ID: playerID, Username: "bot_easy_1", Role: "player", IsBot: true}

	h := newHandler(st)
	w := postBotLogin(t, h, "wrong-secret", `{"player_id":"`+playerID.String()+`"}`)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401 on secret mismatch", w.Code)
	}
}

func TestHandleBotLogin_MissingSecretHeader_Rejects(t *testing.T) {
	t.Setenv("BOT_SERVICE_SECRET", "correct-horse")

	h := newHandler(newMockStore())
	w := postBotLogin(t, h, "", `{"player_id":"`+uuid.New().String()+`"}`)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401 when header missing", w.Code)
	}
}

func TestHandleBotLogin_InvalidBody(t *testing.T) {
	t.Setenv("BOT_SERVICE_SECRET", "correct-horse")

	h := newHandler(newMockStore())
	w := postBotLogin(t, h, "correct-horse", `not json`)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400 for bad JSON", w.Code)
	}
}

func TestHandleBotLogin_InvalidPlayerID(t *testing.T) {
	t.Setenv("BOT_SERVICE_SECRET", "correct-horse")

	h := newHandler(newMockStore())
	w := postBotLogin(t, h, "correct-horse", `{"player_id":"not-a-uuid"}`)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400 for bad uuid", w.Code)
	}
}

func TestHandleBotLogin_PlayerNotFound(t *testing.T) {
	t.Setenv("BOT_SERVICE_SECRET", "correct-horse")

	h := newHandler(newMockStore())
	w := postBotLogin(t, h, "correct-horse", `{"player_id":"`+uuid.New().String()+`"}`)

	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404 for unknown player", w.Code)
	}
}

func TestHandleBotLogin_NonBotPlayer_Rejects(t *testing.T) {
	t.Setenv("BOT_SERVICE_SECRET", "correct-horse")

	st := newMockStore()
	playerID := uuid.New()
	// Human player — even with a valid secret, must not be allowed to log in.
	st.players[playerID] = Player{ID: playerID, Username: "alice", Role: "player", IsBot: false}

	h := newHandler(st)
	w := postBotLogin(t, h, "correct-horse", `{"player_id":"`+playerID.String()+`"}`)

	if w.Code != http.StatusForbidden {
		t.Errorf("status = %d, want 403 when target player is not a bot", w.Code)
	}
}

func TestHandleBotLogin_Success(t *testing.T) {
	t.Setenv("BOT_SERVICE_SECRET", "correct-horse")

	st := newMockStore()
	playerID := uuid.New()
	st.players[playerID] = Player{ID: playerID, Username: "bot_medium_1", Role: "player", IsBot: true}

	h := newHandler(st)
	w := postBotLogin(t, h, "correct-horse", `{"player_id":"`+playerID.String()+`"}`)

	if w.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", w.Code)
	}

	cookies := w.Result().Cookies()
	var gotSession bool
	for _, c := range cookies {
		if c.Name == middleware.CookieName {
			gotSession = true
			if c.Value == "" {
				t.Error("session cookie is empty")
			}
		}
	}
	if !gotSession {
		t.Error("expected session cookie to be set")
	}
}
