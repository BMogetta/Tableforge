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

func (s *pgStore) GetRoomMessages(ctx context.Context, roomID uuid.UUID) ([]RoomMessage, error) {
	rows, err := s.db.Query(ctx,
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

func (s *pgStore) MarkDMRead(ctx context.Context, messageID uuid.UUID) error {
	_, err := s.db.Exec(ctx,
		`UPDATE direct_messages SET read_at = NOW() WHERE id = $1 AND read_at IS NULL`,
		messageID,
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

func (s *pgStore) ReportDM(ctx context.Context, messageID uuid.UUID) error {
	tag, err := s.db.Exec(ctx,
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
