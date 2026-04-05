package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/recess/notification-service/internal/store"
)

func TestListNotifications_InvalidPlayerID(t *testing.T) {
	handler := newTestHandler(newMockStore(), &stubPublisher{})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/players/not-a-uuid/notifications", nil)
	// Need auth for a valid player but the URL param is invalid
	req = withAuth(req, uuid.New(), "player")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

func TestListNotifications_Unauthorized(t *testing.T) {
	handler := newTestHandler(newMockStore(), &stubPublisher{})
	playerID := uuid.New()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/players/"+playerID.String()+"/notifications", nil)
	// No auth context
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rec.Code)
	}
}

func TestListNotifications_WithPagination(t *testing.T) {
	st := newMockStore()
	pub := &stubPublisher{}
	playerID := uuid.New()

	// Add a few notifications
	for i := 0; i < 5; i++ {
		id := uuid.New()
		st.notifications[id] = newTestNotification(id, playerID, "friend_request")
	}

	handler := newTestHandler(st, pub)
	rec := getAs(t, handler, "/api/v1/players/"+playerID.String()+"/notifications?limit=2&offset=0", playerID)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestListNotifications_IncludeRead(t *testing.T) {
	st := newMockStore()
	pub := &stubPublisher{}
	playerID := uuid.New()

	handler := newTestHandler(st, pub)
	rec := getAs(t, handler, "/api/v1/players/"+playerID.String()+"/notifications?include_read=true", playerID)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestMarkRead_InvalidNotificationID(t *testing.T) {
	handler := newTestHandler(newMockStore(), &stubPublisher{})
	rec := postAs(t, handler, "/api/v1/notifications/not-a-uuid/read", uuid.New(), nil)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

func TestMarkRead_WrongPlayer(t *testing.T) {
	st := newMockStore()
	pub := &stubPublisher{}
	ownerID := uuid.New()
	otherID := uuid.New()
	notifID := uuid.New()

	st.notifications[notifID] = newTestNotification(notifID, ownerID, "friend_request")

	handler := newTestHandler(st, pub)
	// Try to mark read as a different player
	rec := postAs(t, handler, "/api/v1/notifications/"+notifID.String()+"/read", otherID, nil)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404 (ownership check), got %d", rec.Code)
	}
}

func TestDecline_NotFound(t *testing.T) {
	handler := newTestHandler(newMockStore(), &stubPublisher{})
	rec := postAs(t, handler, "/api/v1/notifications/"+uuid.New().String()+"/decline", uuid.New(), nil)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rec.Code)
	}
}

func TestCreate_InvalidJSON(t *testing.T) {
	handler := newTestHandler(newMockStore(), &stubPublisher{})
	req := httptest.NewRequest(http.MethodPost, "/internal/notifications", nil)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

// helper
func newTestNotification(id, playerID uuid.UUID, ntype string) store.Notification {
	return store.Notification{
		ID:       id,
		PlayerID: playerID,
		Type:     store.NotificationType(ntype),
		Payload:  []byte(`{}`),
	}
}
