// Package presence manages online/offline state via Redis TTL keys.
// Key format: presence:{player_id} → "1" with 30s sliding TTL.
// Refreshed on every WebSocket ping/pong cycle by the client's WritePump.
package presence

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

const ttl = 30 * time.Second

type Store struct {
	rdb *redis.Client
}

func New(rdb *redis.Client) *Store {
	return &Store{rdb: rdb}
}

func (s *Store) Set(ctx context.Context, playerID uuid.UUID) error {
	return s.rdb.Set(ctx, key(playerID), "1", ttl).Err()
}

func (s *Store) Del(ctx context.Context, playerID uuid.UUID) error {
	return s.rdb.Del(ctx, key(playerID)).Err()
}

func (s *Store) IsOnline(ctx context.Context, playerID uuid.UUID) (bool, error) {
	n, err := s.rdb.Exists(ctx, key(playerID)).Result()
	return n > 0, err
}

func (s *Store) ListOnline(ctx context.Context, playerIDs []uuid.UUID) (map[uuid.UUID]bool, error) {
	if len(playerIDs) == 0 {
		return map[uuid.UUID]bool{}, nil
	}

	keys := make([]string, len(playerIDs))
	for i, id := range playerIDs {
		keys[i] = key(id)
	}

	vals, err := s.rdb.MGet(ctx, keys...).Result()
	if err != nil {
		return nil, err
	}

	result := make(map[uuid.UUID]bool, len(playerIDs))
	for i, v := range vals {
		result[playerIDs[i]] = v != nil
	}
	return result, nil
}

func key(playerID uuid.UUID) string {
	return fmt.Sprintf("presence:%s", playerID)
}
