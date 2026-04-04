package handler

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	authjwt "github.com/recess/auth-service/internal/jwt"
	"github.com/recess/shared/middleware"
)

// Store is the minimal DB interface auth-service needs.
// Implemented by the real store (pgx) and a mock in tests.
type Store interface {
	IsEmailAllowed(ctx context.Context, email string) (bool, error)
	UpsertOAuthIdentity(ctx context.Context, params UpsertOAuthParams) (OAuthIdentity, error)
	GetPlayer(ctx context.Context, id uuid.UUID) (Player, error)
	CreateSession(ctx context.Context, params CreateSessionParams) (uuid.UUID, error)
	GetSession(ctx context.Context, sessionID uuid.UUID) (Session, error)
	RevokeSession(ctx context.Context, sessionID uuid.UUID) error
	RevokeAllSessions(ctx context.Context, playerID uuid.UUID) error
	TouchSession(ctx context.Context, sessionID uuid.UUID) error
	CheckActiveBan(ctx context.Context, playerID uuid.UUID) (bool, error)
}

// CreateSessionParams holds data for a new player_sessions row.
type CreateSessionParams struct {
	PlayerID       uuid.UUID
	UserAgent      string
	AcceptLanguage string
	IPAddress      string
	ExpiresAt      time.Time
}

// UpsertOAuthParams mirrors store.UpsertOAuthParams without importing the monolith store.
type UpsertOAuthParams struct {
	Provider   string
	ProviderID string
	Email      string
	AvatarURL  *string
	Username   string
}

// OAuthIdentity is the minimal shape auth-service cares about.
type OAuthIdentity struct {
	PlayerID uuid.UUID
}

// Player is the minimal shape auth-service cares about.
type Player struct {
	ID       uuid.UUID `json:"id"`
	Username string    `json:"username"`
	Role     string    `json:"role"`
}

// Session represents an active refresh session.
type Session struct {
	ID         uuid.UUID
	PlayerID   uuid.UUID
	ExpiresAt  time.Time
	LastSeenAt time.Time
}

// Handler holds dependencies for all auth routes.
type Handler struct {
	store        Store
	clientID     string
	clientSecret string
	jwtSecret    []byte
	secure       bool
}

// New constructs a Handler.
func New(st Store, clientID, clientSecret, jwtSecret string, secure bool) *Handler {
	return &Handler{
		store:        st,
		clientID:     clientID,
		clientSecret: clientSecret,
		jwtSecret:    []byte(jwtSecret),
		secure:       secure,
	}
}

// HandleGitHubLogin redirects the user to GitHub's OAuth consent screen.
func (h *Handler) HandleGitHubLogin(w http.ResponseWriter, r *http.Request) {
	state, err := randomState()
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "oauth_state",
		Value:    state,
		Path:     "/auth",
		HttpOnly: true,
		Secure:   h.secure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   300,
	})

	redirectURL := fmt.Sprintf(
		"%s?client_id=%s&scope=read:user,user:email&state=%s",
		githubAuthURL, h.clientID, state,
	)
	http.Redirect(w, r, redirectURL, http.StatusTemporaryRedirect)
}

// HandleGitHubCallback handles the OAuth callback from GitHub.
func (h *Handler) HandleGitHubCallback(w http.ResponseWriter, r *http.Request) {
	stateCookie, err := r.Cookie("oauth_state")
	if err != nil || stateCookie.Value != r.URL.Query().Get("state") {
		http.Error(w, "invalid oauth state", http.StatusBadRequest)
		return
	}
	http.SetCookie(w, &http.Cookie{Name: "oauth_state", Path: "/auth", MaxAge: -1})

	code := r.URL.Query().Get("code")
	if code == "" {
		http.Error(w, "missing code", http.StatusBadRequest)
		return
	}

	ctx := r.Context()

	accessToken, err := exchangeCode(ctx, h.clientID, h.clientSecret, code)
	if err != nil {
		slog.Error("auth: exchange code", "error", err)
		http.Error(w, "oauth failed", http.StatusBadGateway)
		return
	}

	ghUser, err := fetchGitHubUser(ctx, accessToken)
	if err != nil {
		slog.Error("auth: fetch github user", "error", err)
		http.Error(w, "failed to fetch user", http.StatusBadGateway)
		return
	}

	email, err := fetchPrimaryEmail(ctx, accessToken)
	if err != nil {
		slog.Error("auth: fetch email", "error", err)
		http.Error(w, "failed to fetch email", http.StatusBadGateway)
		return
	}

	allowed, err := h.store.IsEmailAllowed(ctx, email)
	if err != nil {
		slog.Error("auth: check email", "error", err)
		http.Redirect(w, r, "/?error=internal_error", http.StatusTemporaryRedirect)
		return
	}
	if !allowed {
		slog.Warn("auth: email not allowed", "email", maskEmail(email))
		http.Redirect(w, r, "/?error=email_not_allowed", http.StatusTemporaryRedirect)
		return
	}

	avatarURL := ghUser.AvatarURL
	if !strings.HasPrefix(avatarURL, "https://avatars.githubusercontent.com/") {
		avatarURL = ""
	}
	identity, err := h.store.UpsertOAuthIdentity(ctx, UpsertOAuthParams{
		Provider:   "github",
		ProviderID: fmt.Sprintf("%d", ghUser.ID),
		Email:      email,
		AvatarURL:  &avatarURL,
		Username:   sanitizeUsername(ghUser.Login),
	})
	if err != nil {
		slog.Error("auth: upsert identity", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	player, err := h.store.GetPlayer(ctx, identity.PlayerID)
	if err != nil {
		slog.Error("auth: get player", "player_id", identity.PlayerID, "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	token, err := authjwt.SignToken(h.jwtSecret, player.ID, player.Username, player.Role)
	if err != nil {
		slog.Error("auth: sign token", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	// Create session row for refresh token rotation.
	sessionID, err := h.store.CreateSession(ctx, CreateSessionParams{
		PlayerID:       player.ID,
		UserAgent:      r.UserAgent(),
		AcceptLanguage: r.Header.Get("Accept-Language"),
		IPAddress:      clientIP(r),
		ExpiresAt:      time.Now().Add(middleware.RefreshTTL),
	})
	if err != nil {
		slog.Error("auth: create session", "player_id", player.ID, "error", err)
		// Still set the access token — the user can log in, just won't have refresh.
	}

	authjwt.SetSessionCookie(w, token, h.secure)
	if sessionID != uuid.Nil {
		authjwt.SetRefreshCookie(w, sessionID.String(), h.secure)
	}
	http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
}

// HandleMe returns the currently authenticated player.
func (h *Handler) HandleMe(w http.ResponseWriter, r *http.Request) {
	playerID, ok := middleware.PlayerIDFromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	player, err := h.store.GetPlayer(r.Context(), playerID)
	if err != nil {
		slog.Error("auth: get player for /me", "player_id", playerID, "error", err)
		http.Error(w, "player not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(player)
}

// HandleLogout revokes the refresh session and clears both cookies.
func (h *Handler) HandleLogout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(middleware.RefreshCookieName); err == nil {
		if sessionID, err := uuid.Parse(c.Value); err == nil {
			if err := h.store.RevokeSession(r.Context(), sessionID); err != nil {
				slog.Error("auth: revoke session on logout", "session_id", sessionID, "error", err)
			}
		}
	}
	authjwt.ClearAllCookies(w)
	w.WriteHeader(http.StatusNoContent)
}

// HandleRefresh rotates the refresh session and issues new access + refresh cookies.
// POST /auth/refresh — no auth middleware (the access token may be expired).
func (h *Handler) HandleRefresh(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie(middleware.RefreshCookieName)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "missing refresh token"})
		return
	}

	sessionID, err := uuid.Parse(cookie.Value)
	if err != nil {
		authjwt.ClearAllCookies(w)
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid refresh token"})
		return
	}

	ctx := r.Context()

	sess, err := h.store.GetSession(ctx, sessionID)
	if err != nil {
		authjwt.ClearAllCookies(w)
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "session expired or revoked"})
		return
	}

	// Check if the player is banned — if so, revoke and reject.
	banned, err := h.store.CheckActiveBan(ctx, sess.PlayerID)
	if err != nil {
		slog.Error("auth: check ban on refresh", "player_id", sess.PlayerID, "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}
	if banned {
		_ = h.store.RevokeSession(ctx, sessionID)
		authjwt.ClearAllCookies(w)
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "account banned"})
		return
	}

	// Fetch player to get current role/username for the new JWT.
	player, err := h.store.GetPlayer(ctx, sess.PlayerID)
	if err != nil {
		slog.Error("auth: get player on refresh", "player_id", sess.PlayerID, "error", err)
		authjwt.ClearAllCookies(w)
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "player not found"})
		return
	}

	// Rotate: revoke current session, create new one.
	if err := h.store.RevokeSession(ctx, sessionID); err != nil {
		slog.Error("auth: revoke old session", "session_id", sessionID, "error", err)
	}

	newSessionID, err := h.store.CreateSession(ctx, CreateSessionParams{
		PlayerID:       player.ID,
		UserAgent:      r.UserAgent(),
		AcceptLanguage: r.Header.Get("Accept-Language"),
		IPAddress:      clientIP(r),
		ExpiresAt:      time.Now().Add(middleware.RefreshTTL),
	})
	if err != nil {
		slog.Error("auth: create rotated session", "player_id", player.ID, "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}

	token, err := authjwt.SignToken(h.jwtSecret, player.ID, player.Username, player.Role)
	if err != nil {
		slog.Error("auth: sign token on refresh", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}

	authjwt.SetSessionCookie(w, token, h.secure)
	authjwt.SetRefreshCookie(w, newSessionID.String(), h.secure)
	w.WriteHeader(http.StatusNoContent)
}

// --- helpers -----------------------------------------------------------------

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func randomState() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(b), nil
}

// clientIP extracts the client IP from the request, preferring X-Forwarded-For
// (set by Traefik) and falling back to RemoteAddr.
func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		if i := strings.Index(xff, ","); i != -1 {
			return strings.TrimSpace(xff[:i])
		}
		return strings.TrimSpace(xff)
	}
	host := r.RemoteAddr
	if i := strings.LastIndex(host, ":"); i != -1 {
		return host[:i]
	}
	return host
}

// maskEmail masks the local part of an email address for safe logging.
// "alice@example.com" -> "al***@example.com"
func maskEmail(email string) string {
	at := strings.LastIndex(email, "@")
	if at <= 0 {
		return "***"
	}
	local := email[:at]
	domain := email[at:]
	if len(local) <= 2 {
		return "***" + domain
	}
	return local[:2] + "***" + domain
}

// sanitizeUsername lowercases and strips characters that aren't alphanumeric
// or underscores, so GitHub logins map cleanly to our username format.
func sanitizeUsername(login string) string {
	login = strings.ToLower(login)
	var b strings.Builder
	for _, r := range login {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_' {
			b.WriteRune(r)
		} else if r == '-' {
			b.WriteRune('_')
		}
	}
	return b.String()
}
