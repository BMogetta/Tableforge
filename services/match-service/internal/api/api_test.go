package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	sharedmw "github.com/recess/shared/middleware"
)

// noopAuthMW is a pass-through middleware (no JWT validation).
// Tests that need an authenticated context inject it manually via
// sharedmw.ContextWithPlayer.
func noopAuthMW(next http.Handler) http.Handler { return next }

// newTestRouter builds a router with a nil queue.Service.
// Only handlers that return before calling any Service method are safe to hit.
func newTestRouter() http.Handler {
	return NewRouter(nil, noopAuthMW)
}

// ---------------------------------------------------------------------------
// POST /api/v1/queue — join queue
// ---------------------------------------------------------------------------

func TestJoinQueue_Unauthorized(t *testing.T) {
	// No player ID in context → 401
	req := httptest.NewRequest(http.MethodPost, "/api/v1/queue", nil)
	rec := httptest.NewRecorder()

	newTestRouter().ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
	assertErrorJSON(t, rec, "unauthorized")
}

// ---------------------------------------------------------------------------
// DELETE /api/v1/queue — leave queue
// ---------------------------------------------------------------------------

func TestLeaveQueue_Unauthorized(t *testing.T) {
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/queue", nil)
	rec := httptest.NewRecorder()

	newTestRouter().ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
	assertErrorJSON(t, rec, "unauthorized")
}

// ---------------------------------------------------------------------------
// POST /api/v1/queue/accept
// ---------------------------------------------------------------------------

func TestAcceptMatch_InvalidBody(t *testing.T) {
	// Malformed JSON → 400 before auth or service is touched
	req := httptest.NewRequest(http.MethodPost, "/api/v1/queue/accept",
		strings.NewReader("not json"))
	rec := httptest.NewRecorder()

	newTestRouter().ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
	assertErrorJSON(t, rec, "invalid request body")
}

func TestAcceptMatch_InvalidMatchID(t *testing.T) {
	// Valid JSON but match_id is not a UUID → 400
	body := `{"match_id":"not-a-uuid"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/queue/accept",
		strings.NewReader(body))
	req = req.WithContext(sharedmw.ContextWithPlayer(req.Context(), uuid.New(), "testuser", "player"))
	rec := httptest.NewRecorder()

	newTestRouter().ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
	assertErrorJSON(t, rec, "invalid match_id")
}

func TestAcceptMatch_Unauthorized(t *testing.T) {
	// Valid JSON with valid UUID, but no auth context → 401
	matchID := uuid.New()
	body := `{"match_id":"` + matchID.String() + `"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/queue/accept",
		strings.NewReader(body))
	rec := httptest.NewRecorder()

	newTestRouter().ServeHTTP(rec, req)

	// Body decodes OK, then auth check fails → 401
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

// ---------------------------------------------------------------------------
// POST /api/v1/queue/decline
// ---------------------------------------------------------------------------

func TestDeclineMatch_InvalidBody(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/v1/queue/decline",
		strings.NewReader("{invalid"))
	rec := httptest.NewRecorder()

	newTestRouter().ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
	assertErrorJSON(t, rec, "invalid request body")
}

func TestDeclineMatch_InvalidMatchID(t *testing.T) {
	body := `{"match_id":"nope"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/queue/decline",
		strings.NewReader(body))
	req = req.WithContext(sharedmw.ContextWithPlayer(req.Context(), uuid.New(), "testuser", "player"))
	rec := httptest.NewRecorder()

	newTestRouter().ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
	assertErrorJSON(t, rec, "invalid match_id")
}

func TestDeclineMatch_Unauthorized(t *testing.T) {
	matchID := uuid.New()
	body := `{"match_id":"` + matchID.String() + `"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/queue/decline",
		strings.NewReader(body))
	rec := httptest.NewRecorder()

	newTestRouter().ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

// ---------------------------------------------------------------------------
// GET /healthz
// ---------------------------------------------------------------------------

func TestHealthz(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()

	newTestRouter().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if rec.Body.String() != "ok" {
		t.Fatalf("expected body 'ok', got %q", rec.Body.String())
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func assertErrorJSON(t *testing.T, rec *httptest.ResponseRecorder, wantMsg string) {
	t.Helper()
	var resp map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("response is not valid JSON: %v", err)
	}
	if got := resp["error"]; got != wantMsg {
		t.Errorf("error = %q, want %q", got, wantMsg)
	}
}
