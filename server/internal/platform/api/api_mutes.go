package api

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/tableforge/server/internal/platform/auth"
	"github.com/tableforge/server/internal/platform/store"
)

// POST /api/v1/players/{playerID}/mute
// Records that the caller has muted the target player.
// The playerID in the path is the target (muted); player_id in the body is the muter.
// Idempotent — muting an already-muted player is a no-op.
func handleMutePlayer(st store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		mutedID, err := uuid.Parse(chi.URLParam(r, "playerID"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid player id")
			return
		}

		var req struct {
			PlayerID string `json:"player_id"`
			MutedID  string `json:"muted_id"`
		}
		if err := decodeJSON(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}

		muterID, err := uuid.Parse(req.PlayerID)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid player_id")
			return
		}

		// Confirm muted_id in body matches the path parameter.
		bodyMutedID, err := uuid.Parse(req.MutedID)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid muted_id")
			return
		}
		if bodyMutedID != mutedID {
			writeError(w, http.StatusBadRequest, "muted_id in body does not match path")
			return
		}

		if muterID == mutedID {
			writeError(w, http.StatusBadRequest, "cannot mute yourself")
			return
		}

		if err := st.MutePlayer(r.Context(), muterID, mutedID); err != nil {
			var pgErr *pgconn.PgError
			if errors.As(err, &pgErr) && pgErr.Code == "23503" {
				writeError(w, http.StatusNotFound, "player not found")
				return
			}
			writeError(w, http.StatusInternalServerError, "failed to mute player")
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}

// DELETE /api/v1/players/{playerID}/mute/{mutedID}
// Removes a mute record. No-op if the mute does not exist.
// player_id in the body must match the playerID path parameter (the muter).
func handleUnmutePlayer(st store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		muterID, err := uuid.Parse(chi.URLParam(r, "playerID"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid player id")
			return
		}

		mutedID, err := uuid.Parse(chi.URLParam(r, "mutedID"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid muted id")
			return
		}

		var req struct {
			PlayerID string `json:"player_id"`
		}
		if err := decodeJSON(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}

		callerID, err := uuid.Parse(req.PlayerID)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid player_id")
			return
		}

		// Only the muter can remove their own mute.
		if callerID != muterID {
			writeError(w, http.StatusForbidden, "forbidden")
			return
		}

		if err := st.UnmutePlayer(r.Context(), muterID, mutedID); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to unmute player")
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}

// GET /api/v1/players/{playerID}/mutes
// Returns all players muted by playerID.
// player_id in the body must match the playerID path parameter,
// unless the caller holds manager role or above.
func handleGetMutedPlayers(st store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		targetID, err := uuid.Parse(chi.URLParam(r, "playerID"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid player id")
			return
		}

		var req struct {
			PlayerID string `json:"player_id"`
		}
		if err := decodeJSON(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}

		callerID, err := uuid.Parse(req.PlayerID)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid player_id")
			return
		}

		// Allow managers to fetch any player's mute list.
		if callerID != targetID {
			role, ok := auth.RoleFromContext(r.Context())
			if !ok || (role != store.RoleManager && role != store.RoleOwner) {
				writeError(w, http.StatusForbidden, "forbidden")
				return
			}
		}

		mutes, err := st.GetMutedPlayers(r.Context(), targetID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to get muted players")
			return
		}

		writeJSON(w, http.StatusOK, mutes)
	}
}
