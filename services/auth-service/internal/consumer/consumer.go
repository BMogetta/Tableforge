// Package consumer subscribes to Redis events that trigger auth-side actions.
//
// Subscribed channels:
//
//   - player.banned → clear session cookie (force logout) + publish player.session.revoked
package consumer

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	sharedevents "github.com/recess/shared/events"
)

const (
	channelPlayerBanned   = "player.banned"
	channelSessionRevoked = "player.session.revoked"
)

// SessionRevoker can revoke all sessions for a player.
type SessionRevoker interface {
	RevokeAllSessions(ctx context.Context, playerID uuid.UUID) error
}

// Consumer subscribes to event channels and reacts to auth-relevant events.
type Consumer struct {
	rdb     *redis.Client
	log     *slog.Logger
	revoker SessionRevoker
}

func New(rdb *redis.Client, log *slog.Logger, revoker SessionRevoker) *Consumer {
	return &Consumer{rdb: rdb, log: log, revoker: revoker}
}

// Run blocks until ctx is cancelled.
func (c *Consumer) Run(ctx context.Context) error {
	sub := c.rdb.Subscribe(ctx, channelPlayerBanned)
	defer sub.Close()

	ch := sub.Channel()
	c.log.Info("subscribed to Redis channels", "channels", []string{channelPlayerBanned})

	for {
		select {
		case <-ctx.Done():
			return nil
		case msg, ok := <-ch:
			if !ok {
				return errors.New("redis subscription channel closed unexpectedly")
			}
			if err := c.handle(ctx, msg); err != nil {
				c.log.Error("failed to handle event", "channel", msg.Channel, "error", err)
			}
		}
	}
}

func (c *Consumer) handle(ctx context.Context, msg *redis.Message) error {
	switch msg.Channel {
	case channelPlayerBanned:
		return c.handlePlayerBanned(ctx, msg.Payload)
	default:
		return fmt.Errorf("unknown channel: %s", msg.Channel)
	}
}

func (c *Consumer) handlePlayerBanned(ctx context.Context, payload string) error {
	var evt sharedevents.PlayerBanned
	if err := json.Unmarshal([]byte(payload), &evt); err != nil {
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
