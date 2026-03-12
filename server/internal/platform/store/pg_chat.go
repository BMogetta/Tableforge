package store

import (
	"context"
	"fmt"

	"github.com/google/uuid"
)

// --- Room chat ---------------------------------------------------------------

// SaveRoomMessage inserts a new chat message into a room.
func (s *PGStore) SaveRoomMessage(ctx context.Context, roomID, playerID uuid.UUID, content string) (RoomMessage, error) {
	row := s.pool.QueryRow(ctx,
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

// GetRoomMessages returns all visible messages for a room ordered by creation time.
// Hidden messages are included so managers can see them; the caller or UI layer
// decides whether to render them.
func (s *PGStore) GetRoomMessages(ctx context.Context, roomID uuid.UUID) ([]RoomMessage, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, room_id, player_id, content, reported, hidden, created_at
		 FROM room_messages
		 WHERE room_id = $1
		 ORDER BY created_at ASC`,
		roomID,
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

// HideRoomMessage marks a message as hidden. Manager-only action.
func (s *PGStore) HideRoomMessage(ctx context.Context, messageID uuid.UUID) error {
	tag, err := s.pool.Exec(ctx,
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

// ReportRoomMessage flags a message as reported.
func (s *PGStore) ReportRoomMessage(ctx context.Context, messageID uuid.UUID) error {
	tag, err := s.pool.Exec(ctx,
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

// --- Direct messages ---------------------------------------------------------

// SaveDM inserts a direct message from senderID to receiverID.
func (s *PGStore) SaveDM(ctx context.Context, senderID, receiverID uuid.UUID, content string) (DirectMessage, error) {
	row := s.pool.QueryRow(ctx,
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

// GetDMHistory returns all direct messages exchanged between two players,
// ordered chronologically. The query is direction-agnostic: it returns messages
// where (sender, receiver) is either (playerA, playerB) or (playerB, playerA).
func (s *PGStore) GetDMHistory(ctx context.Context, playerA, playerB uuid.UUID) ([]DirectMessage, error) {
	rows, err := s.pool.Query(ctx,
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

// MarkDMRead sets read_at to now for a direct message.
// Only the receiver should call this.
func (s *PGStore) MarkDMRead(ctx context.Context, messageID uuid.UUID) error {
	tag, err := s.pool.Exec(ctx,
		`UPDATE direct_messages SET read_at = NOW() WHERE id = $1 AND read_at IS NULL`,
		messageID,
	)
	if err != nil {
		return fmt.Errorf("MarkDMRead: %w", err)
	}
	if tag.RowsAffected() == 0 {
		// Already read or not found — not an error worth surfacing.
		return nil
	}
	return nil
}

// GetUnreadDMCount returns the number of unread messages addressed to playerID.
func (s *PGStore) GetUnreadDMCount(ctx context.Context, playerID uuid.UUID) (int, error) {
	var count int
	err := s.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM direct_messages
		 WHERE receiver_id = $1 AND read_at IS NULL AND hidden = false`,
		playerID,
	).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("GetUnreadDMCount: %w", err)
	}
	return count, nil
}

// ReportDM flags a direct message as reported.
func (s *PGStore) ReportDM(ctx context.Context, messageID uuid.UUID) error {
	tag, err := s.pool.Exec(ctx,
		`UPDATE direct_messages SET reported = true WHERE id = $1`,
		messageID,
	)
	if err != nil {
		return fmt.Errorf("ReportDM: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("ReportDM: message not found")
	}
	return nil
}
