package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/recess/services/user-service/internal/store"
	sharedmw "github.com/recess/shared/middleware"
)

// GET /api/v1/players/{playerID}/settings
func handleGetPlayerSettings(st store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		playerID, err := uuid.Parse(chi.URLParam(r, "playerID"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid player_id")
			return
		}

		callerID, _ := sharedmw.PlayerIDFromContext(r.Context())
		if callerID != playerID {
			writeError(w, http.StatusForbidden, "forbidden")
			return
		}

		settings, err := st.GetPlayerSettings(r.Context(), playerID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to get settings")
			return
		}

		writeJSON(w, http.StatusOK, settings)
	}
}

// PUT /api/v1/players/{playerID}/settings
func handleUpsertPlayerSettings(st store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		playerID, err := uuid.Parse(chi.URLParam(r, "playerID"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid player_id")
			return
		}

		callerID, _ := sharedmw.PlayerIDFromContext(r.Context())
		if callerID != playerID {
			writeError(w, http.StatusForbidden, "forbidden")
			return
		}

		var body store.PlayerSettingMap
		if err := decodeJSON(r, &body); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}

		settings, err := st.UpsertPlayerSettings(r.Context(), playerID, body)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to save settings")
			return
		}

		writeJSON(w, http.StatusOK, settings)
	}
}
