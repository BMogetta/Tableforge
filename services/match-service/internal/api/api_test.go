package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/recess/match-service/internal/queue"
	sharedmw "github.com/recess/shared/middleware"
	ratingv1 "github.com/recess/shared/proto/rating/v1"
	"github.com/redis/go-redis/v9"
	"github.com/recess/shared/testutil"
	"google.golang.org/grpc"
)

// noopAuthMW is a pass-through middleware (no JWT validation).
// Tests that need an authenticated context inject it manually via
// sharedmw.ContextWithPlayer.
func noopAuthMW(next http.Handler) http.Handler { return next }

// newTestRouter builds a router with a nil queue.Service.
// Only handlers that return before calling any Service method are safe to hit.
func newTestRouter() http.Handler {
	return NewRouter(nil, noopAuthMW, nil)
}

// stubRatingClient returns default rating for any request.
type stubRatingClient struct {
	ratingv1.RatingServiceClient
}

func (s *stubRatingClient) GetRating(_ context.Context, _ *ratingv1.GetRatingRequest, _ ...grpc.CallOption) (*ratingv1.GetRatingResponse, error) {
	return &ratingv1.GetRatingResponse{Mmr: 1500}, nil
}

// newTestRouterWithService builds a router backed by a real Service using miniredis.
func newTestRouterWithService(t *testing.T) (http.Handler, *queue.Service, *miniredis.Miniredis) {
	t.Helper()
	rdb, mr := testutil.NewTestRedis(t)

	svc := queue.New(rdb, &stubRatingClient{}, nil, nil, "", nil, nil)
	router := NewRouter(svc, noopAuthMW, nil)
	return router, svc, mr
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

// ---------------------------------------------------------------------------
// POST /api/v1/queue — join queue (success paths)
// ---------------------------------------------------------------------------

func TestJoinQueue_Success(t *testing.T) {
	router, _, _ := newTestRouterWithService(t)

	playerID := uuid.New()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/queue", nil)
	req = req.WithContext(sharedmw.ContextWithPlayer(req.Context(), playerID, "testuser", "player"))
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var pos queue.QueuePosition
	if err := json.NewDecoder(rec.Body).Decode(&pos); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if pos.Position != 1 {
		t.Errorf("expected position 1, got %d", pos.Position)
	}
}

func TestJoinQueue_AlreadyQueued(t *testing.T) {
	router, _, _ := newTestRouterWithService(t)

	playerID := uuid.New()

	// First join
	req := httptest.NewRequest(http.MethodPost, "/api/v1/queue", nil)
	req = req.WithContext(sharedmw.ContextWithPlayer(req.Context(), playerID, "testuser", "player"))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("first join: expected 200, got %d", rec.Code)
	}

	// Second join → conflict
	req = httptest.NewRequest(http.MethodPost, "/api/v1/queue", nil)
	req = req.WithContext(sharedmw.ContextWithPlayer(req.Context(), playerID, "testuser", "player"))
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d", rec.Code)
	}
	assertErrorJSON(t, rec, "already in queue")
}

func TestJoinQueue_Banned(t *testing.T) {
	router, _, mr := newTestRouterWithService(t)

	playerID := uuid.New()
	banKey := "queue:ban:" + playerID.String()
	mr.Set(banKey, "1")
	mr.SetTTL(banKey, 120*time.Second)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/queue", nil)
	req = req.WithContext(sharedmw.ContextWithPlayer(req.Context(), playerID, "testuser", "player"))
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429, got %d: %s", rec.Code, rec.Body.String())
	}

	retryAfter := rec.Header().Get("Retry-After")
	if retryAfter == "" {
		t.Error("expected Retry-After header")
	}
}

// ---------------------------------------------------------------------------
// DELETE /api/v1/queue — leave queue (success paths)
// ---------------------------------------------------------------------------

func TestLeaveQueue_Success(t *testing.T) {
	router, _, _ := newTestRouterWithService(t)

	playerID := uuid.New()

	// Join first
	req := httptest.NewRequest(http.MethodPost, "/api/v1/queue", nil)
	req = req.WithContext(sharedmw.ContextWithPlayer(req.Context(), playerID, "testuser", "player"))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	// Leave
	req = httptest.NewRequest(http.MethodDelete, "/api/v1/queue", nil)
	req = req.WithContext(sharedmw.ContextWithPlayer(req.Context(), playerID, "testuser", "player"))
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", rec.Code)
	}
}

func TestLeaveQueue_NotQueued_Idempotent(t *testing.T) {
	router, _, _ := newTestRouterWithService(t)

	playerID := uuid.New()
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/queue", nil)
	req = req.WithContext(sharedmw.ContextWithPlayer(req.Context(), playerID, "testuser", "player"))
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	// Should be 204 even if not queued (idempotent)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", rec.Code)
	}
}

// ---------------------------------------------------------------------------
// POST /api/v1/queue/accept — success paths
// ---------------------------------------------------------------------------

func TestAcceptMatch_MatchNotFound(t *testing.T) {
	router, _, _ := newTestRouterWithService(t)

	playerID := uuid.New()
	matchID := uuid.New()
	body := `{"match_id":"` + matchID.String() + `"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/queue/accept", strings.NewReader(body))
	req = req.WithContext(sharedmw.ContextWithPlayer(req.Context(), playerID, "testuser", "player"))
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rec.Code, rec.Body.String())
	}
	assertErrorJSON(t, rec, "match not found or already expired")
}

func TestAcceptMatch_NotParticipant(t *testing.T) {
	router, _, mr := newTestRouterWithService(t)

	matchID := uuid.New()
	playerA := uuid.New()
	playerB := uuid.New()
	outsider := uuid.New()

	// Inject pending match directly via miniredis
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer rdb.Close()
	rdb.HSet(context.Background(), "queue:pending:"+matchID.String(), map[string]any{
		"player_a": playerA.String(), "player_b": playerB.String(),
		"accepted_a": "0", "accepted_b": "0",
		"mmr_a": "1500", "mmr_b": "1500",
	})

	body := `{"match_id":"` + matchID.String() + `"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/queue/accept", strings.NewReader(body))
	req = req.WithContext(sharedmw.ContextWithPlayer(req.Context(), outsider, "outsider", "player"))
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", rec.Code, rec.Body.String())
	}
	assertErrorJSON(t, rec, "not a participant in this match")
}

// ---------------------------------------------------------------------------
// POST /api/v1/queue/decline — success paths
// ---------------------------------------------------------------------------

func TestDeclineMatch_MatchNotFound(t *testing.T) {
	router, _, _ := newTestRouterWithService(t)

	playerID := uuid.New()
	matchID := uuid.New()
	body := `{"match_id":"` + matchID.String() + `"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/queue/decline", strings.NewReader(body))
	req = req.WithContext(sharedmw.ContextWithPlayer(req.Context(), playerID, "testuser", "player"))
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rec.Code, rec.Body.String())
	}
	assertErrorJSON(t, rec, "match not found or already expired")
}

func TestDeclineMatch_NotParticipant(t *testing.T) {
	router, _, mr := newTestRouterWithService(t)

	matchID := uuid.New()
	playerA := uuid.New()
	playerB := uuid.New()
	outsider := uuid.New()

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer rdb.Close()
	rdb.HSet(context.Background(), "queue:pending:"+matchID.String(), map[string]any{
		"player_a": playerA.String(), "player_b": playerB.String(),
		"accepted_a": "0", "accepted_b": "0",
	})

	body := `{"match_id":"` + matchID.String() + `"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/queue/decline", strings.NewReader(body))
	req = req.WithContext(sharedmw.ContextWithPlayer(req.Context(), outsider, "outsider", "player"))
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", rec.Code, rec.Body.String())
	}
	assertErrorJSON(t, rec, "not a participant in this match")
}

// ---------------------------------------------------------------------------
// Accept with empty body
// ---------------------------------------------------------------------------

func TestAcceptMatch_EmptyBody(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/v1/queue/accept", nil)
	rec := httptest.NewRecorder()
	newTestRouter().ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestDeclineMatch_EmptyBody(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/v1/queue/decline", nil)
	rec := httptest.NewRecorder()
	newTestRouter().ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

// ---------------------------------------------------------------------------
// 404 for unknown routes
// ---------------------------------------------------------------------------

func TestUnknownRoute(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/nonexistent", nil)
	rec := httptest.NewRecorder()
	newTestRouter().ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

// ---------------------------------------------------------------------------
// Method not allowed
// ---------------------------------------------------------------------------

// ---------------------------------------------------------------------------
// DELETE /api/v1/queue/players/{id}/state — reset player (test-only)
// ---------------------------------------------------------------------------

func TestResetPlayer_NotInTestMode(t *testing.T) {
	t.Setenv("TEST_MODE", "false")
	router, _, _ := newTestRouterWithService(t)

	playerID := uuid.New()
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/queue/players/"+playerID.String()+"/state", nil)
	req = req.WithContext(sharedmw.ContextWithPlayer(req.Context(), playerID, "tester", "player"))
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 when TEST_MODE is off, got %d", rec.Code)
	}
}

func TestResetPlayer_ClearsBanAndDeclines(t *testing.T) {
	t.Setenv("TEST_MODE", "true")
	router, _, mr := newTestRouterWithService(t)

	playerID := uuid.New()
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer rdb.Close()

	ctx := context.Background()
	rdb.Set(ctx, "queue:ban:"+playerID.String(), "1", 5*time.Minute)
	rdb.RPush(ctx, "queue:declines:"+playerID.String(), time.Now().Format(time.RFC3339))
	rdb.ZAdd(ctx, "queue:ranked", redis.Z{Score: 1500, Member: playerID.String()})

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/queue/players/"+playerID.String()+"/state", nil)
	req = req.WithContext(sharedmw.ContextWithPlayer(req.Context(), playerID, "tester", "player"))
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", rec.Code, rec.Body.String())
	}

	// Verify everything was cleaned up.
	if exists, _ := rdb.Exists(ctx, "queue:ban:"+playerID.String()).Result(); exists != 0 {
		t.Error("ban key should have been deleted")
	}
	if exists, _ := rdb.Exists(ctx, "queue:declines:"+playerID.String()).Result(); exists != 0 {
		t.Error("declines key should have been deleted")
	}
	if score, err := rdb.ZScore(ctx, "queue:ranked", playerID.String()).Result(); err == nil {
		t.Errorf("player should not be in queue, got score %f", score)
	}
}

func TestResetPlayer_InvalidID(t *testing.T) {
	t.Setenv("TEST_MODE", "true")
	router, _, _ := newTestRouterWithService(t)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/queue/players/not-a-uuid/state", nil)
	req = req.WithContext(sharedmw.ContextWithPlayer(req.Context(), uuid.New(), "tester", "player"))
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

// ---------------------------------------------------------------------------

func TestJoinQueue_WrongMethod(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/queue", nil)
	rec := httptest.NewRecorder()
	newTestRouter().ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", rec.Code)
	}
}
