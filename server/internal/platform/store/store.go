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

// RoomSetting is a single key/value setting for a room.
// Settings are stored in the room_settings table and scoped to a room.
type RoomSetting struct {
	RoomID    uuid.UUID `json:"room_id"`
	Key       string    `json:"key"`
	Value     string    `json:"value"`
	UpdatedAt time.Time `json:"updated_at"`
}

// SessionMode controls whether rating changes apply after the session ends.
type SessionMode string

const (
	SessionModeCasual SessionMode = "casual"
	SessionModeRanked SessionMode = "ranked"
)

type GameSession struct {
	ID              uuid.UUID   `json:"id"`
	RoomID          uuid.UUID   `json:"room_id"`
	GameID          string      `json:"game_id"`
	Name            *string     `json:"name,omitempty"`
	State           []byte      `json:"state"`
	Mode            SessionMode `json:"mode"`
	MoveCount       int         `json:"move_count"`
	SuspendCount    int         `json:"suspend_count"`
	SuspendedAt     *time.Time  `json:"suspended_at,omitempty"`
	SuspendedReason *string     `json:"suspended_reason,omitempty"`
	PauseVotes      []string    `json:"pause_votes"`
	ResumeVotes     []string    `json:"resume_votes"`
	TurnTimeoutSecs *int        `json:"turn_timeout_secs,omitempty"`
	LastMoveAt      time.Time   `json:"last_move_at"`
	StartedAt       time.Time   `json:"started_at"`
	FinishedAt      *time.Time  `json:"finished_at,omitempty"`
	DeletedAt       *time.Time  `json:"deleted_at,omitempty"`
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

type RematchVote struct {
	SessionID uuid.UUID `json:"session_id"`
	PlayerID  uuid.UUID `json:"player_id"`
	VotedAt   time.Time `json:"voted_at"`
}

// RoomMessage is a single chat message in a room.
// Messages are auditable and never deleted; managers may hide them.
type RoomMessage struct {
	ID        uuid.UUID `json:"id"`
	RoomID    uuid.UUID `json:"room_id"`
	PlayerID  uuid.UUID `json:"player_id"`
	Content   string    `json:"content"`
	Reported  bool      `json:"reported"`
	Hidden    bool      `json:"hidden"`
	CreatedAt time.Time `json:"created_at"`
}

// DirectMessage is a private message between two players.
type DirectMessage struct {
	ID         uuid.UUID  `json:"id"`
	SenderID   uuid.UUID  `json:"sender_id"`
	ReceiverID uuid.UUID  `json:"receiver_id"`
	Content    string     `json:"content"`
	ReadAt     *time.Time `json:"read_at,omitempty"`
	Reported   bool       `json:"reported"`
	Hidden     bool       `json:"hidden"`
	CreatedAt  time.Time  `json:"created_at"`
}

// PlayerMute records that MuterID has muted MutedID.
// Mutes are cross-session and server-side — they survive reconnects.
type PlayerMute struct {
	MuterID   uuid.UUID `json:"muter_id"`
	MutedID   uuid.UUID `json:"muted_id"`
	CreatedAt time.Time `json:"created_at"`
}

// Rating holds the Elo-based rating for a player.
// Only updated after ranked sessions.
type Rating struct {
	PlayerID      uuid.UUID `json:"player_id"`
	MMR           float64   `json:"mmr"`
	DisplayRating float64   `json:"display_rating"`
	GamesPlayed   int       `json:"games_played"`
	WinStreak     int       `json:"win_streak"`
	LossStreak    int       `json:"loss_streak"`
	UpdatedAt     time.Time `json:"updated_at"`
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

	// Room settings
	// GetRoomSettings returns all settings for a room as a key/value map.
	GetRoomSettings(ctx context.Context, roomID uuid.UUID) (map[string]string, error)
	// SetRoomSetting upserts a single setting for a room.
	// The caller (lobby.validateSetting) is responsible for validating the key and value
	// before calling this method.
	SetRoomSetting(ctx context.Context, roomID uuid.UUID, key, value string) error

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
	// CountFinishedSessions returns the number of completed sessions for a room.
	// Used by lobby.StartGame to detect rematches and apply rematch_first_mover_policy.
	CountFinishedSessions(ctx context.Context, roomID uuid.UUID) (int, error)
	// GetLastFinishedSession returns the most recently finished session for a room.
	// Returns an error if no finished session exists.
	GetLastFinishedSession(ctx context.Context, roomID uuid.UUID) (GameSession, error)
	// ListSessionsNeedingTimer returns active, non-suspended sessions with a
	// turn timeout configured. Used by TurnTimer.ReschedulePending on startup.
	ListSessionsNeedingTimer(ctx context.Context) ([]GameSession, error)

	// Moves
	RecordMove(ctx context.Context, params RecordMoveParams) (Move, error)
	ListSessionMoves(ctx context.Context, sessionID uuid.UUID) ([]Move, error)
	GetMoveAt(ctx context.Context, sessionID uuid.UUID, moveNumber int) (Move, error)

	// Session events
	BulkCreateSessionEvents(ctx context.Context, params []CreateSessionEventParams) error
	ListSessionEvents(ctx context.Context, sessionID uuid.UUID) ([]SessionEvent, error)

	// OAuth
	UpsertOAuthIdentity(ctx context.Context, params UpsertOAuthParams) (OAuthIdentity, error)
	GetOAuthIdentity(ctx context.Context, provider, providerID string) (OAuthIdentity, error)
	GetOAuthIdentityByEmail(ctx context.Context, email string) (OAuthIdentity, error)
	IsEmailAllowed(ctx context.Context, email string) (bool, error)

	// Results
	CreateGameResult(ctx context.Context, params CreateGameResultParams) (GameResult, error)
	GetPlayerStats(ctx context.Context, playerID uuid.UUID) (PlayerStats, error)
	ListPlayerHistory(ctx context.Context, playerID uuid.UUID, limit, offset int) ([]GameResult, error)

	// Rematch
	UpsertRematchVote(ctx context.Context, sessionID, playerID uuid.UUID) error
	ListRematchVotes(ctx context.Context, sessionID uuid.UUID) ([]RematchVote, error)
	DeleteRematchVotes(ctx context.Context, sessionID uuid.UUID) error

	// Room chat
	SaveRoomMessage(ctx context.Context, roomID, playerID uuid.UUID, content string) (RoomMessage, error)
	GetRoomMessages(ctx context.Context, roomID uuid.UUID) ([]RoomMessage, error)
	HideRoomMessage(ctx context.Context, messageID uuid.UUID) error
	ReportRoomMessage(ctx context.Context, messageID uuid.UUID) error

	// Direct messages
	SaveDM(ctx context.Context, senderID, receiverID uuid.UUID, content string) (DirectMessage, error)
	GetDMHistory(ctx context.Context, playerA, playerB uuid.UUID) ([]DirectMessage, error)
	MarkDMRead(ctx context.Context, messageID uuid.UUID) error
	GetUnreadDMCount(ctx context.Context, playerID uuid.UUID) (int, error)
	ReportDM(ctx context.Context, messageID uuid.UUID) error

	// Mutes
	MutePlayer(ctx context.Context, muterID, mutedID uuid.UUID) error
	UnmutePlayer(ctx context.Context, muterID, mutedID uuid.UUID) error
	GetMutedPlayers(ctx context.Context, playerID uuid.UUID) ([]PlayerMute, error)

	// Ratings
	GetRating(ctx context.Context, playerID uuid.UUID) (Rating, error)
	UpsertRating(ctx context.Context, r Rating) error
	GetRatingLeaderboard(ctx context.Context, limit int) ([]Rating, error)

	// Pause / resume votes
	VotePause(ctx context.Context, sessionID uuid.UUID, playerID uuid.UUID) (allVoted bool, err error)
	ClearPauseVotes(ctx context.Context, sessionID uuid.UUID) error
	VoteResume(ctx context.Context, sessionID uuid.UUID, playerID uuid.UUID) (allVoted bool, err error)
	ClearResumeVotes(ctx context.Context, sessionID uuid.UUID) error
	ForceCloseSession(ctx context.Context, sessionID uuid.UUID) error
}

// --- Params ------------------------------------------------------------------

type CreateRoomParams struct {
	Code            string
	GameID          string
	OwnerID         uuid.UUID
	MaxPlayers      int
	TurnTimeoutSecs *int
	// DefaultSettings is a map of key/value pairs to insert into room_settings
	// when the room is created. Populated by lobby.Service from engine.DefaultLobbySettings()
	// merged with any game-specific settings from engine.LobbySettingsProvider.
	DefaultSettings map[string]string
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
