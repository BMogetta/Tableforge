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
	// IsBot is true for fill bots and named bots.
	// Set at insert time via CreateBotPlayer — never mutated after creation.
	IsBot bool `json:"is_bot"`
	// BotProfile labels the bot's difficulty ("easy" | "medium" | "hard" |
	// "aggressive") for UI badges and future param lookups. Nil for humans;
	// a CHECK on the players table enforces that humans never carry one.
	BotProfile *string    `json:"bot_profile,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
	DeletedAt  *time.Time `json:"deleted_at,omitempty"`
}

type PlayerRole string

const (
	RolePlayer  PlayerRole = "player"
	RoleManager PlayerRole = "manager"
	RoleOwner   PlayerRole = "owner"
)

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
	ReadyPlayers    []string    `json:"ready_players"`
	State           []byte      `json:"state"`
	Mode            SessionMode `json:"mode"`
	MoveCount       int         `json:"move_count"`
	SuspendCount    int         `json:"suspend_count"`
	SuspendedAt     *time.Time  `json:"suspended_at,omitempty"`
	SuspendedReason *string     `json:"suspended_reason,omitempty"`
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

type EndedBy string

const (
	EndedByNormal       EndedBy = "win"
	EndedByDraw         EndedBy = "draw"
	EndedByForfeit      EndedBy = "forfeit"
	EndedByTimeout      EndedBy = "timeout"
	EndedByReadyTimeout EndedBy = "ready_timeout"
	EndedBySuspended    EndedBy = "suspended"
)

type GameResult struct {
	ID           uuid.UUID  `json:"id"`
	SessionID    uuid.UUID  `json:"session_id"`
	GameID       string     `json:"game_id"`
	WinnerID     *uuid.UUID `json:"winner_id,omitempty"`
	IsDraw       bool       `json:"is_draw"`
	EndedBy      EndedBy    `json:"ended_by"`
	DurationSecs *int       `json:"duration_secs,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
}

// MatchHistoryEntry is a GameResult enriched with the queried player's outcome.
type MatchHistoryEntry struct {
	ID                 uuid.UUID  `json:"id"`
	SessionID          uuid.UUID  `json:"session_id"`
	GameID             string     `json:"game_id"`
	Outcome            Outcome    `json:"outcome"`
	EndedBy            EndedBy    `json:"ended_by"`
	DurationSecs       *int       `json:"duration_secs,omitempty"`
	CreatedAt          time.Time  `json:"created_at"`
	OpponentID         *uuid.UUID `json:"opponent_id,omitempty"`
	OpponentUsername   *string    `json:"opponent_username,omitempty"`
	OpponentIsBot      *bool      `json:"opponent_is_bot,omitempty"`
	OpponentBotProfile *string    `json:"opponent_bot_profile,omitempty"`
}

type Outcome string

const (
	OutcomeWin     Outcome = "win"
	OutcomeLoss    Outcome = "loss"
	OutcomeDraw    Outcome = "draw"
	OutcomeForgeit Outcome = "forfeit"
)

type GameResultPlayer struct {
	ResultID uuid.UUID `json:"result_id"`
	PlayerID uuid.UUID `json:"player_id"`
	Seat     int       `json:"seat"`
	Outcome  Outcome   `json:"outcome"`
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
	// CreateBotPlayer inserts a player with is_bot = TRUE.
	CreateBotPlayer(ctx context.Context, username string) (Player, error)
	GetPlayer(ctx context.Context, id uuid.UUID) (Player, error)
	GetPlayerByUsername(ctx context.Context, username string) (Player, error)
	UpdatePlayerAvatar(ctx context.Context, id uuid.UUID, avatarURL string) error
	SoftDeletePlayer(ctx context.Context, id uuid.UUID) error

	// Rooms
	CreateRoom(ctx context.Context, params CreateRoomParams) (Room, error)
	GetRoom(ctx context.Context, id uuid.UUID) (Room, error)
	GetRoomByCode(ctx context.Context, code string) (Room, error)
	UpdateRoomStatus(ctx context.Context, id uuid.UUID, status RoomStatus) error
	ListWaitingRooms(ctx context.Context, limit, offset int) ([]Room, error)
	CountWaitingRooms(ctx context.Context) (int, error)
	SoftDeleteRoom(ctx context.Context, id uuid.UUID) error
	UpdateRoomOwner(ctx context.Context, roomID, newOwnerID uuid.UUID) error
	DeleteRoom(ctx context.Context, roomID uuid.UUID) error

	// Room settings
	GetRoomSettings(ctx context.Context, roomID uuid.UUID) (map[string]string, error)
	// SetRoomSetting upserts a single setting for a room.
	// The caller (lobby.validateSetting) is responsible for validating the key and value
	// before calling this method.
	SetRoomSetting(ctx context.Context, roomID uuid.UUID, key, value string) error

	// Room players
	AddPlayerToRoom(ctx context.Context, roomID, playerID uuid.UUID, seat int) error
	RemovePlayerFromRoom(ctx context.Context, roomID, playerID uuid.UUID) error
	ListRoomPlayers(ctx context.Context, roomID uuid.UUID) ([]RoomPlayer, error)
	// IsPlayerInActiveRoom returns true if the player is in any room with
	// status 'waiting' or 'in_progress'. Used to prevent joining multiple rooms.
	IsPlayerInActiveRoom(ctx context.Context, playerID uuid.UUID) (bool, error)

	// Game sessions
	CreateGameSession(ctx context.Context, roomID uuid.UUID, gameID string, initialState []byte, turnTimeoutSecs *int, mode SessionMode) (GameSession, error)
	GetGameSession(ctx context.Context, id uuid.UUID) (GameSession, error)
	GetGameResult(ctx context.Context, sessionID uuid.UUID) (GameResult, error)
	CountPauseVotes(ctx context.Context, sessionID uuid.UUID) (int, error)
	CountResumeVotes(ctx context.Context, sessionID uuid.UUID) (int, error)
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
	ListSessionMoves(ctx context.Context, sessionID uuid.UUID, limit, offset int) ([]Move, error)
	GetMoveAt(ctx context.Context, sessionID uuid.UUID, moveNumber int) (Move, error)

	// Session events
	BulkCreateSessionEvents(ctx context.Context, params []CreateSessionEventParams) error
	ListSessionEvents(ctx context.Context, sessionID uuid.UUID) ([]SessionEvent, error)

	// Results
	CreateGameResult(ctx context.Context, params CreateGameResultParams) (GameResult, error)
	// UpdateGameResultEndedBy corrects the ended_by field on an existing game
	// result. Used by the timeout handler to mark engine-routed timeouts.
	UpdateGameResultEndedBy(ctx context.Context, sessionID uuid.UUID, endedBy EndedBy) error
	GetPlayerStats(ctx context.Context, playerID uuid.UUID) (PlayerStats, error)
	ListPlayerHistory(ctx context.Context, playerID uuid.UUID, limit, offset int) ([]GameResult, error)
	ListPlayerMatches(ctx context.Context, playerID uuid.UUID, limit, offset int) ([]MatchHistoryEntry, error)
	CountPlayerMatches(ctx context.Context, playerID uuid.UUID) (int, error)

	// Rematch
	UpsertRematchVote(ctx context.Context, sessionID, playerID uuid.UUID) error
	ListRematchVotes(ctx context.Context, sessionID uuid.UUID) ([]RematchVote, error)
	DeleteRematchVotes(ctx context.Context, sessionID uuid.UUID) error


	// Pause / resume votes
	VotePause(ctx context.Context, sessionID uuid.UUID, playerID uuid.UUID) (allVoted bool, err error)
	ClearPauseVotes(ctx context.Context, sessionID uuid.UUID) error
	VoteResume(ctx context.Context, sessionID uuid.UUID, playerID uuid.UUID) (allVoted bool, err error)
	ClearResumeVotes(ctx context.Context, sessionID uuid.UUID) error
	ForceCloseSession(ctx context.Context, sessionID uuid.UUID) error

	// Ready handshake
	VoteReady(ctx context.Context, sessionID uuid.UUID, playerID uuid.UUID) (allVoted bool, err error)
	ClearReadyVotes(ctx context.Context, sessionID uuid.UUID) error

	// Admin stats
	CountActiveRooms(ctx context.Context) (int, error)
	CountActiveSessions(ctx context.Context) (int, error)
	CountTotalPlayers(ctx context.Context) (int, error)
	CountSessionsToday(ctx context.Context) (int, error)

	// Cleanup
	// CleanupOrphanRooms deletes waiting rooms with 0 players older than
	// waitingMaxAge and finished rooms older than finishedMaxAge.
	// Returns the number of rooms cleaned up.
	CleanupOrphanRooms(ctx context.Context, waitingMaxAge, finishedMaxAge time.Duration) (int, error)

	// Transactions
	// WithTx executes fn inside a database transaction. The Store passed to fn
	// routes all queries through the transaction. If fn returns an error or
	// panics, the transaction is rolled back; otherwise it is committed.
	WithTx(ctx context.Context, fn func(Store) error) error
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

type CreateGameResultParams struct {
	SessionID uuid.UUID
	GameID    string
	WinnerID  *uuid.UUID
	IsDraw    bool
	EndedBy   EndedBy
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
