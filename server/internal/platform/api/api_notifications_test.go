package api_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/tableforge/server/internal/domain/lobby"
	"github.com/tableforge/server/internal/domain/notification"
	"github.com/tableforge/server/internal/domain/runtime"
	"github.com/tableforge/server/internal/platform/api"
	"github.com/tableforge/server/internal/platform/store"
	"github.com/tableforge/server/internal/testutil"
)

// ---------------------------------------------------------------------------
// Router with notification service
// ---------------------------------------------------------------------------

func newNotificationRouter(t *testing.T) (http.Handler, *testutil.FakeStore, *notification.Service) {
	t.Helper()
	fs := testutil.NewFakeStore()
	reg := newFakeRegistry(&stubGame{})
	lobbySvc := lobby.New(fs, reg)
	rt := runtime.New(fs, reg, nil)
	notifSvc := notification.New(fs, nil)
	router := api.NewRouter(lobbySvc, rt, fs, nil, nil, nil, nil, nil, nil, notifSvc)
	return router, fs, notifSvc
}

// seedNotification creates a notification directly in the fake store.
func seedNotification(t *testing.T, fs *testutil.FakeStore, playerID uuid.UUID, nType store.NotificationType, actionExpiry *time.Time) store.Notification {
	t.Helper()
	payload, _ := json.Marshal(map[string]string{"test": "payload"})
	n, err := fs.CreateNotification(context.Background(), store.CreateNotificationParams{
		PlayerID:        playerID,
		Type:            nType,
		Payload:         payload,
		ActionExpiresAt: actionExpiry,
	})
	if err != nil {
		t.Fatalf("seedNotification: %v", err)
	}
	return n
}

// ---------------------------------------------------------------------------
// GET /players/{playerID}/notifications
// ---------------------------------------------------------------------------

func TestListNotifications_ReturnsUnread(t *testing.T) {
	router, fs, _ := newNotificationRouter(t)
	playerID := uuid.New()
	seedNotification(t, fs, playerID, store.NotificationTypeBanIssued, nil)

	body := fmt.Sprintf(`{"player_id":"%s"}`, playerID)
	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/players/%s/notifications", playerID), strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var notifications []store.Notification
	if err := json.NewDecoder(w.Body).Decode(&notifications); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(notifications) != 1 {
		t.Errorf("expected 1 notification, got %d", len(notifications))
	}
}

func TestListNotifications_ForbiddenForOtherPlayer(t *testing.T) {
	router, fs, _ := newNotificationRouter(t)
	playerID := uuid.New()
	otherID := uuid.New()
	seedNotification(t, fs, playerID, store.NotificationTypeBanIssued, nil)

	body := fmt.Sprintf(`{"player_id":"%s"}`, otherID)
	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/players/%s/notifications", playerID), strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", w.Code)
	}
}

func TestListNotifications_EmptyList(t *testing.T) {
	router, _, _ := newNotificationRouter(t)
	playerID := uuid.New()

	body := fmt.Sprintf(`{"player_id":"%s"}`, playerID)
	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/players/%s/notifications", playerID), strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var notifications []store.Notification
	json.NewDecoder(w.Body).Decode(&notifications)
	if len(notifications) != 0 {
		t.Errorf("expected empty list, got %d", len(notifications))
	}
}

// ---------------------------------------------------------------------------
// POST /notifications/{notificationID}/read
// ---------------------------------------------------------------------------

func TestMarkNotificationRead_Success(t *testing.T) {
	router, fs, _ := newNotificationRouter(t)
	playerID := uuid.New()
	n := seedNotification(t, fs, playerID, store.NotificationTypeBanIssued, nil)

	body := fmt.Sprintf(`{"player_id":"%s"}`, playerID)
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/notifications/%s/read", n.ID), strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("expected 204, got %d: %s", w.Code, w.Body.String())
	}

	updated, _ := fs.GetNotification(context.Background(), n.ID)
	if updated.ReadAt == nil {
		t.Error("expected read_at to be set")
	}
}

func TestMarkNotificationRead_NotFound(t *testing.T) {
	router, _, _ := newNotificationRouter(t)
	playerID := uuid.New()

	body := fmt.Sprintf(`{"player_id":"%s"}`, playerID)
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/notifications/%s/read", uuid.New()), strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestMarkNotificationRead_WrongPlayer(t *testing.T) {
	router, fs, _ := newNotificationRouter(t)
	playerID := uuid.New()
	otherID := uuid.New()
	n := seedNotification(t, fs, playerID, store.NotificationTypeBanIssued, nil)

	body := fmt.Sprintf(`{"player_id":"%s"}`, otherID)
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/notifications/%s/read", n.ID), strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

// ---------------------------------------------------------------------------
// POST /notifications/{notificationID}/accept
// ---------------------------------------------------------------------------

func TestAcceptNotification_Success(t *testing.T) {
	router, fs, _ := newNotificationRouter(t)
	playerID := uuid.New()
	expiry := time.Now().Add(time.Hour)
	n := seedNotification(t, fs, playerID, store.NotificationTypeFriendRequest, &expiry)

	body := fmt.Sprintf(`{"player_id":"%s"}`, playerID)
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/notifications/%s/accept", n.ID), strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("expected 204, got %d: %s", w.Code, w.Body.String())
	}

	updated, _ := fs.GetNotification(context.Background(), n.ID)
	if updated.ActionTaken == nil || *updated.ActionTaken != "accepted" {
		t.Errorf("expected action_taken=accepted, got %v", updated.ActionTaken)
	}
}

func TestAcceptNotification_BanIssued_UnprocessableEntity(t *testing.T) {
	router, fs, _ := newNotificationRouter(t)
	playerID := uuid.New()
	n := seedNotification(t, fs, playerID, store.NotificationTypeBanIssued, nil)

	body := fmt.Sprintf(`{"player_id":"%s"}`, playerID)
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/notifications/%s/accept", n.ID), strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnprocessableEntity {
		t.Errorf("expected 422, got %d", w.Code)
	}
}

func TestAcceptNotification_AlreadyTaken_Conflict(t *testing.T) {
	router, fs, _ := newNotificationRouter(t)
	playerID := uuid.New()
	expiry := time.Now().Add(time.Hour)
	n := seedNotification(t, fs, playerID, store.NotificationTypeFriendRequest, &expiry)

	// Accept once.
	action := "accepted"
	stored := fs.Notifications[n.ID]
	stored.ActionTaken = &action
	fs.Notifications[n.ID] = stored

	body := fmt.Sprintf(`{"player_id":"%s"}`, playerID)
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/notifications/%s/accept", n.ID), strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusConflict {
		t.Errorf("expected 409, got %d", w.Code)
	}
}

func TestAcceptNotification_Expired_Gone(t *testing.T) {
	router, fs, _ := newNotificationRouter(t)
	playerID := uuid.New()
	past := time.Now().Add(-time.Hour)
	n := seedNotification(t, fs, playerID, store.NotificationTypeFriendRequest, &past)

	body := fmt.Sprintf(`{"player_id":"%s"}`, playerID)
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/notifications/%s/accept", n.ID), strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusGone {
		t.Errorf("expected 410, got %d", w.Code)
	}
}

// ---------------------------------------------------------------------------
// POST /notifications/{notificationID}/decline
// ---------------------------------------------------------------------------

func TestDeclineNotification_Success(t *testing.T) {
	router, fs, _ := newNotificationRouter(t)
	playerID := uuid.New()
	expiry := time.Now().Add(time.Hour)
	n := seedNotification(t, fs, playerID, store.NotificationTypeRoomInvitation, &expiry)

	body := fmt.Sprintf(`{"player_id":"%s"}`, playerID)
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/notifications/%s/decline", n.ID), strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("expected 204, got %d: %s", w.Code, w.Body.String())
	}

	updated, _ := fs.GetNotification(context.Background(), n.ID)
	if updated.ActionTaken == nil || *updated.ActionTaken != "declined" {
		t.Errorf("expected action_taken=declined, got %v", updated.ActionTaken)
	}
}

func TestDeclineNotification_NotFound(t *testing.T) {
	router, _, _ := newNotificationRouter(t)
	playerID := uuid.New()

	body := fmt.Sprintf(`{"player_id":"%s"}`, playerID)
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/notifications/%s/decline", uuid.New()), strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}
