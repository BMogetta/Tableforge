package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/recess/services/ws-gateway/internal/presence"
	"github.com/recess/shared/testutil"
)

func newPresenceHandler(t *testing.T) (http.HandlerFunc, *presence.Store) {
	t.Helper()
	rdb, _ := testutil.NewTestRedis(t)
	ps := presence.New(rdb)
	return PresenceHandler(ps), ps
}

func TestPresenceHandler_MissingIDs(t *testing.T) {
	h, _ := newPresenceHandler(t)

	req := httptest.NewRequest("GET", "/api/v1/presence", nil)
	w := httptest.NewRecorder()
	h(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestPresenceHandler_EmptyIDs(t *testing.T) {
	h, _ := newPresenceHandler(t)

	req := httptest.NewRequest("GET", "/api/v1/presence?ids=", nil)
	w := httptest.NewRecorder()
	h(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestPresenceHandler_InvalidUUIDs(t *testing.T) {
	h, _ := newPresenceHandler(t)

	req := httptest.NewRequest("GET", "/api/v1/presence?ids=notauuid,alsonotauuid", nil)
	w := httptest.NewRecorder()
	h(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for all-invalid UUIDs, got %d", w.Code)
	}
}

func TestPresenceHandler_MixedValidInvalid(t *testing.T) {
	h, ps := newPresenceHandler(t)

	id := uuid.New()
	_ = ps.Set(context.Background(), id)

	req := httptest.NewRequest("GET", "/api/v1/presence?ids=invalid,"+id.String(), nil)
	w := httptest.NewRecorder()
	h(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var result map[string]bool
	if err := json.NewDecoder(w.Body).Decode(&result); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !result[id.String()] {
		t.Error("expected player to be online")
	}
	if len(result) != 1 {
		t.Errorf("expected 1 entry (invalid skipped), got %d", len(result))
	}
}

func TestPresenceHandler_OnlineOffline(t *testing.T) {
	h, ps := newPresenceHandler(t)

	online := uuid.New()
	offline := uuid.New()
	_ = ps.Set(context.Background(), online)

	ids := online.String() + "," + offline.String()
	req := httptest.NewRequest("GET", "/api/v1/presence?ids="+ids, nil)
	w := httptest.NewRecorder()
	h(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var result map[string]bool
	_ = json.NewDecoder(w.Body).Decode(&result)

	if !result[online.String()] {
		t.Error("expected online player to be true")
	}
	if result[offline.String()] {
		t.Error("expected offline player to be false")
	}
}

func TestPresenceHandler_CapsAt100(t *testing.T) {
	h, ps := newPresenceHandler(t)

	// Generate 105 UUIDs, set first one online.
	ids := make([]string, 105)
	for i := range ids {
		ids[i] = uuid.New().String()
	}
	first := uuid.MustParse(ids[0])
	_ = ps.Set(context.Background(), first)

	req := httptest.NewRequest("GET", "/api/v1/presence?ids="+strings.Join(ids, ","), nil)
	w := httptest.NewRecorder()
	h(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var result map[string]bool
	_ = json.NewDecoder(w.Body).Decode(&result)

	if len(result) != 100 {
		t.Errorf("expected 100 entries (capped), got %d", len(result))
	}
}

func TestPresenceHandler_ContentType(t *testing.T) {
	h, _ := newPresenceHandler(t)

	id := uuid.New()
	req := httptest.NewRequest("GET", "/api/v1/presence?ids="+id.String(), nil)
	w := httptest.NewRecorder()
	h(w, req)

	ct := w.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("expected Content-Type application/json, got %q", ct)
	}
}

func TestPresenceHandler_WhitespaceInIDs(t *testing.T) {
	h, ps := newPresenceHandler(t)

	id := uuid.New()
	_ = ps.Set(context.Background(), id)

	// IDs with extra whitespace around them (URL-encoded spaces).
	req := httptest.NewRequest("GET", "/api/v1/presence?ids=%20"+id.String()+"%20", nil)
	w := httptest.NewRecorder()
	h(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var result map[string]bool
	_ = json.NewDecoder(w.Body).Decode(&result)
	if !result[id.String()] {
		t.Error("expected trimmed UUID to resolve as online")
	}
}
