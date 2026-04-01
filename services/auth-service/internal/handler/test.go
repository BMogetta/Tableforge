package handler

import (
	"net/http"
	"os"

	"github.com/google/uuid"
	authjwt "github.com/recess/auth-service/internal/jwt"
)

// TestModeEnabled returns true when TEST_MODE=true.
// Must never be enabled in production.
func TestModeEnabled() bool {
	return os.Getenv("TEST_MODE") == "true"
}

// HandleTestLogin is a test-only endpoint that sets a session cookie for any
// player ID without requiring GitHub OAuth.
//
// Usage: GET /auth/test-login?player_id=<uuid>
//
// Only available when TEST_MODE=true. Returns 404 in production.
func (h *Handler) HandleTestLogin(w http.ResponseWriter, r *http.Request) {
	if !TestModeEnabled() {
		http.NotFound(w, r)
		return
	}

	rawID := r.URL.Query().Get("player_id")
	playerID, err := uuid.Parse(rawID)
	if err != nil {
		http.Error(w, "invalid player_id", http.StatusBadRequest)
		return
	}

	player, err := h.store.GetPlayer(r.Context(), playerID)
	if err != nil {
		http.Error(w, "player not found", http.StatusNotFound)
		return
	}

	token, err := authjwt.SignToken(h.jwtSecret, player.ID, player.Username, player.Role)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	authjwt.SetSessionCookie(w, token, h.secure)
	w.WriteHeader(http.StatusNoContent)
}
