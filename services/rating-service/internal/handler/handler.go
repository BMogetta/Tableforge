package handler

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/recess/rating-service/internal/store"
)

const (
	defaultLeaderboardLimit = 100
	maxLeaderboardLimit     = 500
	leaderboardMinGames     = 5
)

// Handler holds dependencies for public HTTP routes.
type Handler struct {
	store store.Store
	log   *slog.Logger
}

// New constructs a Handler.
func New(st store.Store, log *slog.Logger) *Handler {
	return &Handler{store: st, log: log}
}

// RegisterRoutes mounts all public API routes onto r.
//
//	GET /api/v1/ratings/{game_id}/leaderboard
//	GET /api/v1/players/{id}/ratings/{game_id}
func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Get("/api/v1/ratings/{game_id}/leaderboard", h.handleLeaderboard)
	r.Get("/api/v1/players/{id}/ratings/{game_id}", h.handlePlayerRating)
}

// handleLeaderboard returns the top-N players for a given game.
//
// GET /api/v1/ratings/{game_id}/leaderboard
// Query params:
//
//	limit  int  (default 100, max 500)
//	offset int  (default 0)
func (h *Handler) handleLeaderboard(w http.ResponseWriter, r *http.Request) {
	gameID := chi.URLParam(r, "game_id")
	if gameID == "" {
		http.Error(w, "game_id is required", http.StatusBadRequest)
		return
	}

	limit := queryInt(r, "limit", defaultLeaderboardLimit)
	if limit <= 0 || limit > maxLeaderboardLimit {
		http.Error(w, "limit must be between 1 and 500", http.StatusBadRequest)
		return
	}
	offset := max(queryInt(r, "offset", 0), 0)

	rows, err := h.store.GetLeaderboard(r.Context(), gameID, limit, offset, leaderboardMinGames)
	if err != nil {
		h.log.Error("handler: get leaderboard", "game_id", gameID, "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	type entry struct {
		Rank          int     `json:"rank"`
		PlayerID      string  `json:"player_id"`
		DisplayRating float64 `json:"display_rating"`
		GamesPlayed   int     `json:"games_played"`
	}

	entries := make([]entry, len(rows))
	for i, row := range rows {
		entries[i] = entry{
			Rank:          offset + i + 1,
			PlayerID:      row.PlayerID.String(),
			DisplayRating: row.DisplayRating,
			GamesPlayed:   row.GamesPlayed,
		}
	}

	total, err := h.store.CountLeaderboard(r.Context(), gameID, leaderboardMinGames)
	if err != nil {
		h.log.Error("handler: count leaderboard", "game_id", gameID, "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	writeJSON(w, map[string]any{
		"game_id": gameID,
		"entries": entries,
		"total":   total,
	})
}

// handlePlayerRating returns the current rating for a single player + game.
//
// GET /api/v1/players/{id}/ratings/{game_id}
func (h *Handler) handlePlayerRating(w http.ResponseWriter, r *http.Request) {
	rawID := chi.URLParam(r, "id")
	gameID := chi.URLParam(r, "game_id")

	playerID, err := uuid.Parse(rawID)
	if err != nil {
		http.Error(w, "invalid player id", http.StatusBadRequest)
		return
	}
	if gameID == "" {
		http.Error(w, "game_id is required", http.StatusBadRequest)
		return
	}

	pr, err := h.store.GetRating(r.Context(), playerID, gameID)
	if err != nil {
		h.log.Error("handler: get rating", "player_id", rawID, "game_id", gameID, "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	// MMR is intentionally omitted — this endpoint is public / browser-facing.
	writeJSON(w, map[string]any{
		"player_id":      pr.PlayerID.String(),
		"game_id":        pr.GameID,
		"display_rating": pr.DisplayRating,
		"games_played":   pr.GamesPlayed,
		"win_streak":     pr.WinStreak,
		"loss_streak":    pr.LossStreak,
	})
}

// --- helpers -----------------------------------------------------------------

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Error("handler: encode json", "error", err)
	}
}

// queryInt reads an integer query param, returning fallback if missing or invalid.
func queryInt(r *http.Request, key string, fallback int) int {
	s := r.URL.Query().Get(key)
	if s == "" {
		return fallback
	}
	var n int
	if _, err := fmt.Sscanf(s, "%d", &n); err != nil {
		return fallback
	}
	return n
}
