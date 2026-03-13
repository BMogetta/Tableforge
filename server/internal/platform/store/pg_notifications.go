package store

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

var ErrNotificationActionExpired = fmt.Errorf("notification action has expired or already taken")

func (s *PGStore) CreateNotification(ctx context.Context, params CreateNotificationParams) (Notification, error) {
	row := s.pool.QueryRow(ctx,
		`INSERT INTO notifications (player_id, type, payload, action_expires_at)
         VALUES ($1, $2, $3, $4)
         RETURNING id, player_id, type, payload, action_taken, action_expires_at, read_at, created_at`,
		params.PlayerID, params.Type, params.Payload, params.ActionExpiresAt,
	)
	return scanNotification(row)
}

func (s *PGStore) GetNotification(ctx context.Context, id uuid.UUID) (Notification, error) {
	row := s.pool.QueryRow(ctx,
		`SELECT id, player_id, type, payload, action_taken, action_expires_at, read_at, created_at
         FROM notifications
         WHERE id = $1`,
		id,
	)
	n, err := scanNotification(row)
	if err != nil {
		if err == pgx.ErrNoRows {
			return Notification{}, pgx.ErrNoRows
		}
		return Notification{}, fmt.Errorf("GetNotification: %w", err)
	}
	return n, nil
}

func (s *PGStore) ListNotifications(ctx context.Context, playerID uuid.UUID, includeRead bool, readCutoff time.Time) ([]Notification, error) {
	var rows pgx.Rows
	var err error

	if includeRead {
		rows, err = s.pool.Query(ctx,
			`SELECT id, player_id, type, payload, action_taken, action_expires_at, read_at, created_at
             FROM notifications
             WHERE player_id = $1
               AND (read_at IS NULL OR (read_at IS NOT NULL AND created_at >= $2))
             ORDER BY created_at DESC`,
			playerID, readCutoff,
		)
	} else {
		rows, err = s.pool.Query(ctx,
			`SELECT id, player_id, type, payload, action_taken, action_expires_at, read_at, created_at
             FROM notifications
             WHERE player_id = $1
               AND read_at IS NULL
             ORDER BY created_at DESC`,
			playerID,
		)
	}
	if err != nil {
		return nil, fmt.Errorf("ListNotifications: query: %w", err)
	}
	defer rows.Close()

	var notifications []Notification
	for rows.Next() {
		n, err := scanNotification(rows)
		if err != nil {
			return nil, fmt.Errorf("ListNotifications: scan: %w", err)
		}
		notifications = append(notifications, n)
	}
	if notifications == nil {
		notifications = []Notification{}
	}
	return notifications, rows.Err()
}

func (s *PGStore) MarkNotificationRead(ctx context.Context, id uuid.UUID) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE notifications
         SET read_at = NOW()
         WHERE id = $1 AND read_at IS NULL`,
		id,
	)
	if err != nil {
		return fmt.Errorf("MarkNotificationRead: %w", err)
	}
	return nil
}

func (s *PGStore) SetNotificationAction(ctx context.Context, id uuid.UUID, action string) error {
	tag, err := s.pool.Exec(ctx,
		`UPDATE notifications
         SET action_taken = $2
         WHERE id = $1
           AND action_taken IS NULL
           AND (action_expires_at IS NULL OR action_expires_at > NOW())`,
		id, action,
	)
	if err != nil {
		return fmt.Errorf("SetNotificationAction: %w", err)
	}
	if tag.RowsAffected() == 0 {
		// Either already acted on or action has expired.
		return ErrNotificationActionExpired
	}
	return nil
}

// ---------------------------------------------------------------------------
// Scanner
// ---------------------------------------------------------------------------

type notificationScanner interface {
	Scan(dest ...any) error
}

func scanNotification(s notificationScanner) (Notification, error) {
	var n Notification
	err := s.Scan(
		&n.ID,
		&n.PlayerID,
		&n.Type,
		&n.Payload,
		&n.ActionTaken,
		&n.ActionExpiresAt,
		&n.ReadAt,
		&n.CreatedAt,
	)
	if err != nil {
		return Notification{}, fmt.Errorf("scanNotification: %w", err)
	}
	return n, nil
}
