package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/recess/services/user-service/internal/store"
	sharedmw "github.com/recess/shared/middleware"
)

func handleGetProfile(st store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		playerID, err := uuid.Parse(chi.URLParam(r, "playerID"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid player id")
			return
		}
		profile, err := st.GetProfile(r.Context(), playerID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to get profile")
			return
		}
		writeJSON(w, http.StatusOK, profile)
	}
}

func handleUpsertProfile(st store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		playerID, err := uuid.Parse(chi.URLParam(r, "playerID"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid player id")
			return
		}
		callerID, _ := sharedmw.PlayerIDFromContext(r.Context())
		if callerID != playerID {
			writeError(w, http.StatusForbidden, "forbidden")
			return
		}

		var req struct {
			Bio     *string `json:"bio"`
			Country *string `json:"country"`
		}
		if err := decodeJSON(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}

		profile, err := st.UpsertProfile(r.Context(), store.UpsertProfileParams{
			PlayerID: playerID,
			Bio:      req.Bio,
			Country:  req.Country,
		})
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to update profile")
			return
		}
		writeJSON(w, http.StatusOK, profile)
	}
}

func handleListAchievements(st store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		playerID, err := uuid.Parse(chi.URLParam(r, "playerID"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid player id")
			return
		}
		achievements, err := st.ListAchievements(r.Context(), playerID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to list achievements")
			return
		}
		writeJSON(w, http.StatusOK, achievements)
	}
}
