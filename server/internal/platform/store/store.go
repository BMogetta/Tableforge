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
	IsBot     bool       `json:"is_bot"`
	CreatedAt time.Time  `json:"created_at"`
	DeletedAt *time.Time `json:"deleted_at,omitempty"`
}

type PlayerRole string

const (
	RolePlayer  PlayerRole = "player"
	RoleManager PlayerRole = "manager"
	RoleOwner   PlayerRole = "owner"
)

// PlayerSettings holds all configurable preferences for a player.
// Stored as a single JSONB row in player_settings.
// The application layer merges stored values over DefaultPlayerSettings() at
// read time, so the schema never needs updating for new keys.
type PlayerSettings struct {
	PlayerID  uuid.UUID        `json:"player_id"`
	Settings  PlayerSettingMap `json:"settings"`
	UpdatedAt time.Time        `json:"updated_at"`
}

// PlayerSettingMap is the flat key/value map serialized into the JSONB column.
// All fields are pointers — nil means "use the application default".
type PlayerSettingMap struct {
	// Appearance
	Theme        *string `json:"theme,omitempty"`         // "dark" | "light" | "system"
	Language     *string `json:"language,omitempty"`      // "en" | "es" | ...
	ReduceMotion *bool   `json:"reduce_motion,omitempty"` // disable CSS animations
	FontSize     *string `json:"font_size,omitempty"`     // "small" | "medium" | "large"

	// Notifications
	NotifyDM            *bool `json:"notify_dm,omitempty"`
	NotifyGameInvite    *bool `json:"notify_game_invite,omitempty"`
	NotifyFriendRequest *bool `json:"notify_friend_request,omitempty"`
	NotifySound         *bool `json:"notify_sound,omitempty"`

	// Audio (stubs — no audio system yet; stored but not applied)
	MuteAll             *bool    `json:"mute_all,omitempty"`
	VolumeMaster        *float64 `json:"volume_master,omitempty"`
	VolumeSFX           *float64 `json:"volume_sfx,omitempty"`
	VolumeUI            *float64 `json:"volume_ui,omitempty"`
	VolumeNotifications *float64 `json:"volume_notifications,omitempty"`
	VolumeMusic         *float64 `json:"volume_music,omitempty"`

	// Gameplay
	ShowMoveHints    *bool `json:"show_move_hints,omitempty"`
	ConfirmMove      *bool `json:"confirm_move,omitempty"`
	ShowTimerWarning *bool `json:"show_timer_warning,omitempty"`

	// Privacy
	ShowOnlineStatus *bool   `json:"show_online_status,omitempty"`
	AllowDMs         *string `json:"allow_dms,omitempty"` // "anyone" | "friends_only" | "nobody"
}

// DefaultPlayerSettings returns the canonical defaults applied when a key is
// absent from the stored settings.
func DefaultPlayerSettings() PlayerSettingMap {
	t := true
	f := false
	dark := "dark"
	en := "en"
	medium := "medium"
	anyone := "anyone"
	vol1 := 1.0

	return PlayerSettingMap{
		Theme:               &dark,
		Language:            &en,
		ReduceMotion:        &f,
		FontSize:            &medium,
		NotifyDM:            &t,
		NotifyGameInvite:    &t,
		NotifyFriendRequest: &t,
		NotifySound:         &t,
		MuteAll:             &f,
		VolumeMaster:        &vol1,
		VolumeSFX:           &vol1,
		VolumeUI:            &vol1,
		VolumeNotifications: &vol1,
		VolumeMusic:         &vol1,
		ShowMoveHints:       &t,
		ConfirmMove:         &f,
		ShowTimerWarning:    &t,
		ShowOnlineStatus:    &t,
		AllowDMs:            &anyone,
	}
}

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

// Rating holds the Elo-based rating for a player.
// Only updated after ranked sessions.
type Rating struct {
	PlayerID      uuid.UUID `json:"player_id"`
	GameID        string    `json:"game_id"`
	MMR           float64   `json:"mmr"`
	DisplayRating float64   `json:"display_rating"`
	GamesPlayed   int       `json:"games_played"`
	WinStreak     int       `json:"win_streak"`
	LossStreak    int       `json:"loss_streak"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// RatingLeaderboardEntry is a leaderboard row with player info joined in.
// MMR is intentionally excluded — it must not be serialized to the frontend.
type RatingLeaderboardEntry struct {
	PlayerID      uuid.UUID `json:"player_id"`
	GameID        string    `json:"game_id"`
	Username      string    `json:"username"`
	AvatarURL     string    `json:"avatar_url,omitempty"`
	DisplayRating float64   `json:"display_rating"`
	GamesPlayed   int       `json:"games_played"`
	WinStreak     int       `json:"win_streak"`
	LossStreak    int       `json:"loss_streak"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// NotificationType identifies the kind of inbox notification.
type NotificationType string

const (
	NotificationTypeFriendRequest  NotificationType = "friend_request"
	NotificationTypeRoomInvitation NotificationType = "room_invitation"
	NotificationTypeBanIssued      NotificationType = "ban_issued"
)

// BanReason identifies why a queue ban was issued.
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
	Payload         []byte           `json:"payload"` // raw JSONB
	ActionTaken     *string          `json:"action_taken,omitempty"`
	ActionExpiresAt *time.Time       `json:"action_expires_at,omitempty"`
	ReadAt          *time.Time       `json:"read_at,omitempty"`
	CreatedAt       time.Time        `json:"created_at"`
}

// NotificationPayloadFriendRequest is the payload for friend_request notifications.
type NotificationPayloadFriendRequest struct {
	FromPlayerID string `json:"from_player_id"`
	FromUsername string `json:"from_username"`
}

// NotificationPayloadRoomInvitation is the payload for room_invitation notifications.
type NotificationPayloadRoomInvitation struct {
	FromPlayerID string `json:"from_player_id"`
	FromUsername string `json:"from_username"`
	RoomID       string `json:"room_id"`
	RoomCode     string `json:"room_code"`
	GameID       string `json:"game_id"`
	GameName     string `json:"game_name"`
}

// NotificationPayloadBanIssued is the payload for ban_issued notifications.
type NotificationPayloadBanIssued struct {
	Reason       BanReason `json:"reason"`
	DurationSecs int       `json:"duration_secs"`
	ExpiresAt    time.Time `json:"expires_at"`
}

// CreateNotificationParams holds the fields needed to create a notification.
type CreateNotificationParams struct {
	PlayerID        uuid.UUID
	Type            NotificationType
	Payload         []byte
	ActionExpiresAt *time.Time
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

	// Player settings
	// GetPlayerSettings returns the stored settings merged over DefaultPlayerSettings().
	// If no row exists for the player, returns a PlayerSettings with all defaults.
	GetPlayerSettings(ctx context.Context, playerID uuid.UUID) (PlayerSettings, error)
	// UpsertPlayerSettings replaces the settings JSONB for the player atomically.
	UpsertPlayerSettings(ctx context.Context, playerID uuid.UUID, settings PlayerSettingMap) (PlayerSettings, error)

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
	ListSessionMoves(ctx context.Context, sessionID uuid.UUID) ([]Move, error)
	GetMoveAt(ctx context.Context, sessionID uuid.UUID, moveNumber int) (Move, error)

	// Session events
	BulkCreateSessionEvents(ctx context.Context, params []CreateSessionEventParams) error
	ListSessionEvents(ctx context.Context, sessionID uuid.UUID) ([]SessionEvent, error)

	// Results
	CreateGameResult(ctx context.Context, params CreateGameResultParams) (GameResult, error)
	GetPlayerStats(ctx context.Context, playerID uuid.UUID) (PlayerStats, error)
	ListPlayerHistory(ctx context.Context, playerID uuid.UUID, limit, offset int) ([]GameResult, error)

	// Rematch
	UpsertRematchVote(ctx context.Context, sessionID, playerID uuid.UUID) error
	ListRematchVotes(ctx context.Context, sessionID uuid.UUID) ([]RematchVote, error)
	DeleteRematchVotes(ctx context.Context, sessionID uuid.UUID) error

	// Ratings
	GetRating(ctx context.Context, playerID uuid.UUID, gameID string) (Rating, error)
	UpsertRating(ctx context.Context, r Rating) error
	GetRatingLeaderboard(ctx context.Context, gameID string, limit int) ([]RatingLeaderboardEntry, error)

	// Pause / resume votes
	VotePause(ctx context.Context, sessionID uuid.UUID, playerID uuid.UUID) (allVoted bool, err error)
	ClearPauseVotes(ctx context.Context, sessionID uuid.UUID) error
	VoteResume(ctx context.Context, sessionID uuid.UUID, playerID uuid.UUID) (allVoted bool, err error)
	ClearResumeVotes(ctx context.Context, sessionID uuid.UUID) error
	ForceCloseSession(ctx context.Context, sessionID uuid.UUID) error

	// Ready handshake
	VoteReady(ctx context.Context, sessionID uuid.UUID, playerID uuid.UUID) (allVoted bool, err error)
	ClearReadyVotes(ctx context.Context, sessionID uuid.UUID) error

	// Notifications
	CreateNotification(ctx context.Context, params CreateNotificationParams) (Notification, error)
	GetNotification(ctx context.Context, id uuid.UUID) (Notification, error)
	// ListNotifications returns notifications for a player.
	// If includeRead is false, only unread notifications are returned.
	// Read notifications older than readCutoff are excluded even if includeRead is true.
	ListNotifications(ctx context.Context, playerID uuid.UUID, includeRead bool, readCutoff time.Time) ([]Notification, error)
	// MarkNotificationRead sets read_at to now for the given notification.
	// No-op if already read.
	MarkNotificationRead(ctx context.Context, id uuid.UUID) error
	// SetNotificationAction records an accepted or declined action on a notification.
	// Returns an error if action_expires_at has passed or action_taken is already set.
	SetNotificationAction(ctx context.Context, id uuid.UUID, action string) error
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
