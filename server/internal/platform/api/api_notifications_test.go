package api_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
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

// --- Router with notification service ---------------------------------------

func newNotificationRouter(t *testing.T) (http.Handler, *testutil.FakeStore, *notification.Service) {
	t.Helper()
	fs := testutil.NewFakeStore()
	reg := newFakeRegistry(&stubGame{})
	lobbySvc := lobby.New(fs, reg)
	rt := runtime.New(fs, reg, nil, nil, nil)
	notifSvc := notification.New(fs, nil)
	// Passing notifSvc as the last argument to the router
	router := api.NewRouter(lobbySvc, rt, fs, nil, nil, nil, nil, nil, notifSvc, nil)
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

// --- GET /players/{playerID}/notifications ----------------------------------

func TestListNotifications_ReturnsUnread(t *testing.T) {
	router, fs, _ := newNotificationRouter(t)
	playerID := uuid.New()
	seedNotification(t, fs, playerID, store.NotificationTypeBanIssued, nil)

	path := fmt.Sprintf("/api/v1/players/%s/notifications", playerID)
	// getJSON injects playerID into the context to pass middleware checks
	w := getJSON(t, router, path, playerID)

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

	path := fmt.Sprintf("/api/v1/players/%s/notifications", playerID)
	// Requesting Alice's notifications while being authenticated as Bob
	w := getJSON(t, router, path, otherID)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", w.Code)
	}
}

func TestListNotifications_EmptyList(t *testing.T) {
	router, _, _ := newNotificationRouter(t)
	playerID := uuid.New()

	path := fmt.Sprintf("/api/v1/players/%s/notifications", playerID)
	w := getJSON(t, router, path, playerID)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var notifications []store.Notification
	json.NewDecoder(w.Body).Decode(&notifications)
	if len(notifications) != 0 {
		t.Errorf("expected empty list, got %d", len(notifications))
	}
}

// --- POST /notifications/{notificationID}/read ------------------------------

func TestMarkNotificationRead_Success(t *testing.T) {
	router, fs, _ := newNotificationRouter(t)
	playerID := uuid.New()
	n := seedNotification(t, fs, playerID, store.NotificationTypeBanIssued, nil)

	path := fmt.Sprintf("/api/v1/notifications/%s/read", n.ID)
	body := map[string]string{"player_id": playerID.String()}

	// postJSONAs handles context injection and body marshaling
	w := postJSONAs(t, router, path, playerID, "player", body)

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

	path := fmt.Sprintf("/api/v1/notifications/%s/read", uuid.New())
	body := map[string]string{"player_id": playerID.String()}
	w := postJSONAs(t, router, path, playerID, "player", body)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestMarkNotificationRead_WrongPlayer(t *testing.T) {
	router, fs, _ := newNotificationRouter(t)
	playerID := uuid.New()
	otherID := uuid.New()
	n := seedNotification(t, fs, playerID, store.NotificationTypeBanIssued, nil)

	path := fmt.Sprintf("/api/v1/notifications/%s/read", n.ID)
	body := map[string]string{"player_id": otherID.String()}

	// If svc.MarkRead uses the callerID from context, it won't find the notification for otherID
	w := postJSONAs(t, router, path, otherID, "player", body)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

// --- POST /notifications/{notificationID}/accept ----------------------------

func TestAcceptNotification_Success(t *testing.T) {
	router, fs, _ := newNotificationRouter(t)
	playerID := uuid.New()
	expiry := time.Now().Add(time.Hour)
	n := seedNotification(t, fs, playerID, store.NotificationTypeFriendRequest, &expiry)

	path := fmt.Sprintf("/api/v1/notifications/%s/accept", n.ID)
	body := map[string]string{"player_id": playerID.String()}
	w := postJSONAs(t, router, path, playerID, "player", body)

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

	path := fmt.Sprintf("/api/v1/notifications/%s/accept", n.ID)
	body := map[string]string{"player_id": playerID.String()}
	w := postJSONAs(t, router, path, playerID, "player", body)

	if w.Code != http.StatusUnprocessableEntity {
		t.Errorf("expected 422, got %d", w.Code)
	}
}

func TestAcceptNotification_AlreadyTaken_Conflict(t *testing.T) {
	router, fs, _ := newNotificationRouter(t)
	playerID := uuid.New()
	expiry := time.Now().Add(time.Hour)
	n := seedNotification(t, fs, playerID, store.NotificationTypeFriendRequest, &expiry)

	// Pre-set action in the store to simulate conflict
	action := "accepted"
	stored := fs.Notifications[n.ID]
	stored.ActionTaken = &action
	fs.Notifications[n.ID] = stored

	path := fmt.Sprintf("/api/v1/notifications/%s/accept", n.ID)
	body := map[string]string{"player_id": playerID.String()}
	w := postJSONAs(t, router, path, playerID, "player", body)

	if w.Code != http.StatusConflict {
		t.Errorf("expected 409, got %d", w.Code)
	}
}

func TestAcceptNotification_Expired_Gone(t *testing.T) {
	router, fs, _ := newNotificationRouter(t)
	playerID := uuid.New()
	past := time.Now().Add(-time.Hour)
	n := seedNotification(t, fs, playerID, store.NotificationTypeFriendRequest, &past)

	path := fmt.Sprintf("/api/v1/notifications/%s/accept", n.ID)
	body := map[string]string{"player_id": playerID.String()}
	w := postJSONAs(t, router, path, playerID, "player", body)

	if w.Code != http.StatusGone {
		t.Errorf("expected 410, got %d", w.Code)
	}
}

// --- POST /notifications/{notificationID}/decline ---------------------------

func TestDeclineNotification_Success(t *testing.T) {
	router, fs, _ := newNotificationRouter(t)
	playerID := uuid.New()
	expiry := time.Now().Add(time.Hour)
	n := seedNotification(t, fs, playerID, store.NotificationTypeRoomInvitation, &expiry)

	path := fmt.Sprintf("/api/v1/notifications/%s/decline", n.ID)
	body := map[string]string{"player_id": playerID.String()}
	w := postJSONAs(t, router, path, playerID, "player", body)

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

	path := fmt.Sprintf("/api/v1/notifications/%s/decline", uuid.New())
	body := map[string]string{"player_id": playerID.String()}
	w := postJSONAs(t, router, path, playerID, "player", body)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

// --- Additional tests for GET /players/{playerID}/notifications -------------

func TestListNotifications_IncludeRead(t *testing.T) {
	router, fs, _ := newNotificationRouter(t)
	playerID := uuid.New()

	// Seed one unread and one read notification
	seedNotification(t, fs, playerID, store.NotificationTypeBanIssued, nil)
	readNotif := seedNotification(t, fs, playerID, store.NotificationTypeFriendRequest, nil)

	// Mark the second one as read manually in the store
	now := time.Now()
	stored := fs.Notifications[readNotif.ID]
	stored.ReadAt = &now
	fs.Notifications[readNotif.ID] = stored

	// Case 1: Default (unread only)
	pathUnread := fmt.Sprintf("/api/v1/players/%s/notifications", playerID)
	w1 := getJSON(t, router, pathUnread, playerID)

	var list1 []store.Notification
	json.NewDecoder(w1.Body).Decode(&list1)
	if len(list1) != 1 {
		t.Errorf("expected 1 unread notification, got %d", len(list1))
	}

	// Case 2: include_read=true
	pathWithRead := fmt.Sprintf("/api/v1/players/%s/notifications?include_read=true", playerID)
	w2 := getJSON(t, router, pathWithRead, playerID)

	var list2 []store.Notification
	json.NewDecoder(w2.Body).Decode(&list2)
	if len(list2) != 2 {
		t.Errorf("expected 2 notifications (including read), got %d", len(list2))
	}
}

// --- Additional tests for Malformed Bodies (POST) ---------------------------

func TestMarkNotificationRead_MalformedBody(t *testing.T) {
	router, _, _ := newNotificationRouter(t)
	playerID := uuid.New()
	notifID := uuid.New()

	path := fmt.Sprintf("/api/v1/notifications/%s/read", notifID)

	// Sending invalid JSON syntax
	malformedBody := `{"player_id": "missing-quote}`

	// We use postJSONAs but manually pass the string to simulate raw corruption if needed,
	// or simply pass a type that won't marshal to expected schema.
	w := postJSONAs(t, router, path, playerID, "player", malformedBody)

	// Most routers/handlers return 400 Bad Request for invalid JSON
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for malformed JSON, got %d", w.Code)
	}
}

func TestAcceptNotification_InvalidPlayerUUID(t *testing.T) {
	router, _ := newTestRouter(t)
	notifID := uuid.New().String()

	w := postJSONAs(t, router, "/api/v1/notifications/"+notifID+"/accept", uuid.Nil, "player", map[string]any{
		"player_id": "esto-no-es-un-uuid",
	})

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for invalid UUID format, got %d. Body: %s", w.Code, w.Body.String())
	}
}

func TestDeclineNotification_MissingPlayerID(t *testing.T) {
	router, _ := newTestRouter(t)
	notifID := uuid.New().String()

	w := postJSONAs(t, router, "/api/v1/notifications/"+notifID+"/decline", uuid.Nil, "player", nil)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for missing player_id, got %d. Body: %s", w.Code, w.Body.String())
	}
}
