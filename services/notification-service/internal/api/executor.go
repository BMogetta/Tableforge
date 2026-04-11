package api

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	userv1 "github.com/recess/shared/proto/user/v1"
)

// GRPCExecutor implements ActionExecutor using gRPC calls to other services.
type GRPCExecutor struct {
	userClient userv1.UserServiceClient
}

func NewGRPCExecutor(uc userv1.UserServiceClient) *GRPCExecutor {
	return &GRPCExecutor{userClient: uc}
}

func (e *GRPCExecutor) AcceptFriendRequest(ctx context.Context, requesterID, addresseeID uuid.UUID) error {
	_, err := e.userClient.AcceptFriendRequest(ctx, &userv1.AcceptFriendRequestRequest{
		RequesterId: requesterID.String(),
		AddresseeId: addresseeID.String(),
	})
	if err != nil {
		return fmt.Errorf("grpc AcceptFriendRequest: %w", err)
	}
	return nil
}
