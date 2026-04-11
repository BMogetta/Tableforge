package grpc

import (
	"context"

	"github.com/google/uuid"
	"github.com/recess/services/user-service/internal/store"
	userv1 "github.com/recess/shared/proto/user/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Server implements userv1.UserServiceServer.
type Server struct {
	userv1.UnimplementedUserServiceServer
	store store.Store
}

func NewServer(st store.Store) *Server {
	return &Server{store: st}
}

// CheckBan is called by game-server on login and WebSocket connect.
func (s *Server) CheckBan(ctx context.Context, req *userv1.CheckBanRequest) (*userv1.CheckBanResponse, error) {
	playerID, err := uuid.Parse(req.PlayerId)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid player_id: %v", err)
	}

	ban, err := s.store.CheckActiveBan(ctx, playerID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to check ban: %v", err)
	}

	if ban == nil {
		return &userv1.CheckBanResponse{IsBanned: false}, nil
	}

	resp := &userv1.CheckBanResponse{
		IsBanned: true,
	}
	if ban.Reason != nil {
		resp.Reason = *ban.Reason
	}
	if ban.ExpiresAt != nil {
		resp.ExpiresAt = ban.ExpiresAt.UTC().Format("2006-01-02T15:04:05Z07:00")
	}
	return resp, nil
}

// GetFriendship is called by game-server to gate DMs and room joins.
func (s *Server) GetFriendship(ctx context.Context, req *userv1.GetFriendshipRequest) (*userv1.GetFriendshipResponse, error) {
	playerA, err := uuid.Parse(req.PlayerAId)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid player_a_id: %v", err)
	}
	playerB, err := uuid.Parse(req.PlayerBId)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid player_b_id: %v", err)
	}

	friendship, err := s.store.GetFriendship(ctx, playerA, playerB)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get friendship: %v", err)
	}

	// No row returned — no relationship exists.
	if friendship.RequesterID == uuid.Nil {
		return &userv1.GetFriendshipResponse{Status: "none"}, nil
	}

	resp := &userv1.GetFriendshipResponse{
		Status: string(friendship.Status),
	}
	if friendship.Status == store.FriendshipStatusBlocked {
		resp.BlockedBy = friendship.RequesterID.String()
	}
	return resp, nil
}

// AcceptFriendRequest is called by notification-service when a friend_request
// notification action is accepted.
func (s *Server) AcceptFriendRequest(ctx context.Context, req *userv1.AcceptFriendRequestRequest) (*userv1.AcceptFriendRequestResponse, error) {
	requesterID, err := uuid.Parse(req.RequesterId)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid requester_id: %v", err)
	}
	addresseeID, err := uuid.Parse(req.AddresseeId)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid addressee_id: %v", err)
	}

	if _, err := s.store.AcceptFriendRequest(ctx, requesterID, addresseeID); err != nil {
		return nil, status.Errorf(codes.Internal, "accept friend request: %v", err)
	}

	return &userv1.AcceptFriendRequestResponse{}, nil
}

// GetMutes is called by game-server on WS connect to populate the mute set.
func (s *Server) GetMutes(ctx context.Context, req *userv1.GetMutesRequest) (*userv1.GetMutesResponse, error) {
	playerID, err := uuid.Parse(req.PlayerId)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid player_id: %v", err)
	}

	mutes, err := s.store.GetMutedPlayers(ctx, playerID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get mutes: %v", err)
	}

	ids := make([]string, 0, len(mutes))
	for _, m := range mutes {
		ids = append(ids, m.MutedID.String())
	}
	return &userv1.GetMutesResponse{MutedPlayerIds: ids}, nil
}
