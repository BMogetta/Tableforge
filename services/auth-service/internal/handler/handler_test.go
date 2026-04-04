package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/recess/shared/middleware"
)

// --- mock store --------------------------------------------------------------

type mockStore struct {
	players         map[uuid.UUID]Player
	allowedEmails   map[string]bool
	identities      map[string]OAuthIdentity // key: providerID
	sessions        map[uuid.UUID]Session
	bannedPlayers   map[uuid.UUID]bool
	upsertCallCount int
}

func newMockStore() *mockStore {
	return &mockStore{
		players:       make(map[uuid.UUID]Player),
		allowedEmails: make(map[string]bool),
		identities:    make(map[string]OAuthIdentity),
		sessions:      make(map[uuid.UUID]Session),
		bannedPlayers: make(map[uuid.UUID]bool),
	}
}

func (m *mockStore) IsEmailAllowed(_ context.Context, email string) (bool, error) {
	return m.allowedEmails[email], nil
}

func (m *mockStore) UpsertOAuthIdentity(_ context.Context, params UpsertOAuthParams) (OAuthIdentity, error) {
	m.upsertCallCount++
	if id, ok := m.identities[params.ProviderID]; ok {
		return id, nil
	}
	playerID := uuid.New()
	identity := OAuthIdentity{PlayerID: playerID}
	m.identities[params.ProviderID] = identity
	m.players[playerID] = Player{
		ID:       playerID,
		Username: params.Username,
		Role:     "player",
	}
	return identity, nil
}

func (m *mockStore) CreateSession(_ context.Context, params CreateSessionParams) (uuid.UUID, error) {
	id := uuid.New()
	m.sessions[id] = Session{
		ID:         id,
		PlayerID:   params.PlayerID,
		ExpiresAt:  params.ExpiresAt,
		LastSeenAt: time.Now(),
	}
	return id, nil
}

func (m *mockStore) GetSession(_ context.Context, id uuid.UUID) (Session, error) {
	s, ok := m.sessions[id]
	if !ok {
		return Session{}, fmt.Errorf("session not found")
	}
	return s, nil
}

func (m *mockStore) RevokeSession(_ context.Context, id uuid.UUID) error {
	delete(m.sessions, id)
	return nil
}

func (m *mockStore) RevokeAllSessions(_ context.Context, playerID uuid.UUID) error {
	for id, s := range m.sessions {
		if s.PlayerID == playerID {
			delete(m.sessions, id)
		}
	}
	return nil
}

func (m *mockStore) TouchSession(_ context.Context, id uuid.UUID) error {
	if s, ok := m.sessions[id]; ok {
		s.LastSeenAt = time.Now()
		m.sessions[id] = s
	}
	return nil
}

func (m *mockStore) CheckActiveBan(_ context.Context, playerID uuid.UUID) (bool, error) {
	return m.bannedPlayers[playerID], nil
}

func (m *mockStore) GetPlayer(_ context.Context, id uuid.UUID) (Player, error) {
	p, ok := m.players[id]
	if !ok {
		return Player{}, fmt.Errorf("player not found")
	}
	return p, nil
}

// --- helpers -----------------------------------------------------------------

func newHandler(st Store) *Handler {
	return New(st, "test-client-id", "test-client-secret", "test-jwt-secret-32bytes-long!!!!", false)
}

func withAuth(r *http.Request, playerID uuid.UUID, role string) *http.Request {
	ctx := middleware.ContextWithPlayer(r.Context(), playerID, "testuser", role)
	return r.WithContext(ctx)
}

// --- tests -------------------------------------------------------------------

func TestHandleMe(t *testing.T) {
	st := newMockStore()
	playerID := uuid.New()
	st.players[playerID] = Player{
		ID:       playerID,
		Username: "alice",
		Role:     "player",
	}

	h := newHandler(st)
	req := httptest.NewRequest(http.MethodGet, "/auth/me", nil)
	req = withAuth(req, playerID, "player")
	rec := httptest.NewRecorder()

	h.HandleMe(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var p Player
	if err := json.NewDecoder(rec.Body).Decode(&p); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if p.Username != "alice" {
		t.Errorf("expected username 'alice', got %q", p.Username)
	}
	if p.ID != playerID {
		t.Errorf("expected player ID %s, got %s", playerID, p.ID)
	}
}

func TestHandleMe_Unauthorized(t *testing.T) {
	h := newHandler(newMockStore())
	req := httptest.NewRequest(http.MethodGet, "/auth/me", nil)
	rec := httptest.NewRecorder()

	h.HandleMe(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rec.Code)
	}
}

func TestHandleMe_PlayerNotFound(t *testing.T) {
	st := newMockStore()
	h := newHandler(st)

	playerID := uuid.New() // not in store
	req := httptest.NewRequest(http.MethodGet, "/auth/me", nil)
	req = withAuth(req, playerID, "player")
	rec := httptest.NewRecorder()

	h.HandleMe(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rec.Code)
	}
}

func TestHandleLogout(t *testing.T) {
	st := newMockStore()
	h := newHandler(st)

	// Create a session so we can verify it gets revoked.
	playerID := uuid.New()
	st.players[playerID] = Player{ID: playerID, Username: "alice", Role: "player"}
	sessID, _ := st.CreateSession(context.Background(), CreateSessionParams{
		PlayerID: playerID, ExpiresAt: time.Now().Add(time.Hour),
	})

	req := httptest.NewRequest(http.MethodPost, "/auth/logout", nil)
	req.AddCookie(&http.Cookie{Name: middleware.RefreshCookieName, Value: sessID.String()})
	rec := httptest.NewRecorder()

	h.HandleLogout(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Errorf("expected 204, got %d", rec.Code)
	}

	// Verify both cookies are cleared.
	cookies := rec.Result().Cookies()
	foundSession, foundRefresh := false, false
	for _, c := range cookies {
		if c.Name == "tf_session" {
			foundSession = true
			if c.MaxAge >= 0 {
				t.Error("expected session cookie MaxAge < 0 (cleared)")
			}
		}
		if c.Name == "tf_refresh" {
			foundRefresh = true
			if c.MaxAge >= 0 {
				t.Error("expected refresh cookie MaxAge < 0 (cleared)")
			}
		}
	}
	if !foundSession {
		t.Error("expected tf_session cookie to be set (cleared)")
	}
	if !foundRefresh {
		t.Error("expected tf_refresh cookie to be set (cleared)")
	}

	// Verify session was revoked in store.
	if _, err := st.GetSession(context.Background(), sessID); err == nil {
		t.Error("expected session to be revoked")
	}
}

func TestHandleRefresh_Success(t *testing.T) {
	st := newMockStore()
	h := newHandler(st)

	playerID := uuid.New()
	st.players[playerID] = Player{ID: playerID, Username: "alice", Role: "player"}
	sessID, _ := st.CreateSession(context.Background(), CreateSessionParams{
		PlayerID: playerID, ExpiresAt: time.Now().Add(time.Hour),
	})

	req := httptest.NewRequest(http.MethodPost, "/auth/refresh", nil)
	req.AddCookie(&http.Cookie{Name: middleware.RefreshCookieName, Value: sessID.String()})
	rec := httptest.NewRecorder()

	h.HandleRefresh(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", rec.Code, rec.Body.String())
	}

	// Should have new session and access cookies.
	cookies := rec.Result().Cookies()
	foundSession, foundRefresh := false, false
	for _, c := range cookies {
		if c.Name == "tf_session" && c.Value != "" {
			foundSession = true
		}
		if c.Name == "tf_refresh" && c.Value != "" && c.Value != sessID.String() {
			foundRefresh = true
		}
	}
	if !foundSession {
		t.Error("expected new tf_session cookie")
	}
	if !foundRefresh {
		t.Error("expected new tf_refresh cookie with rotated session ID")
	}

	// Old session should be revoked.
	if _, err := st.GetSession(context.Background(), sessID); err == nil {
		t.Error("expected old session to be revoked")
	}
}

func TestHandleRefresh_MissingCookie(t *testing.T) {
	h := newHandler(newMockStore())
	req := httptest.NewRequest(http.MethodPost, "/auth/refresh", nil)
	rec := httptest.NewRecorder()

	h.HandleRefresh(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rec.Code)
	}
}

func TestHandleRefresh_ExpiredSession(t *testing.T) {
	st := newMockStore()
	h := newHandler(st)

	req := httptest.NewRequest(http.MethodPost, "/auth/refresh", nil)
	req.AddCookie(&http.Cookie{Name: middleware.RefreshCookieName, Value: uuid.New().String()})
	rec := httptest.NewRecorder()

	h.HandleRefresh(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rec.Code)
	}
}

func TestHandleRefresh_BannedPlayer(t *testing.T) {
	st := newMockStore()
	h := newHandler(st)

	playerID := uuid.New()
	st.players[playerID] = Player{ID: playerID, Username: "banned_user", Role: "player"}
	sessID, _ := st.CreateSession(context.Background(), CreateSessionParams{
		PlayerID: playerID, ExpiresAt: time.Now().Add(time.Hour),
	})
	st.bannedPlayers[playerID] = true

	req := httptest.NewRequest(http.MethodPost, "/auth/refresh", nil)
	req.AddCookie(&http.Cookie{Name: middleware.RefreshCookieName, Value: sessID.String()})
	rec := httptest.NewRecorder()

	h.HandleRefresh(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestHandleGitHubLogin_RedirectsToGitHub(t *testing.T) {
	h := newHandler(newMockStore())
	req := httptest.NewRequest(http.MethodGet, "/auth/github", nil)
	rec := httptest.NewRecorder()

	h.HandleGitHubLogin(rec, req)

	if rec.Code != http.StatusTemporaryRedirect {
		t.Fatalf("expected 307, got %d", rec.Code)
	}

	location := rec.Header().Get("Location")
	if location == "" {
		t.Fatal("expected Location header")
	}

	// Should contain GitHub auth URL and client_id
	if !contains(location, "github.com") {
		t.Errorf("expected redirect to github.com, got %s", location)
	}
	if !contains(location, "client_id=test-client-id") {
		t.Errorf("expected client_id in redirect, got %s", location)
	}

	// Should set oauth_state cookie
	cookies := rec.Result().Cookies()
	found := false
	for _, c := range cookies {
		if c.Name == "oauth_state" && c.Value != "" {
			found = true
		}
	}
	if !found {
		t.Error("expected oauth_state cookie to be set")
	}
}

func TestHandleTestLogin_Disabled(t *testing.T) {
	// TEST_MODE is not set — should return 404.
	st := newMockStore()
	playerID := uuid.New()
	st.players[playerID] = Player{ID: playerID, Username: "alice", Role: "player"}

	h := newHandler(st)
	req := httptest.NewRequest(http.MethodGet, "/auth/test-login?player_id="+playerID.String(), nil)
	w := httptest.NewRecorder()
	h.HandleTestLogin(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404 when TEST_MODE is off", w.Code)
	}
}

func TestHandleTestLogin_Enabled(t *testing.T) {
	t.Setenv("TEST_MODE", "true")

	st := newMockStore()
	playerID := uuid.New()
	st.players[playerID] = Player{ID: playerID, Username: "alice", Role: "player"}

	h := newHandler(st)
	req := httptest.NewRequest(http.MethodGet, "/auth/test-login?player_id="+playerID.String(), nil)
	w := httptest.NewRecorder()
	h.HandleTestLogin(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("status = %d, want 204", w.Code)
	}

	cookies := w.Result().Cookies()
	found := false
	for _, c := range cookies {
		if c.Name == middleware.CookieName {
			found = true
			if c.Value == "" {
				t.Error("expected non-empty session cookie")
			}
		}
	}
	if !found {
		t.Error("expected session cookie to be set")
	}
}

func TestHandleTestLogin_InvalidPlayerID(t *testing.T) {
	t.Setenv("TEST_MODE", "true")

	h := newHandler(newMockStore())
	req := httptest.NewRequest(http.MethodGet, "/auth/test-login?player_id=not-a-uuid", nil)
	w := httptest.NewRecorder()
	h.HandleTestLogin(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

func TestHandleTestLogin_PlayerNotFound(t *testing.T) {
	t.Setenv("TEST_MODE", "true")

	h := newHandler(newMockStore())
	req := httptest.NewRequest(http.MethodGet, "/auth/test-login?player_id="+uuid.New().String(), nil)
	w := httptest.NewRecorder()
	h.HandleTestLogin(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", w.Code)
	}
}

func TestSanitizeUsername(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"Alice", "alice"},
		{"user-name", "user_name"},
		{"user.name!", "username"},
		{"test_123", "test_123"},
		{"UPPER", "upper"},
	}
	for _, tt := range tests {
		got := sanitizeUsername(tt.input)
		if got != tt.want {
			t.Errorf("sanitizeUsername(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
