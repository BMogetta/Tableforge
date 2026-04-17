package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/recess/services/user-service/internal/store"
	"github.com/recess/shared/achievements"
	sharedmw "github.com/recess/shared/middleware"
)

// Intentionally public — players can view each other's profiles in a multiplayer game.
// The write endpoint (handleUpsertProfile) checks ownership.
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

// Intentionally public — achievements are visible on player profiles.
func handleListAchievements(st store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		playerID, err := uuid.Parse(chi.URLParam(r, "playerID"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid player id")
			return
		}
		rows, err := st.ListAchievements(r.Context(), playerID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to list achievements")
			return
		}
		writeJSON(w, http.StatusOK, rows)
	}
}

// Public — the definitions are static metadata shared by all clients, with no
// per-player secrets. Carries i18n keys only; the frontend resolves them.
//
// Shape is deliberately minimal and JSON-friendly: ComputeProgress closures
// are server-only and never serialized.
type achievementTierDTO struct {
	Threshold      int    `json:"threshold"`
	NameKey        string `json:"name_key"`
	DescriptionKey string `json:"description_key"`
}

type achievementDefinitionDTO struct {
	Key            string               `json:"key"`
	NameKey        string               `json:"name_key"`
	DescriptionKey string               `json:"description_key,omitempty"`
	GameID         string               `json:"game_id"`
	Type           string               `json:"type"`
	Tiers          []achievementTierDTO `json:"tiers"`
}

func handleListAchievementDefinitions() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		all := achievements.All()
		out := make([]achievementDefinitionDTO, 0, len(all))
		for _, d := range all {
			tiers := make([]achievementTierDTO, len(d.Tiers))
			for i, t := range d.Tiers {
				tiers[i] = achievementTierDTO{
					Threshold:      t.Threshold,
					NameKey:        t.NameKey,
					DescriptionKey: t.DescriptionKey,
				}
			}
			out = append(out, achievementDefinitionDTO{
				Key:            d.Key,
				NameKey:        d.NameKey,
				DescriptionKey: d.DescriptionKey,
				GameID:         d.GameID,
				Type:           d.Type,
				Tiers:          tiers,
			})
		}
		writeJSON(w, http.StatusOK, map[string]any{"definitions": out})
	}
}
