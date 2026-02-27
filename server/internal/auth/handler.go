package auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/tableforge/server/internal/store"
)

// Handler holds dependencies for all auth routes.
type Handler struct {
	store        store.Store
	clientID     string
	clientSecret string
	jwtSecret    []byte
	secure       bool // set false in dev (HTTP), true in prod (HTTPS)
}

// ContextWithPlayer injects player data into a context. Used in tests.
func ContextWithPlayer(ctx context.Context, id uuid.UUID, username string) context.Context {
	ctx = context.WithValue(ctx, keyPlayerID, id)
	return context.WithValue(ctx, keyUsername, username)
}

// NewHandler constructs a Handler. secure should be true in production.
func NewHandler(st store.Store, clientID, clientSecret, jwtSecret string, secure bool) *Handler {
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

	// Store state in a short-lived cookie to validate on callback.
	http.SetCookie(w, &http.Cookie{
		Name:     "oauth_state",
		Value:    state,
		Path:     "/auth",
		HttpOnly: true,
		Secure:   h.secure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   300, // 5 minutes
	})

	redirectURL := fmt.Sprintf(
		"%s?client_id=%s&scope=read:user,user:email&state=%s",
		githubAuthURL, h.clientID, state,
	)
	http.Redirect(w, r, redirectURL, http.StatusTemporaryRedirect)
}

// HandleGitHubCallback handles the OAuth callback from GitHub.
func (h *Handler) HandleGitHubCallback(w http.ResponseWriter, r *http.Request) {
	// Validate state.
	stateCookie, err := r.Cookie("oauth_state")
	if err != nil || stateCookie.Value != r.URL.Query().Get("state") {
		http.Error(w, "invalid oauth state", http.StatusBadRequest)
		return
	}
	// Clear state cookie.
	http.SetCookie(w, &http.Cookie{Name: "oauth_state", Path: "/auth", MaxAge: -1})

	code := r.URL.Query().Get("code")
	if code == "" {
		http.Error(w, "missing code", http.StatusBadRequest)
		return
	}

	ctx := r.Context()

	// Exchange code for access token.
	accessToken, err := exchangeCode(ctx, h.clientID, h.clientSecret, code)
	if err != nil {
		log.Printf("auth: exchange code: %v", err)
		http.Error(w, "oauth failed", http.StatusBadGateway)
		return
	}

	// Fetch GitHub user profile and primary email.
	ghUser, err := fetchGitHubUser(ctx, accessToken)
	if err != nil {
		log.Printf("auth: fetch github user: %v", err)
		http.Error(w, "failed to fetch user", http.StatusBadGateway)
		return
	}

	email, err := fetchPrimaryEmail(ctx, accessToken)
	if err != nil {
		log.Printf("auth: fetch email: %v", err)
		http.Error(w, "failed to fetch email", http.StatusBadGateway)
		return
	}

	// Check email whitelist.
	allowed, err := h.store.IsEmailAllowed(ctx, email)
	if err != nil {
		log.Printf("auth: check email: %v", err)
		http.Redirect(w, r, "/?error=internal_error", http.StatusTemporaryRedirect)
		return
	}
	if !allowed {
		http.Redirect(w, r, "/?error=email_not_allowed", http.StatusTemporaryRedirect)
		return
	}

	// Upsert OAuth identity — creates or links a player automatically.
	avatarURL := ghUser.AvatarURL
	identity, err := h.store.UpsertOAuthIdentity(ctx, store.UpsertOAuthParams{
		Provider:   "github",
		ProviderID: fmt.Sprintf("%d", ghUser.ID),
		Email:      email,
		AvatarURL:  &avatarURL,
		Username:   sanitizeUsername(ghUser.Login),
	})
	if err != nil {
		log.Printf("auth: upsert identity: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	// Fetch player to get username for the JWT.
	player, err := h.store.GetPlayer(ctx, identity.PlayerID)
	if err != nil {
		log.Printf("auth: get player: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	// Issue JWT session cookie.
	token, err := signToken(h.jwtSecret, player.ID, player.Username)
	if err != nil {
		log.Printf("auth: sign token: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	setSessionCookie(w, token, h.secure)
	http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
}

// HandleMe returns the currently authenticated player.
func (h *Handler) HandleMe(w http.ResponseWriter, r *http.Request) {
	playerID, ok := PlayerIDFromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	player, err := h.store.GetPlayer(r.Context(), playerID)
	if err != nil {
		http.Error(w, "player not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(player)
}

// HandleLogout clears the session cookie.
func (h *Handler) HandleLogout(w http.ResponseWriter, r *http.Request) {
	clearSessionCookie(w)
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

// sanitizeUsername lowercases and strips characters that aren't alphanumeric or
// underscores, so GitHub logins map cleanly to our username format.
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
