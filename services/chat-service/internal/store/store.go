package store

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// --- Models ------------------------------------------------------------------

// RoomMessage is a single chat message in a room.
// Messages are auditable and never deleted; managers may hide them.
type RoomMessage struct {
	ID        uuid.UUID `json:"message_id"`
	RoomID    uuid.UUID `json:"room_id"`
	PlayerID  uuid.UUID `json:"player_id"`
	Content   string    `json:"content"`
	Reported  bool      `json:"reported"`
	Hidden    bool      `json:"hidden"`
	CreatedAt time.Time `json:"timestamp"`
}

// DirectMessage is a private message between two players.
type DirectMessage struct {
	ID         uuid.UUID  `json:"message_id"`
	SenderID   uuid.UUID  `json:"sender_id"`
	ReceiverID uuid.UUID  `json:"receiver_id"`
	Content    string     `json:"content"`
	ReadAt     *time.Time `json:"read_at,omitempty"`
	Reported   bool       `json:"reported"`
	Hidden     bool       `json:"hidden"`
	CreatedAt  time.Time  `json:"timestamp"`
}

// DMConversation represents a unique DM conversation partner with summary info.
type DMConversation struct {
	OtherPlayerID  uuid.UUID  `json:"other_player_id"`
	OtherUsername  string     `json:"other_username"`
	OtherAvatarURL *string    `json:"other_avatar_url,omitempty"`
	LastMessage    string     `json:"last_message"`
	LastMessageAt  time.Time  `json:"last_message_at"`
	UnreadCount    int        `json:"unread_count"`
}

// --- Store interface ---------------------------------------------------------

type Store interface {
	// Room chat
	SaveRoomMessage(ctx context.Context, roomID, playerID uuid.UUID, content string) (RoomMessage, error)
	GetRoomMessages(ctx context.Context, roomID uuid.UUID) ([]RoomMessage, error)
	HideRoomMessage(ctx context.Context, messageID uuid.UUID) error
	ReportRoomMessage(ctx context.Context, messageID uuid.UUID) error

	// Direct messages
	SaveDM(ctx context.Context, senderID, receiverID uuid.UUID, content string) (DirectMessage, error)
	GetDMHistory(ctx context.Context, playerA, playerB uuid.UUID) ([]DirectMessage, error)
	MarkDMRead(ctx context.Context, messageID, receiverID uuid.UUID) error
	GetUnreadDMCount(ctx context.Context, playerID uuid.UUID) (int, error)
	ListDMConversations(ctx context.Context, playerID uuid.UUID) ([]DMConversation, error)
	ReportDM(ctx context.Context, messageID, playerA, playerB uuid.UUID) error

	// Room participants — needed to validate sender is not a spectator.
	// Chat-service reads from public.room_players (owned by game-server/monolith).
	// Acceptable during migration phase — will be replaced by gRPC call
	// when room-service is extracted.
	IsRoomParticipant(ctx context.Context, roomID, playerID uuid.UUID) (bool, error)

	// GetAllowDMs returns the receiver's allow_dms setting ("anyone", "friends_only", "nobody").
	// Reads from public.player_settings (owned by user-service).
	// Returns "anyone" (the default) if no settings row exists.
	GetAllowDMs(ctx context.Context, playerID uuid.UUID) (string, error)
}

// --- Postgres implementation -------------------------------------------------

type pgStore struct {
	db *pgxpool.Pool
}

// New connects to Postgres and returns a Store.
func New(ctx context.Context, connStr string) (*pgStore, error) {
	pool, err := pgxpool.New(ctx, connStr)
	if err != nil {
		return nil, fmt.Errorf("pgxpool.New: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		return nil, fmt.Errorf("ping: %w", err)
	}
	return &pgStore{db: pool}, nil
}

// Close releases the connection pool.
func (s *pgStore) Close() {
	s.db.Close()
}
