// Package notification creates and delivers persistent inbox notifications.
//
// # Notification types
//
//   - friend_request: delivered when a player sends a friend request.
//     Action (accept/decline) expires after 7 days.
//   - room_invitation: delivered when a player invites another to their room.
//     Action (accept/decline) expires after 24 hours.
//   - ban_issued: delivered when a queue ban is issued (decline threshold or
//     moderator action). Informational only — no inline action.
//
// # Future hooks
//
// NotifyFriendRequest and NotifyRoomInvitation are fully implemented here.
// The endpoints that call them (POST /friends/request, POST /rooms/{id}/invite)
// do not exist yet and will be added when the friend system is built.
package notification

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/tableforge/server/internal/platform/store"
	"github.com/tableforge/server/internal/platform/ws"
)

const (
	// FriendRequestActionExpiry is how long a friend request stays actionable.
	FriendRequestActionExpiry = 7 * 24 * time.Hour

	// RoomInvitationActionExpiry is how long a room invitation stays actionable.
	RoomInvitationActionExpiry = 24 * time.Hour

	// ReadNotificationCutoff is the age beyond which read notifications are
	// excluded from ListNotifications results.
	ReadNotificationCutoff = 30 * 24 * time.Hour
)

// Service creates and delivers inbox notifications.
type Service struct {
	store store.Store
	hub   *ws.Hub
}

// New creates a new notification Service.
func New(st store.Store, hub *ws.Hub) *Service {
	return &Service{store: st, hub: hub}
}

// ---------------------------------------------------------------------------
// Public notify helpers
// ---------------------------------------------------------------------------

// NotifyFriendRequest creates a friend_request notification for toPlayerID.
// Called when fromPlayer sends a friend request.
func (svc *Service) NotifyFriendRequest(ctx context.Context, toPlayerID uuid.UUID, fromPlayerID uuid.UUID, fromUsername string) error {
	payload := store.NotificationPayloadFriendRequest{
		FromPlayerID: fromPlayerID.String(),
		FromUsername: fromUsername,
	}
	expiry := time.Now().Add(FriendRequestActionExpiry)
	return svc.create(ctx, toPlayerID, store.NotificationTypeFriendRequest, payload, &expiry)
}

// NotifyRoomInvitation creates a room_invitation notification for toPlayerID.
// Called when fromPlayer invites another player to their room directly.
func (svc *Service) NotifyRoomInvitation(
	ctx context.Context,
	toPlayerID uuid.UUID,
	fromPlayerID uuid.UUID,
	fromUsername string,
	roomID uuid.UUID,
	roomCode string,
	gameID string,
	gameName string,
) error {
	payload := store.NotificationPayloadRoomInvitation{
		FromPlayerID: fromPlayerID.String(),
		FromUsername: fromUsername,
		RoomID:       roomID.String(),
		RoomCode:     roomCode,
		GameID:       gameID,
		GameName:     gameName,
	}
	expiry := time.Now().Add(RoomInvitationActionExpiry)
	return svc.create(ctx, toPlayerID, store.NotificationTypeRoomInvitation, payload, &expiry)
}

// NotifyBanIssued creates a ban_issued notification for playerID.
// Called by the queue service when a penalty ban is applied.
func (svc *Service) NotifyBanIssued(
	ctx context.Context,
	playerID uuid.UUID,
	reason store.BanReason,
	duration time.Duration,
) error {
	expiresAt := time.Now().Add(duration)
	payload := store.NotificationPayloadBanIssued{
		Reason:       reason,
		DurationSecs: int(duration.Seconds()),
		ExpiresAt:    expiresAt,
	}
	return svc.create(ctx, playerID, store.NotificationTypeBanIssued, payload, nil)
}

// ---------------------------------------------------------------------------
// Read / action
// ---------------------------------------------------------------------------

// MarkRead marks a notification as read. No-op if already read.
// Returns ErrNotificationNotFound if the notification does not exist or does
// not belong to playerID.
func (svc *Service) MarkRead(ctx context.Context, playerID uuid.UUID, notificationID uuid.UUID) error {
	n, err := svc.store.GetNotification(ctx, notificationID)
	if err != nil {
		return ErrNotificationNotFound
	}
	if n.PlayerID != playerID {
		return ErrNotificationNotFound // do not leak existence to other players
	}
	return svc.store.MarkNotificationRead(ctx, notificationID)
}

// TakeAction records an accept or decline action on a notification.
// Validates ownership, action validity for the notification type, and expiry.
func (svc *Service) TakeAction(ctx context.Context, playerID uuid.UUID, notificationID uuid.UUID, action string) error {
	if action != "accepted" && action != "declined" {
		return ErrInvalidAction
	}

	n, err := svc.store.GetNotification(ctx, notificationID)
	if err != nil {
		return ErrNotificationNotFound
	}
	if n.PlayerID != playerID {
		return ErrNotificationNotFound
	}
	if n.Type == store.NotificationTypeBanIssued {
		return ErrActionNotSupported
	}
	if n.ActionTaken != nil {
		return ErrActionAlreadyTaken
	}
	if n.ActionExpiresAt != nil && time.Now().After(*n.ActionExpiresAt) {
		return ErrNotificationActionExpired
	}

	if err := svc.store.SetNotificationAction(ctx, notificationID, action); err != nil {
		return fmt.Errorf("TakeAction: %w", err)
	}

	// Auto-mark as read when an action is taken.
	_ = svc.store.MarkNotificationRead(ctx, notificationID)

	return nil
}

// List returns notifications for playerID.
// Read notifications older than ReadNotificationCutoff are excluded.
func (svc *Service) List(ctx context.Context, playerID uuid.UUID, includeRead bool) ([]store.Notification, error) {
	cutoff := time.Now().Add(-ReadNotificationCutoff)
	return svc.store.ListNotifications(ctx, playerID, includeRead, cutoff)
}

// ---------------------------------------------------------------------------
// Internal
// ---------------------------------------------------------------------------

func (svc *Service) create(
	ctx context.Context,
	playerID uuid.UUID,
	nType store.NotificationType,
	payload any,
	actionExpiresAt *time.Time,
) error {
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("notification.create: marshal payload: %w", err)
	}

	n, err := svc.store.CreateNotification(ctx, store.CreateNotificationParams{
		PlayerID:        playerID,
		Type:            nType,
		Payload:         payloadJSON,
		ActionExpiresAt: actionExpiresAt,
	})
	if err != nil {
		return fmt.Errorf("notification.create: %w", err)
	}

	// Push real-time delivery via WebSocket player channel.
	if svc.hub != nil {
		svc.hub.BroadcastToPlayer(playerID, ws.Event{
			Type:    ws.EventNotificationReceived,
			Payload: n,
		})
	}

	return nil
}

// ---------------------------------------------------------------------------
// Errors
// ---------------------------------------------------------------------------

var (
	ErrNotificationNotFound      = fmt.Errorf("notification not found")
	ErrInvalidAction             = fmt.Errorf("action must be 'accepted' or 'declined'")
	ErrActionNotSupported        = fmt.Errorf("this notification type does not support actions")
	ErrActionAlreadyTaken        = fmt.Errorf("action already taken on this notification")
	ErrNotificationActionExpired = fmt.Errorf("notification action has expired")
)
