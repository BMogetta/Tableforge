package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/tableforge/services/user-service/internal/store"
	sharedmw "github.com/tableforge/shared/middleware"
)

func handleMutePlayer(st store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		muterID, err := uuid.Parse(chi.URLParam(r, "playerID"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid player id")
			return
		}
		var req struct {
			MutedID string `json:"muted_id"`
		}
		if err := decodeJSON(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}
		mutedID, err := uuid.Parse(req.MutedID)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid muted_id")
			return
		}
		callerID, _ := sharedmw.PlayerIDFromContext(r.Context())
		if callerID != muterID {
			writeError(w, http.StatusForbidden, "forbidden")
			return
		}
		if muterID == mutedID {
			writeError(w, http.StatusBadRequest, "cannot mute yourself")
			return
		}
		if err := st.MutePlayer(r.Context(), muterID, mutedID); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to mute player")
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

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
		callerID, _ := sharedmw.PlayerIDFromContext(r.Context())
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

func handleGetMutes(st store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		playerID, err := uuid.Parse(chi.URLParam(r, "playerID"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid player id")
			return
		}
		callerID, _ := sharedmw.PlayerIDFromContext(r.Context())
		// Allow managers to fetch any player's mute list.
		if callerID != playerID {
			role, ok := sharedmw.RoleFromContext(r.Context())
			if !ok || !hasRole(string(role), sharedmw.RoleManager) {
				writeError(w, http.StatusForbidden, "forbidden")
				return
			}
		}
		mutes, err := st.GetMutedPlayers(r.Context(), playerID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to get muted players")
			return
		}
		writeJSON(w, http.StatusOK, mutes)
	}
}
