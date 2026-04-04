package api

import (
	"context"
	"fmt"

	userv1 "github.com/recess/shared/proto/user/v1"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// GRPCUserChecker implements UserChecker via gRPC calls to user-service.
type GRPCUserChecker struct {
	conn   *grpc.ClientConn
	client userv1.UserServiceClient
}

// NewGRPCUserChecker dials the user-service gRPC server and returns a checker.
// addr is host:port (e.g. "user-service:9082").
func NewGRPCUserChecker(addr string) (*GRPCUserChecker, error) {
	conn, err := grpc.NewClient(addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithStatsHandler(otelgrpc.NewClientHandler()),
	)
	if err != nil {
		return nil, fmt.Errorf("user checker: dial %s: %w", addr, err)
	}
	return &GRPCUserChecker{
		conn:   conn,
		client: userv1.NewUserServiceClient(conn),
	}, nil
}

// Close releases the underlying gRPC connection.
func (c *GRPCUserChecker) Close() error {
	return c.conn.Close()
}

// AreFriends returns true when the friendship status is "accepted".
func (c *GRPCUserChecker) AreFriends(ctx context.Context, playerAID, playerBID string) (bool, error) {
	resp, err := c.client.GetFriendship(ctx, &userv1.GetFriendshipRequest{
		PlayerAId: playerAID,
		PlayerBId: playerBID,
	})
	if err != nil {
		return false, fmt.Errorf("user checker: GetFriendship: %w", err)
	}
	return resp.Status == "accepted", nil
}
