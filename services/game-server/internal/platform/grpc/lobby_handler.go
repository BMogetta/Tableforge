package grpchandler

import (
	"context"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/tableforge/game-server/internal/domain/lobby"
	"github.com/tableforge/game-server/internal/platform/store"
	lobbyv1 "github.com/tableforge/shared/proto/lobby/v1"
)

// LobbyHandler implements lobbyv1.LobbyServiceServer.
// CreateRankedRoom is called by match-service after both players accept a match.
type LobbyHandler struct {
	lobbyv1.UnimplementedLobbyServiceServer
	lobbySvc *lobby.Service
	st       store.Store
}

func NewLobbyHandler(lobbySvc *lobby.Service, st store.Store) *LobbyHandler {
	return &LobbyHandler{lobbySvc: lobbySvc, st: st}
}

func (h *LobbyHandler) CreateRankedRoom(ctx context.Context, req *lobbyv1.CreateRankedRoomRequest) (*lobbyv1.CreateRankedRoomResponse, error) {
	if req.PlayerAId == "" || req.PlayerBId == "" || req.GameId == "" {
		return nil, status.Error(codes.InvalidArgument, "player_a_id, player_b_id, and game_id are required")
	}

	playerA, err := uuid.Parse(req.PlayerAId)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid player_a_id: %v", err)
	}
	playerB, err := uuid.Parse(req.PlayerBId)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid player_b_id: %v", err)
	}

	roomView, err := h.lobbySvc.CreateRoom(ctx, req.GameId, playerA, nil)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "create room: %v", err)
	}

	if err := h.st.AddPlayerToRoom(ctx, roomView.Room.ID, playerB, 1); err != nil {
		return nil, status.Errorf(codes.Internal, "add player b to room: %v", err)
	}

	if err := h.st.SetRoomSetting(ctx, roomView.Room.ID, "session_mode", string(store.SessionModeRanked)); err != nil {
		return nil, status.Errorf(codes.Internal, "set session_mode: %v", err)
	}

	return &lobbyv1.CreateRankedRoomResponse{
		RoomId:   roomView.Room.ID.String(),
		RoomCode: roomView.Room.Code,
	}, nil
}
