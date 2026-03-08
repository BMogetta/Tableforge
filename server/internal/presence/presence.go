package presence

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

const (
	// presenceTTL is how long a presence key lives without a heartbeat.
	presenceTTL = 30 * time.Second
)

func presenceKey(playerID uuid.UUID) string {
	return fmt.Sprintf("presence:%s", playerID)
}

// Store manages player presence via Redis SETEX.
type Store struct {
	rdb *redis.Client
}

func New(rdb *redis.Client) *Store {
	return &Store{rdb: rdb}
}

// Set marks the player as online, refreshing the TTL.
func (s *Store) Set(ctx context.Context, playerID uuid.UUID) error {
	return s.rdb.Set(ctx, presenceKey(playerID), "1", presenceTTL).Err()
}

// Del marks the player as offline immediately.
func (s *Store) Del(ctx context.Context, playerID uuid.UUID) error {
	return s.rdb.Del(ctx, presenceKey(playerID)).Err()
}

// IsOnline returns true if the player has an active presence key.
func (s *Store) IsOnline(ctx context.Context, playerID uuid.UUID) (bool, error) {
	n, err := s.rdb.Exists(ctx, presenceKey(playerID)).Result()
	if err != nil {
		return false, fmt.Errorf("presence.IsOnline: %w", err)
	}
	return n > 0, nil
}

// ListOnline returns a map of playerID → online for the given player IDs.
func (s *Store) ListOnline(ctx context.Context, playerIDs []uuid.UUID) (map[uuid.UUID]bool, error) {
	result := make(map[uuid.UUID]bool, len(playerIDs))
	if len(playerIDs) == 0 {
		return result, nil
	}

	keys := make([]string, len(playerIDs))
	for i, id := range playerIDs {
		keys[i] = presenceKey(id)
		result[id] = false
	}

	// MGET returns nil for missing keys.
	vals, err := s.rdb.MGet(ctx, keys...).Result()
	if err != nil {
		return nil, fmt.Errorf("presence.ListOnline: %w", err)
	}

	for i, val := range vals {
		if val != nil {
			result[playerIDs[i]] = true
		}
	}

	return result, nil
}
