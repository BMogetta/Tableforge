package store

import (
	"context"
	"fmt"

	"github.com/google/uuid"
)

// --- Room chat ---------------------------------------------------------------

func (s *pgStore) SaveRoomMessage(ctx context.Context, roomID, playerID uuid.UUID, content string) (RoomMessage, error) {
	row := s.db.QueryRow(ctx,
		`INSERT INTO room_messages (room_id, player_id, content)
		 VALUES ($1, $2, $3)
		 RETURNING id, room_id, player_id, content, reported, hidden, created_at`,
		roomID, playerID, content,
	)
	var m RoomMessage
	if err := row.Scan(&m.ID, &m.RoomID, &m.PlayerID, &m.Content, &m.Reported, &m.Hidden, &m.CreatedAt); err != nil {
		return RoomMessage{}, fmt.Errorf("SaveRoomMessage: %w", err)
	}
	return m, nil
}

func (s *pgStore) GetRoomMessages(ctx context.Context, roomID uuid.UUID, limit, offset int) ([]RoomMessage, error) {
	rows, err := s.db.Query(ctx,
		`SELECT id, room_id, player_id, content, reported, hidden, created_at
		 FROM room_messages
		 WHERE room_id = $1
		 ORDER BY created_at ASC
		 LIMIT $2 OFFSET $3`,
		roomID, limit, offset,
	)
	if err != nil {
		return nil, fmt.Errorf("GetRoomMessages: %w", err)
	}
	defer rows.Close()

	messages := []RoomMessage{}
	for rows.Next() {
		var m RoomMessage
		if err := rows.Scan(&m.ID, &m.RoomID, &m.PlayerID, &m.Content, &m.Reported, &m.Hidden, &m.CreatedAt); err != nil {
			return nil, fmt.Errorf("GetRoomMessages scan: %w", err)
		}
		messages = append(messages, m)
	}
	return messages, rows.Err()
}

func (s *pgStore) CountRoomMessages(ctx context.Context, roomID uuid.UUID) (int, error) {
	var count int
	err := s.db.QueryRow(ctx,
		`SELECT COUNT(*) FROM room_messages WHERE room_id = $1`,
		roomID,
	).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("CountRoomMessages: %w", err)
	}
	return count, nil
}

func (s *pgStore) HideRoomMessage(ctx context.Context, messageID uuid.UUID) error {
	tag, err := s.db.Exec(ctx,
		`UPDATE room_messages SET hidden = true WHERE id = $1`,
		messageID,
	)
	if err != nil {
		return fmt.Errorf("HideRoomMessage: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("HideRoomMessage: message not found")
	}
	return nil
}

func (s *pgStore) ReportRoomMessage(ctx context.Context, messageID uuid.UUID) error {
	tag, err := s.db.Exec(ctx,
		`UPDATE room_messages SET reported = true WHERE id = $1`,
		messageID,
	)
	if err != nil {
		return fmt.Errorf("ReportRoomMessage: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("ReportRoomMessage: message not found")
	}
	return nil
}

// IsRoomParticipant checks if playerID is seated in roomID.
// Reads from public.room_players — owned by game-server during migration phase.
func (s *pgStore) IsRoomParticipant(ctx context.Context, roomID, playerID uuid.UUID) (bool, error) {
	var exists bool
	err := s.db.QueryRow(ctx,
		`SELECT EXISTS (
			SELECT 1 FROM room_players
			WHERE room_id = $1 AND player_id = $2
		)`,
		roomID, playerID,
	).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("IsRoomParticipant: %w", err)
	}
	return exists, nil
}

// --- Direct messages ---------------------------------------------------------

func (s *pgStore) SaveDM(ctx context.Context, senderID, receiverID uuid.UUID, content string) (DirectMessage, error) {
	row := s.db.QueryRow(ctx,
		`INSERT INTO direct_messages (sender_id, receiver_id, content)
		 VALUES ($1, $2, $3)
		 RETURNING id, sender_id, receiver_id, content, read_at, reported, hidden, created_at`,
		senderID, receiverID, content,
	)
	var m DirectMessage
	if err := row.Scan(&m.ID, &m.SenderID, &m.ReceiverID, &m.Content, &m.ReadAt, &m.Reported, &m.Hidden, &m.CreatedAt); err != nil {
		return DirectMessage{}, fmt.Errorf("SaveDM: %w", err)
	}
	return m, nil
}

func (s *pgStore) GetDMHistory(ctx context.Context, playerA, playerB uuid.UUID) ([]DirectMessage, error) {
	rows, err := s.db.Query(ctx,
		`SELECT id, sender_id, receiver_id, content, read_at, reported, hidden, created_at
		 FROM direct_messages
		 WHERE (sender_id = $1 AND receiver_id = $2)
		    OR (sender_id = $2 AND receiver_id = $1)
		 ORDER BY created_at ASC`,
		playerA, playerB,
	)
	if err != nil {
		return nil, fmt.Errorf("GetDMHistory: %w", err)
	}
	defer rows.Close()

	messages := []DirectMessage{}
	for rows.Next() {
		var m DirectMessage
		if err := rows.Scan(&m.ID, &m.SenderID, &m.ReceiverID, &m.Content, &m.ReadAt, &m.Reported, &m.Hidden, &m.CreatedAt); err != nil {
			return nil, fmt.Errorf("GetDMHistory scan: %w", err)
		}
		messages = append(messages, m)
	}
	return messages, rows.Err()
}

func (s *pgStore) MarkDMRead(ctx context.Context, messageID, receiverID uuid.UUID) error {
	_, err := s.db.Exec(ctx,
		`UPDATE direct_messages SET read_at = NOW() WHERE id = $1 AND receiver_id = $2 AND read_at IS NULL`,
		messageID, receiverID,
	)
	if err != nil {
		return fmt.Errorf("MarkDMRead: %w", err)
	}
	return nil
}

func (s *pgStore) GetUnreadDMCount(ctx context.Context, playerID uuid.UUID) (int, error) {
	var count int
	err := s.db.QueryRow(ctx,
		`SELECT COUNT(*) FROM direct_messages
		 WHERE receiver_id = $1 AND read_at IS NULL AND hidden = false`,
		playerID,
	).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("GetUnreadDMCount: %w", err)
	}
	return count, nil
}

func (s *pgStore) ListDMConversations(ctx context.Context, playerID uuid.UUID) ([]DMConversation, error) {
	rows, err := s.db.Query(ctx, `
		WITH conversations AS (
			SELECT DISTINCT
				CASE WHEN sender_id = $1 THEN receiver_id ELSE sender_id END as other_id
			FROM direct_messages
			WHERE sender_id = $1 OR receiver_id = $1
		),
		latest_msg AS (
			SELECT DISTINCT ON (
				CASE WHEN dm.sender_id = $1 THEN dm.receiver_id ELSE dm.sender_id END
			)
				CASE WHEN dm.sender_id = $1 THEN dm.receiver_id ELSE dm.sender_id END as other_id,
				dm.content as last_message,
				dm.created_at as last_message_at
			FROM direct_messages dm
			WHERE dm.sender_id = $1 OR dm.receiver_id = $1
			ORDER BY
				CASE WHEN dm.sender_id = $1 THEN dm.receiver_id ELSE dm.sender_id END,
				dm.created_at DESC
		),
		unread AS (
			SELECT
				sender_id as other_id,
				COUNT(*) as unread_count
			FROM direct_messages
			WHERE receiver_id = $1 AND read_at IS NULL AND hidden = false
			GROUP BY sender_id
		)
		SELECT
			c.other_id,
			p.username,
			p.avatar_url,
			COALESCE(lm.last_message, ''),
			COALESCE(lm.last_message_at, NOW()),
			COALESCE(u.unread_count, 0)
		FROM conversations c
		JOIN public.players p ON p.id = c.other_id
		LEFT JOIN latest_msg lm ON lm.other_id = c.other_id
		LEFT JOIN unread u ON u.other_id = c.other_id
		ORDER BY lm.last_message_at DESC NULLS LAST`,
		playerID,
	)
	if err != nil {
		return nil, fmt.Errorf("ListDMConversations: %w", err)
	}
	defer rows.Close()

	convos := []DMConversation{}
	for rows.Next() {
		var c DMConversation
		if err := rows.Scan(&c.OtherPlayerID, &c.OtherUsername, &c.OtherAvatarURL, &c.LastMessage, &c.LastMessageAt, &c.UnreadCount); err != nil {
			return nil, fmt.Errorf("ListDMConversations scan: %w", err)
		}
		convos = append(convos, c)
	}
	return convos, rows.Err()
}

// GetAllowDMs returns the receiver's allow_dms preference.
// Falls back to "anyone" when no settings row exists.
func (s *pgStore) GetAllowDMs(ctx context.Context, playerID uuid.UUID) (string, error) {
	var raw []byte
	err := s.db.QueryRow(ctx,
		`SELECT settings->'allow_dms' FROM player_settings WHERE player_id = $1`,
		playerID,
	).Scan(&raw)
	if err != nil {
		// No row → default is "anyone".
		return "anyone", nil
	}
	// JSONB text comes back as a quoted string, e.g. `"friends_only"`.
	if len(raw) >= 2 && raw[0] == '"' {
		return string(raw[1 : len(raw)-1]), nil
	}
	return "anyone", nil
}

func (s *pgStore) ReportDM(ctx context.Context, messageID, playerA, playerB uuid.UUID) error {
	tag, err := s.db.Exec(ctx,
		`UPDATE direct_messages SET reported = true
		 WHERE id = $1
		   AND ((sender_id = $2 AND receiver_id = $3) OR (sender_id = $3 AND receiver_id = $2))`,
		messageID, playerA, playerB,
	)
	if err != nil {
		return fmt.Errorf("ReportDM: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("ReportDM: message not found")
	}
	return nil
}
