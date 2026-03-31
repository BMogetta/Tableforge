package store

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// NotificationType identifies the kind of inbox notification.
type NotificationType string

const (
	NotificationTypeFriendRequest         NotificationType = "friend_request"
	NotificationTypeFriendRequestAccepted NotificationType = "friend_request_accepted"
	NotificationTypeRoomInvitation        NotificationType = "room_invitation"
	NotificationTypeBanIssued             NotificationType = "ban_issued"
)

// BanReason mirrors the queue domain's ban reason values.
type BanReason string

const (
	BanReasonDeclineThreshold BanReason = "decline_threshold"
	BanReasonModerator        BanReason = "moderator"
)

// Notification is a persistent inbox notification for a player.
type Notification struct {
	ID              uuid.UUID        `json:"id"`
	PlayerID        uuid.UUID        `json:"player_id"`
	Type            NotificationType `json:"type"`
	Payload         json.RawMessage  `json:"payload"`
	ReadAt          *time.Time       `json:"read_at,omitempty"`
	ActionTaken     *string          `json:"action_taken,omitempty"`
	ActionExpiresAt *time.Time       `json:"action_expires_at,omitempty"`
	CreatedAt       time.Time        `json:"created_at"`
}

// CreateParams holds the fields needed to create a notification.
type CreateParams struct {
	PlayerID        uuid.UUID
	Type            NotificationType
	Payload         json.RawMessage
	ActionExpiresAt *time.Time
}

// PayloadFriendRequest is the payload for friend_request notifications.
type PayloadFriendRequest struct {
	FromPlayerID string `json:"from_player_id"`
	FromUsername string `json:"from_username"`
}

// PayloadFriendRequestAccepted is the payload for friend_request_accepted notifications.
type PayloadFriendRequestAccepted struct {
	FromPlayerID string `json:"from_player_id"` // the one who accepted
	FromUsername string `json:"from_username"`
}

// PayloadRoomInvitation is the payload for room_invitation notifications.
type PayloadRoomInvitation struct {
	FromPlayerID string `json:"from_player_id"`
	FromUsername string `json:"from_username"`
	RoomID       string `json:"room_id"`
	RoomCode     string `json:"room_code"`
	GameID       string `json:"game_id"`
	GameName     string `json:"game_name"`
}

// PayloadBanIssued is the payload for ban_issued notifications.
type PayloadBanIssued struct {
	Reason       BanReason `json:"reason"`
	DurationSecs int       `json:"duration_secs"`
	ExpiresAt    time.Time `json:"expires_at"`
}

// Store handles notification persistence.
type Store struct {
	db *pgxpool.Pool
}

func New(ctx context.Context, dsn string) (*Store, error) {
	db, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, fmt.Errorf("notification store: connect: %w", err)
	}
	if err := db.Ping(ctx); err != nil {
		return nil, fmt.Errorf("notification store: ping: %w", err)
	}
	return &Store{db: db}, nil
}

func (s *Store) Create(ctx context.Context, p CreateParams) (Notification, error) {
	var n Notification
	err := s.db.QueryRow(ctx, `
		INSERT INTO notifications (player_id, type, payload, action_expires_at)
		VALUES ($1, $2, $3, $4)
		RETURNING id, player_id, type, payload, read_at, action_taken, action_expires_at, created_at
	`, p.PlayerID, p.Type, p.Payload, p.ActionExpiresAt).Scan(
		&n.ID, &n.PlayerID, &n.Type, &n.Payload,
		&n.ReadAt, &n.ActionTaken, &n.ActionExpiresAt, &n.CreatedAt,
	)
	if err != nil {
		return Notification{}, fmt.Errorf("notification store: create: %w", err)
	}
	return n, nil
}

func (s *Store) Get(ctx context.Context, id uuid.UUID) (Notification, error) {
	var n Notification
	err := s.db.QueryRow(ctx, `
		SELECT id, player_id, type, payload, read_at, action_taken, action_expires_at, created_at
		FROM notifications WHERE id = $1
	`, id).Scan(
		&n.ID, &n.PlayerID, &n.Type, &n.Payload,
		&n.ReadAt, &n.ActionTaken, &n.ActionExpiresAt, &n.CreatedAt,
	)
	if err != nil {
		return Notification{}, fmt.Errorf("notification store: get: %w", err)
	}
	return n, nil
}

func (s *Store) List(ctx context.Context, playerID uuid.UUID, includeRead bool, readCutoff time.Time) ([]Notification, error) {
	query := `
		SELECT id, player_id, type, payload, read_at, action_taken, action_expires_at, created_at
		FROM notifications
		WHERE player_id = $1
		  AND (read_at IS NULL OR (read_at IS NOT NULL AND $2 AND read_at > $3))
		ORDER BY created_at DESC
	`
	rows, err := s.db.Query(ctx, query, playerID, includeRead, readCutoff)
	if err != nil {
		return nil, fmt.Errorf("notification store: list: %w", err)
	}
	defer rows.Close()

	var out []Notification
	for rows.Next() {
		var n Notification
		if err := rows.Scan(
			&n.ID, &n.PlayerID, &n.Type, &n.Payload,
			&n.ReadAt, &n.ActionTaken, &n.ActionExpiresAt, &n.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("notification store: list scan: %w", err)
		}
		out = append(out, n)
	}
	return out, rows.Err()
}

func (s *Store) MarkRead(ctx context.Context, id uuid.UUID) error {
	_, err := s.db.Exec(ctx,
		`UPDATE notifications SET read_at = now() WHERE id = $1 AND read_at IS NULL`,
		id,
	)
	return err
}

func (s *Store) SetAction(ctx context.Context, id uuid.UUID, action string) error {
	_, err := s.db.Exec(ctx,
		`UPDATE notifications SET action_taken = $2 WHERE id = $1 AND action_taken IS NULL`,
		id, action,
	)
	return err
}
