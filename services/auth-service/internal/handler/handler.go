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

	"github.com/google/uuid"
	authjwt "github.com/recess/auth-service/internal/jwt"
	"github.com/recess/shared/middleware"
)

// Store is the minimal DB interface auth-service needs.
// Implemented by the real store (pgx) and a mock in tests.
// Only touches: players, oauth_identities, allowed_emails.
type Store interface {
	IsEmailAllowed(ctx context.Context, email string) (bool, error)
	UpsertOAuthIdentity(ctx context.Context, params UpsertOAuthParams) (OAuthIdentity, error)
	GetPlayer(ctx context.Context, id uuid.UUID) (Player, error)
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
		slog.Warn("auth: email not allowed", "email", email)
		http.Redirect(w, r, "/?error=email_not_allowed", http.StatusTemporaryRedirect)
		return
	}

	avatarURL := ghUser.AvatarURL
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

	authjwt.SetSessionCookie(w, token, h.secure)
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

// HandleLogout clears the session cookie.
func (h *Handler) HandleLogout(w http.ResponseWriter, r *http.Request) {
	authjwt.ClearSessionCookie(w)
	w.WriteHeader(http.StatusNoContent)
}

// --- helpers -----------------------------------------------------------------

func randomState() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(b), nil
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
