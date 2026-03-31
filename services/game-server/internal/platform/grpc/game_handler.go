package grpchandler

import (
	"context"
	"log/slog"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/tableforge/game-server/internal/domain/lobby"
	"github.com/tableforge/game-server/internal/domain/runtime"
	"github.com/tableforge/game-server/internal/platform/store"
	"github.com/tableforge/game-server/internal/platform/ws"
	gamev1 "github.com/tableforge/shared/proto/game/v1"
)

// GameHandler implements gamev1.GameServiceServer.
// StartSession is called by match-service after CreateRankedRoom.
// IsParticipant is called by ws-gateway before upgrading a room WebSocket.
type GameHandler struct {
	gamev1.UnimplementedGameServiceServer
	lobbySvc *lobby.Service
	rt       *runtime.Service
	st       store.Store
	hub      *ws.Hub
}

func NewGameHandler(lobbySvc *lobby.Service, rt *runtime.Service, st store.Store, hub *ws.Hub) *GameHandler {
	return &GameHandler{lobbySvc: lobbySvc, rt: rt, st: st, hub: hub}
}

func (h *GameHandler) StartSession(ctx context.Context, req *gamev1.StartSessionRequest) (*gamev1.StartSessionResponse, error) {
	if req.RoomId == "" {
		return nil, status.Error(codes.InvalidArgument, "room_id is required")
	}

	roomID, err := uuid.Parse(req.RoomId)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid room_id: %v", err)
	}

	room, err := h.st.GetRoom(ctx, roomID)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "room not found: %v", err)
	}

	mode := store.SessionModeCasual
	if req.Mode == "ranked" {
		mode = store.SessionModeRanked
	}

	session, err := h.lobbySvc.StartGame(ctx, roomID, room.OwnerID, mode)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "start game: %v", err)
	}

	h.rt.StartSession(context.Background(), session, h.hub, runtime.DefaultReadyTimeout)

	slog.Info("grpc: session started", "room_id", roomID, "session_id", session.ID, "mode", mode)
	return &gamev1.StartSessionResponse{SessionId: session.ID.String()}, nil
}

func (h *GameHandler) IsParticipant(ctx context.Context, req *gamev1.IsParticipantRequest) (*gamev1.IsParticipantResponse, error) {
	if req.RoomId == "" || req.PlayerId == "" {
		return nil, status.Error(codes.InvalidArgument, "room_id and player_id are required")
	}

	roomID, err := uuid.Parse(req.RoomId)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid room_id: %v", err)
	}
	playerID, err := uuid.Parse(req.PlayerId)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid player_id: %v", err)
	}

	players, err := h.st.ListRoomPlayers(ctx, roomID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list room players: %v", err)
	}

	isParticipant := false
	for _, p := range players {
		if p.PlayerID == playerID {
			isParticipant = true
			break
		}
	}

	settings, err := h.st.GetRoomSettings(ctx, roomID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "get room settings: %v", err)
	}

	return &gamev1.IsParticipantResponse{
		IsParticipant:     isParticipant,
		SpectatorsAllowed: settings["allow_spectators"] == "true",
	}, nil
}

func (h *GameHandler) GetMoveLog(_ context.Context, _ *gamev1.GetMoveLogRequest) (*gamev1.GetMoveLogResponse, error) {
	// TODO: implement for replay-service
	return nil, status.Error(codes.Unimplemented, "GetMoveLog not yet implemented")
}
