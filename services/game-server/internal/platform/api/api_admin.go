package api

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/redis/go-redis/v9"
	"github.com/recess/game-server/internal/platform/store"
)

type systemStatsResponse struct {
	OnlinePlayers     int `json:"online_players"`
	ActiveRooms       int `json:"active_rooms"`
	ActiveSessions    int `json:"active_sessions"`
	TotalPlayers      int `json:"total_players"`
	TotalSessionsToday int `json:"total_sessions_today"`
}

// countOnlinePlayers counts Redis presence keys (presence:*) set by ws-gateway.
// Returns 0 when rdb is nil (tests).
func countOnlinePlayers(ctx context.Context, rdb *redis.Client) int {
	if rdb == nil {
		return 0
	}
	var count int
	var cursor uint64
	for {
		keys, next, err := rdb.Scan(ctx, cursor, "presence:*", 100).Result()
		if err != nil {
			slog.Error("countOnlinePlayers: redis scan", "error", err)
			return count
		}
		count += len(keys)
		cursor = next
		if cursor == 0 {
			break
		}
	}
	return count
}

func handleGetSystemStats(st store.Store, rdb *redis.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		online := countOnlinePlayers(ctx, rdb)

		activeRooms, err := st.CountActiveRooms(ctx)
		if err != nil {
			slog.Error("admin stats: count active rooms", "error", err)
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}

		activeSessions, err := st.CountActiveSessions(ctx)
		if err != nil {
			slog.Error("admin stats: count active sessions", "error", err)
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}

		totalPlayers, err := st.CountTotalPlayers(ctx)
		if err != nil {
			slog.Error("admin stats: count total players", "error", err)
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}

		sessionsToday, err := st.CountSessionsToday(ctx)
		if err != nil {
			slog.Error("admin stats: count sessions today", "error", err)
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}

		writeJSON(w, http.StatusOK, systemStatsResponse{
			OnlinePlayers:      online,
			ActiveRooms:        activeRooms,
			ActiveSessions:     activeSessions,
			TotalPlayers:       totalPlayers,
			TotalSessionsToday: sessionsToday,
		})
	}
}
