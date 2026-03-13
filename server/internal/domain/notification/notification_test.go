package notification_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/tableforge/server/internal/domain/notification"
	"github.com/tableforge/server/internal/platform/store"
	"github.com/tableforge/server/internal/testutil"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func newService(t *testing.T) (*notification.Service, *testutil.FakeStore) {
	t.Helper()
	fs := testutil.NewFakeStore()
	svc := notification.New(fs, nil) // nil hub — no WS delivery in unit tests
	return svc, fs
}

func newPlayerID(t *testing.T) uuid.UUID {
	t.Helper()
	return uuid.New()
}

// ---------------------------------------------------------------------------
// NotifyBanIssued
// ---------------------------------------------------------------------------

func TestNotifyBanIssued_CreatesNotification(t *testing.T) {
	svc, fs := newService(t)
	ctx := context.Background()
	playerID := newPlayerID(t)

	err := svc.NotifyBanIssued(ctx, playerID, store.BanReasonDeclineThreshold, 5*time.Minute)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	notifications, _ := fs.ListNotifications(ctx, playerID, false, time.Now().Add(-time.Hour))
	if len(notifications) != 1 {
		t.Fatalf("expected 1 notification, got %d", len(notifications))
	}
	if notifications[0].Type != store.NotificationTypeBanIssued {
		t.Errorf("expected type ban_issued, got %s", notifications[0].Type)
	}
	if notifications[0].ActionExpiresAt != nil {
		t.Error("expected no action expiry for ban_issued")
	}
}

func TestNotifyBanIssued_PayloadContainsReason(t *testing.T) {
	svc, fs := newService(t)
	ctx := context.Background()
	playerID := newPlayerID(t)

	svc.NotifyBanIssued(ctx, playerID, store.BanReasonDeclineThreshold, 25*time.Minute)

	notifications, _ := fs.ListNotifications(ctx, playerID, false, time.Now().Add(-time.Hour))
	var payload store.NotificationPayloadBanIssued
	if err := json.Unmarshal(notifications[0].Payload, &payload); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	if payload.Reason != store.BanReasonDeclineThreshold {
		t.Errorf("expected reason=decline_threshold, got %s", payload.Reason)
	}
	if payload.DurationSecs != int((25 * time.Minute).Seconds()) {
		t.Errorf("expected duration_secs=%d, got %d", int((25 * time.Minute).Seconds()), payload.DurationSecs)
	}
}

// ---------------------------------------------------------------------------
// NotifyFriendRequest
// ---------------------------------------------------------------------------

func TestNotifyFriendRequest_CreatesNotification(t *testing.T) {
	svc, fs := newService(t)
	ctx := context.Background()
	toPlayerID := newPlayerID(t)
	fromPlayerID := newPlayerID(t)

	err := svc.NotifyFriendRequest(ctx, toPlayerID, fromPlayerID, "alice")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	notifications, _ := fs.ListNotifications(ctx, toPlayerID, false, time.Now().Add(-time.Hour))
	if len(notifications) != 1 {
		t.Fatalf("expected 1 notification, got %d", len(notifications))
	}
	if notifications[0].Type != store.NotificationTypeFriendRequest {
		t.Errorf("expected type friend_request, got %s", notifications[0].Type)
	}
	if notifications[0].ActionExpiresAt == nil {
		t.Error("expected action expiry for friend_request")
	}
}

func TestNotifyFriendRequest_PayloadContainsSender(t *testing.T) {
	svc, fs := newService(t)
	ctx := context.Background()
	toPlayerID := newPlayerID(t)
	fromPlayerID := newPlayerID(t)

	svc.NotifyFriendRequest(ctx, toPlayerID, fromPlayerID, "alice")

	notifications, _ := fs.ListNotifications(ctx, toPlayerID, false, time.Now().Add(-time.Hour))
	var payload store.NotificationPayloadFriendRequest
	if err := json.Unmarshal(notifications[0].Payload, &payload); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	if payload.FromPlayerID != fromPlayerID.String() {
		t.Errorf("expected from_player_id=%s, got %s", fromPlayerID, payload.FromPlayerID)
	}
	if payload.FromUsername != "alice" {
		t.Errorf("expected from_username=alice, got %s", payload.FromUsername)
	}
}

// ---------------------------------------------------------------------------
// NotifyRoomInvitation
// ---------------------------------------------------------------------------

func TestNotifyRoomInvitation_CreatesNotification(t *testing.T) {
	svc, fs := newService(t)
	ctx := context.Background()
	toPlayerID := newPlayerID(t)
	fromPlayerID := newPlayerID(t)
	roomID := uuid.New()

	err := svc.NotifyRoomInvitation(ctx, toPlayerID, fromPlayerID, "bob", roomID, "ABCD12", "tictactoe", "Tic Tac Toe")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	notifications, _ := fs.ListNotifications(ctx, toPlayerID, false, time.Now().Add(-time.Hour))
	if len(notifications) != 1 {
		t.Fatalf("expected 1 notification, got %d", len(notifications))
	}
	if notifications[0].Type != store.NotificationTypeRoomInvitation {
		t.Errorf("expected type room_invitation, got %s", notifications[0].Type)
	}
}

func TestNotifyRoomInvitation_PayloadContainsRoomDetails(t *testing.T) {
	svc, fs := newService(t)
	ctx := context.Background()
	toPlayerID := newPlayerID(t)
	fromPlayerID := newPlayerID(t)
	roomID := uuid.New()

	svc.NotifyRoomInvitation(ctx, toPlayerID, fromPlayerID, "bob", roomID, "ABCD12", "tictactoe", "Tic Tac Toe")

	notifications, _ := fs.ListNotifications(ctx, toPlayerID, false, time.Now().Add(-time.Hour))
	var payload store.NotificationPayloadRoomInvitation
	if err := json.Unmarshal(notifications[0].Payload, &payload); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	if payload.RoomID != roomID.String() {
		t.Errorf("expected room_id=%s, got %s", roomID, payload.RoomID)
	}
	if payload.RoomCode != "ABCD12" {
		t.Errorf("expected room_code=ABCD12, got %s", payload.RoomCode)
	}
	if payload.GameID != "tictactoe" {
		t.Errorf("expected game_id=tictactoe, got %s", payload.GameID)
	}
	if payload.GameName != "Tic Tac Toe" {
		t.Errorf("expected game_name=Tic Tac Toe, got %s", payload.GameName)
	}
}

// ---------------------------------------------------------------------------
// MarkRead
// ---------------------------------------------------------------------------

func TestMarkRead_MarksNotificationAsRead(t *testing.T) {
	svc, fs := newService(t)
	ctx := context.Background()
	playerID := newPlayerID(t)

	svc.NotifyBanIssued(ctx, playerID, store.BanReasonDeclineThreshold, 5*time.Minute)
	notifications, _ := fs.ListNotifications(ctx, playerID, false, time.Now().Add(-time.Hour))
	notifID := notifications[0].ID

	if err := svc.MarkRead(ctx, playerID, notifID); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should no longer appear in unread list.
	unread, _ := fs.ListNotifications(ctx, playerID, false, time.Now().Add(-time.Hour))
	if len(unread) != 0 {
		t.Errorf("expected 0 unread notifications, got %d", len(unread))
	}
}

func TestMarkRead_WrongPlayer_ReturnsNotFound(t *testing.T) {
	svc, fs := newService(t)
	ctx := context.Background()
	playerID := newPlayerID(t)
	otherID := newPlayerID(t)

	svc.NotifyBanIssued(ctx, playerID, store.BanReasonDeclineThreshold, 5*time.Minute)
	notifications, _ := fs.ListNotifications(ctx, playerID, false, time.Now().Add(-time.Hour))

	err := svc.MarkRead(ctx, otherID, notifications[0].ID)
	if err == nil {
		t.Fatal("expected ErrNotificationNotFound, got nil")
	}
}

func TestMarkRead_NotFound(t *testing.T) {
	svc, _ := newService(t)
	ctx := context.Background()

	err := svc.MarkRead(ctx, newPlayerID(t), uuid.New())
	if err == nil {
		t.Fatal("expected ErrNotificationNotFound, got nil")
	}
}

// ---------------------------------------------------------------------------
// TakeAction
// ---------------------------------------------------------------------------

func TestTakeAction_Accept_FriendRequest(t *testing.T) {
	svc, fs := newService(t)
	ctx := context.Background()
	toPlayerID := newPlayerID(t)

	svc.NotifyFriendRequest(ctx, toPlayerID, newPlayerID(t), "alice")
	notifications, _ := fs.ListNotifications(ctx, toPlayerID, false, time.Now().Add(-time.Hour))

	if err := svc.TakeAction(ctx, toPlayerID, notifications[0].ID, "accepted"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	n, _ := fs.GetNotification(ctx, notifications[0].ID)
	if n.ActionTaken == nil || *n.ActionTaken != "accepted" {
		t.Errorf("expected action_taken=accepted, got %v", n.ActionTaken)
	}
	// Auto-marked as read.
	if n.ReadAt == nil {
		t.Error("expected notification to be auto-marked as read after action")
	}
}

func TestTakeAction_Decline_RoomInvitation(t *testing.T) {
	svc, fs := newService(t)
	ctx := context.Background()
	toPlayerID := newPlayerID(t)

	svc.NotifyRoomInvitation(ctx, toPlayerID, newPlayerID(t), "bob", uuid.New(), "ABCD12", "tictactoe", "Tic Tac Toe")
	notifications, _ := fs.ListNotifications(ctx, toPlayerID, false, time.Now().Add(-time.Hour))

	if err := svc.TakeAction(ctx, toPlayerID, notifications[0].ID, "declined"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	n, _ := fs.GetNotification(ctx, notifications[0].ID)
	if n.ActionTaken == nil || *n.ActionTaken != "declined" {
		t.Errorf("expected action_taken=declined, got %v", n.ActionTaken)
	}
}

func TestTakeAction_BanIssued_NotSupported(t *testing.T) {
	svc, fs := newService(t)
	ctx := context.Background()
	playerID := newPlayerID(t)

	svc.NotifyBanIssued(ctx, playerID, store.BanReasonDeclineThreshold, 5*time.Minute)
	notifications, _ := fs.ListNotifications(ctx, playerID, false, time.Now().Add(-time.Hour))

	err := svc.TakeAction(ctx, playerID, notifications[0].ID, "accepted")
	if err == nil {
		t.Fatal("expected ErrActionNotSupported, got nil")
	}
}

func TestTakeAction_AlreadyTaken(t *testing.T) {
	svc, fs := newService(t)
	ctx := context.Background()
	toPlayerID := newPlayerID(t)

	svc.NotifyFriendRequest(ctx, toPlayerID, newPlayerID(t), "alice")
	notifications, _ := fs.ListNotifications(ctx, toPlayerID, false, time.Now().Add(-time.Hour))
	notifID := notifications[0].ID

	svc.TakeAction(ctx, toPlayerID, notifID, "accepted")

	err := svc.TakeAction(ctx, toPlayerID, notifID, "declined")
	if err == nil {
		t.Fatal("expected ErrActionAlreadyTaken, got nil")
	}
}

func TestTakeAction_InvalidAction(t *testing.T) {
	svc, fs := newService(t)
	ctx := context.Background()
	toPlayerID := newPlayerID(t)

	svc.NotifyFriendRequest(ctx, toPlayerID, newPlayerID(t), "alice")
	notifications, _ := fs.ListNotifications(ctx, toPlayerID, false, time.Now().Add(-time.Hour))

	err := svc.TakeAction(ctx, toPlayerID, notifications[0].ID, "maybe")
	if err == nil {
		t.Fatal("expected ErrInvalidAction, got nil")
	}
}

func TestTakeAction_WrongPlayer_ReturnsNotFound(t *testing.T) {
	svc, fs := newService(t)
	ctx := context.Background()
	toPlayerID := newPlayerID(t)
	otherID := newPlayerID(t)

	svc.NotifyFriendRequest(ctx, toPlayerID, newPlayerID(t), "alice")
	notifications, _ := fs.ListNotifications(ctx, toPlayerID, false, time.Now().Add(-time.Hour))

	err := svc.TakeAction(ctx, otherID, notifications[0].ID, "accepted")
	if err == nil {
		t.Fatal("expected ErrNotificationNotFound for wrong player, got nil")
	}
}

func TestTakeAction_Expired(t *testing.T) {
	svc, fs := newService(t)
	ctx := context.Background()
	toPlayerID := newPlayerID(t)

	svc.NotifyFriendRequest(ctx, toPlayerID, newPlayerID(t), "alice")
	notifications, _ := fs.ListNotifications(ctx, toPlayerID, false, time.Now().Add(-time.Hour))
	notifID := notifications[0].ID

	// Manually expire the action by setting action_expires_at in the past.
	n := fs.Notifications[notifID]
	past := time.Now().Add(-time.Hour)
	n.ActionExpiresAt = &past
	fs.Notifications[notifID] = n

	err := svc.TakeAction(ctx, toPlayerID, notifID, "accepted")
	if err == nil {
		t.Fatal("expected expiry error, got nil")
	}
}

// ---------------------------------------------------------------------------
// List
// ---------------------------------------------------------------------------

func TestList_ExcludesOldReadNotifications(t *testing.T) {
	svc, fs := newService(t)
	ctx := context.Background()
	playerID := newPlayerID(t)

	svc.NotifyBanIssued(ctx, playerID, store.BanReasonDeclineThreshold, 5*time.Minute)
	notifications, _ := fs.ListNotifications(ctx, playerID, false, time.Now().Add(-time.Hour))
	notifID := notifications[0].ID

	// Mark as read and backdate created_at beyond the 30-day cutoff.
	svc.MarkRead(ctx, playerID, notifID)
	n := fs.Notifications[notifID]
	n.CreatedAt = time.Now().Add(-31 * 24 * time.Hour)
	fs.Notifications[notifID] = n

	result, err := svc.List(ctx, playerID, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 0 {
		t.Errorf("expected old read notifications to be excluded, got %d", len(result))
	}
}

func TestList_UnreadAlwaysReturned(t *testing.T) {
	svc, fs := newService(t)
	ctx := context.Background()
	playerID := newPlayerID(t)

	svc.NotifyBanIssued(ctx, playerID, store.BanReasonDeclineThreshold, 5*time.Minute)

	// Backdate created_at but leave unread.
	notifications, _ := fs.ListNotifications(ctx, playerID, false, time.Now().Add(-time.Hour))
	n := fs.Notifications[notifications[0].ID]
	n.CreatedAt = time.Now().Add(-31 * 24 * time.Hour)
	fs.Notifications[notifications[0].ID] = n

	result, err := svc.List(ctx, playerID, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 1 {
		t.Errorf("expected unread notification to always be returned, got %d", len(result))
	}
}
