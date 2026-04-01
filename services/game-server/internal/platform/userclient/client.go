// Package userclient provides a gRPC client for user-service.
// The monolith uses this for synchronous read operations: ban checks,
// friendship status, and mute list retrieval.
//
// The client is initialized once at startup and injected into handlers
// that need it. All methods are safe for concurrent use.
package userclient

import (
	"context"
	"fmt"

	userv1 "github.com/recess/shared/proto/user/v1"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// Client wraps the generated gRPC client with higher-level methods.
type Client struct {
	conn   *grpc.ClientConn
	client userv1.UserServiceClient
}

// New dials the user-service gRPC server and returns a Client.
// addr is the host:port of the user-service gRPC server (e.g. "user-service:9082").
// The connection is not authenticated — internal network only.
func New(addr string) (*Client, error) {
	conn, err := grpc.NewClient(addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithStatsHandler(otelgrpc.NewClientHandler()),
	)
	if err != nil {
		return nil, fmt.Errorf("userclient: dial %s: %w", addr, err)
	}
	return &Client{
		conn:   conn,
		client: userv1.NewUserServiceClient(conn),
	}, nil
}

// Close releases the underlying gRPC connection.
func (c *Client) Close() error {
	return c.conn.Close()
}

// BanResult holds the result of a ban check.
type BanResult struct {
	IsBanned  bool
	Reason    string
	ExpiresAt string // RFC3339; empty if permanent
}

// CheckBan returns the active ban for playerID, or IsBanned=false if not banned.
func (c *Client) CheckBan(ctx context.Context, playerID string) (BanResult, error) {
	resp, err := c.client.CheckBan(ctx, &userv1.CheckBanRequest{PlayerId: playerID})
	if err != nil {
		return BanResult{}, fmt.Errorf("userclient: CheckBan: %w", err)
	}
	return BanResult{
		IsBanned:  resp.IsBanned,
		Reason:    resp.Reason,
		ExpiresAt: resp.ExpiresAt,
	}, nil
}

// FriendshipStatus holds the relationship between two players.
type FriendshipStatus struct {
	Status    string // "none" | "pending" | "accepted" | "blocked"
	BlockedBy string // UUID of the player who issued the block; empty if not blocked
}

// GetFriendship returns the relationship status between two players.
func (c *Client) GetFriendship(ctx context.Context, playerAID, playerBID string) (FriendshipStatus, error) {
	resp, err := c.client.GetFriendship(ctx, &userv1.GetFriendshipRequest{
		PlayerAId: playerAID,
		PlayerBId: playerBID,
	})
	if err != nil {
		return FriendshipStatus{}, fmt.Errorf("userclient: GetFriendship: %w", err)
	}
	return FriendshipStatus{
		Status:    resp.Status,
		BlockedBy: resp.BlockedBy,
	}, nil
}

// GetMutes returns all player IDs muted by the given player.
func (c *Client) GetMutes(ctx context.Context, playerID string) ([]string, error) {
	resp, err := c.client.GetMutes(ctx, &userv1.GetMutesRequest{PlayerId: playerID})
	if err != nil {
		return nil, fmt.Errorf("userclient: GetMutes: %w", err)
	}
	return resp.MutedPlayerIds, nil
}
