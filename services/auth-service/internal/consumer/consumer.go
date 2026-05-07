// Package consumer reads player.banned from a Redis Stream and revokes
// the affected sessions.
//
// Stream: player.banned. Consumer group: "auth-service".
//
// On each ban event:
//  1. Revoke all refresh sessions for the player (DB).
//  2. Publish player.session.revoked over Pub/Sub so ws-gateway closes
//     the live WebSocket. (session.revoked moves to Asynq in P3.6 Phase 3c.)
package consumer

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"

	sharedevents "github.com/recess/shared/events"
	"github.com/recess/shared/streams"
)

const (
	streamPlayerBanned    = "player.banned"
	consumerGroup         = "auth-service"
	channelSessionRevoked = "player.session.revoked"
)

// SessionRevoker can revoke all sessions for a player.
type SessionRevoker interface {
	RevokeAllSessions(ctx context.Context, playerID uuid.UUID) error
}

// Consumer reads player.banned and reacts on the auth side.
type Consumer struct {
	rdb     *redis.Client
	worker  *streams.Worker
	log     *slog.Logger
	revoker SessionRevoker
}

func New(rdb *redis.Client, log *slog.Logger, revoker SessionRevoker, consumerName string) *Consumer {
	c := &Consumer{rdb: rdb, log: log, revoker: revoker}
	c.worker = streams.NewWorker(
		rdb,
		streamPlayerBanned,
		consumerGroup,
		consumerName,
		c.handle,
		streams.WithLogger(log),
	)
	return c
}

// Run blocks until ctx is cancelled.
func (c *Consumer) Run(ctx context.Context) error {
	c.log.Info("auth consumer starting", "stream", streamPlayerBanned, "group", consumerGroup)
	return c.worker.Run(ctx)
}

func (c *Consumer) handle(ctx context.Context, payload []byte) error {
	var evt sharedevents.PlayerBanned
	if err := json.Unmarshal(payload, &evt); err != nil {
		return fmt.Errorf("unmarshal PlayerBanned: %w", err)
	}

	playerID, err := uuid.Parse(evt.PlayerID)
	if err != nil {
		return fmt.Errorf("invalid player_id: %w", err)
	}

	// Revoke all refresh sessions in the DB so the player cannot obtain new tokens.
	if c.revoker != nil {
		if err := c.revoker.RevokeAllSessions(ctx, playerID); err != nil {
			c.log.Error("failed to revoke sessions for banned player", "player_id", playerID, "error", err)
		}
	}

	// Publish player.session.revoked so ws-gateway closes the connection.
	// session_id is empty — ws-gateway matches on player_id and closes all connections.
	revoked := sharedevents.PlayerSessionRevoked{
		Meta:      sharedevents.Meta{EventID: uuid.New().String(), Version: 1},
		PlayerID:  playerID.String(),
		SessionID: "", // ws-gateway closes all sessions for this player
		Reason:    "banned",
	}
	data, err := json.Marshal(revoked)
	if err != nil {
		return fmt.Errorf("marshal PlayerSessionRevoked: %w", err)
	}

	if err := c.rdb.Publish(ctx, channelSessionRevoked, data).Err(); err != nil {
		return fmt.Errorf("publish player.session.revoked: %w", err)
	}

	c.log.Info("player banned: session revoked", "player_id", playerID, "ban_event_id", evt.EventID)
	return nil
}
