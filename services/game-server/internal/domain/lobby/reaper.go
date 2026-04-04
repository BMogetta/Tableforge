package lobby

import (
	"context"
	"log/slog"
	"time"

	"github.com/recess/game-server/internal/platform/store"
)

const (
	// DefaultReaperInterval is how often the reaper runs.
	DefaultReaperInterval = 60 * time.Second

	// waitingOrphanMaxAge is how long a waiting room with 0 players can exist
	// before being cleaned up.
	waitingOrphanMaxAge = 5 * time.Minute

	// finishedRoomMaxAge is how long a finished room can exist before being
	// soft-deleted.
	finishedRoomMaxAge = 10 * time.Minute
)

// StartReaper launches a background goroutine that periodically cleans up
// orphan rooms. It blocks until ctx is cancelled.
func StartReaper(ctx context.Context, st store.Store, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	slog.Info("room reaper started", "interval", interval)

	for {
		select {
		case <-ctx.Done():
			slog.Info("room reaper stopped")
			return
		case <-ticker.C:
			cleaned, err := st.CleanupOrphanRooms(ctx, waitingOrphanMaxAge, finishedRoomMaxAge)
			if err != nil {
				slog.Error("room reaper failed", "error", err)
				continue
			}
			if cleaned > 0 {
				slog.Info("room reaper cleaned rooms", "count", cleaned)
			}
		}
	}
}
