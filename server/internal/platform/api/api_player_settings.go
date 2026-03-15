package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/tableforge/server/internal/platform/store"
)

// GET /api/v1/players/{playerID}/settings
//
// Returns the player's settings merged over application defaults.
// If the player has no stored settings, returns the defaults.
func handleGetPlayerSettings(st store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		playerID, err := uuid.Parse(chi.URLParam(r, "playerID"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid player_id")
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
//
// Replaces the player's settings atomically. The body must be a partial or
// full PlayerSettingMap — only provided keys are stored. Missing keys will
// fall back to application defaults at read time.
//
// Request body: PlayerSettingMap (any subset of keys)
func handleUpsertPlayerSettings(st store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		playerID, err := uuid.Parse(chi.URLParam(r, "playerID"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid player_id")
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
