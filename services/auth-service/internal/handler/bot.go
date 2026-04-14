package handler

import (
	"crypto/subtle"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"
	authjwt "github.com/recess/auth-service/internal/jwt"
	"github.com/recess/shared/config"
	"github.com/recess/shared/middleware"
)

// BotServiceSecret returns the configured secret used by bot-runner to
// authenticate to /auth/bot-login. Empty string means the endpoint is
// disabled (the handler will short-circuit on 401).
func BotServiceSecret() string {
	return config.Env("BOT_SERVICE_SECRET", "")
}

type botLoginRequest struct {
	PlayerID string `json:"player_id"`
}

// HandleBotLogin issues a session cookie for a bot player. Bot-runner uses
// it instead of the GitHub OAuth flow (which would require a real human).
//
// Auth: the caller must send X-Bot-Secret matching BOT_SERVICE_SECRET.
// The endpoint additionally refuses to log in any player with is_bot=false,
// so a leaked secret cannot be used to impersonate a human.
//
// Usage:
//
//	POST /auth/bot-login
//	X-Bot-Secret: <shared secret>
//	{"player_id": "<uuid>"}
func (h *Handler) HandleBotLogin(w http.ResponseWriter, r *http.Request) {
	want := BotServiceSecret()
	got := r.Header.Get("X-Bot-Secret")
	// Constant-time compare on equal-length slices. An unset server-side
	// secret makes every request fail — production-safe default.
	if want == "" || len(want) != len(got) || subtle.ConstantTimeCompare([]byte(want), []byte(got)) != 1 {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var req botLoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}

	playerID, err := uuid.Parse(req.PlayerID)
	if err != nil {
		http.Error(w, "invalid player_id", http.StatusBadRequest)
		return
	}

	player, err := h.store.GetPlayer(r.Context(), playerID)
	if err != nil {
		http.Error(w, "player not found", http.StatusNotFound)
		return
	}
	if !player.IsBot {
		// Defense in depth: even with a valid secret, refuse to mint a
		// token for a human account.
		slog.Warn("auth: bot-login rejected for non-bot player", "player_id", player.ID)
		http.Error(w, "not a bot", http.StatusForbidden)
		return
	}

	token, err := authjwt.SignToken(h.jwtSecret, player.ID, player.Username, player.Role)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	sessionID, err := h.store.CreateSession(r.Context(), CreateSessionParams{
		PlayerID:       player.ID,
		UserAgent:      r.UserAgent(),
		AcceptLanguage: r.Header.Get("Accept-Language"),
		IPAddress:      clientIP(r),
		ExpiresAt:      time.Now().Add(middleware.RefreshTTL),
	})
	if err != nil {
		slog.Error("auth: create bot session", "player_id", player.ID, "error", err)
	}

	authjwt.SetSessionCookie(w, token, h.secure)
	if sessionID != uuid.Nil {
		authjwt.SetRefreshCookie(w, sessionID.String(), h.secure)
	}
	w.WriteHeader(http.StatusNoContent)
}
