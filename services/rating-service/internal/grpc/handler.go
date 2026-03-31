package grpchandler

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/tableforge/rating-service/internal/store"
	ratingv1 "github.com/tableforge/shared/proto/rating/v1"
)

const (
	defaultLeaderboardLimit = 100
	maxLeaderboardLimit     = 500
	leaderboardMinGames     = 5
)

// Handler implements ratingv1.RatingServiceServer.
type Handler struct {
	ratingv1.UnimplementedRatingServiceServer
	store store.Store
}

func New(st store.Store) *Handler {
	return &Handler{store: st}
}

func (h *Handler) GetRating(ctx context.Context, req *ratingv1.GetRatingRequest) (*ratingv1.GetRatingResponse, error) {
	if req.PlayerId == "" {
		return nil, status.Error(codes.InvalidArgument, "player_id is required")
	}
	if req.GameId == "" {
		return nil, status.Error(codes.InvalidArgument, "game_id is required")
	}

	playerID, err := uuid.Parse(req.PlayerId)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid player_id: %v", err)
	}

	pr, err := h.store.GetRating(ctx, playerID, req.GameId)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "get rating: %v", err)
	}
	return toProto(pr), nil
}

func (h *Handler) GetRatings(ctx context.Context, req *ratingv1.GetRatingsRequest) (*ratingv1.GetRatingsResponse, error) {
	if len(req.PlayerIds) == 0 {
		return nil, status.Error(codes.InvalidArgument, "player_ids must not be empty")
	}
	if req.GameId == "" {
		return nil, status.Error(codes.InvalidArgument, "game_id is required")
	}
	if len(req.PlayerIds) > 100 {
		return nil, status.Errorf(codes.InvalidArgument, "player_ids exceeds max batch size of 100, got %d", len(req.PlayerIds))
	}

	playerIDs := make([]uuid.UUID, len(req.PlayerIds))
	for i, raw := range req.PlayerIds {
		id, err := uuid.Parse(raw)
		if err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "invalid player_id %q: %v", raw, err)
		}
		playerIDs[i] = id
	}

	ratingsByID, err := h.store.GetRatings(ctx, playerIDs, req.GameId)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "get ratings: %v", err)
	}

	// Return in the same order as the request.
	ratings := make([]*ratingv1.GetRatingResponse, len(playerIDs))
	for i, id := range playerIDs {
		ratings[i] = toProto(ratingsByID[id])
	}
	return &ratingv1.GetRatingsResponse{Ratings: ratings}, nil
}

func (h *Handler) GetLeaderboard(ctx context.Context, req *ratingv1.GetLeaderboardRequest) (*ratingv1.GetLeaderboardResponse, error) {
	if req.GameId == "" {
		return nil, status.Error(codes.InvalidArgument, "game_id is required")
	}

	limit := int(req.Limit)
	if limit <= 0 {
		limit = defaultLeaderboardLimit
	}
	if limit > maxLeaderboardLimit {
		return nil, status.Errorf(codes.InvalidArgument, "limit exceeds maximum of %d", maxLeaderboardLimit)
	}
	offset := int(req.Offset)
	if offset < 0 {
		offset = 0
	}

	rows, err := h.store.GetLeaderboard(ctx, req.GameId, limit, offset, leaderboardMinGames)
	if err != nil {
		return nil, status.Errorf(codes.Internal, fmt.Sprintf("get leaderboard: %v", err))
	}

	entries := make([]*ratingv1.LeaderboardEntry, len(rows))
	for i, row := range rows {
		entries[i] = &ratingv1.LeaderboardEntry{
			Rank:          int32(offset + i + 1),
			PlayerId:      row.PlayerID.String(),
			DisplayRating: row.DisplayRating,
			GamesPlayed:   int32(row.GamesPlayed),
		}
	}

	return &ratingv1.GetLeaderboardResponse{
		GameId:  req.GameId,
		Entries: entries,
		Total:   int32(len(rows)), // TODO: COUNT(*) for accurate pagination total
	}, nil
}

// toProto converts a store.PlayerRating to the proto response type.
// MMR is included — callers are trusted services, not browser clients.
func toProto(pr *store.PlayerRating) *ratingv1.GetRatingResponse {
	return &ratingv1.GetRatingResponse{
		PlayerId:      pr.PlayerID.String(),
		GameId:        pr.GameID,
		Mmr:           pr.MMR,
		DisplayRating: pr.DisplayRating,
		GamesPlayed:   int32(pr.GamesPlayed),
		WinStreak:     int32(pr.WinStreak),
		LossStreak:    int32(pr.LossStreak),
	}
}
