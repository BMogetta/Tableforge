package lobby

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/recess/game-server/internal/domain/engine"
	"github.com/recess/game-server/internal/platform/store"
	"github.com/recess/shared/platform/randutil"
)

var (
	ErrRoomNotFound      = errors.New("room not found")
	ErrRoomFull          = errors.New("room is full")
	ErrRoomNotWaiting    = errors.New("room is not in waiting state")
	ErrNotEnoughPlayer   = errors.New("not enough players to start")
	ErrNotOwner          = errors.New("only the room owner can start the game")
	ErrAlreadyInRoom     = errors.New("player is already in the room")
	ErrGameNotFound      = errors.New("game not found")
	ErrUnknownSetting    = errors.New("unknown setting key")
	ErrInvalidSettingVal = errors.New("invalid setting value")
)

// RoomViewPlayer combines full player data with room-specific metadata.
type RoomViewPlayer struct {
	store.Player
	Seat     int       `json:"seat"`
	JoinedAt time.Time `json:"joined_at"`
}

// RoomView is the full state of a room as seen by clients.
type RoomView struct {
	Room     store.Room        `json:"room"`
	Players  []RoomViewPlayer  `json:"players"`
	Settings map[string]string `json:"settings"`
}

// GameRegistry is a read-only view of the game plugin registry.
type GameRegistry interface {
	Get(id string) (engine.Game, error)
}

// Service contains all lobby operations.
type Service struct {
	store    store.Store
	registry GameRegistry
}

// New creates a new lobby Service.
func New(s store.Store, registry GameRegistry) *Service {
	return &Service{store: s, registry: registry}
}

// CreateRoom creates a new room and adds the owner as the first player.
// TODO: CreateRoom and JoinRoom should reject human players who are already
// in another waiting room or have an active (unfinished) game session.
// Currently only the frontend blocks this via ActiveGameBanner; the backend
// does not enforce it. Enforcement requires a store query like
// "SELECT 1 FROM room_players rp JOIN rooms r ON r.id = rp.room_id
//  WHERE rp.player_id = $1 AND r.status IN ('waiting','in_progress')
//  AND r.deleted_at IS NULL" or checking ListActiveSessions.

func (svc *Service) CreateRoom(ctx context.Context, gameID string, ownerID uuid.UUID, turnTimeoutSecs *int) (RoomView, error) {
	game, err := svc.registry.Get(gameID)
	if err != nil {
		return RoomView{}, ErrGameNotFound
	}

	code, err := generateRoomCode()
	if err != nil {
		return RoomView{}, fmt.Errorf("CreateRoom: generate code: %w", err)
	}

	room, err := svc.store.CreateRoom(ctx, store.CreateRoomParams{
		Code:            code,
		GameID:          gameID,
		OwnerID:         ownerID,
		MaxPlayers:      game.MaxPlayers(),
		TurnTimeoutSecs: turnTimeoutSecs,
		DefaultSettings: buildDefaultSettings(game),
	})
	if err != nil {
		return RoomView{}, fmt.Errorf("CreateRoom: %w", err)
	}

	if err := svc.store.AddPlayerToRoom(ctx, room.ID, ownerID, 0); err != nil {
		return RoomView{}, fmt.Errorf("CreateRoom: add owner: %w", err)
	}

	return svc.getRoomView(ctx, room)
}

// JoinRoom adds a player to an existing room by join code.
func (svc *Service) JoinRoom(ctx context.Context, code string, playerID uuid.UUID) (RoomView, error) {
	room, err := svc.store.GetRoomByCode(ctx, strings.ToUpper(code))
	if err != nil {
		return RoomView{}, ErrRoomNotFound
	}

	if room.Status != store.RoomStatusWaiting {
		return RoomView{}, ErrRoomNotWaiting
	}

	players, err := svc.store.ListRoomPlayers(ctx, room.ID)
	if err != nil {
		return RoomView{}, fmt.Errorf("JoinRoom: list players: %w", err)
	}

	if len(players) >= room.MaxPlayers {
		return RoomView{}, ErrRoomFull
	}

	for _, p := range players {
		if p.PlayerID == playerID {
			return RoomView{}, ErrAlreadyInRoom
		}
	}

	seat := len(players)
	if err := svc.store.AddPlayerToRoom(ctx, room.ID, playerID, seat); err != nil {
		return RoomView{}, fmt.Errorf("JoinRoom: add player: %w", err)
	}

	return svc.getRoomView(ctx, room)
}

// LeaveResult is returned by LeaveRoom.
type LeaveResult struct {
	NewOwnerID *uuid.UUID
	RoomClosed bool
}

// LeaveRoom removes a player from a room and handles ownership transfer or closure.
func (svc *Service) LeaveRoom(ctx context.Context, roomID, playerID uuid.UUID) (LeaveResult, error) {
	room, err := svc.store.GetRoom(ctx, roomID)
	if err != nil {
		return LeaveResult{}, ErrRoomNotFound
	}

	if room.Status != store.RoomStatusWaiting {
		return LeaveResult{}, ErrRoomNotWaiting
	}

	if err := svc.store.RemovePlayerFromRoom(ctx, roomID, playerID); err != nil {
		return LeaveResult{}, fmt.Errorf("LeaveRoom: remove player: %w", err)
	}

	remaining, err := svc.store.ListRoomPlayers(ctx, roomID)
	if err != nil {
		return LeaveResult{}, fmt.Errorf("LeaveRoom: list remaining: %w", err)
	}

	// No players left — close the room.
	if len(remaining) == 0 {
		if err := svc.store.DeleteRoom(ctx, roomID); err != nil {
			return LeaveResult{}, fmt.Errorf("LeaveRoom: delete room: %w", err)
		}
		return LeaveResult{RoomClosed: true}, nil
	}

	// Owner left — transfer ownership to the player with the earliest joined_at.
	if room.OwnerID == playerID {
		// ListRoomPlayers returns players ordered by joined_at ASC.
		newOwner := remaining[0]
		if err := svc.store.UpdateRoomOwner(ctx, roomID, newOwner.PlayerID); err != nil {
			return LeaveResult{}, fmt.Errorf("LeaveRoom: transfer ownership: %w", err)
		}
		return LeaveResult{NewOwnerID: &newOwner.PlayerID}, nil
	}

	return LeaveResult{}, nil
}

// UpdateRoomSetting updates a single setting for a room.
// Only the room owner can update settings, and only while the room is waiting.
func (svc *Service) UpdateRoomSetting(ctx context.Context, roomID, requesterID uuid.UUID, key, value string) error {
	room, err := svc.store.GetRoom(ctx, roomID)
	if err != nil {
		return ErrRoomNotFound
	}
	if room.Status != store.RoomStatusWaiting {
		return ErrRoomNotWaiting
	}
	if room.OwnerID != requesterID {
		return ErrNotOwner
	}

	game, err := svc.registry.Get(room.GameID)
	if err != nil {
		return ErrGameNotFound
	}

	if err := validateSetting(key, value, game); err != nil {
		return err
	}

	return svc.store.SetRoomSetting(ctx, roomID, key, value)
}

// StartGame transitions the room to in_progress and creates a game session.
// Only the room owner can start, and the minimum player count must be met.
// The first mover is resolved from the room's settings. For rematches (rooms
// with at least one finished session), rematch_first_mover_policy is used
// instead of first_mover_policy.
func (svc *Service) StartGame(ctx context.Context, roomID, requesterID uuid.UUID, mode store.SessionMode) (store.GameSession, error) {
	room, err := svc.store.GetRoom(ctx, roomID)
	if err != nil {
		return store.GameSession{}, ErrRoomNotFound
	}
	if room.Status != store.RoomStatusWaiting {
		return store.GameSession{}, ErrRoomNotWaiting
	}
	if room.OwnerID != requesterID {
		return store.GameSession{}, ErrNotOwner
	}

	roomPlayers, err := svc.store.ListRoomPlayers(ctx, roomID)
	if err != nil {
		return store.GameSession{}, fmt.Errorf("StartGame: list players: %w", err)
	}

	game, err := svc.registry.Get(room.GameID)
	if err != nil {
		return store.GameSession{}, ErrGameNotFound
	}

	if len(roomPlayers) < game.MinPlayers() {
		return store.GameSession{}, ErrNotEnoughPlayer
	}

	settings, err := svc.store.GetRoomSettings(ctx, roomID)
	if err != nil {
		return store.GameSession{}, fmt.Errorf("StartGame: get settings: %w", err)
	}

	effectiveTimeout, err := svc.resolveTimeout(ctx, room)
	if err != nil {
		return store.GameSession{}, fmt.Errorf("StartGame: resolve timeout: %w", err)
	}

	// Build the engine player slice ordered by seat.
	enginePlayers := make([]engine.Player, len(roomPlayers))
	for _, rp := range roomPlayers {
		p, err := svc.store.GetPlayer(ctx, rp.PlayerID)
		if err != nil {
			return store.GameSession{}, fmt.Errorf("StartGame: get player: %w", err)
		}
		enginePlayers[rp.Seat] = engine.Player{
			ID:       engine.PlayerID(p.ID.String()),
			Username: p.Username,
		}
	}

	// Detect whether this is a rematch (at least one finished session exists).
	// If so, use rematch_first_mover_policy instead of first_mover_policy.
	finishedCount, err := svc.store.CountFinishedSessions(ctx, roomID)
	if err != nil {
		return store.GameSession{}, fmt.Errorf("StartGame: count finished sessions: %w", err)
	}

	var firstSeat int
	if finishedCount > 0 {
		// Rematch — resolve using rematch_first_mover_policy.
		lastSession, err := svc.store.GetLastFinishedSession(ctx, roomID)
		if err != nil {
			return store.GameSession{}, fmt.Errorf("StartGame: get last session: %w", err)
		}
		lastResult, err := svc.store.GetGameResult(ctx, lastSession.ID)
		if err != nil {
			return store.GameSession{}, fmt.Errorf("StartGame: get last result: %w", err)
		}
		firstSeat, err = resolveRematchFirstMover(
			settings["rematch_first_mover_policy"],
			settings["first_mover_seat"],
			enginePlayers,
			roomPlayers,
			lastResult,
		)
		if err != nil {
			return store.GameSession{}, fmt.Errorf("StartGame: resolve rematch first mover: %w", err)
		}
	} else {
		// First game — resolve using first_mover_policy.
		firstSeat, err = resolveFirstMover(
			settings["first_mover_policy"],
			settings["first_mover_seat"],
			enginePlayers,
		)
		if err != nil {
			return store.GameSession{}, fmt.Errorf("StartGame: resolve first mover: %w", err)
		}
	}

	enginePlayers = rotatePlayers(enginePlayers, firstSeat)

	initialState, err := game.Init(enginePlayers)
	if err != nil {
		return store.GameSession{}, fmt.Errorf("StartGame: init game: %w", err)
	}

	stateJSON, err := json.Marshal(initialState)
	if err != nil {
		return store.GameSession{}, fmt.Errorf("StartGame: marshal state: %w", err)
	}

	session, err := svc.store.CreateGameSession(ctx, roomID, room.GameID, stateJSON, effectiveTimeout, mode)
	if err != nil {
		return store.GameSession{}, fmt.Errorf("StartGame: create session: %w", err)
	}

	if err := svc.store.UpdateRoomStatus(ctx, roomID, store.RoomStatusInProgress); err != nil {
		return store.GameSession{}, fmt.Errorf("StartGame: update room status: %w", err)
	}

	return session, nil
}

// resolveTimeout returns the effective turn timeout for a session.
// If the room has a timeout set, it is clamped to the game's min/max.
// If the room has no timeout, the game's default is used.
func (svc *Service) resolveTimeout(ctx context.Context, room store.Room) (*int, error) {
	cfg, err := svc.store.GetGameConfig(ctx, room.GameID)
	if err != nil {
		return nil, err
	}

	var secs int
	if room.TurnTimeoutSecs != nil {
		secs = clamp(*room.TurnTimeoutSecs, cfg.MinTimeoutSecs, cfg.MaxTimeoutSecs)
	} else {
		secs = cfg.DefaultTimeoutSecs
	}

	return &secs, nil
}

// GetRoom returns the full view of a room by ID.
// The room code is always included — this endpoint is used by room participants
// who already know which room they're in.
func (svc *Service) GetRoom(ctx context.Context, roomID uuid.UUID) (RoomView, error) {
	room, err := svc.store.GetRoom(ctx, roomID)
	if err != nil {
		return RoomView{}, ErrRoomNotFound
	}
	return svc.getRoomView(ctx, room)
}

// ListWaitingRooms returns all rooms currently open for players to join.
// Private rooms (room_visibility = "private") are included in the list but
// have their Code field redacted — the frontend renders them with a lock icon
// and no direct-join button, requiring the player to know the code.
func (svc *Service) ListWaitingRooms(ctx context.Context) ([]RoomView, error) {
	rooms, err := svc.store.ListWaitingRooms(ctx)
	if err != nil {
		return nil, fmt.Errorf("ListWaitingRooms: %w", err)
	}

	views := make([]RoomView, 0, len(rooms))
	for _, r := range rooms {
		v, err := svc.getRoomView(ctx, r)
		if err != nil {
			return nil, err
		}
		// Redact the join code for private rooms so it never reaches
		// clients who haven't been given the code out-of-band.
		if v.Settings["room_visibility"] == "private" {
			v.Room.Code = ""
		}
		views = append(views, v)
	}
	return views, nil
}

func (svc *Service) getRoomView(ctx context.Context, room store.Room) (RoomView, error) {
	roomPlayers, err := svc.store.ListRoomPlayers(ctx, room.ID)
	if err != nil {
		return RoomView{}, fmt.Errorf("getRoomView: list players: %w", err)
	}

	players := make([]RoomViewPlayer, 0, len(roomPlayers))
	for _, rp := range roomPlayers {
		p, err := svc.store.GetPlayer(ctx, rp.PlayerID)
		if err != nil {
			return RoomView{}, fmt.Errorf("getRoomView: get player %s: %w", rp.PlayerID, err)
		}
		players = append(players, RoomViewPlayer{
			Player:   p,
			Seat:     rp.Seat,
			JoinedAt: rp.JoinedAt,
		})
	}

	settings, err := svc.store.GetRoomSettings(ctx, room.ID)
	if err != nil {
		return RoomView{}, fmt.Errorf("getRoomView: get settings: %w", err)
	}

	return RoomView{Room: room, Players: players, Settings: settings}, nil
}

// --- Settings ----------------------------------------------------------------

// buildDefaultSettings merges the platform-level default settings with any
// game-specific settings declared by the game via LobbySettingsProvider.
// Game-specific settings take precedence on key collisions.
func buildDefaultSettings(game engine.Game) map[string]string {
	settings := engine.DefaultLobbySettings()

	if provider, ok := game.(engine.LobbySettingsProvider); ok {
		for _, s := range provider.LobbySettings() {
			settings = upsertSetting(settings, s)
		}
	}

	result := make(map[string]string, len(settings))
	for _, s := range settings {
		result[s.Key] = s.Default
	}
	return result
}

// upsertSetting replaces an existing LobbySetting with the same key, or appends it.
func upsertSetting(settings []engine.LobbySetting, s engine.LobbySetting) []engine.LobbySetting {
	for i, existing := range settings {
		if existing.Key == s.Key {
			settings[i] = s
			return settings
		}
	}
	return append(settings, s)
}

// lobbySettingsForGame returns the full merged list of LobbySetting descriptors
// for a game. Used by validateSetting to look up allowed values and types.
func lobbySettingsForGame(game engine.Game) []engine.LobbySetting {
	settings := engine.DefaultLobbySettings()
	if provider, ok := game.(engine.LobbySettingsProvider); ok {
		for _, s := range provider.LobbySettings() {
			settings = upsertSetting(settings, s)
		}
	}
	return settings
}

// validateSetting checks that key is known and value is valid for the given game.
// For first_mover_seat it also validates the upper bound against game.MaxPlayers()-1,
// since the descriptor intentionally omits Max — see engine/lobby_settings.go.
func validateSetting(key, value string, game engine.Game) error {
	settings := lobbySettingsForGame(game)

	for _, s := range settings {
		if s.Key != key {
			continue
		}

		switch s.Type {
		case engine.SettingTypeSelect:
			for _, opt := range s.Options {
				if opt.Value == value {
					return nil
				}
			}
			allowed := make([]string, len(s.Options))
			for i, o := range s.Options {
				allowed[i] = o.Value
			}
			return fmt.Errorf("%w: %q must be one of [%s]", ErrInvalidSettingVal, key, strings.Join(allowed, ", "))

		case engine.SettingTypeInt:
			n, err := strconv.Atoi(value)
			if err != nil {
				return fmt.Errorf("%w: %q must be an integer", ErrInvalidSettingVal, key)
			}
			if s.Min != nil && n < *s.Min {
				return fmt.Errorf("%w: %q must be >= %d", ErrInvalidSettingVal, key, *s.Min)
			}
			if s.Max != nil && n > *s.Max {
				return fmt.Errorf("%w: %q must be <= %d", ErrInvalidSettingVal, key, *s.Max)
			}
			// Special case: first_mover_seat upper bound is game.MaxPlayers()-1.
			if key == "first_mover_seat" {
				maxSeat := game.MaxPlayers() - 1
				if n > maxSeat {
					return fmt.Errorf("%w: %q must be <= %d for this game", ErrInvalidSettingVal, key, maxSeat)
				}
			}
			return nil
		}
	}

	return fmt.Errorf("%w: %q", ErrUnknownSetting, key)
}

// --- First mover resolution --------------------------------------------------

// resolveFirstMover returns the seat index of the player who should go first.
// Handles policies for the first game: random, fixed, game_default.
// Rematch policies are handled by resolveRematchFirstMover.
func resolveFirstMover(policy, fixedSeatStr string, players []engine.Player) (int, error) {
	switch policy {
	case "fixed":
		seat, err := strconv.Atoi(fixedSeatStr)
		if err != nil || seat < 0 || seat >= len(players) {
			// Malformed stored value — fall back to seat 0.
			return 0, nil
		}
		return seat, nil

	case "game_default":
		// The engine's Init already assigns who goes first; return 0 so the
		// player slice is not rotated.
		return 0, nil

	case "random":
		fallthrough
	default:
		return randutil.Intn(len(players))
	}
}

// resolveRematchFirstMover returns the seat index for a rematch game.
// Handles all rematch policies: random, fixed, game_default, winner_first, loser_first.
// winner_chooses and loser_chooses are not yet implemented and fall back to random.
func resolveRematchFirstMover(
	policy string,
	fixedSeatStr string,
	enginePlayers []engine.Player,
	roomPlayers []store.RoomPlayer,
	lastResult store.GameResult,
) (int, error) {
	switch policy {
	case "winner_first", "loser_first":
		if lastResult.WinnerID == nil {
			return randutil.Intn(len(enginePlayers))
		}
		for _, rp := range roomPlayers {
			isWinner := rp.PlayerID == *lastResult.WinnerID
			if (policy == "winner_first" && isWinner) || (policy == "loser_first" && !isWinner) {
				return rp.Seat, nil
			}
		}
		return randutil.Intn(len(enginePlayers))

	case "winner_chooses", "loser_chooses":
		return randutil.Intn(len(enginePlayers))

	default:
		return resolveFirstMover(policy, fixedSeatStr, enginePlayers)
	}
}

// rotatePlayers returns a new slice with players rotated so that the player
// at firstSeat ends up at index 0. Relative order of remaining players is preserved.
func rotatePlayers(players []engine.Player, firstSeat int) []engine.Player {
	if firstSeat == 0 || len(players) == 0 {
		return players
	}
	n := len(players)
	rotated := make([]engine.Player, n)
	for i := range players {
		rotated[i] = players[(firstSeat+i)%n]
	}
	return rotated
}

// --- Room code generation ----------------------------------------------------

const roomCodeChars = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"
const roomCodeLen = 6

func generateRoomCode() (string, error) {
	b := make([]byte, roomCodeLen)
	for i := range b {
		n, err := randutil.Intn(len(roomCodeChars))
		if err != nil {
			return "", fmt.Errorf("generateRoomCode: %w", err)
		}
		b[i] = roomCodeChars[n]
	}
	return string(b), nil
}

// --- Misc helpers ------------------------------------------------------------

func clamp(val, min, max int) int {
	if val < min {
		return min
	}
	if val > max {
		return max
	}
	return val
}
