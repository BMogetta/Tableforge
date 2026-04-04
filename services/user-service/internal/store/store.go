package store

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// --- Models ------------------------------------------------------------------

type FriendshipStatus string

const (
	FriendshipStatusPending  FriendshipStatus = "pending"
	FriendshipStatusAccepted FriendshipStatus = "accepted"
	FriendshipStatusBlocked  FriendshipStatus = "blocked"
)

type Friendship struct {
	RequesterID uuid.UUID        `json:"requester_id"`
	AddresseeID uuid.UUID        `json:"addressee_id"`
	Status      FriendshipStatus `json:"status"`
	Note        *string          `json:"note,omitempty"`
	CreatedAt   time.Time        `json:"created_at"`
	UpdatedAt   time.Time        `json:"updated_at"`
}

type FriendshipView struct {
	FriendID        uuid.UUID        `json:"friend_id"`
	FriendUsername  string           `json:"friend_username"`
	FriendAvatarURL *string          `json:"friend_avatar_url,omitempty"`
	Status          FriendshipStatus `json:"status"`
	CreatedAt       time.Time        `json:"created_at"`
}

type PlayerMute struct {
	MuterID   uuid.UUID `json:"muter_id"`
	MutedID   uuid.UUID `json:"muted_id"`
	CreatedAt time.Time `json:"created_at"`
}

type Ban struct {
	ID        uuid.UUID  `json:"id"`
	PlayerID  uuid.UUID  `json:"player_id"`
	BannedBy  uuid.UUID  `json:"banned_by"`
	Reason    *string    `json:"reason,omitempty"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
	LiftedAt  *time.Time `json:"lifted_at,omitempty"`
	LiftedBy  *uuid.UUID `json:"lifted_by,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
}

type ReportStatus string

const (
	ReportStatusPending  ReportStatus = "pending"
	ReportStatusReviewed ReportStatus = "reviewed"
)

type PlayerReport struct {
	ID         uuid.UUID    `json:"id"`
	ReporterID uuid.UUID    `json:"reporter_id"`
	ReportedID uuid.UUID    `json:"reported_id"`
	Reason     string       `json:"reason"`
	Context    []byte       `json:"context"`
	Status     ReportStatus `json:"status"`
	ReviewedBy *uuid.UUID   `json:"reviewed_by,omitempty"`
	ReviewedAt *time.Time   `json:"reviewed_at,omitempty"`
	Resolution *string      `json:"resolution,omitempty"`
	BanID      *uuid.UUID   `json:"ban_id,omitempty"`
	CreatedAt  time.Time    `json:"created_at"`
}

type PlayerProfile struct {
	PlayerID  uuid.UUID `json:"player_id"`
	Bio       *string   `json:"bio,omitempty"`
	Country   *string   `json:"country,omitempty"` // ISO 3166-1 alpha-2
	UpdatedAt time.Time `json:"updated_at"`
}

type PlayerAchievement struct {
	ID             uuid.UUID `json:"id"`
	PlayerID       uuid.UUID `json:"player_id"`
	AchievementKey string    `json:"achievement_key"`
	UnlockedAt     time.Time `json:"unlocked_at"`
}

// --- Params ------------------------------------------------------------------

type IssueBanParams struct {
	PlayerID  uuid.UUID
	BannedBy  uuid.UUID
	Reason    *string
	ExpiresAt *time.Time
}

type CreateReportParams struct {
	ReporterID uuid.UUID
	ReportedID uuid.UUID
	Reason     string
	Context    []byte
}

type ReviewReportParams struct {
	ReportID   uuid.UUID
	ReviewedBy uuid.UUID
	Resolution *string
	BanID      *uuid.UUID
}

type UpsertProfileParams struct {
	PlayerID uuid.UUID
	Bio      *string
	Country  *string
}

// --- Store interface ---------------------------------------------------------

type Store interface {
	// Friendships
	GetFriendship(ctx context.Context, playerA, playerB uuid.UUID) (Friendship, error)
	ListFriends(ctx context.Context, playerID uuid.UUID) ([]FriendshipView, error)
	ListPendingFriendRequests(ctx context.Context, playerID uuid.UUID) ([]FriendshipView, error)
	SendFriendRequest(ctx context.Context, requesterID, addresseeID uuid.UUID) (Friendship, error)
	AcceptFriendRequest(ctx context.Context, requesterID, addresseeID uuid.UUID) (Friendship, error)
	DeclineFriendRequest(ctx context.Context, requesterID, addresseeID uuid.UUID) error
	BlockPlayer(ctx context.Context, requesterID, addresseeID uuid.UUID) (Friendship, error)
	UnblockPlayer(ctx context.Context, requesterID, addresseeID uuid.UUID) error
	RemoveFriend(ctx context.Context, playerA, playerB uuid.UUID) error

	// Mutes
	MutePlayer(ctx context.Context, muterID, mutedID uuid.UUID) error
	UnmutePlayer(ctx context.Context, muterID, mutedID uuid.UUID) error
	GetMutedPlayers(ctx context.Context, playerID uuid.UUID) ([]PlayerMute, error)

	// Bans
	IssueBan(ctx context.Context, params IssueBanParams) (Ban, error)
	GetBan(ctx context.Context, banID uuid.UUID) (Ban, error)
	LiftBan(ctx context.Context, banID, liftedBy uuid.UUID) error
	CheckActiveBan(ctx context.Context, playerID uuid.UUID) (*Ban, error) // nil = not banned
	ListBans(ctx context.Context, playerID uuid.UUID) ([]Ban, error)

	// Reports
	CreateReport(ctx context.Context, params CreateReportParams) (PlayerReport, error)
	ReviewReport(ctx context.Context, params ReviewReportParams) error
	ListPendingReports(ctx context.Context) ([]PlayerReport, error)
	ListReportsByPlayer(ctx context.Context, reportedID uuid.UUID) ([]PlayerReport, error)

	// Profiles
	GetProfile(ctx context.Context, playerID uuid.UUID) (PlayerProfile, error)
	UpsertProfile(ctx context.Context, params UpsertProfileParams) (PlayerProfile, error)

	// Achievements
	UnlockAchievement(ctx context.Context, playerID uuid.UUID, key string) (PlayerAchievement, error)
	ListAchievements(ctx context.Context, playerID uuid.UUID) ([]PlayerAchievement, error)

	// Search
	FindPlayerByUsername(ctx context.Context, username string) (Player, error)

	// Admin — players
	ListPlayers(ctx context.Context) ([]Player, error)
	SetPlayerRole(ctx context.Context, playerID uuid.UUID, role PlayerRole) error

	// Admin — allowed emails
	ListAllowedEmails(ctx context.Context) ([]AllowedEmail, error)
	AddAllowedEmail(ctx context.Context, params AddAllowedEmailParams) (AllowedEmail, error)
	RemoveAllowedEmail(ctx context.Context, email string) error

	// Admin — audit logs
	LogAction(ctx context.Context, actorID uuid.UUID, action, targetType, targetID string, details map[string]any) error
	ListAuditLogs(ctx context.Context, filter AuditFilter) ([]AuditLog, error)

	// Player settings
	GetPlayerSettings(ctx context.Context, playerID uuid.UUID) (PlayerSettings, error)
	UpsertPlayerSettings(ctx context.Context, playerID uuid.UUID, settings PlayerSettingMap) (PlayerSettings, error)
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
