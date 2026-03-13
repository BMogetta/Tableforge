package api

import (
	"net/http"
	"strconv"

	"github.com/tableforge/server/internal/platform/store"
)

// --- Leaderboard -------------------------------------------------------------

// GET /api/v1/leaderboard?game_id=tictactoe&limit=20
// Returns the top players for a game ordered by display_rating descending.
// Required query param: game_id.
// Optional query param: limit (1-100, default 20).
func handleGetLeaderboard(st store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		gameID := r.URL.Query().Get("game_id")
		if gameID == "" {
			writeError(w, http.StatusBadRequest, "game_id is required")
			return
		}
		limit := 20
		if l := r.URL.Query().Get("limit"); l != "" {
			if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 && parsed <= 100 {
				limit = parsed
			}
		}
		entries, err := st.GetRatingLeaderboard(r.Context(), gameID, limit)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to get leaderboard")
			return
		}
		writeJSON(w, http.StatusOK, entries)
	}
}
