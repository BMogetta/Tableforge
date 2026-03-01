package lobby

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"strings"

	"github.com/google/uuid"
	"github.com/tableforge/server/internal/engine"
	"github.com/tableforge/server/internal/store"
)

var (
	ErrRoomNotFound    = errors.New("room not found")
	ErrRoomFull        = errors.New("room is full")
	ErrRoomNotWaiting  = errors.New("room is not in waiting state")
	ErrNotEnoughPlayer = errors.New("not enough players to start")
	ErrNotOwner        = errors.New("only the room owner can start the game")
	ErrAlreadyInRoom   = errors.New("player is already in the room")
	ErrGameNotFound    = errors.New("game not found")
)

// RoomView is the full state of a room as seen by clients.
type RoomView struct {
	Room    store.Room         `json:"room"`
	Players []store.RoomPlayer `json:"players"`
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

// LeaveRoom removes a player from a waiting room.
func (svc *Service) LeaveRoom(ctx context.Context, roomID, playerID uuid.UUID) error {
	room, err := svc.store.GetRoom(ctx, roomID)
	if err != nil {
		return ErrRoomNotFound
	}

	if room.Status != store.RoomStatusWaiting {
		return ErrRoomNotWaiting
	}

	return svc.store.RemovePlayerFromRoom(ctx, roomID, playerID)
}

// StartGame transitions the room to in_progress and creates a game session.
// Only the room owner can start, and the minimum player count must be met.
// The effective turn timeout is resolved from: request value → game default → no timeout.
func (svc *Service) StartGame(ctx context.Context, roomID, requesterID uuid.UUID) (store.GameSession, error) {
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

	// Resolve effective turn timeout:
	// 1. Use value set on room (chosen when room was created).
	// 2. Clamp it to the game's configured min/max.
	// 3. If room has no timeout, use the game's default.
	effectiveTimeout, err := svc.resolveTimeout(ctx, room)
	if err != nil {
		return store.GameSession{}, fmt.Errorf("StartGame: resolve timeout: %w", err)
	}

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

	initialState, err := game.Init(enginePlayers)
	if err != nil {
		return store.GameSession{}, fmt.Errorf("StartGame: init game: %w", err)
	}

	stateJSON, err := json.Marshal(initialState)
	if err != nil {
		return store.GameSession{}, fmt.Errorf("StartGame: marshal state: %w", err)
	}

	session, err := svc.store.CreateGameSession(ctx, roomID, room.GameID, stateJSON, effectiveTimeout)
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
func (svc *Service) GetRoom(ctx context.Context, roomID uuid.UUID) (RoomView, error) {
	room, err := svc.store.GetRoom(ctx, roomID)
	if err != nil {
		return RoomView{}, ErrRoomNotFound
	}
	return svc.getRoomView(ctx, room)
}

// ListWaitingRooms returns all rooms currently open for players to join.
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
		views = append(views, v)
	}
	return views, nil
}

func (svc *Service) getRoomView(ctx context.Context, room store.Room) (RoomView, error) {
	players, err := svc.store.ListRoomPlayers(ctx, room.ID)
	if err != nil {
		return RoomView{}, fmt.Errorf("getRoomView: %w", err)
	}
	return RoomView{Room: room, Players: players}, nil
}

const roomCodeChars = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"
const roomCodeLen = 6

// generateRoomCode returns a random human-friendly uppercase room code.
// Ambiguous characters (0, O, 1, I) are excluded.
func generateRoomCode() (string, error) {
	b := make([]byte, roomCodeLen)
	for i := range b {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(roomCodeChars))))
		if err != nil {
			return "", err
		}
		b[i] = roomCodeChars[n.Int64()]
	}
	return string(b), nil
}

func clamp(val, min, max int) int {
	if val < min {
		return min
	}
	if val > max {
		return max
	}
	return val
}
