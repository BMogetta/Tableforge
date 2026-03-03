package store

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// --- Models ------------------------------------------------------------------

type Player struct {
	ID        uuid.UUID  `json:"id"`
	Username  string     `json:"username"`
	Role      PlayerRole `json:"role"`
	AvatarURL *string    `json:"avatar_url,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
	DeletedAt *time.Time `json:"deleted_at,omitempty"`
}

type PlayerRole string

const (
	RolePlayer  PlayerRole = "player"
	RoleManager PlayerRole = "manager"
	RoleOwner   PlayerRole = "owner"
)

type AddAllowedEmailParams struct {
	Email     string
	Role      PlayerRole
	InvitedBy *uuid.UUID
}

type RoomStatus string

const (
	RoomStatusWaiting    RoomStatus = "waiting"
	RoomStatusInProgress RoomStatus = "in_progress"
	RoomStatusFinished   RoomStatus = "finished"
)

type Room struct {
	ID              uuid.UUID  `json:"id"`
	Code            string     `json:"code"`
	GameID          string     `json:"game_id"`
	OwnerID         uuid.UUID  `json:"owner_id"`
	Status          RoomStatus `json:"status"`
	MaxPlayers      int        `json:"max_players"`
	TurnTimeoutSecs *int       `json:"turn_timeout_secs,omitempty"`
	AllowSpectators bool       `json:"allow_spectators"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
	DeletedAt       *time.Time `json:"deleted_at,omitempty"`
}

type RoomPlayer struct {
	RoomID   uuid.UUID `json:"room_id"`
	PlayerID uuid.UUID `json:"player_id"`
	Seat     int       `json:"seat"`
	JoinedAt time.Time `json:"joined_at"`
}

type GameSession struct {
	ID              uuid.UUID  `json:"id"`
	RoomID          uuid.UUID  `json:"room_id"`
	GameID          string     `json:"game_id"`
	Name            *string    `json:"name,omitempty"`
	State           []byte     `json:"state"`
	MoveCount       int        `json:"move_count"`
	SuspendCount    int        `json:"suspend_count"`
	SuspendedAt     *time.Time `json:"suspended_at,omitempty"`
	SuspendedReason *string    `json:"suspended_reason,omitempty"`
	TurnTimeoutSecs *int       `json:"turn_timeout_secs,omitempty"`
	LastMoveAt      time.Time  `json:"last_move_at"`
	StartedAt       time.Time  `json:"started_at"`
	FinishedAt      *time.Time `json:"finished_at,omitempty"`
	DeletedAt       *time.Time `json:"deleted_at,omitempty"`
}
type Move struct {
	ID         uuid.UUID `json:"id"`
	SessionID  uuid.UUID `json:"session_id"`
	PlayerID   uuid.UUID `json:"player_id"`
	Payload    []byte    `json:"payload"`
	StateAfter []byte    `json:"state_after,omitempty"`
	MoveNumber int       `json:"move_number"`
	AppliedAt  time.Time `json:"applied_at"`
}

// TimeoutPenalty defines what happens when a player's turn expires.
type TimeoutPenalty string

const (
	PenaltyLoseTurn TimeoutPenalty = "lose_turn"
	PenaltyLoseGame TimeoutPenalty = "lose_game"
)

// GameConfig holds the configurable defaults for a specific game.
type GameConfig struct {
	GameID             string         `json:"game_id"`
	DefaultTimeoutSecs int            `json:"default_timeout_secs"`
	MinTimeoutSecs     int            `json:"min_timeout_secs"`
	MaxTimeoutSecs     int            `json:"max_timeout_secs"`
	TimeoutPenalty     TimeoutPenalty `json:"timeout_penalty"`
}

type OAuthIdentity struct {
	ID         uuid.UUID `json:"id"`
	PlayerID   uuid.UUID `json:"player_id"`
	Provider   string    `json:"provider"`
	ProviderID string    `json:"provider_id"`
	Email      string    `json:"email"`
	AvatarURL  *string   `json:"avatar_url,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
}

type AllowedEmail struct {
	Email     string     `json:"email"`
	Role      PlayerRole `json:"role"`
	Note      *string    `json:"note,omitempty"`
	InvitedBy *uuid.UUID `json:"invited_by,omitempty"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
}

type GameResult struct {
	ID           uuid.UUID  `json:"id"`
	SessionID    uuid.UUID  `json:"session_id"`
	GameID       string     `json:"game_id"`
	WinnerID     *uuid.UUID `json:"winner_id,omitempty"`
	IsDraw       bool       `json:"is_draw"`
	EndedBy      string     `json:"ended_by"`
	DurationSecs *int       `json:"duration_secs,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
}

type GameResultPlayer struct {
	ResultID uuid.UUID `json:"result_id"`
	PlayerID uuid.UUID `json:"player_id"`
	Seat     int       `json:"seat"`
	Outcome  string    `json:"outcome"`
}

type SpectatorLink struct {
	Token     string    `json:"token"`
	RoomID    uuid.UUID `json:"room_id"`
	CreatedBy uuid.UUID `json:"created_by"`
	CreatedAt time.Time `json:"created_at"`
}

type RematchVote struct {
	SessionID uuid.UUID `json:"session_id"`
	PlayerID  uuid.UUID `json:"player_id"`
	VotedAt   time.Time `json:"voted_at"`
}

// --- Store interface ---------------------------------------------------------

// Store is the interface for all database operations.
// Both the real pgx implementation and the fake test implementation satisfy this.
type Store interface {
	// Players
	CreatePlayer(ctx context.Context, username string) (Player, error)
	GetPlayer(ctx context.Context, id uuid.UUID) (Player, error)
	GetPlayerByUsername(ctx context.Context, username string) (Player, error)
	UpdatePlayerAvatar(ctx context.Context, id uuid.UUID, avatarURL string) error
	SoftDeletePlayer(ctx context.Context, id uuid.UUID) error

	// Admin — allowed emails
	ListAllowedEmails(ctx context.Context) ([]AllowedEmail, error)
	AddAllowedEmail(ctx context.Context, params AddAllowedEmailParams) (AllowedEmail, error)
	RemoveAllowedEmail(ctx context.Context, email string) error
	// Admin — players
	ListPlayers(ctx context.Context) ([]Player, error)
	SetPlayerRole(ctx context.Context, playerID uuid.UUID, role PlayerRole) error

	// Rooms
	CreateRoom(ctx context.Context, params CreateRoomParams) (Room, error)
	GetRoom(ctx context.Context, id uuid.UUID) (Room, error)
	GetRoomByCode(ctx context.Context, code string) (Room, error)
	UpdateRoomStatus(ctx context.Context, id uuid.UUID, status RoomStatus) error
	ListWaitingRooms(ctx context.Context) ([]Room, error)
	SoftDeleteRoom(ctx context.Context, id uuid.UUID) error
	UpdateRoomOwner(ctx context.Context, roomID, newOwnerID uuid.UUID) error
	DeleteRoom(ctx context.Context, roomID uuid.UUID) error

	// Room players
	AddPlayerToRoom(ctx context.Context, roomID, playerID uuid.UUID, seat int) error
	RemovePlayerFromRoom(ctx context.Context, roomID, playerID uuid.UUID) error
	ListRoomPlayers(ctx context.Context, roomID uuid.UUID) ([]RoomPlayer, error)

	// Game sessions
	CreateGameSession(ctx context.Context, roomID uuid.UUID, gameID string, initialState []byte, turnTimeoutSecs *int) (GameSession, error)
	GetGameSession(ctx context.Context, id uuid.UUID) (GameSession, error)
	GetGameResult(ctx context.Context, sessionID uuid.UUID) (GameResult, error)
	GetActiveSessionByRoom(ctx context.Context, roomID uuid.UUID) (GameSession, error)
	UpdateSessionState(ctx context.Context, id uuid.UUID, state []byte) error
	FinishSession(ctx context.Context, id uuid.UUID) error
	SuspendSession(ctx context.Context, id uuid.UUID, reason string) error
	ResumeSession(ctx context.Context, id uuid.UUID) error
	ListActiveSessions(ctx context.Context, playerID uuid.UUID) ([]GameSession, error)
	SoftDeleteSession(ctx context.Context, id uuid.UUID) error
	GetGameConfig(ctx context.Context, gameID string) (GameConfig, error)
	TouchLastMoveAt(ctx context.Context, sessionID uuid.UUID) error

	// Moves
	RecordMove(ctx context.Context, params RecordMoveParams) (Move, error)
	ListSessionMoves(ctx context.Context, sessionID uuid.UUID) ([]Move, error)
	GetMoveAt(ctx context.Context, sessionID uuid.UUID, moveNumber int) (Move, error)

	// OAuth
	UpsertOAuthIdentity(ctx context.Context, params UpsertOAuthParams) (OAuthIdentity, error)
	GetOAuthIdentity(ctx context.Context, provider, providerID string) (OAuthIdentity, error)
	GetOAuthIdentityByEmail(ctx context.Context, email string) (OAuthIdentity, error)
	IsEmailAllowed(ctx context.Context, email string) (bool, error)

	// Results & leaderboard
	CreateGameResult(ctx context.Context, params CreateGameResultParams) (GameResult, error)
	GetPlayerStats(ctx context.Context, playerID uuid.UUID) (PlayerStats, error)
	GetLeaderboard(ctx context.Context, gameID string, limit int) ([]LeaderboardEntry, error)
	ListPlayerHistory(ctx context.Context, playerID uuid.UUID, limit, offset int) ([]GameResult, error)

	// Spectators
	CreateSpectatorLink(ctx context.Context, roomID, createdBy uuid.UUID) (SpectatorLink, error)
	GetSpectatorLink(ctx context.Context, token string) (SpectatorLink, error)

	// Rematch
	UpsertRematchVote(ctx context.Context, sessionID, playerID uuid.UUID) error
	ListRematchVotes(ctx context.Context, sessionID uuid.UUID) ([]RematchVote, error)
	DeleteRematchVotes(ctx context.Context, sessionID uuid.UUID) error
}

// --- Params ------------------------------------------------------------------

type CreateRoomParams struct {
	Code            string
	GameID          string
	OwnerID         uuid.UUID
	MaxPlayers      int
	TurnTimeoutSecs *int
	AllowSpectators bool
}

type RecordMoveParams struct {
	SessionID  uuid.UUID
	PlayerID   uuid.UUID
	Payload    []byte
	StateAfter []byte
	MoveNumber int
}

type UpsertOAuthParams struct {
	Provider   string
	ProviderID string
	Email      string
	AvatarURL  *string
	Username   string
}

type CreateGameResultParams struct {
	SessionID uuid.UUID
	GameID    string
	WinnerID  *uuid.UUID
	IsDraw    bool
	EndedBy   string
	Players   []GameResultPlayer
}

// --- Query results -----------------------------------------------------------

type PlayerStats struct {
	PlayerID   uuid.UUID `json:"player_id"`
	TotalGames int       `json:"total_games"`
	Wins       int       `json:"wins"`
	Losses     int       `json:"losses"`
	Draws      int       `json:"draws"`
	Forfeits   int       `json:"forfeits"`
}

type LeaderboardEntry struct {
	PlayerID  uuid.UUID `json:"player_id"`
	Username  string    `json:"username"`
	AvatarURL *string   `json:"avatar_url,omitempty"`
	Wins      int       `json:"wins"`
	Losses    int       `json:"losses"`
	Draws     int       `json:"draws"`
}
